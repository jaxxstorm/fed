package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
)

func TestSelectionCommitsScopeAndSourcesAtomically(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "connection", Provider: GitHub, Name: "Repositories", Mode: AgentManaged, Configuration: Configuration{AccountID: "example"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.StartAgentDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "start"})
	if err != nil {
		t.Fatal(err)
	}
	resources := []DiscoveredResource{{Resource: ResourceKey{Provider: GitHub, Tenant: "example", Kind: "repository", ID: "1"}, Name: "example/one", Locator: "https://github.com/example/one"}, {Resource: ResourceKey{Provider: GitHub, Tenant: "example", Kind: "repository", ID: "2"}, Name: "example/two", Locator: "https://github.com/example/two"}}
	if _, err = s.SubmitAgentDiscovery(ctx, a, DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "gh api", Resources: resources, Complete: true, Coverage: []KindCoverage{{Kind: "repository", Complete: true, Authoritative: true}}}); err != nil {
		t.Fatal(err)
	}
	in := SelectionInput{ServiceID: "svc", IdempotencyKey: "select", AddToScope: true}
	for _, r := range resources {
		in.Sources = append(in.Sources, SourceConfigurationInput{ServiceID: "svc", SourceID: "source-" + r.Resource.ID, ConnectionID: d.Connection.ID, Generation: 1, Resource: r.Resource, Source: assessment.EvidenceSource{Type: "repository", Name: r.Name, Repository: r.Name, Locator: r.Locator, Commit: "1111111111111111111111111111111111111111", Paths: []string{".github/workflows"}, FreshnessHours: 48, Enabled: true}})
	}
	in.Sources[1].Source.Locator = "https://github.com/example/outside"
	if _, err = s.SelectSources(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid resource selected", err)
	}
	var records, bindings int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records`).Scan(&records); err != nil || records != 0 {
		t.Fatal("scope/source partial commit", records, err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_source_bindings`).Scan(&bindings); err != nil || bindings != 0 {
		t.Fatal("binding partial commit")
	}
	in.Sources[1].Source.Locator = resources[1].Locator
	out, err := s.SelectSources(ctx, a, in)
	if err != nil || out.ScopeRevision != 1 || len(out.Sources) != 2 || len(out.ScopeAdditions) != 2 {
		t.Fatalf("selection: %+v %v", out, err)
	}
	replay, err := s.SelectSources(ctx, a, in)
	if err != nil || replay.ScopeRevision != 1 {
		t.Fatal("selection retry failed")
	}
	in.IdempotencyKey = "stale"
	if _, err = s.SelectSources(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("stale scope accepted")
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"read", "connect"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectSources(ctx, agent, in); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("scope change bypassed edit permission")
	}
	store := assessment.Store{DB: s.DB}
	if err = store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind='evidence_source'`).Scan(&records); err != nil || records != 2 {
		t.Fatal("reconciliation duplicated explicitly selected sources", records, err)
	}
}

func TestWorkspaceAndStandaloneNamespaceSelection(t *testing.T) {
	for _, provider := range []Provider{Workspace, Kubernetes} {
		t.Run(string(provider), func(t *testing.T) {
			ctx := context.Background()
			s, a := setupService(t)
			configuration := Configuration{AccountID: "customer"}
			resource := DiscoveredResource{Resource: ResourceKey{Provider: Workspace, Tenant: "customer", Kind: "group", ID: "group-id"}, Name: "Security", Locator: "google-workspace://customer/group/group-id"}
			source := assessment.EvidenceSource{Type: "google_workspace", Name: "Security", Locator: resource.Locator, Enabled: true, FreshnessHours: 48}
			if provider == Kubernetes {
				uid := "11111111-1111-4111-8111-111111111111"
				configuration = Configuration{AccountID: "standalone", ClusterID: "standalone", Server: "https://cluster.example"}
				resource = DiscoveredResource{Resource: ResourceKey{Provider: Kubernetes, Tenant: "standalone", Kind: "namespace", ID: uid}, Name: "production", Locator: "kubernetes://cluster/standalone/namespace/" + uid}
				source = assessment.EvidenceSource{Type: "kubernetes", Name: "Production namespace", Locator: resource.Locator, ClusterID: "standalone", Namespace: "production", NamespaceUID: uid, Enabled: true, FreshnessHours: 48}
			}
			d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: provider, Name: "Provider", Mode: AgentManaged, Configuration: configuration})
			if err != nil {
				t.Fatal(err)
			}
			session, err := s.StartAgentDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "start"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.SubmitAgentDiscovery(ctx, a, DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "provider CLI", Resources: []DiscoveredResource{resource}}); err != nil {
				t.Fatal(err)
			}
			out, err := s.SelectSources(ctx, a, SelectionInput{ServiceID: "svc", IdempotencyKey: "select", AddToScope: true, Sources: []SourceConfigurationInput{{ServiceID: "svc", SourceID: "selected", ConnectionID: d.Connection.ID, Generation: 1, Resource: resource.Resource, Source: source}}})
			if err != nil || out.ScopeRevision != 1 {
				t.Fatalf("selection: %+v %v", out, err)
			}
			store := assessment.Store{DB: s.DB}
			record, err := store.Get(ctx, a, "svc", "scope", "scope")
			if err != nil {
				t.Fatal(err)
			}
			scope, err := assessment.Decode[assessment.Scope](record)
			if err != nil || len(scope.Accounts) != 0 || len(scope.Repositories) != 0 {
				t.Fatal("provider selection fabricated legacy scope")
			}
			if provider == Workspace && (len(scope.Workspace) != 1 || scope.Workspace[0].CustomerID != "customer") {
				t.Fatal("Workspace identity lost")
			}
			if provider == Kubernetes && (len(scope.Kubernetes) != 1 || scope.Kubernetes[0].ClusterWide || len(scope.Kubernetes[0].Namespaces) != 1) {
				t.Fatal("namespace scope widened to cluster")
			}
		})
	}
}
