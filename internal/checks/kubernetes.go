package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apps "k8s.io/client-go/kubernetes/typed/apps/v1"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	network "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	AWSConfig    func(context.Context, assessment.Account, string) (aws.Config, error)
	ClientConfig func(string) (*rest.Config, error)
}

func (collector Kubernetes) Collect(ctx context.Context, scope assessment.Scope, m assessment.Measure) (Result, error) {
	if collector.AWSConfig == nil {
		collector.AWSConfig = AccountConfig
	}
	if collector.ClientConfig == nil {
		collector.ClientConfig = func(name string) (*rest.Config, error) {
			return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{CurrentContext: name}).ClientConfig()
		}
	}
	var excluded []string
	_ = json.Unmarshal(m.Policy["namespace_exclusions"], &excluded)
	out := Result{Observations: []Observation{}, Limitations: []string{"Configuration-only workload isolation review; no traffic probes. Enforcement detection currently supports AWS VPC CNI network-policy-agent only; other CNIs remain unknown."}}
	bindings := map[string]string{}
	if raw := os.Getenv("KSI_KUBERNETES_CONTEXTS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &bindings); err != nil {
			return out, err
		}
	}
	for _, account := range scope.Accounts {
		for _, region := range account.Regions {
			names := append([]string{}, account.Clusters...)
			if len(names) == 0 {
				cfg, err := collector.AWSConfig(ctx, account, region)
				if err != nil {
					out.Observations = append(out.Observations, Observation{Scope: account.ID + "/" + region, Outcome: "error", Error: collectorErrorCode(err, "credentials_or_account_validation_failed")})
					continue
				}
				pages := eks.NewListClustersPaginator(eks.NewFromConfig(cfg), &eks.ListClustersInput{})
				failed := false
				for pages.HasMorePages() {
					page, err := pages.NextPage(ctx)
					if err != nil {
						out.Observations = append(out.Observations, Observation{Scope: account.ID + "/" + region, Outcome: "error", Error: "cluster_enumeration_failed"})
						failed = true
						break
					}
					names = append(names, page.Clusters...)
				}
				if len(names) == 0 && !failed {
					out.Observations = append(out.Observations, Observation{Scope: account.ID + "/" + region, Outcome: "unknown", Data: map[string]string{"inventory": "no_resources"}})
				}
			}
			for _, cluster := range names {
				key := account.ID + "/" + region + "/" + cluster
				failure := func(reason string) {
					out.Observations = append(out.Observations, Observation{Scope: key, Outcome: "error", Error: reason})
				}
				contextName := bindings[key]
				if contextName == "" {
					failure("kubernetes_context_binding_missing")
					continue
				}
				cfg, err := collector.AWSConfig(ctx, account, region)
				if err != nil {
					failure(collectorErrorCode(err, "credentials_or_account_validation_failed"))
					continue
				}
				expected, err := eks.NewFromConfig(cfg).DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(cluster)})
				if err != nil || expected.Cluster == nil || expected.Cluster.CertificateAuthority == nil {
					failure("cluster_identity_read_failed")
					continue
				}
				config, err := collector.ClientConfig(contextName)
				if err != nil {
					failure("kubeconfig_unavailable")
					continue
				}
				ca, err := base64.StdEncoding.DecodeString(aws.ToString(expected.Cluster.CertificateAuthority.Data))
				if err != nil {
					failure("cluster_ca_invalid")
					continue
				}
				configuredCA := config.CAData
				if len(configuredCA) == 0 && config.CAFile != "" {
					configuredCA, err = os.ReadFile(config.CAFile)
					if err != nil {
						failure("kubeconfig_ca_unavailable")
						continue
					}
				}
				if config.Insecure || strings.TrimRight(config.Host, "/") != strings.TrimRight(aws.ToString(expected.Cluster.Endpoint), "/") || !bytes.Equal(bytes.TrimSpace(ca), bytes.TrimSpace(configuredCA)) {
					failure("cluster_binding_identity_mismatch")
					continue
				}
				config.Timeout = 20 * time.Second
				cc, err := core.NewForConfig(config)
				if err != nil {
					failure("kubernetes_client_configuration_failed")
					continue
				}
				nc, err := network.NewForConfig(config)
				if err != nil {
					failure("kubernetes_client_configuration_failed")
					continue
				}
				ac, err := apps.NewForConfig(config)
				if err != nil {
					failure("kubernetes_client_configuration_failed")
					continue
				}
				enforcement := "unknown"
				if ds, e := ac.DaemonSets("kube-system").Get(ctx, "aws-node", metav1.GetOptions{}); e == nil {
					enabled, agent := false, false
					for _, container := range ds.Spec.Template.Spec.Containers {
						if container.Name == "aws-eks-nodeagent" {
							agent = true
						}
						if container.Name == "aws-node" {
							for _, v := range container.Env {
								if v.Name == "ENABLE_NETWORK_POLICY" && v.Value == "true" {
									enabled = true
								}
							}
						}
					}
					if enabled && agent {
						enforcement = "configured"
					}
				}
				next := ""
				for {
					namespaces, e := cc.Namespaces().List(ctx, metav1.ListOptions{Limit: 100, Continue: next})
					if e != nil {
						failure(collectorErrorCode(e, "namespace_enumeration_failed"))
						break
					}
					for _, ns := range namespaces.Items {
						if slices.Contains(excluded, ns.Name) {
							out.Observations = append(out.Observations, Observation{Scope: key + "/" + ns.Name, Outcome: "unknown", Data: map[string]string{"namespace_excluded": ns.Name, "reason": "explicit customer policy"}})
							continue
						}
						policies, e := nc.NetworkPolicies(ns.Name).List(ctx, metav1.ListOptions{})
						if e != nil {
							failure("network_policy_read_failed:" + ns.Name)
							continue
						}
						podsNext := ""
						for {
							pods, e := cc.Pods(ns.Name).List(ctx, metav1.ListOptions{Limit: 100, Continue: podsNext})
							if e != nil {
								failure("workload_enumeration_failed:" + ns.Name)
								break
							}
							for _, pod := range pods.Items {
								ingress, egress := false, false
								names := []string{}
								for _, policy := range policies.Items {
									selector, e := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
									if e != nil {
										continue
									}
									if !selector.Matches(labels.Set(pod.Labels)) {
										continue
									}
									names = append(names, policy.Name)
									if len(policy.Spec.PolicyTypes) == 0 {
										ingress = true
										if len(policy.Spec.Egress) > 0 {
											egress = true
										}
									}
									for _, typ := range policy.Spec.PolicyTypes {
										if string(typ) == "Ingress" {
											ingress = true
										}
										if string(typ) == "Egress" {
											egress = true
										}
									}
								}
								evals := []observation.Evaluation{condition("ingress_isolation", &ingress, "Whether a policy selects this workload for ingress isolation."), condition("egress_isolation", &egress, "Whether a policy selects this workload for egress isolation.")}
								if enforcement == "unknown" {
									evals = append(evals, observation.Evaluation{Condition: "network_policy_enforcement", Outcome: "unknown", Explanation: "Supported enforcement configuration could not be established."})
								}
								o := configObservation(account.ID, region, pod.Name, "kubernetes.network-isolation/v1", map[string]any{"policy_names": names, "ingress_isolated": ingress, "egress_isolated": egress, "enforcement": enforcement, "cni": "aws-vpc-cni-or-unknown", "namespace_excluded": false}, evals...)
								o.Target = &observation.Target{Kind: "kubernetes", AccountID: account.ID, Region: region, Cluster: cluster, Namespace: ns.Name, ResourceKind: "Pod", ResourceID: pod.Name, UID: string(pod.UID)}
								o.Scope = key + "/" + ns.Name + "/" + pod.Name
								out.Observations = append(out.Observations, o)
							}
							podsNext = pods.Continue
							if podsNext == "" {
								break
							}
						}
					}
					next = namespaces.Continue
					if next == "" {
						break
					}
				}
			}
		}
	}
	out.Summarize()
	return out, nil
}
