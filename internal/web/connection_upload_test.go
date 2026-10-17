package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type uploadProvider struct {
	browserProvider
	tests int
}

func (p *uploadProvider) Test(_ context.Context, c connections.Connection, k connections.Credentials) (connections.AccessHealth, error) {
	p.tests++
	if k.Value("kubeconfig") != "write-only-test-credential" {
		return connections.AccessHealth{}, connections.ErrInvalid
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: c.Tenant()}, nil
}

func TestBrowserCredentialHandoffWriteOnlyAndSingleUse(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Cluster")
	if err != nil {
		t.Fatal(err)
	}
	provider := &uploadProvider{}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{connections.Kubernetes: provider})
	keys, _ := connections.NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db, Keys: keys}}
	d, err := s.Configure(ctx, a, connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "create", Provider: connections.Kubernetes, Name: "Cluster", Mode: connections.ServerManaged, Configuration: connections.Configuration{AccountID: "cluster", ClusterID: "cluster", Server: "https://cluster.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES($1,$2,'csrf',now()+interval '1 hour')`, storage.Hash([]byte("session")), a.ID); err != nil {
		t.Fatal(err)
	}
	handoff, err := s.BeginAuthorization(ctx, a, connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "upload"}, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	handler := (&auth.Browser{DB: db}).Protect((&App{Store: store, Connections: s, ConnectionOrigin: "https://app.example"}).Handler())
	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "ksi_session", Value: "session"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	page := do("GET", handoff.URL, "")
	state := regexp.MustCompile(`name="state" value="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if page.Code != 200 || len(state) != 2 {
		t.Fatal("upload handoff missing", page.Code, page.Body.String())
	}
	form := url.Values{"state": {state[1]}, "kubeconfig": {"write-only-test-credential"}}
	if response := do("POST", "https://app.example/connections/upload/kubernetes", form.Encode()); response.Code != 403 || provider.tests != 0 {
		t.Fatal("upload bypassed CSRF")
	}
	form.Set("csrf_token", "csrf")
	response := do("POST", "https://app.example/connections/upload/kubernetes", form.Encode())
	if response.Code != 303 || strings.Contains(response.Body.String(), "write-only-test-credential") || provider.tests != 1 {
		t.Fatal("upload failed or leaked credential", response.Code)
	}
	if response = do("POST", "https://app.example/connections/upload/kubernetes", form.Encode()); response.Code != 409 || provider.tests != 1 {
		t.Fatal("upload replay accepted")
	}
	stored, err := s.Credentials.Get(ctx, a, service.ID, d.Connection.ID, "connect", 2)
	if err != nil || stored.Value("kubeconfig") != "write-only-test-credential" {
		t.Fatal("encrypted upload missing", err)
	}
}
