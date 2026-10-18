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
		if w.Code != 200 || !strings.Contains(w.Body.String(), `href="/auth/login"`) {
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
		{"missing session", "", "", 401}, {"invalid session", "bad", "csrf", 401}, {"missing csrf", "session", "", 403}, {"valid", "session", "csrf", 204},
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

func TestExpiredSessionNavigationRecoversToLogin(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := BootstrapAdmin(ctx, db, "https://id.example", "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range []struct {
		name        string
		expiry      time.Time
		fingerprint string
	}{
		{"expired", time.Now().Add(-time.Hour), (AdmissionConfig{}).Fingerprint()},
		{"policy-changed", time.Now().Add(time.Hour), "old-policy"},
	} {
		if _, err := db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at,policy_fingerprint) VALUES($1,$2,'csrf',$3,$4)`, storage.Hash([]byte(session.name)), a.ID, session.expiry, session.fingerprint); err != nil {
			t.Fatal(err)
		}
	}
	b := Browser{DB: db}
	h := b.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid session reached protected handler") }))
	for _, session := range []string{"deleted-by-migration", "expired", "policy-changed"} {
		for _, request := range []struct {
			method, path string
			status       int
		}{
			{"GET", "/", 200}, {"GET", "/services/example", 302}, {"HEAD", "/", 200}, {"GET", "/api/ui/v1/bootstrap", 401}, {"POST", "/services", 401},
		} {
			t.Run(session+request.method+request.path, func(t *testing.T) {
				r := httptest.NewRequest(request.method, "https://ksi.example"+request.path, nil)
				r.AddCookie(&http.Cookie{Name: "ksi_session", Value: session})
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != request.status {
					t.Fatal(w.Code, w.Body.String())
				}
				if request.status == 302 && w.Header().Get("Location") != "/" {
					t.Fatal("missing home redirect")
				}
				if request.method == "GET" && request.path == "/" && !strings.Contains(w.Body.String(), `href="/auth/login"`) {
					t.Fatal("missing sign-in action")
				}
				if request.status == 401 && w.Header().Get("Location") != "" {
					t.Fatal("API/mutation redirected")
				}
				cleared := false
				for _, c := range w.Result().Cookies() {
					if c.Name == "ksi_session" && c.MaxAge < 0 && c.Secure && c.HttpOnly {
						cleared = true
					}
				}
				if !cleared || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("stale cookie retained or response cacheable")
				}
			})
		}
	}
}

func TestSignedOutHomeDoesNotLoadProtectedApplication(t *testing.T) {
	b := Browser{}
	h := b.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("anonymous request reached application") }))
	for _, path := range []string{"/", "/services/example", "/api/ui/v1/bootstrap"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://ksi.example"+path, nil))
		switch path {
		case "/":
			if w.Code != 200 || !strings.Contains(w.Body.String(), `href="/auth/login"`) {
				t.Fatal("home unavailable", w.Code)
			}
			if strings.Contains(w.Body.String(), "<script") || strings.Contains(w.Body.String(), "/api/") {
				t.Fatal("home loaded authenticated application")
			}
		case "/services/example":
			if w.Code != 302 || w.Header().Get("Location") != "/" {
				t.Fatal("missing home redirect")
			}
		default:
			if w.Code != 401 {
				t.Fatal("API did not reject anonymous request")
			}
		}
	}
	w := httptest.NewRecorder()
	b.Logout(w, httptest.NewRequest("POST", "https://ksi.example/auth/logout", nil))
	if w.Code != 303 || w.Header().Get("Location") != "/" {
		t.Fatal("logout did not return home")
	}
}
