package providers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/connections"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestGitHubInstallationOwnershipAndRestrictedTokens(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	denied := false
	removed := false
	rateLimited := false
	mints := 0
	p, err := NewGitHub(GitHubConfig{AppID: "42", Slug: "evidence", ClientID: "client", ClientSecret: "secret", PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			return jsonResponse(200, `{"access_token":"user","token_type":"bearer"}`), nil
		case "/user/installations/7/repositories":
			if r.Header.Get("Authorization") != "Bearer user" {
				t.Error("ownership check did not use user credentials")
			}
			if denied {
				return jsonResponse(403, `{}`), nil
			}
			return jsonResponse(200, `{"total_count":1}`), nil
		case "/app/installations/7":
			if removed {
				return jsonResponse(404, `{}`), nil
			}
			return jsonResponse(200, `{"id":7,"app_id":42,"account":{"id":99,"login":"example"},"permissions":{"contents":"read"}}`), nil
		case "/app/installations/7/access_tokens":
			mints++
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "repository_ids") && !strings.Contains(string(body), `"repository_ids":[123]`) {
				t.Error("repository token was not narrowed")
			}
			return jsonResponse(201, `{"token":"installation","expires_at":"2099-01-01T00:00:00Z"}`), nil
		case "/repositories/123":
			return jsonResponse(200, `{"id":123,"full_name":"example/renamed","owner":{"id":99}}`), nil
		case "/repos/example/renamed/commits/main":
			return jsonResponse(200, `{"sha":"1111111111111111111111111111111111111111"}`), nil
		case "/installation/repositories":
			if rateLimited {
				return jsonResponse(429, `{}`), nil
			}
			if r.URL.Query().Get("per_page") != "1" {
				t.Error("unbounded repository discovery")
			}
			if r.URL.Query().Get("page") == "2" {
				return jsonResponse(200, `{"total_count":2,"repositories":[{"id":124,"full_name":"example/second","owner":{"id":99},"permissions":{"pull":true}}]}`), nil
			}
			return jsonResponse(200, `{"total_count":2,"repositories":[{"id":123,"full_name":"example/renamed","owner":{"id":99},"default_branch":"main","permissions":{"pull":true}}]}`), nil
		case "/repos/example/renamed/contents/ci.yaml":
			if r.URL.Query().Get("ref") != "1111111111111111111111111111111111111111" {
				t.Error("unpinned repository read")
			}
			return jsonResponse(200, `{"type":"file","encoding":"base64","content":"cHVsdW1p","sha":"blob"}`), nil
		case "/repos/example/renamed/contents/missing.yaml":
			return jsonResponse(404, `{}`), nil
		default:
			t.Errorf("unexpected endpoint %s", r.URL)
			return jsonResponse(404, `{}`), nil
		}
	})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c := connections.Connection{ID: "connection", ServiceID: "service", Generation: 1, Configuration: connections.Configuration{AccountID: "example", InstallationID: "7"}}
	claim := connections.AuthorizationClaim{RedirectURI: "https://app.example/connections/callback/github", Verifier: connections.NewCredentials(map[string]string{"verifier": "verifier"})}
	response := connections.AuthorizationResponse{Code: "code", InstallationID: "7"}
	grant, err := p.ExchangeAuthorization(ctx, c, claim, response)
	if err != nil || grant.AccountID != "99" {
		t.Fatalf("grant: %+v %v", grant, err)
	}
	c.VerifiedAccountID = "99"
	for range 2 {
		commit, err := p.ResolveCommit(ctx, c, grant.Credentials, "123", "main")
		if err != nil || len(commit) != 40 {
			t.Fatal("resolve commit", err)
		}
	}
	if mints != 1 {
		t.Fatal("installation token cache missed")
	}
	page, err := p.Discover(ctx, c, grant.Credentials, "", 1)
	if err != nil || len(page.Resources) != 1 || page.NextCursor != "2" || page.Resources[0].DefaultBranch != "main" || !page.Resources[0].Permissions["pull"] {
		t.Fatal("repository metadata or pagination lost", err)
	}
	resource := page.Resources[0]
	page, err = p.Discover(ctx, c, grant.Credentials, page.NextCursor, 1)
	if err != nil || len(page.Resources) != 1 || page.NextCursor != "" {
		t.Fatal("repository second page", err)
	}
	snapshot, err := p.ReadPinned(ctx, c, grant.Credentials, resource, "1111111111111111111111111111111111111111", "ci.yaml")
	if err != nil || string(snapshot.Content) != "pulumi" {
		t.Fatal("renamed pinned file read", err)
	}
	if _, err = p.ReadPinned(ctx, c, grant.Credentials, resource, "1111111111111111111111111111111111111111", "missing.yaml"); err == nil {
		t.Fatal("missing file accepted")
	}
	rateLimited = true
	if _, err = p.Discover(ctx, c, grant.Credentials, "", 1); err == nil || err.Error() != "rate_limited" {
		t.Fatal("rate limit classification lost")
	}
	removed = true
	if _, err = p.Test(ctx, c, grant.Credentials); err == nil {
		t.Fatal("removed installation accepted")
	}
	removed = false
	response.InstallationID = "8"
	if _, err = p.ExchangeAuthorization(ctx, c, claim, response); err == nil {
		t.Fatal("substituted installation accepted")
	}
	response.InstallationID = "7"
	c.Configuration.AccountID = "unrelated"
	if _, err = p.ExchangeAuthorization(ctx, c, claim, response); err == nil {
		t.Fatal("wrong organization accepted")
	}
	c.Configuration.AccountID = "example"
	denied = true
	if _, err = p.ExchangeAuthorization(ctx, c, claim, response); err == nil {
		t.Fatal("user without installation access accepted")
	}
	denied = false
	rateLimited = false
	t.Run("shared-service-journey", func(t *testing.T) { verifyOAuthJourney(t, connections.GitHub, p, c.Configuration, grant) })
}

func TestWorkspaceOfflineIdentityPartialDiscoveryAndRenewal(t *testing.T) {
	ctx := context.Background()
	customer := "C123"
	identityDenied := false
	revoked := false
	p, err := NewWorkspace("client", "secret", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/token":
			if revoked {
				return jsonResponse(400, `{"error":"invalid_grant"}`), nil
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" {
				return jsonResponse(200, `{"access_token":"renewed","expires_in":3600,"token_type":"Bearer"}`), nil
			}
			return jsonResponse(200, `{"access_token":"initial","refresh_token":"offline","expires_in":3600,"token_type":"Bearer"}`), nil
		case "/admin/directory/v1/customers/my_customer":
			if identityDenied {
				return jsonResponse(403, `{}`), nil
			}
			return jsonResponse(200, `{"id":"`+customer+`"}`), nil
		case "/admin/directory/v1/customer/C123/orgunits":
			return jsonResponse(403, `{}`), nil
		case "/admin/directory/v1/groups":
			if r.URL.Query().Get("customer") != "C123" || (r.URL.Query().Get("maxResults") != "1" && r.URL.Query().Get("fields") == "") {
				t.Error("unscoped/unbounded group query")
			}
			if r.URL.Query().Get("pageToken") == "next" {
				return jsonResponse(200, `{"groups":[{"id":"g2","name":"Second"}]}`), nil
			}
			return jsonResponse(200, `{"groups":[{"id":"g1","name":"Security"}],"nextPageToken":"next"}`), nil
		case "/admin/directory/v1/groups/g1":
			return jsonResponse(200, `{"id":"g1","name":"Security","etag":"metadata-v1","members":["not-included"]}`), nil
		default:
			t.Errorf("unexpected endpoint %s", r.URL)
			return jsonResponse(404, `{}`), nil
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	c := connections.Connection{Configuration: connections.Configuration{AccountID: customer}}
	claim := connections.AuthorizationClaim{RedirectURI: "https://app.example/connections/callback/google_workspace", Verifier: connections.NewCredentials(map[string]string{"verifier": "verifier"})}
	authorization, _ := p.AuthorizationURL(c, connections.AuthorizationChallenge{State: "state", PKCEChallenge: "challenge", RedirectURI: claim.RedirectURI})
	u, _ := url.Parse(authorization)
	if u.Query().Get("access_type") != "offline" || u.Query().Get("code_challenge") != "challenge" {
		t.Fatal("offline PKCE missing")
	}
	grant, err := p.ExchangeAuthorization(ctx, c, claim, connections.AuthorizationResponse{Code: "code"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := p.Discover(ctx, c, grant.Credentials, "", 1)
	if err != nil || page.Coverage[0].ErrorCode != "insufficient_permissions" || page.NextCursor == "" {
		t.Fatal("partial OU failure lost", err)
	}
	page, err = p.Discover(ctx, c, grant.Credentials, page.NextCursor, 1)
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Resource.ID != "g1" || page.NextCursor == "" {
		t.Fatal("first group page", err)
	}
	page, err = p.Discover(ctx, c, grant.Credentials, page.NextCursor, 1)
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Resource.ID != "g2" || page.NextCursor != "" {
		t.Fatal("second group page", err)
	}
	renewed, err := p.Renew(ctx, c, grant.Credentials)
	if err != nil || renewed.Value("refresh_token") != "offline" || renewed.Value("access_token") != "renewed" {
		t.Fatal("refresh replacement semantics", err)
	}
	customer = "other"
	identityDenied = true
	if _, err = p.ExchangeAuthorization(ctx, c, claim, connections.AuthorizationResponse{Code: "code"}); err == nil {
		t.Fatal("missing customer identity permission accepted")
	}
	identityDenied = false
	if _, err = p.ExchangeAuthorization(ctx, c, claim, connections.AuthorizationResponse{Code: "code"}); err == nil {
		t.Fatal("wrong customer accepted")
	}
	revoked = true
	if _, err = p.Renew(ctx, c, grant.Credentials); err == nil {
		t.Fatal("revoked refresh accepted")
	}
	customer = "C123"
	revoked = false
	t.Run("shared-service-journey", func(t *testing.T) { verifyOAuthJourney(t, connections.Workspace, p, c.Configuration, grant) })
}
