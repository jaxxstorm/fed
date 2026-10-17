package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jaxxstorm/fed/internal/connections"
	"golang.org/x/oauth2"
)

type Workspace struct {
	oauth oauth2.Config
	http  *connections.ProviderHTTP
}

func NewWorkspace(clientID, secret string, transport http.RoundTripper) (*Workspace, error) {
	if clientID == "" || secret == "" {
		return nil, ProviderError("missing_application_configuration")
	}
	registry, _ := connections.NewRegistry(nil)
	definition, _ := registry.Provider(connections.Workspace)
	return &Workspace{oauth: oauth2.Config{ClientID: clientID, ClientSecret: secret, Scopes: definition.Permissions, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams}}, http: connections.NewProviderHTTP(transport, "oauth2.googleapis.com", "admin.googleapis.com")}, nil
}

func (p *Workspace) AuthorizationURL(_ connections.Connection, c connections.AuthorizationChallenge) (string, error) {
	config := p.oauth
	config.RedirectURL = c.RedirectURI
	return config.AuthCodeURL(c.State, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"), oauth2.SetAuthURLParam("code_challenge", c.PKCEChallenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

func workspaceCredentials(t *oauth2.Token, old connections.Credentials) connections.Credentials {
	refresh := t.RefreshToken
	if refresh == "" {
		refresh = old.Value("refresh_token")
	}
	return connections.NewCredentials(map[string]string{"access_token": t.AccessToken, "refresh_token": refresh, "expiry": t.Expiry.UTC().Format(time.RFC3339Nano)})
}

func (p *Workspace) customer(ctx context.Context, token string) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	_, err := requestJSON(ctx, p.http, "GET", "https://admin.googleapis.com/admin/directory/v1/customers/my_customer", token, nil, &result)
	if err != nil {
		return "", err
	}
	if result.ID == "" {
		return "", ProviderError("invalid_provider_response")
	}
	return result.ID, nil
}

func (p *Workspace) ExchangeAuthorization(ctx context.Context, c connections.Connection, claim connections.AuthorizationClaim, response connections.AuthorizationResponse) (connections.AuthorizedGrant, error) {
	config := p.oauth
	config.RedirectURL = claim.RedirectURI
	token, err := config.Exchange(oauthContext(ctx, p.http), response.Code, oauth2.VerifierOption(claim.Verifier.Value("verifier")))
	if err != nil {
		return connections.AuthorizedGrant{}, ProviderError("authorization_exchange_failed")
	}
	if token.RefreshToken == "" {
		return connections.AuthorizedGrant{}, ProviderError("offline_consent_required")
	}
	id, err := p.customer(ctx, token.AccessToken)
	if err != nil {
		return connections.AuthorizedGrant{}, err
	}
	if id != c.Configuration.AccountID {
		return connections.AuthorizedGrant{}, ProviderError("wrong_tenant")
	}
	return connections.AuthorizedGrant{AccountID: id, Credentials: workspaceCredentials(token, connections.Credentials{})}, nil
}

// Refresh is explicitly coordinated by the connection service; ordinary reads
// never rotate an offline grant behind its generation fence.
func (p *Workspace) Renew(ctx context.Context, c connections.Connection, k connections.Credentials) (connections.Credentials, error) {
	if k.Value("refresh_token") == "" {
		return connections.Credentials{}, ProviderError("offline_consent_required")
	}
	token, err := p.oauth.TokenSource(oauthContext(ctx, p.http), &oauth2.Token{RefreshToken: k.Value("refresh_token")}).Token()
	if err != nil {
		return connections.Credentials{}, ProviderError("grant_expired")
	}
	id, err := p.customer(ctx, token.AccessToken)
	if err != nil {
		return connections.Credentials{}, err
	}
	if id != c.Tenant() {
		return connections.Credentials{}, ProviderError("wrong_tenant")
	}
	return workspaceCredentials(token, k), nil
}

func (p *Workspace) Test(ctx context.Context, c connections.Connection, k connections.Credentials) (connections.AccessHealth, error) {
	id, err := p.customer(ctx, k.Value("access_token"))
	if err != nil {
		return connections.AccessHealth{}, err
	}
	if id != c.Tenant() {
		return connections.AccessHealth{}, ProviderError("wrong_tenant")
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: id, Limitations: []string{"Customer identity verified; resource permissions are reported separately by discovery."}}, nil
}

type workspaceCursor struct {
	Kind   string `json:"kind"`
	Token  string `json:"token,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

func (p *Workspace) Discover(ctx context.Context, c connections.Connection, k connections.Credentials, cursor string, requested int) (connections.DiscoveryPage, error) {
	limit, err := connections.PageSize(requested)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	position := workspaceCursor{Kind: "org_unit"}
	if cursor != "" && json.Unmarshal([]byte(cursor), &position) != nil {
		return connections.DiscoveryPage{}, connections.ErrInvalid
	}
	if (position.Kind != "org_unit" && position.Kind != "group") || position.Offset < 0 || position.Offset > 100000 || len(position.Token) > 4096 {
		return connections.DiscoveryPage{}, connections.ErrInvalid
	}
	if _, err = p.Test(ctx, c, k); err != nil {
		return connections.DiscoveryPage{}, err
	}
	out := connections.DiscoveryPage{Resources: []connections.DiscoveredResource{}}
	endpoint := "https://admin.googleapis.com/admin/directory/v1/"
	var entries []struct {
		ID        string `json:"id"`
		OrgUnitID string `json:"orgUnitId"`
		Name      string `json:"name"`
	}
	if position.Kind == "org_unit" {
		var result struct {
			Units json.RawMessage `json:"organizationUnits"`
		}
		_, err = requestJSON(ctx, p.http, "GET", endpoint+"customer/"+url.PathEscape(c.Tenant())+"/orgunits?type=all", k.Value("access_token"), nil, &result)
		if err == nil && len(result.Units) > 0 {
			err = json.Unmarshal(result.Units, &entries)
		}
		if err == nil {
			if position.Offset > len(entries) {
				return out, connections.ErrConflict
			}
			end := min(position.Offset+limit, len(entries))
			page := entries[position.Offset:end]
			if end < len(entries) {
				position.Offset = end
			} else {
				position = workspaceCursor{Kind: "group"}
			}
			entries = page
		}
	} else {
		var result struct {
			Groups json.RawMessage `json:"groups"`
			Next   string          `json:"nextPageToken"`
		}
		query := url.Values{"customer": {c.Tenant()}, "maxResults": {strconv.Itoa(limit)}, "pageToken": {position.Token}}
		_, err = requestJSON(ctx, p.http, "GET", endpoint+"groups?"+query.Encode(), k.Value("access_token"), nil, &result)
		if err == nil && len(result.Groups) > 0 {
			err = json.Unmarshal(result.Groups, &entries)
		}
		position.Token = result.Next
	}
	kind := "org_unit"
	if cursor != "" {
		var original workspaceCursor
		_ = json.Unmarshal([]byte(cursor), &original)
		kind = original.Kind
	}
	coverage := connections.KindCoverage{Kind: kind, Authoritative: true}
	if err != nil {
		code := "provider_error"
		if e, ok := err.(ProviderError); ok {
			code = string(e)
		}
		coverage.Authoritative = false
		coverage.ErrorCode = code
		if code != "insufficient_permissions" {
			return out, err
		}
		if kind == "org_unit" {
			position = workspaceCursor{Kind: "group"}
		} else {
			position.Token = ""
		}
	} else {
		coverage.Complete = kind == "org_unit" && position.Kind == "group" || kind == "group" && position.Token == ""
		for _, entry := range entries {
			id := entry.ID
			if kind == "org_unit" {
				id = entry.OrgUnitID
			}
			if id == "" || entry.Name == "" {
				return out, ProviderError("invalid_provider_response")
			}
			out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.Workspace, Tenant: c.Tenant(), Kind: kind, ID: id}, Name: entry.Name, Locator: "google-workspace://" + url.PathEscape(c.Tenant()) + "/" + kind + "/" + url.PathEscape(id)})
		}
	}
	out.Coverage = []connections.KindCoverage{coverage}
	if kind == "org_unit" || position.Token != "" {
		b, _ := json.Marshal(position)
		out.NextCursor = string(b)
	}
	return out, nil
}

func (p *Workspace) Read(ctx context.Context, c connections.Connection, k connections.Credentials, r connections.DiscoveredResource) (connections.Snapshot, error) {
	if r.Resource.Provider != connections.Workspace || r.Resource.Tenant != c.Tenant() || r.Resource.ID == "" {
		return connections.Snapshot{}, ProviderError("wrong_tenant")
	}
	if _, err := p.Test(ctx, c, k); err != nil {
		return connections.Snapshot{}, err
	}
	endpoint := "https://admin.googleapis.com/admin/directory/v1/"
	switch r.Resource.Kind {
	case "org_unit":
		endpoint += "customer/" + url.PathEscape(c.Tenant()) + "/orgunits/" + url.PathEscape(r.Resource.ID)
	case "group":
		// Group get accepts globally addressable IDs. Establish customer
		// membership with a customer-scoped listing before fetching metadata.
		if err := p.verifyGroupCustomer(ctx, c, k, r.Resource.ID); err != nil {
			return connections.Snapshot{}, err
		}
		endpoint += "groups/" + url.PathEscape(r.Resource.ID)
	default:
		return connections.Snapshot{}, connections.ErrInvalid
	}
	var result map[string]json.RawMessage
	etag, err := requestJSON(ctx, p.http, "GET", endpoint, k.Value("access_token"), nil, &result)
	if err != nil {
		return connections.Snapshot{}, err
	}
	field := "id"
	if r.Resource.Kind == "org_unit" {
		field = "orgUnitId"
	}
	var id string
	if json.Unmarshal(result[field], &id) != nil || id != r.Resource.ID {
		return connections.Snapshot{}, ProviderError("wrong_tenant")
	}
	// Retain only Directory metadata, never membership or arbitrary API fields.
	allowed := []string{"id", "orgUnitId", "name", "description", "email", "adminCreated", "directMembersCount", "orgUnitPath", "parentOrgUnitId", "parentOrgUnitPath", "blockInheritance", "etag"}
	metadata := map[string]json.RawMessage{}
	for _, key := range allowed {
		if value, ok := result[key]; ok {
			metadata[key] = value
		}
	}
	content, _ := json.Marshal(metadata)
	if etag == "" {
		_ = json.Unmarshal(result["etag"], &etag)
	}
	return connections.Snapshot{Content: content, MediaType: "application/json", ObservedAt: time.Now().UTC(), ETag: strings.TrimSpace(etag)}, nil
}

func (p *Workspace) verifyGroupCustomer(ctx context.Context, c connections.Connection, k connections.Credentials, id string) error {
	token := ""
	seen := map[string]bool{}
	for range 1000 {
		var result struct {
			Groups []struct {
				ID string `json:"id"`
			} `json:"groups"`
			Next string `json:"nextPageToken"`
		}
		query := url.Values{"customer": {c.Tenant()}, "maxResults": {"100"}, "pageToken": {token}, "fields": {"groups/id,nextPageToken"}}
		_, err := requestJSON(ctx, p.http, "GET", "https://admin.googleapis.com/admin/directory/v1/groups?"+query.Encode(), k.Value("access_token"), nil, &result)
		if err != nil {
			return err
		}
		for _, group := range result.Groups {
			if group.ID == id {
				return nil
			}
		}
		if result.Next == "" {
			return ProviderError("wrong_tenant")
		}
		if seen[result.Next] {
			return ProviderError("invalid_provider_response")
		}
		seen[result.Next] = true
		token = result.Next
	}
	return ProviderError("inventory_limit_exceeded")
}
