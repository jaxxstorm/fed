package connections

import (
	"context"
	"errors"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"
)

const DefaultPageSize = 25
const MaxPageSize = 100
const MaxResponseBytes = 1 << 20

func PageSize(requested int) (int, error) {
	if requested < 0 || requested > MaxPageSize {
		return 0, ErrInvalid
	}
	if requested == 0 {
		return DefaultPageSize, nil
	}
	return requested, nil
}

// Credentials are runtime-only values; they must never be serialized or logged.
type Credentials struct{ values map[string]string }

func NewCredentials(values map[string]string) Credentials {
	copy := make(map[string]string, len(values))
	for k, v := range values {
		copy[k] = v
	}
	return Credentials{values: copy}
}
func (c Credentials) Value(name string) string     { return c.values[name] }
func (c Credentials) String() string               { return "[redacted credentials]" }
func (c Credentials) GoString() string             { return c.String() }
func (c Credentials) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

type DiscoveryPage struct {
	Resources  []DiscoveredResource `json:"resources"`
	Coverage   []KindCoverage       `json:"coverage"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type Snapshot struct {
	Content    []byte
	MediaType  string
	ObservedAt time.Time
	ETag       string
}

// Adapters perform provider reads, never assessment mutations. The service must
// authorize before calls and fence generation when committing their results.
type Adapter interface {
	Test(context.Context, Connection, Credentials) (AccessHealth, error)
	Discover(context.Context, Connection, Credentials, string, int) (DiscoveryPage, error)
	Renew(context.Context, Connection, Credentials) (Credentials, error)
	Read(context.Context, Connection, Credentials, DiscoveredResource) (Snapshot, error)
}

type AuthorizationResponse struct {
	Code           string `json:"-"`
	InstallationID string `json:"-"`
}

type AuthorizedGrant struct {
	AccountID   string
	AccountHint string
	Credentials Credentials
}

// Authorizer is optional: environment/manual providers have no OAuth callback.
type Authorizer interface {
	AuthorizationURL(Connection, AuthorizationChallenge) (string, error)
	ExchangeAuthorization(context.Context, Connection, AuthorizationClaim, AuthorizationResponse) (AuthorizedGrant, error)
}

func AllowedAuthorizationURL(provider Provider, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return false
	}
	return provider == GitHub && u.Host == "github.com" || provider == Workspace && u.Host == "accounts.google.com"
}

type ProviderDefinition struct {
	ServerAdapterAvailable bool         `json:"server_adapter_available"`
	ID                     Provider     `json:"id"`
	Name                   string       `json:"name"`
	AccessModes            []AccessMode `json:"access_modes"`
	ResourceKinds          []string     `json:"resource_kinds"`
	EvidenceOperations     []string     `json:"evidence_operations"`
	Permissions            []string     `json:"permissions"`
	Guide                  SetupGuide   `json:"guide"`
}

type Registry struct {
	adapters map[Provider]Adapter
}

func NewRegistry(adapters map[Provider]Adapter) (*Registry, error) {
	r := &Registry{adapters: map[Provider]Adapter{}}
	for provider, adapter := range adapters {
		if _, ok := r.Provider(provider); !ok || adapter == nil {
			return nil, ErrInvalid
		}
		r.adapters[provider] = adapter
	}
	return r, nil
}
func (r *Registry) Provider(id Provider) (ProviderDefinition, bool) {
	// Return fresh catalogs so callers cannot mutate shared capability slices.
	for _, d := range providerDefinitions() {
		if d.ID == id {
			d.ServerAdapterAvailable = r.adapters[id] != nil
			return d, true
		}
	}
	return ProviderDefinition{}, false
}
func (r *Registry) Providers() []ProviderDefinition {
	out := providerDefinitions()
	for i := range out {
		out[i].ServerAdapterAvailable = r.adapters[out[i].ID] != nil
	}
	return out
}
func (r *Registry) Adapter(provider Provider) (Adapter, error) {
	adapter, ok := r.adapters[provider]
	if !ok {
		return nil, errors.New("provider server adapter unavailable")
	}
	return adapter, nil
}

// ProviderHTTP rejects unapproved destinations before sending credentials and
// does not follow redirects. SDK adapters must apply equivalent policies.
type ProviderHTTP struct {
	client *http.Client
	hosts  []string
}

func NewProviderHTTP(transport http.RoundTripper, hosts ...string) *ProviderHTTP {
	return &ProviderHTTP{client: &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, hosts: slices.Clone(hosts)}
}
func (p *ProviderHTTP) Do(req *http.Request) (*http.Response, error) {
	if req.URL == nil || req.URL.Scheme != "https" || req.URL.User != nil || !slices.Contains(p.hosts, req.URL.Host) || (req.Host != "" && req.Host != req.URL.Host) {
		return nil, errors.New("provider endpoint not allowed")
	}
	start := time.Now()
	response, err := p.client.Do(req)
	provider := "registered"
	switch req.URL.Host {
	case "github.com", "api.github.com":
		provider = "github"
	case "admin.googleapis.com", "oauth2.googleapis.com", "accounts.google.com":
		provider = "google_workspace"
	}
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	level, outcome := zerolog.DebugLevel, "success"
	if err != nil || status >= 400 {
		level = zerolog.WarnLevel
		outcome = "failed"
	}
	logging.Emit(req.Context(), level, logging.Fields{Event: "provider.operation.http", Component: "provider", Provider: provider, Stage: "request", Status: status, Duration: time.Since(start), ErrorCode: logging.Code(err), Outcome: outcome})
	return response, err
}
func ReadProviderBody(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, MaxResponseBytes+1))
	if err != nil {
		return nil, errors.New("provider response read failed")
	}
	if len(b) > MaxResponseBytes {
		return nil, errors.New("provider response exceeds size limit")
	}
	return b, nil
}
