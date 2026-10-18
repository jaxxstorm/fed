package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type browserProvider struct{ exchanges int }

func (*browserProvider) Test(context.Context, connections.Connection, connections.Credentials) (connections.AccessHealth, error) {
	return connections.AccessHealth{}, nil
}
func (*browserProvider) Discover(context.Context, connections.Connection, connections.Credentials, string, int) (connections.DiscoveryPage, error) {
	return connections.DiscoveryPage{}, nil
}
func (*browserProvider) Renew(context.Context, connections.Connection, connections.Credentials) (connections.Credentials, error) {
	return connections.Credentials{}, nil
}
func (*browserProvider) Read(context.Context, connections.Connection, connections.Credentials, connections.DiscoveredResource) (connections.Snapshot, error) {
	return connections.Snapshot{}, nil
}
func (*browserProvider) AuthorizationURL(_ connections.Connection, c connections.AuthorizationChallenge) (string, error) {
	return "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{"state": {c.State}, "redirect_uri": {c.RedirectURI}, "code_challenge": {c.PKCEChallenge}}.Encode(), nil
}
func (p *browserProvider) ExchangeAuthorization(_ context.Context, c connections.Connection, claim connections.AuthorizationClaim, response connections.AuthorizationResponse) (connections.AuthorizedGrant, error) {
	if response.Code != "provider-code" || claim.Verifier.Value("verifier") == "" {
		return connections.AuthorizedGrant{}, connections.ErrInvalid
	}
	p.exchanges++
	return connections.AuthorizedGrant{AccountID: c.Configuration.AccountID, Credentials: connections.NewCredentials(map[string]string{"access_token": "provider-secret"})}, nil
}

func TestProviderCallbackBoundSessionAndConfiguredOrigin(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Test")
	if err != nil {
		t.Fatal(err)
	}
	provider := &browserProvider{}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{connections.Workspace: provider})
	keys, _ := connections.NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db, Keys: keys}}
	d, err := s.Configure(ctx, a, connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "create", Provider: connections.Workspace, Name: "Directory", Mode: connections.ServerManaged, Configuration: connections.Configuration{AccountID: "customer"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at,policy_fingerprint) VALUES($1,$2,'csrf',now()+interval '1 hour',$3)`, storage.Hash([]byte("session")), a.ID, (auth.AdmissionConfig{}).Fingerprint()); err != nil {
		t.Fatal(err)
	}
	app := &App{Store: store, Connections: s, ConnectionOrigin: "https://app.example"}
	protected := (&auth.Browser{DB: db}).Protect(app.Handler())
	request := func(target, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r.Host = "attacker.example"
		r.Header.Set("X-Forwarded-Host", "attacker.example")
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "ksi_session", Value: cookie})
		}
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, r)
		return w
	}
	in := connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "begin"}
	handoff, err := s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated := request(handoff.URL, "")
	if unauthenticated.Header().Get("Location") != "/auth/login" {
		t.Fatal("handoff bypassed login")
	}
	w := request(handoff.URL, "session")
	providerURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil || providerURL.Host != "accounts.google.com" || providerURL.Query().Get("redirect_uri") != "https://app.example/connections/callback/google_workspace" {
		t.Fatalf("untrusted callback origin: %s", w.Header().Get("Location"))
	}
	callback := "https://app.example/connections/callback/google_workspace?" + url.Values{"state": {providerURL.Query().Get("state")}, "code": {"provider-code"}}.Encode()
	w = request(callback, "session")
	if w.Code != http.StatusSeeOther || strings.Contains(w.Header().Get("Location"), "provider-code") || provider.exchanges != 1 {
		t.Fatalf("callback: %d %s", w.Code, w.Body.String())
	}
	w = request(callback, "session")
	if provider.exchanges != 1 || !strings.Contains(w.Header().Get("Location"), "authorization_expired") {
		t.Fatal("callback replay exchanged code")
	}
	in.Generation, in.IdempotencyKey = 2, "cancel"
	handoff, err = s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	w = request(handoff.URL, "session")
	providerURL, _ = url.Parse(w.Header().Get("Location"))
	w = request("https://app.example/connections/callback/google_workspace?"+url.Values{"state": {providerURL.Query().Get("state")}, "error": {"access_denied"}}.Encode(), "session")
	op, err := s.GetOperation(ctx, a, service.ID, handoff.OperationID)
	if err != nil || op.State != "cancelled" || provider.exchanges != 1 {
		t.Fatalf("cancelled: %+v %v", op, err)
	}
	if strings.Contains(w.Header().Get("Location"), "state=") {
		t.Fatal("callback state retained in URL")
	}
}
