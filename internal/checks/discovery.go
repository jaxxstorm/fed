package checks

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
)

type BoundaryDiscovery struct {
	Scope            assessment.Scope `json:"scope"`
	ExpectedRevision int              `json:"expected_revision"`
	ObservedAt       time.Time        `json:"observed_at"`
	Coverage         []Observation    `json:"coverage"`
	Limitations      []string         `json:"limitations"`
}

// DiscoverBoundary expands only the accounts already in this service's scope.
// It proposes a revision; it never silently replaces saved scope on partial access.
func DiscoverBoundary(ctx context.Context, store *assessment.Store, actor auth.Actor, service string) (BoundaryDiscovery, error) {
	if err := auth.Authorize(ctx, store.DB, actor, service, "check"); err != nil {
		return BoundaryDiscovery{}, err
	}
	r, err := store.Get(ctx, actor, service, "scope", "scope")
	if err != nil {
		return BoundaryDiscovery{}, err
	}
	scope, err := assessment.Decode[assessment.Scope](r)
	if err != nil {
		return BoundaryDiscovery{}, err
	}
	return discoverBoundary(ctx, scope, r.Revision, AccountConfig), nil
}

func discoverBoundary(ctx context.Context, scope assessment.Scope, revision int, config func(context.Context, assessment.Account, string) (aws.Config, error)) BoundaryDiscovery {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	out := BoundaryDiscovery{Scope: scope, ExpectedRevision: revision, ObservedAt: time.Now().UTC(), Coverage: []Observation{}, Limitations: []string{"Candidate AWS regions and EKS inventory, not a complete assessment boundary. Only saved accounts are inspected; organization membership, non-EKS resources, external dependencies, and repository discovery require separate evidence. Repositories are preserved from the saved scope. Failed accounts retain their saved scope; do not interpret empty clusters as proof of absence."}}
	out.Scope.Accounts = []assessment.Account{}
	seen := map[string]bool{}
	for _, account := range scope.Accounts {
		if seen[account.ID] {
			continue
		}
		seen[account.ID] = true
		cfg, err := config(ctx, account, account.Regions[0])
		if err != nil {
			out.Coverage = append(out.Coverage, Observation{Scope: account.ID, Outcome: "error", Error: "credentials_or_account_validation_failed"})
			for _, old := range scope.Accounts {
				if old.ID == account.ID {
					out.Scope.Accounts = append(out.Scope.Accounts, old)
				}
			}
			continue
		}
		regions := map[string]bool{}
		for _, saved := range scope.Accounts {
			if saved.ID == account.ID {
				for _, region := range saved.Regions {
					regions[region] = true
				}
			}
		}
		for name := range regions {
			if name == "" {
				continue
			}
			regional := cfg
			regional.Region = name
			candidate := assessment.Account{ID: account.ID, RoleARN: account.RoleARN, Regions: []string{name}, Clusters: []string{}}
			observation := Observation{Scope: account.ID + "/" + name, Outcome: "pass"}
			pages := eks.NewListClustersPaginator(eks.NewFromConfig(regional), &eks.ListClustersInput{})
			for pages.HasMorePages() {
				page, e := pages.NextPage(ctx)
				if e != nil {
					observation.Outcome = "error"
					observation.Error = "cluster_enumeration_failed"
					break
				}
				candidate.Clusters = append(candidate.Clusters, page.Clusters...)
			}
			if observation.Error != "" {
				for _, old := range scope.Accounts {
					if old.ID != account.ID {
						continue
					}
					for _, r := range old.Regions {
						if r == name {
							candidate.Clusters = append(candidate.Clusters, old.Clusters...)
						}
					}
				}
			}
			observation.Data = map[string]any{"clusters": candidate.Clusters}
			out.Coverage = append(out.Coverage, observation)
			out.Scope.Accounts = append(out.Scope.Accounts, candidate)
		}
	}
	out.Scope.Discovery = &assessment.ScopeDiscovery{ObservedAt: out.ObservedAt, Errors: []string{}, Limitations: out.Limitations}
	for _, observation := range out.Coverage {
		if observation.Error != "" {
			out.Scope.Discovery.Errors = append(out.Scope.Discovery.Errors, observation.Scope+": "+observation.Error)
		}
	}
	return out
}
