package checks

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/observation"
	coreapi "k8s.io/api/core/v1"
	networkapi "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	network "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/rest"
)

func (r *Runner) collectStandalone(ctx context.Context, a auth.Actor, service string, scope assessment.Scope) (Result, []connections.CollectionAccess, error) {
	out := Result{Limitations: []string{"Explicit namespace scope only. Cluster-wide enforcement and traffic behavior are not established by this configuration inspection."}}
	accesses := []connections.CollectionAccess{}
	for _, cluster := range scope.Kubernetes {
		namespaces := cluster.Namespaces
		if cluster.ClusterWide {
			namespaces = []assessment.NamespaceScope{{}}
		}
		for _, selected := range namespaces {
			kind, id := "namespace", selected.UID
			if cluster.ClusterWide {
				kind, id = "cluster", cluster.ClusterID
			}
			label := cluster.ClusterID + "/" + selected.Name
			failure := func(code string) {
				out.Observations = append(out.Observations, Observation{Scope: label, Outcome: "error", Error: code})
			}
			locator := "kubernetes://cluster/" + url.PathEscape(cluster.ClusterID) + "/" + kind + "/" + url.PathEscape(id)
			access, bound, err := r.Connections.CollectorAccess(ctx, a, service, "kubernetes", locator)
			if err != nil {
				failure(connections.CollectionError(err))
				continue
			}
			if !bound {
				failure("connection_binding_required")
				continue
			}
			accesses = append(accesses, access)
			adapter, err := r.Connections.Registry.Adapter(connections.Kubernetes)
			if err != nil {
				failure("connection_adapter_unavailable")
				continue
			}
			provider, ok := adapter.(interface {
				Configuration(connections.Connection, connections.Credentials) (*rest.Config, error)
			})
			if !ok {
				failure("connection_adapter_unavailable")
				continue
			}
			if _, err = adapter.Test(ctx, access.Connection, access.Credentials); err != nil {
				failure(collectorErrorCode(err, "cluster_access_failed"))
				continue
			}
			config, err := provider.Configuration(access.Connection, access.Credentials)
			if err != nil {
				failure(collectorErrorCode(err, "cluster_access_failed"))
				continue
			}
			client, err := rest.HTTPClientFor(config)
			if err != nil {
				failure("cluster_tls_error")
				continue
			}
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			cc, err := core.NewForConfigAndClient(config, client)
			if err != nil {
				failure("kubernetes_client_configuration_failed")
				continue
			}
			nc, err := network.NewForConfigAndClient(config, client)
			if err != nil {
				failure("kubernetes_client_configuration_failed")
				continue
			}
			observed := []coreapi.Namespace{}
			if !cluster.ClusterWide {
				namespace, err := cc.Namespaces().Get(ctx, selected.Name, metav1.GetOptions{})
				if err != nil {
					failure("namespace_access_failed")
					continue
				}
				if string(namespace.UID) != selected.UID {
					failure("resource_identity_changed")
					continue
				}
				observed = append(observed, *namespace)
			} else {
				next := ""
				for {
					page, err := cc.Namespaces().List(ctx, metav1.ListOptions{Limit: 100, Continue: next})
					if err != nil {
						failure("namespace_enumeration_failed")
						break
					}
					observed = append(observed, page.Items...)
					next = page.Continue
					if next == "" {
						break
					}
				}
			}
			for _, namespace := range observed {
				label = cluster.ClusterID + "/" + namespace.Name
				policies := []networkapi.NetworkPolicy{}
				next := ""
				complete := true
				for {
					page, err := nc.NetworkPolicies(namespace.Name).List(ctx, metav1.ListOptions{Limit: 100, Continue: next})
					if err != nil {
						failure("network_policy_read_failed")
						complete = false
						break
					}
					policies = append(policies, page.Items...)
					next = page.Continue
					if next == "" {
						break
					}
				}
				if !complete {
					continue
				}
				next = ""
				for {
					pods, err := cc.Pods(namespace.Name).List(ctx, metav1.ListOptions{Limit: 100, Continue: next})
					if err != nil {
						failure("workload_enumeration_failed")
						break
					}
					for _, pod := range pods.Items {
						ingress, egress := false, false
						policyNames := []string{}
						for _, policy := range policies {
							selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
							if err != nil || !selector.Matches(labels.Set(pod.Labels)) {
								continue
							}
							policyNames = append(policyNames, policy.Name)
							if len(policy.Spec.PolicyTypes) == 0 {
								ingress = true
								egress = egress || len(policy.Spec.Egress) > 0
							}
							for _, kind := range policy.Spec.PolicyTypes {
								if kind == networkapi.PolicyTypeIngress {
									ingress = true
								}
								if kind == networkapi.PolicyTypeEgress {
									egress = true
								}
							}
						}
						evals := []observation.Evaluation{condition("ingress_isolation", &ingress, "Policy selects this workload for ingress isolation."), condition("egress_isolation", &egress, "Policy selects this workload for egress isolation."), {Condition: "network_policy_enforcement", Outcome: "unknown", Explanation: "Cluster-wide enforcement is outside this namespace inspection."}}
						o := configObservation("", "", pod.Name, "kubernetes.network-isolation/v1", map[string]any{"policy_names": policyNames, "ingress_isolated": ingress, "egress_isolated": egress, "enforcement": "unknown"}, evals...)
						o.Target = &observation.Target{Kind: "kubernetes", ClusterID: cluster.ClusterID, Namespace: namespace.Name, NamespaceUID: string(namespace.UID), ResourceKind: "Pod", ResourceID: pod.Name, UID: string(pod.UID)}
						o.Scope = label + "/" + pod.Name
						out.Observations = append(out.Observations, o)
					}
					next = pods.Continue
					if next == "" {
						break
					}
				}
			}
		}
	}
	out.Summarize()
	return out, accesses, nil
}
