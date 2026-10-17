package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/storage"
	"golang.org/x/oauth2"
)

type Browser struct {
	DB       *pgxpool.Pool
	OAuth    oauth2.Config
	Verifier *oidc.IDTokenVerifier
	Dev      *Actor
}

func NewBrowser(ctx context.Context, db *pgxpool.Pool, issuer, clientID, clientSecret, publicURL string) (*Browser, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &Browser{DB: db, OAuth: oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: strings.TrimRight(publicURL, "/") + "/auth/callback", Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, Verifier: provider.Verifier(&oidc.Config{ClientID: clientID})}, nil
}

func cookie(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func (b *Browser) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	state, nonce, verifier := Secret(), Secret(), oauth2.GenerateVerifier()
	_, err := b.DB.Exec(r.Context(), `INSERT INTO login_attempts(state_hash,nonce,verifier,expires_at) VALUES($1,$2,$3,$4)`, storage.Hash([]byte(state)), nonce, verifier, time.Now().Add(5*time.Minute))
	if err != nil {
		http.Error(w, "login unavailable", 503)
		return
	}
	cookie(w, "ksi_login", state, 300, true)
	http.Redirect(w, r, b.OAuth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (b *Browser) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	c, err := r.Cookie("ksi_login")
	state := r.URL.Query().Get("state")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		http.Error(w, "invalid login state", 400)
		return
	}
	cookie(w, "ksi_login", "", -1, true)
	var nonce, verifier string
	err = b.DB.QueryRow(r.Context(), `DELETE FROM login_attempts WHERE state_hash=$1 AND expires_at>now() RETURNING nonce,verifier`, storage.Hash([]byte(state))).Scan(&nonce, &verifier)
	if err != nil {
		http.Error(w, "expired login state", 400)
		return
	}
	token, err := b.OAuth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		http.Error(w, "login failed", 401)
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing identity token", 401)
		return
	}
	id, err := b.Verifier.Verify(r.Context(), raw)
	if err != nil || id.Nonce != nonce {
		http.Error(w, "invalid identity token", 401)
		return
	}
	var actorID string
	err = b.DB.QueryRow(r.Context(), `SELECT id FROM identities WHERE issuer=$1 AND subject=$2 AND kind='human'`, id.Issuer, id.Subject).Scan(&actorID)
	if err != nil {
		http.Error(w, "identity has no access", 403)
		return
	}
	session, csrf := Secret(), Secret()
	_, err = b.DB.Exec(r.Context(), `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)`, storage.Hash([]byte(session)), actorID, csrf, time.Now().Add(8*time.Hour))
	if err != nil {
		http.Error(w, "session unavailable", 503)
		return
	}
	cookie(w, "ksi_session", session, 8*3600, true)
	http.Redirect(w, r, "/", http.StatusFound)
}

type csrfKey struct{}
type sessionKey struct{}

func CSRF(ctx context.Context) string        { s, _ := ctx.Value(csrfKey{}).(string); return s }
func SessionHash(ctx context.Context) string { s, _ := ctx.Value(sessionKey{}).(string); return s }

// Protect is shared by all application routes. Bearer credentials are only
// accepted by the MCP middleware, never converted into browser sessions.
func (b *Browser) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a Actor
		var csrf string
		var sessionHash string
		if b.Dev != nil {
			a = *b.Dev
			c, err := r.Cookie("ksi_dev_csrf")
			if err != nil {
				csrf = Secret()
				cookie(w, "ksi_dev_csrf", csrf, 8*3600, false)
			} else {
				csrf = c.Value
			}
			// The prefix cannot be produced by production cookie hashing, so a
			// development browser binding cannot become a production session.
			if b.DB != nil {
				sessionHash = "dev:" + storage.Hash([]byte(a.ID+":"+csrf))
				if _, err := b.DB.Exec(r.Context(), `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES($1,$2,$3,now()+interval '8 hours') ON CONFLICT(hash) DO UPDATE SET expires_at=excluded.expires_at`, sessionHash, a.ID, csrf); err != nil {
					http.Error(w, "development session unavailable", 503)
					return
				}
			}
		} else {
			c, err := r.Cookie("ksi_session")
			if err != nil {
				if strings.HasPrefix(r.URL.Path, "/api/ui/") {
					browserAPIError(w, 401, "session_expired", "Sign in to continue", "Your session has expired. Sign in, then retry your action.")
					return
				}
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			sessionHash = storage.Hash([]byte(c.Value))
			err = b.DB.QueryRow(r.Context(), `SELECT i.id,i.organization_id,i.kind,s.csrf_token FROM sessions s JOIN identities i ON i.id=s.identity_id WHERE s.hash=$1 AND s.expires_at>now()`, sessionHash).Scan(&a.ID, &a.OrganizationID, &a.Kind, &csrf)
			if err != nil {
				if strings.HasPrefix(r.URL.Path, "/api/ui/") {
					browserAPIError(w, 401, "session_expired", "Sign in to continue", "Your session is no longer available.")
					return
				}
				http.Error(w, "unauthenticated", 401)
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
			provided := r.Header.Get("X-CSRF-Token")
			if provided == "" {
				provided = r.FormValue("csrf_token")
			}
			if csrf == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(csrf)) != 1 {
				if strings.HasPrefix(r.URL.Path, "/api/ui/") {
					browserAPIError(w, 403, "csrf_expired", "Refresh your session", "Reload this page before saving again. Keep a copy of your edits.")
					return
				}
				http.Error(w, "invalid CSRF token", 403)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		ctx := context.WithValue(WithActor(r.Context(), a), csrfKey{}, csrf)
		ctx = context.WithValue(ctx, sessionKey{}, sessionHash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func browserAPIError(w http.ResponseWriter, status int, code, title, explanation string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// Values are fixed strings owned by this package.
	_, _ = w.Write([]byte(`{"code":"` + code + `","title":"` + title + `","explanation":"` + explanation + `","action":"/auth/login","retryable":false}`))
}

func (b *Browser) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if c, err := r.Cookie("ksi_session"); err == nil {
		if _, err = b.DB.Exec(r.Context(), `DELETE FROM sessions WHERE hash=$1`, storage.Hash([]byte(c.Value))); err != nil {
			http.Error(w, "logout unavailable", 503)
			return
		}
	}
	cookie(w, "ksi_session", "", -1, b.Dev == nil)
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func Bearer(db *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			http.Error(w, "bearer token required", 401)
			return
		}
		a, err := TokenActor(r.Context(), db, strings.TrimPrefix(value, "Bearer "))
		if err != nil {
			http.Error(w, "invalid bearer token", 401)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), a)))
	})
}
