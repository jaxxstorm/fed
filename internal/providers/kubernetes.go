package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/jaxxstorm/fed/internal/connections"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Kubernetes struct {
	LegacyProfiles  map[string]string
	LegacyAWS       *AWS
	AllowedServers  []string
	AllowedContexts []string
	LoadContext     func(string) (*rest.Config, error)
}

// Uploaded configurations are data, not executable authentication programs.
// Validate every entry, including entries outside the selected context.
func ValidateUploadedKubeconfig(raw []byte) (*rest.Config, error) {
	if len(raw) == 0 || len(raw) > connections.MaxResponseBytes {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	config, err := clientcmd.Load(raw)
	if err != nil || len(config.Clusters) != 1 || len(config.AuthInfos) != 1 || len(config.Contexts) != 1 || config.CurrentContext == "" {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	for _, cluster := range config.Clusters {
		if cluster.InsecureSkipTLSVerify || cluster.CertificateAuthority != "" || cluster.ProxyURL != "" || cluster.TLSServerName != "" || len(cluster.Extensions) > 0 {
			return nil, ProviderError("unsupported_kubeconfig")
		}
	}
	for _, user := range config.AuthInfos {
		if user.Exec != nil || user.AuthProvider != nil || user.TokenFile != "" || user.ClientCertificate != "" || user.ClientKey != "" || user.Impersonate != "" || user.ImpersonateUID != "" || len(user.ImpersonateGroups) > 0 || len(user.ImpersonateUserExtra) > 0 || user.Username != "" || user.Password != "" || len(user.Extensions) > 0 {
			return nil, ProviderError("unsupported_kubeconfig")
		}
		if user.Token == "" && (len(user.ClientCertificateData) == 0 || len(user.ClientKeyData) == 0) {
			return nil, ProviderError("unsupported_kubeconfig")
		}
	}
	for _, entry := range config.Contexts {
		if len(entry.Extensions) > 0 {
			return nil, ProviderError("unsupported_kubeconfig")
		}
	}
	if len(config.Extensions) > 0 {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	result, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	return result, nil
}

func (p Kubernetes) Configuration(c connections.Connection, k connections.Credentials) (*rest.Config, error) {
	if c.Provider != connections.Kubernetes || c.Configuration.ClusterID == "" || c.Tenant() != c.Configuration.ClusterID {
		return nil, ProviderError("cluster_identity_mismatch")
	}
	var err error
	var config *rest.Config
	if c.Mode == connections.ServerEnvironment {
		if !slices.Contains(p.AllowedContexts, c.Configuration.Context) {
			return nil, ProviderError("kubernetes_context_binding_missing")
		}
		load := p.LoadContext
		if load == nil {
			load = func(name string) (*rest.Config, error) {
				return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{CurrentContext: name}).ClientConfig()
			}
		}
		config, err = load(c.Configuration.Context)
	} else if c.Mode == connections.ServerManaged {
		config, err = ValidateUploadedKubeconfig([]byte(k.Value("kubeconfig")))
	} else {
		return nil, connections.ErrInvalid
	}
	if err != nil {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	if config == nil {
		return nil, ProviderError("unsupported_kubeconfig")
	}
	declaredServer := c.Configuration.Server
	legacy := c.Imported && c.Mode == connections.ServerEnvironment
	if declaredServer == "" && legacy {
		declaredServer = config.Host
	}
	u, err := url.Parse(declaredServer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (!legacy && !slices.Contains(p.AllowedServers, declaredServer)) {
		return nil, ProviderError("cluster_identity_mismatch")
	}
	if strings.TrimRight(config.Host, "/") != strings.TrimRight(declaredServer, "/") || config.Insecure || config.Proxy != nil {
		return nil, ProviderError("cluster_identity_mismatch")
	}
	config = rest.CopyConfig(config)
	if legacy && config.ExecProvider != nil {
		parts := strings.Split(c.Configuration.ClusterID, "/")
		if len(parts) != 3 || p.LegacyProfiles[parts[0]] == "" {
			return nil, ProviderError("profile_sign_in_required")
		}
		config.ExecProvider = config.ExecProvider.DeepCopy()
		env := []clientcmdapi.ExecEnvVar{}
		for _, entry := range config.ExecProvider.Env {
			if entry.Name != "AWS_PROFILE" {
				env = append(env, entry)
			}
		}
		config.ExecProvider.Env = append(env, clientcmdapi.ExecEnvVar{Name: "AWS_PROFILE", Value: p.LegacyProfiles[parts[0]]})
	}
	config.Timeout = 20 * time.Second
	config.QPS = 5
	config.Burst = 10
	return config, nil
}

func (p Kubernetes) client(c connections.Connection, k connections.Credentials) (*core.CoreV1Client, error) {
	config, err := p.Configuration(c, k)
	if err != nil {
		return nil, err
	}
	client, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, ProviderError("cluster_tls_error")
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	result, err := core.NewForConfigAndClient(config, client)
	if err != nil {
		return nil, ProviderError("cluster_tls_error")
	}
	return result, nil
}

func (p Kubernetes) Test(ctx context.Context, c connections.Connection, k connections.Credentials) (connections.AccessHealth, error) {
	if c.Imported && c.Mode == connections.ServerEnvironment {
		return p.testLegacyEKS(ctx, c, k)
	}
	client, err := p.client(c, k)
	if err != nil {
		return connections.AccessHealth{}, err
	}
	identity, err := client.Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return connections.AccessHealth{}, kubernetesError(err)
	}
	if string(identity.UID) != c.Configuration.ClusterID {
		return connections.AccessHealth{}, ProviderError("cluster_identity_mismatch")
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: c.Tenant(), Limitations: []string{"Cluster identity is the kube-system namespace UID; namespace and workload access may be narrower."}}, nil
}

func (p Kubernetes) testLegacyEKS(ctx context.Context, c connections.Connection, k connections.Credentials) (connections.AccessHealth, error) {
	parts := strings.Split(c.Configuration.ClusterID, "/")
	if len(parts) != 3 || parts[1] != "us-east-1" || p.LegacyProfiles[parts[0]] == "" {
		return connections.AccessHealth{}, ProviderError("profile_sign_in_required")
	}
	provider := p.LegacyAWS
	if provider == nil {
		provider = &AWS{}
	}
	config, err := provider.Configuration(ctx, connections.Connection{Provider: connections.AWS, Configuration: connections.Configuration{AccountID: parts[0], Region: parts[1], Profile: p.LegacyProfiles[parts[0]]}}, connections.Credentials{})
	if err != nil {
		return connections.AccessHealth{}, err
	}
	expected, err := eks.NewFromConfig(config).DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(parts[2])})
	if err != nil || expected.Cluster == nil || expected.Cluster.CertificateAuthority == nil {
		return connections.AccessHealth{}, ProviderError("cluster_identity_mismatch")
	}
	restConfig, err := p.Configuration(c, k)
	if err != nil {
		return connections.AccessHealth{}, err
	}
	if err = rest.LoadTLSFiles(restConfig); err != nil {
		return connections.AccessHealth{}, ProviderError("cluster_tls_error")
	}
	ca, err := base64.StdEncoding.DecodeString(aws.ToString(expected.Cluster.CertificateAuthority.Data))
	if err != nil || strings.TrimRight(restConfig.Host, "/") != strings.TrimRight(aws.ToString(expected.Cluster.Endpoint), "/") || !bytes.Equal(bytes.TrimSpace(ca), bytes.TrimSpace(restConfig.CAData)) {
		return connections.AccessHealth{}, ProviderError("cluster_identity_mismatch")
	}
	client, err := p.client(c, k)
	if err != nil {
		return connections.AccessHealth{}, err
	}
	if _, err = client.Namespaces().Get(ctx, "kube-system", metav1.GetOptions{}); err != nil {
		return connections.AccessHealth{}, kubernetesError(err)
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: c.Tenant(), Limitations: []string{"Imported EKS identity verified against STS account, EKS endpoint and certificate authority."}}, nil
}

func kubernetesError(err error) error {
	if apierrors.IsForbidden(err) {
		return ProviderError("insufficient_permissions")
	}
	if apierrors.IsUnauthorized(err) {
		return ProviderError("grant_expired")
	}
	if apierrors.IsNotFound(err) {
		return ProviderError("resource_unavailable")
	}
	if apierrors.IsTooManyRequests(err) {
		return ProviderError("rate_limited")
	}
	return ProviderError("cluster_access_failed")
}

func (p Kubernetes) Discover(ctx context.Context, c connections.Connection, k connections.Credentials, cursor string, requested int) (connections.DiscoveryPage, error) {
	limit, err := connections.PageSize(requested)
	if err != nil || len(cursor) > 4096 {
		return connections.DiscoveryPage{}, connections.ErrInvalid
	}
	if _, err = p.Test(ctx, c, k); err != nil {
		return connections.DiscoveryPage{}, err
	}
	client, err := p.client(c, k)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	out := connections.DiscoveryPage{Resources: []connections.DiscoveredResource{}, Coverage: []connections.KindCoverage{{Kind: "cluster", Complete: true, Authoritative: true}}}
	if cursor == "" {
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.Kubernetes, Tenant: c.Tenant(), Kind: "cluster", ID: c.Configuration.ClusterID}, Name: c.Name, Locator: "kubernetes://cluster/" + url.PathEscape(c.Tenant()) + "/cluster/" + url.PathEscape(c.Configuration.ClusterID)})
		if limit == 1 {
			out.NextCursor = "start-namespaces"
			return out, nil
		}
		limit--
	}
	if cursor == "start-namespaces" {
		cursor = ""
	}
	if strings.HasPrefix(cursor, "known:") {
		offset, err := strconv.Atoi(strings.TrimPrefix(cursor, "known:"))
		if err != nil || offset < 0 || offset > len(c.Configuration.KnownNamespaces) {
			return out, connections.ErrInvalid
		}
		return p.knownNamespaces(ctx, c, client, out, offset, limit)
	}
	page, err := client.Namespaces().List(ctx, metav1.ListOptions{Limit: int64(limit), Continue: cursor})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return p.knownNamespaces(ctx, c, client, out, 0, limit)
		}
		return out, kubernetesError(err)
	}
	for _, namespace := range page.Items {
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.Kubernetes, Tenant: c.Tenant(), Kind: "namespace", ID: string(namespace.UID)}, Name: namespace.Name, Locator: "kubernetes://cluster/" + url.PathEscape(c.Tenant()) + "/namespace/" + url.PathEscape(string(namespace.UID))})
	}
	out.NextCursor = page.Continue
	out.Coverage = append(out.Coverage, connections.KindCoverage{Kind: "namespace", Complete: page.Continue == "", Authoritative: true})
	return out, nil
}

func (p Kubernetes) knownNamespaces(ctx context.Context, c connections.Connection, client *core.CoreV1Client, out connections.DiscoveryPage, offset, limit int) (connections.DiscoveryPage, error) {
	end := min(offset+limit, len(c.Configuration.KnownNamespaces))
	coverage := connections.KindCoverage{Kind: "namespace", ErrorCode: "insufficient_permissions", Limitations: []string{"Namespace listing denied; only explicitly configured known namespaces were checked."}}
	for _, name := range c.Configuration.KnownNamespaces[offset:end] {
		namespace, err := client.Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			coverage.Limitations = append(coverage.Limitations, "Namespace "+name+": "+kubernetesError(err).Error())
			continue
		}
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.Kubernetes, Tenant: c.Tenant(), Kind: "namespace", ID: string(namespace.UID)}, Name: name, Locator: "kubernetes://cluster/" + url.PathEscape(c.Tenant()) + "/namespace/" + url.PathEscape(string(namespace.UID))})
	}
	if end < len(c.Configuration.KnownNamespaces) {
		out.NextCursor = "known:" + strconv.Itoa(end)
	}
	out.Coverage = append(out.Coverage, coverage)
	return out, nil
}

func (p Kubernetes) Read(ctx context.Context, c connections.Connection, k connections.Credentials, r connections.DiscoveredResource) (connections.Snapshot, error) {
	if r.Resource.Provider != connections.Kubernetes || r.Resource.Tenant != c.Tenant() || r.Resource.Kind != "namespace" {
		return connections.Snapshot{}, connections.ErrInvalid
	}
	if _, err := p.Test(ctx, c, k); err != nil {
		return connections.Snapshot{}, err
	}
	client, err := p.client(c, k)
	if err != nil {
		return connections.Snapshot{}, err
	}
	namespace, err := client.Namespaces().Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		return connections.Snapshot{}, kubernetesError(err)
	}
	if string(namespace.UID) != r.Resource.ID {
		return connections.Snapshot{}, ProviderError("resource_identity_changed")
	}
	content, _ := json.Marshal(struct {
		Name            string            `json:"name"`
		UID             string            `json:"uid"`
		ResourceVersion string            `json:"resource_version"`
		Labels          map[string]string `json:"labels"`
	}{namespace.Name, string(namespace.UID), namespace.ResourceVersion, namespace.Labels})
	return connections.Snapshot{Content: content, MediaType: "application/json", ObservedAt: time.Now().UTC(), ETag: namespace.ResourceVersion}, nil
}

func (p Kubernetes) Renew(ctx context.Context, c connections.Connection, k connections.Credentials) (_ connections.Credentials, resultErr error) {
	finish := observeOperation(ctx, c, "kubernetes.renew")
	defer func() { finish(resultErr) }()
	_, err := p.Test(ctx, c, k)
	return k, err
}
