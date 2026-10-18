package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestOIDCApprovalJourney(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "key"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var issuer string
	var mu sync.Mutex
	var claims map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "key", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			mu.Lock()
			token, e := jwt.Signed(signer).Claims(claims).Serialize()
			mu.Unlock()
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "token_type": "Bearer", "id_token": token})
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	issuer = provider.URL
	browser, err := NewBrowser(ctx, db, issuer, "client", "secret", "https://fed.example")
	if err != nil {
		t.Fatal(err)
	}
	browser.Admission = AdmissionConfig{AdminEmail: "admin@example.com", OrganizationClaim: "tid", OrganizationValue: "tenant"}
	login := func(sub, email string, verified bool, tenant, nonceOverride string) (*httptest.ResponseRecorder, *http.Cookie) {
		t.Helper()
		start := httptest.NewRecorder()
		browser.Login(start, httptest.NewRequest("GET", "https://fed.example/auth/login", nil))
		location, e := url.Parse(start.Header().Get("Location"))
		if e != nil {
			t.Fatal(e)
		}
		nonce := location.Query().Get("nonce")
		if nonceOverride != "" {
			nonce = nonceOverride
		}
		mu.Lock()
		claims = map[string]any{"iss": issuer, "sub": sub, "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "email": email, "email_verified": verified, "tid": tenant}
		mu.Unlock()
		req := httptest.NewRequest("GET", "https://fed.example/auth/callback?code=code&state="+url.QueryEscape(location.Query().Get("state")), nil)
		for _, c := range start.Result().Cookies() {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		browser.Callback(out, req)
		var session *http.Cookie
		for _, c := range out.Result().Cookies() {
			if c.Name == "ksi_session" {
				session = c
			}
		}
		return out, session
	}
	for _, c := range []struct {
		verified      bool
		tenant, nonce string
		status        int
	}{{false, "tenant", "", 403}, {true, "wrong", "", 403}, {true, "tenant", "wrong-nonce", 401}} {
		out, session := login("admin", "admin@example.com", c.verified, c.tenant, c.nonce)
		if out.Code != c.status || session != nil {
			t.Fatal("invalid identity admitted", out.Code)
		}
	}
	out, _ := login("admin", "admin@example.com", true, "tenant", "")
	if out.Code != 302 || out.Header().Get("Location") != "/" {
		t.Fatal("bootstrap", out.Code, out.Body.String())
	}
	admin, err := ResolveAdministrator(ctx, db, issuer, "", "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	out, session := login("member", "member@example.com", true, "tenant", "")
	if out.Code != 302 || out.Header().Get("Location") != "/approval" || session == nil {
		t.Fatal("first sign-in not jailed", out.Code)
	}
	protected := browser.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "https://fed.example"+path, nil)
		req.AddCookie(session)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		return w
	}
	if get("/api/ui/v1/bootstrap").Code != 403 || get("/api/ui/v1/members").Code != 403 || get("/services/private").Header().Get("Location") != "/approval" {
		t.Fatal("jail bypass")
	}
	if get("/approval").Code != 200 || get("/api/ui/v1/membership-status").Code != 200 {
		t.Fatal("jailed status unavailable")
	}
	page, err := ListMembers(ctx, db, admin, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	var pending Membership
	for _, m := range page.Members {
		if m.State == "pending" {
			pending = m
		}
	}
	approved, err := ChangeMembership(ctx, db, admin, MembershipChange{IdentityID: pending.IdentityID, ExpectedRevision: pending.Revision, Operation: "approve", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if get("/api/ui/v1/bootstrap").Code != 204 || get("/approval").Header().Get("Location") != "/" {
		t.Fatal("approval not reflected in existing session")
	}
	browser.Admission.OrganizationValue = "changed"
	if get("/api/ui/v1/bootstrap").Code != 401 {
		t.Fatal("old tenant session accepted")
	}
	browser.Admission.OrganizationValue = "tenant"
	if _, err = ChangeMembership(ctx, db, admin, MembershipChange{IdentityID: approved.IdentityID, ExpectedRevision: approved.Revision, Operation: "disable"}); err != nil {
		t.Fatal(err)
	}
	if get("/api/ui/v1/bootstrap").Code != 401 {
		t.Fatal("disabled session survived")
	}
	out, _ = login("member", "member@example.com", true, "tenant", "")
	if out.Code != 403 {
		t.Fatal("disabled re-login", out.Code)
	}
}
