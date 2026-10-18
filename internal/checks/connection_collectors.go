package checks

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/providers"
	"k8s.io/client-go/rest"
)

func (r *Runner) collectConnected(ctx context.Context, a auth.Actor, service string, collector Collector, scope assessment.Scope, m assessment.Measure) (Result, []connections.CollectionAccess, error) {
	if r.Connections == nil {
		result, err := collector.Collect(ctx, scope, m)
		return result, nil, err
	}
	accesses := []connections.CollectionAccess{}
	resolveAWS := func(ctx context.Context, account assessment.Account, region string) (aws.Config, error) {
		if region != "us-east-1" {
			return aws.Config{}, connections.ErrInvalid
		}
		access, bound, err := r.Connections.CollectorAccess(ctx, a, service, "aws", account.ID+"/"+region)
		if err != nil {
			return aws.Config{}, err
		}
		if !bound {
			return AccountConfig(ctx, account, region)
		}
		accesses = append(accesses, access)
		adapter, err := r.Connections.Registry.Adapter(connections.AWS)
		if err != nil {
			return aws.Config{}, err
		}
		provider, ok := adapter.(interface {
			Configuration(context.Context, connections.Connection, connections.Credentials) (aws.Config, error)
		})
		if !ok {
			return aws.Config{}, connections.ErrInvalid
		}
		return provider.Configuration(ctx, access.Connection, access.Credentials)
	}
	switch c := collector.(type) {
	case Kubernetes:
		standalone, bindings, err := r.collectStandalone(ctx, a, service, scope)
		if err != nil {
			return standalone, bindings, err
		}
		accesses = append(accesses, bindings...)
		// Preserve existing EKS semantics while fencing any imported binding.
		for _, account := range scope.Accounts {
			for _, region := range account.Regions {
				clusters := account.Clusters
				if len(clusters) == 0 {
					clusters = []string{""}
				}
				for _, cluster := range clusters {
					if cluster != "" && standaloneObservedEKS(ctx, scope, standalone, account, region, cluster, resolveAWS) {
						continue
					}
					access, bound, err := r.Connections.CollectorAccess(ctx, a, service, "kubernetes", account.ID+"/"+region, cluster)
					if err != nil {
						standalone.Observations = append(standalone.Observations, Observation{Scope: account.ID + "/" + region + "/" + cluster, Outcome: "error", Error: connections.CollectionError(err)})
						continue
					}
					if bound {
						accesses = append(accesses, access)
					}
					partition := account
					partition.Regions = []string{region}
					if cluster != "" {
						partition.Clusters = []string{cluster}
					}
					partitionCollector := c
					partitionCollector.AWSConfig = resolveAWS
					if bound {
						if access.Connection.Provider != connections.Kubernetes || access.Connection.Mode != connections.ServerEnvironment || !access.Connection.Imported || access.Connection.Configuration.Context == "" {
							standalone.Observations = append(standalone.Observations, Observation{Scope: account.ID + "/" + region + "/" + cluster, Outcome: "error", Error: "connection_binding_requires_standalone_scope"})
							continue
						}
						partitionCollector.ClientConfig = func(string) (*rest.Config, error) {
							adapter, err := r.Connections.Registry.Adapter(connections.Kubernetes)
							if err != nil {
								return nil, err
							}
							provider, ok := adapter.(interface {
								Configuration(connections.Connection, connections.Credentials) (*rest.Config, error)
							})
							if !ok {
								return nil, connections.ErrInvalid
							}
							return provider.Configuration(access.Connection, access.Credentials)
						}
					}
					legacy, err := partitionCollector.Collect(ctx, assessment.Scope{Accounts: []assessment.Account{partition}}, m)
					if err != nil {
						return standalone, accesses, err
					}
					standalone.Observations = append(standalone.Observations, legacy.Observations...)
				}
			}
		}
		standalone.Summarize()
		return standalone, accesses, nil
	case AWS:
		c.Config = resolveAWS
		collector = c
	case Infrastructure:
		c.Config = resolveAWS
		collector = c
	case GitHub:
		out := Result{Limitations: []string{"Repository configuration intent only; does not prove deployment or recurring execution."}}
		for _, repo := range scope.Repositories {
			access, bound, err := r.Connections.CollectorAccess(ctx, a, service, "repository", "https://github.com/"+repo.Name)
			if err != nil {
				out.Observations = append(out.Observations, Observation{Scope: repo.Name, Outcome: "error", Error: connections.CollectionError(err)})
				continue
			}
			if !bound {
				legacy, err := c.Collect(ctx, assessment.Scope{Repositories: []assessment.Repository{repo}}, m)
				if err != nil {
					return out, accesses, err
				}
				out.Observations = append(out.Observations, legacy.Observations...)
				continue
			}
			accesses = append(accesses, access)
			adapter, err := r.Connections.Registry.Adapter(connections.GitHub)
			provider, ok := adapter.(*providers.GitHub)
			if err != nil || !ok {
				out.Observations = append(out.Observations, Observation{Scope: repo.Name, Outcome: "error", Error: "connection_adapter_unavailable"})
				continue
			}
			for _, path := range repo.Paths {
				if !workflowEvidencePath(path) {
					continue
				}
				label := repo.Name + "@" + repo.Commit + ":" + path
				allowed := false
				for _, selected := range access.Source.Paths {
					if path == selected || selected == "." || strings.HasPrefix(path, strings.TrimRight(selected, "/")+"/") {
						allowed = true
					}
				}
				if !allowed || repo.Commit != access.Source.Commit {
					out.Observations = append(out.Observations, Observation{Scope: label, Outcome: "error", Error: "source_scope_changed"})
					continue
				}
				snapshot, err := provider.ReadPinned(ctx, access.Connection, access.Credentials, connections.DiscoveredResource{Resource: access.Resource}, repo.Commit, path)
				if err != nil {
					out.Observations = append(out.Observations, Observation{Scope: label, Outcome: "error", Error: collectorErrorCode(err, "repository_file_unavailable")})
					continue
				}
				matches := map[string]bool{}
				status := "pass"
				if len(m.Expected) == 0 {
					status = "unknown"
				}
				for _, term := range m.Expected {
					matches[term] = strings.Contains(strings.ToLower(string(snapshot.Content)), strings.ToLower(term))
					if !matches[term] {
						status = "fail"
					}
				}
				out.Observations = append(out.Observations, Observation{Scope: label, Outcome: status, Data: map[string]any{"commit": repo.Commit, "path": path, "matches": matches, "connection_id": access.Connection.ID, "source_id": access.SourceID, "source_revision": access.SourceRevision, "resource_id": access.Resource.ID, "evidence_kind": "configuration_intent"}})
			}
		}
		out.Summarize()
		return out, accesses, nil
	}
	result, err := collector.Collect(ctx, scope, m)
	return result, accesses, err
}

// An AWS inventory entry is not a second Kubernetes credential binding. Avoid
// retrying it only when a cluster-wide source actually produced workload
// observations and AWS independently identifies the same API endpoint. Names or
// namespace-only coverage are insufficient; original scope and history remain.
func standaloneObservedEKS(ctx context.Context, scope assessment.Scope, result Result, account assessment.Account, region, cluster string, resolve func(context.Context, assessment.Account, string) (aws.Config, error)) bool {
	servers := map[string]bool{}
	for _, selected := range scope.Kubernetes {
		if !selected.ClusterWide || selected.Server == "" {
			continue
		}
		for _, observed := range result.Observations {
			if observed.Target != nil && observed.Target.Kind == "kubernetes" && observed.Target.ClusterID == selected.ClusterID && observed.Error == "" {
				servers[strings.TrimRight(selected.Server, "/")] = true
				break
			}
		}
	}
	if len(servers) == 0 || region != "us-east-1" {
		return false
	}
	config, err := resolve(ctx, account, region)
	if err != nil {
		return false
	}
	described, err := eks.NewFromConfig(config).DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(cluster)})
	if err != nil || described.Cluster == nil || aws.ToString(described.Cluster.Arn) != "arn:aws:eks:"+region+":"+account.ID+":cluster/"+cluster {
		return false
	}
	return servers[strings.TrimRight(aws.ToString(described.Cluster.Endpoint), "/")]
}
