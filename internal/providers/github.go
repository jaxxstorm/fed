package providers

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaxxstorm/fed/internal/connections"
	"golang.org/x/oauth2"
)

type GitHubConfig struct {
	AppID        string
	Slug         string
	ClientID     string
	ClientSecret string            `json:"-"`
	PrivateKey   []byte            `json:"-"`
	Transport    http.RoundTripper `json:"-"`
}
type installationToken struct {
	token   string
	expires time.Time
}
type GitHub struct {
	appID, slug string
	key         *rsa.PrivateKey
	oauth       oauth2.Config
	http        *connections.ProviderHTTP
	mu          sync.Mutex
	tokens      map[string]installationToken
}

func NewGitHub(config GitHubConfig) (*GitHub, error) {
	if _, err := strconv.ParseInt(config.AppID, 10, 64); err != nil || config.Slug == "" || config.ClientID == "" || config.ClientSecret == "" {
		return nil, ProviderError("missing_application_configuration")
	}
	block, _ := pem.Decode(config.PrivateKey)
	if block == nil {
		return nil, ProviderError("missing_application_configuration")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			return nil, ProviderError("missing_application_configuration")
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, ProviderError("missing_application_configuration")
		}
	}
	if key.N.BitLen() < 2048 {
		return nil, ProviderError("missing_application_configuration")
	}
	return &GitHub{appID: config.AppID, slug: config.Slug, key: key, http: connections.NewProviderHTTP(config.Transport, "github.com", "api.github.com"), oauth: oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", AuthStyle: oauth2.AuthStyleInParams}}, tokens: map[string]installationToken{}}, nil
}

func (p *GitHub) appToken() (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"iss": p.appID, "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(9 * time.Minute).Unix()})
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", ProviderError("missing_application_configuration")
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

type githubInstallation struct {
	ID      int64 `json:"id"`
	AppID   int64 `json:"app_id"`
	Account struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	} `json:"account"`
	SuspendedAt *time.Time        `json:"suspended_at"`
	Permissions map[string]string `json:"permissions"`
}

func (p *GitHub) installation(ctx context.Context, id string) (githubInstallation, error) {
	var out githubInstallation
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(id) {
		return out, connections.ErrInvalid
	}
	token, err := p.appToken()
	if err != nil {
		return out, err
	}
	_, err = requestJSON(ctx, p.http, "GET", "https://api.github.com/app/installations/"+id, token, nil, &out)
	if err != nil {
		if err.Error() == "resource_unavailable" {
			return out, ProviderError("installation_removed")
		}
		return out, err
	}
	if out.SuspendedAt != nil {
		return out, ProviderError("installation_suspended")
	}
	if strconv.FormatInt(out.ID, 10) != id || strconv.FormatInt(out.AppID, 10) != p.appID || out.SuspendedAt != nil || out.Account.ID == 0 || out.Permissions["contents"] != "read" {
		return out, ProviderError("insufficient_permissions")
	}
	return out, nil
}

func (p *GitHub) AuthorizationURL(_ connections.Connection, c connections.AuthorizationChallenge) (string, error) {
	config := p.oauth
	config.RedirectURL = c.RedirectURI
	return config.AuthCodeURL(c.State, oauth2.SetAuthURLParam("code_challenge", c.PKCEChallenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

func (p *GitHub) InstallationURL(state string) string {
	return "https://github.com/apps/" + url.PathEscape(p.slug) + "/installations/new?state=" + url.QueryEscape(state)
}
func (p *GitHub) ExchangeAuthorization(ctx context.Context, c connections.Connection, claim connections.AuthorizationClaim, response connections.AuthorizationResponse) (_ connections.AuthorizedGrant, resultErr error) {
	finish := observeOperation(ctx, c, "github.consent.exchange")
	defer func() { finish(resultErr) }()
	id := c.Configuration.InstallationID
	if id == "" {
		id = response.InstallationID
	}
	if id == "" || (response.InstallationID != "" && response.InstallationID != id) {
		return connections.AuthorizedGrant{}, ProviderError("wrong_tenant")
	}
	config := p.oauth
	config.RedirectURL = claim.RedirectURI
	token, err := config.Exchange(oauthContext(ctx, p.http), response.Code, oauth2.VerifierOption(claim.Verifier.Value("verifier")))
	if err != nil {
		return connections.AuthorizedGrant{}, ProviderError("authorization_exchange_failed")
	}
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(id) {
		return connections.AuthorizedGrant{}, connections.ErrInvalid
	}
	// This read uses the user's token, not the App JWT: an installation ID by
	// itself is not proof that the authorizing user can access the installation.
	var repositories struct {
		TotalCount int `json:"total_count"`
	}
	if _, err = requestJSON(ctx, p.http, "GET", "https://api.github.com/user/installations/"+id+"/repositories?per_page=1", token.AccessToken, nil, &repositories); err != nil {
		return connections.AuthorizedGrant{}, err
	}
	installation, err := p.installation(ctx, id)
	if err != nil {
		return connections.AuthorizedGrant{}, err
	}
	account := strconv.FormatInt(installation.Account.ID, 10)
	if c.Configuration.AccountID != account && !strings.EqualFold(c.Configuration.AccountID, installation.Account.Login) && !(c.Strategy == connections.GitHubInstallation && c.Configuration.AccountID == "") {
		return connections.AuthorizedGrant{}, ProviderError("wrong_tenant")
	}
	return connections.AuthorizedGrant{AccountID: account, AccountHint: c.Configuration.AccountID, Credentials: connections.NewCredentials(map[string]string{"installation_id": id, "account_id": account})}, nil
}

func (p *GitHub) installationAccess(ctx context.Context, c connections.Connection, k connections.Credentials, repositoryID string) (string, error) {
	id := k.Value("installation_id")
	if id == "" || k.Value("account_id") != c.Tenant() || (c.Configuration.InstallationID != "" && c.Configuration.InstallationID != id) {
		return "", ProviderError("wrong_tenant")
	}
	cacheKey := c.ServiceID + "/" + c.ID + "/" + strconv.FormatInt(c.Generation, 10) + "/" + id + "/" + repositoryID
	p.mu.Lock()
	defer p.mu.Unlock()
	if token, ok := p.tokens[cacheKey]; ok && token.expires.After(time.Now().Add(time.Minute)) {
		return token.token, nil
	}
	jwt, err := p.appToken()
	if err != nil {
		return "", err
	}
	request := map[string]any{"permissions": map[string]string{"contents": "read", "metadata": "read"}}
	if repositoryID != "" {
		n, err := strconv.ParseInt(repositoryID, 10, 64)
		if err != nil || n < 1 {
			return "", connections.ErrInvalid
		}
		request["repository_ids"] = []int64{n}
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	_, err = requestJSON(ctx, p.http, "POST", "https://api.github.com/app/installations/"+url.PathEscape(id)+"/access_tokens", jwt, request, &result)
	if err != nil {
		return "", err
	}
	if result.Token == "" || !result.ExpiresAt.After(time.Now()) {
		return "", ProviderError("invalid_provider_response")
	}
	for key, value := range p.tokens {
		if !value.expires.After(time.Now()) {
			delete(p.tokens, key)
		}
	}
	p.tokens[cacheKey] = installationToken{result.Token, result.ExpiresAt}
	return result.Token, nil
}

func (p *GitHub) Test(ctx context.Context, c connections.Connection, k connections.Credentials) (connections.AccessHealth, error) {
	installation, err := p.installation(ctx, k.Value("installation_id"))
	if err != nil {
		return connections.AccessHealth{}, err
	}
	if strconv.FormatInt(installation.Account.ID, 10) != c.Tenant() {
		return connections.AccessHealth{}, ProviderError("wrong_tenant")
	}
	if _, err = p.installationAccess(ctx, c, k, ""); err != nil {
		return connections.AccessHealth{}, err
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: c.Tenant(), Limitations: []string{}}, nil
}
func (p *GitHub) Renew(ctx context.Context, c connections.Connection, k connections.Credentials) (_ connections.Credentials, resultErr error) {
	finish := observeOperation(ctx, c, "github.renew")
	defer func() { finish(resultErr) }()
	p.mu.Lock()
	for key := range p.tokens {
		if strings.HasPrefix(key, c.ServiceID+"/"+c.ID+"/") {
			delete(p.tokens, key)
		}
	}
	p.mu.Unlock()
	_, err := p.Test(ctx, c, k)
	return k, err
}

type githubRepository struct {
	Permissions   map[string]bool `json:"permissions"`
	ID            int64           `json:"id"`
	FullName      string          `json:"full_name"`
	DefaultBranch string          `json:"default_branch"`
	Owner         struct {
		ID int64 `json:"id"`
	} `json:"owner"`
}

func (p *GitHub) Discover(ctx context.Context, c connections.Connection, k connections.Credentials, cursor string, limit int) (connections.DiscoveryPage, error) {
	size, err := connections.PageSize(limit)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	page := 1
	if cursor != "" {
		page, err = strconv.Atoi(cursor)
		if err != nil || page < 1 || page > 100000 {
			return connections.DiscoveryPage{}, connections.ErrInvalid
		}
	}
	token, err := p.installationAccess(ctx, c, k, "")
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	var result struct {
		TotalCount   int                `json:"total_count"`
		Repositories []githubRepository `json:"repositories"`
	}
	_, err = requestJSON(ctx, p.http, "GET", "https://api.github.com/installation/repositories?per_page="+strconv.Itoa(size)+"&page="+strconv.Itoa(page), token, nil, &result)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	out := connections.DiscoveryPage{Resources: []connections.DiscoveredResource{}}
	for _, repo := range result.Repositories {
		if strconv.FormatInt(repo.Owner.ID, 10) != c.Tenant() {
			return out, ProviderError("wrong_tenant")
		}
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.GitHub, Tenant: c.Tenant(), Kind: "repository", ID: strconv.FormatInt(repo.ID, 10)}, Name: repo.FullName, Locator: "https://github.com/" + repo.FullName, DefaultBranch: repo.DefaultBranch, Permissions: repo.Permissions})
	}
	complete := page*size >= result.TotalCount || len(result.Repositories) < size
	out.Coverage = []connections.KindCoverage{{Kind: "repository", Complete: complete, Authoritative: true}}
	if !complete {
		out.NextCursor = strconv.Itoa(page + 1)
	}
	return out, nil
}

func (p *GitHub) RepositoryToken(ctx context.Context, c connections.Connection, k connections.Credentials, id string) (string, string, error) {
	token, err := p.installationAccess(ctx, c, k, id)
	if err != nil {
		return "", "", err
	}
	var repo githubRepository
	_, err = requestJSON(ctx, p.http, "GET", "https://api.github.com/repositories/"+url.PathEscape(id), token, nil, &repo)
	if err != nil {
		return "", "", err
	}
	if strconv.FormatInt(repo.ID, 10) != id || strconv.FormatInt(repo.Owner.ID, 10) != c.Tenant() || !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo.FullName) {
		return "", "", ProviderError("wrong_tenant")
	}
	return token, repo.FullName, nil
}
func (p *GitHub) ResolveCommit(ctx context.Context, c connections.Connection, k connections.Credentials, id, ref string) (string, error) {
	if ref == "" || len(ref) > 256 {
		return "", connections.ErrInvalid
	}
	token, name, err := p.RepositoryToken(ctx, c, k, id)
	if err != nil {
		return "", err
	}
	var result struct {
		SHA string `json:"sha"`
	}
	_, err = requestJSON(ctx, p.http, "GET", "https://api.github.com/repos/"+name+"/commits/"+url.PathEscape(ref), token, nil, &result)
	if err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(result.SHA) {
		return "", ProviderError("invalid_provider_response")
	}
	return result.SHA, nil
}
func (p *GitHub) ReadPinned(ctx context.Context, c connections.Connection, k connections.Credentials, r connections.DiscoveredResource, commit, filePath string) (connections.Snapshot, error) {
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) || filePath == "" || path.Clean(filePath) != filePath || strings.HasPrefix(filePath, "/") || filePath == ".." || strings.HasPrefix(filePath, "../") || strings.Contains(filePath, "\\") {
		return connections.Snapshot{}, connections.ErrInvalid
	}
	token, name, err := p.RepositoryToken(ctx, c, k, r.Resource.ID)
	if err != nil {
		return connections.Snapshot{}, err
	}
	var file struct{ Content, Encoding, Type, SHA string }
	etag, err := requestJSON(ctx, p.http, "GET", "https://api.github.com/repos/"+name+"/contents/"+url.PathEscape(filePath)+"?ref="+url.QueryEscape(commit), token, nil, &file)
	if err != nil {
		return connections.Snapshot{}, err
	}
	if file.Encoding != "base64" || file.Type != "file" {
		return connections.Snapshot{}, ProviderError("unsupported_repository_response")
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil || len(content) > connections.MaxResponseBytes {
		return connections.Snapshot{}, ProviderError("artifact_too_large")
	}
	return connections.Snapshot{Content: content, MediaType: "text/plain", ObservedAt: time.Now().UTC(), ETag: etag}, nil
}
func (*GitHub) Read(context.Context, connections.Connection, connections.Credentials, connections.DiscoveredResource) (connections.Snapshot, error) {
	return connections.Snapshot{}, ProviderError("pinned_commit_and_path_required")
}
