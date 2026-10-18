package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/storage"
	"golang.org/x/oauth2"
)

type Browser struct {
	DB        *pgxpool.Pool
	OAuth     oauth2.Config
	Verifier  *oidc.IDTokenVerifier
	Dev       *Actor
	Admission AdmissionConfig
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
	var claims map[string]any
	if err = id.Claims(&claims); err != nil {
		http.Error(w, "invalid identity claims", 401)
		return
	}
	email, _ := claims["email"].(string)
	verified, _ := claims["email_verified"].(bool)
	name, _ := claims["name"].(string)
	member, err := Admit(r.Context(), b.DB, b.Admission, VerifiedIdentity{Issuer: id.Issuer, Subject: id.Subject, Email: email, EmailVerified: verified, Name: name, Claims: claims})
	if err != nil {
		http.Error(w, "identity has no access", 403)
		return
	}
	session, csrf := Secret(), Secret()
	_, err = b.DB.Exec(r.Context(), `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at,policy_fingerprint) VALUES($1,$2,$3,$4,$5)`, storage.Hash([]byte(session)), member.IdentityID, csrf, time.Now().Add(8*time.Hour), b.Admission.Fingerprint())
	if err != nil {
		http.Error(w, "session unavailable", 503)
		return
	}
	cookie(w, "ksi_session", session, 8*3600, true)
	if member.State == "pending" {
		http.Redirect(w, r, "/approval", http.StatusFound)
		return
	}
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
				signedOut(w, r)
				return
			}
			sessionHash = storage.Hash([]byte(c.Value))
			var fingerprint string
			err = b.DB.QueryRow(r.Context(), `SELECT i.id,i.organization_id,i.kind,s.csrf_token,s.policy_fingerprint FROM sessions s JOIN identities i ON i.id=s.identity_id WHERE s.hash=$1 AND s.expires_at>now()`, sessionHash).Scan(&a.ID, &a.OrganizationID, &a.Kind, &csrf, &fingerprint)
			if err == nil && fingerprint != b.Admission.Fingerprint() {
				err = ErrForbidden
			}
			if err != nil {
				w.Header().Set("Cache-Control", "no-store")
				if !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, ErrForbidden) {
					if strings.HasPrefix(r.URL.Path, "/api/ui/") {
						browserAPIError(w, 503, "session_unavailable", "Sign-in temporarily unavailable", "Retry once the service is available.")
					} else {
						http.Error(w, "session unavailable", 503)
					}
					return
				}
				cookie(w, "ksi_session", "", -1, true)
				if strings.HasPrefix(r.URL.Path, "/api/ui/") {
					browserAPIError(w, 401, "session_expired", "Sign in to continue", "Your session is no longer available.")
					return
				}
				signedOut(w, r)
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
		// A still-authenticated session must be able to sign out even when a
		// policy edit has made its identity ineligible for application access.
		if r.URL.Path == "/auth/logout" {
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if b.DB != nil {
			m, err := ActiveMember(ctx, b.DB, a)
			if err != nil && !errors.Is(err, ErrMembershipPending) {
				browserAPIError(w, 403, "membership_disabled", "Access unavailable", "Contact your administrator for access.")
				return
			}
			if r.URL.Path == "/api/ui/v1/membership-status" && r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(ApprovalStatus{m.State, m.Email})
				return
			}
			if r.URL.Path == "/approval" && r.Method == "GET" {
				if err == nil {
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_ = approvalPage.Execute(w, struct{ Email, CSRF string }{m.Email, csrf})
				return
			}
			if errors.Is(err, ErrMembershipPending) && r.URL.Path != "/auth/logout" {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					browserAPIError(w, 403, "membership_pending", "Waiting for approval", "An administrator must approve your access.")
				} else {
					http.Redirect(w, r, "/approval", http.StatusSeeOther)
				}
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type ApprovalStatus struct {
	State string `json:"state"`
	Email string `json:"email"`
}

// Signed-out navigation always has a usable home page. No authenticated
// bootstrap data or JavaScript application is loaded before sign-in.
func signedOut(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_ = signedOutPage.Execute(w, nil)
}

var signedOutPage = template.Must(template.New("signed-out").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Fed · Assessment evidence workspace</title><style>body{margin:0;background:#f6f7fb;color:#172033;font:16px/1.6 Inter,ui-sans-serif,system-ui,sans-serif}main{box-sizing:border-box;max-width:680px;margin:12vh auto;padding:40px;background:white;border:1px solid #dce2eb;border-radius:12px}h1{font-size:36px;letter-spacing:-1px;line-height:1.2}.brand{color:#4f46e5;font-weight:650}a{display:inline-block;min-height:44px;box-sizing:border-box;margin-top:16px;padding:10px 24px;border-radius:8px;background:#4f46e5;color:white;font-weight:600;text-decoration:none}a:focus-visible{outline:3px solid #4f46e5;outline-offset:4px}.muted{color:#536078;font-size:14px}@media(max-width:720px){main{margin:32px 16px;padding:24px}h1{font-size:30px}}</style></head><body><main><p class="brand">Fed</p><h1>Assessment evidence workspace</h1><p>Collect evidence, track assessment progress and collaborate with your team.</p><a href="/auth/login">Sign in</a><p class="muted">Sign in with your organization account. New users wait for administrator approval before accessing assessments.</p></main></body></html>`))

var approvalPage = template.Must(template.New("approval").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Waiting for approval · Fed</title><style>body{margin:0;background:#f6f7fb;color:#172033;font:14px/1.5 Inter,ui-sans-serif,system-ui,sans-serif}main{max-width:560px;margin:10vh auto;padding:32px;background:white;border:1px solid #dce2eb;border-radius:12px}h1{font-size:32px;letter-spacing:-1px;line-height:1.2}p{overflow-wrap:anywhere}.brand{color:#4f46e5;font-weight:650}nav{display:flex;gap:16px;flex-wrap:wrap;margin-top:24px}a,button{display:inline-block;box-sizing:border-box;min-height:44px;padding:10px 16px;border:1px solid #dce2eb;border-radius:8px;background:white;color:inherit;font:inherit;text-decoration:none;cursor:pointer}:focus-visible{outline:3px solid #4f46e5;outline-offset:3px}@media(max-width:600px){main{margin:24px 16px;padding:24px}}</style></head><body><main><p class="brand">Fed · Assessment evidence workspace</p><h1>Waiting for approval</h1><p>You are signed in as {{.Email}}. An administrator must approve your access before you can view assessments or evidence.</p><nav aria-label="Approval actions"><a href="/approval">Refresh status</a><form method="post" action="/auth/logout"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><button type="submit">Sign out</button></form></nav></main></body></html>`))

func browserAPIError(w http.ResponseWriter, status int, code, title, explanation string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// Values are fixed strings owned by this package.
	action := "/auth/login"
	if code == "membership_pending" {
		action = "/approval"
	}
	_, _ = w.Write([]byte(`{"code":"` + code + `","title":"` + title + `","explanation":"` + explanation + `","action":"` + action + `","retryable":false}`))
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
