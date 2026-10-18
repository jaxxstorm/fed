package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestDevelopmentBrowserBindingCannotAuthenticateProduction(t *testing.T) {
	db := testutil.Database(t)
	a, err := BootstrapAdmin(context.Background(), db, "local-development", "developer")
	if err != nil {
		t.Fatal(err)
	}
	var binding string
	dev := (&Browser{DB: db, Dev: &a}).Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { binding = SessionHash(r.Context()); w.WriteHeader(204) }))
	r := httptest.NewRequest("GET", "http://127.0.0.1/", nil)
	r.AddCookie(&http.Cookie{Name: "ksi_dev_csrf", Value: "browser-specific"})
	w := httptest.NewRecorder()
	dev.ServeHTTP(w, r)
	if w.Code != 204 || !strings.HasPrefix(binding, "dev:") {
		t.Fatal("missing development binding")
	}
	var valid bool
	if err = db.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM sessions WHERE hash=$1 AND identity_id=$2)`, binding, a.ID).Scan(&valid); err != nil || !valid {
		t.Fatal("development binding not persisted")
	}
	production := (&Browser{DB: db}).Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("development session accepted in production") }))
	for _, value := range []string{"browser-specific", binding} {
		r = httptest.NewRequest("GET", "https://app.example/", nil)
		r.AddCookie(&http.Cookie{Name: "ksi_session", Value: value})
		w = httptest.NewRecorder()
		production.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("development credential authenticated production")
		}
	}
}

func TestSessionAndCSRF(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	a, err := BootstrapAdmin(ctx, db, "https://id.example", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at,policy_fingerprint) VALUES($1,$2,$3,$4,$5)`, storage.Hash([]byte("session")), a.ID, "csrf", time.Now().Add(time.Hour), (AdmissionConfig{}).Fingerprint())
	if err != nil {
		t.Fatal(err)
	}
	b := Browser{DB: db}
	h := b.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()).ID != a.ID || CSRF(r.Context()) != "csrf" {
			t.Error("identity not propagated")
		}
		w.WriteHeader(204)
	}))
	for _, tt := range []struct {
		name, session, csrf string
		status              int
	}{
		{"missing session", "", "", 302}, {"invalid session", "bad", "csrf", 401}, {"missing csrf", "session", "", 403}, {"valid", "session", "csrf", 204},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "https://ksi.example/", nil)
			if tt.session != "" {
				r.AddCookie(&http.Cookie{Name: "ksi_session", Value: tt.session})
			}
			r.Header.Set("X-CSRF-Token", tt.csrf)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
	// A callback cannot consume state belonging to another browser.
	_, err = db.Exec(ctx, `INSERT INTO login_attempts(state_hash,nonce,verifier,expires_at) VALUES($1,'nonce','verifier',$2)`, storage.Hash([]byte("state")), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "https://ksi.example/auth/callback?state=state", nil)
	r.AddCookie(&http.Cookie{Name: "ksi_login", Value: "other"})
	w := httptest.NewRecorder()
	b.Callback(w, r)
	if w.Code != 400 {
		t.Fatalf("callback status %d", w.Code)
	}
	var n int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM login_attempts`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("state was consumed: %d %v", n, err)
	}
}
