package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestOIDCLogin(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, nonce string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "key", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			if r.FormValue("code_verifier") == "" {
				t.Error("missing PKCE verifier")
			}
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "key"}}, nil)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			token, err := jwt.Signed(signer).Claims(map[string]any{"iss": issuer, "sub": "admin", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce}).Serialize()
			if err != nil {
				t.Error(err)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "token_type": "Bearer", "id_token": token})
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	issuer = provider.URL
	if _, err = BootstrapAdmin(ctx, db, issuer, "admin"); err != nil {
		t.Fatal(err)
	}
	b, err := NewBrowser(ctx, db, issuer, "client", "secret", "https://ksi.example")
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRecorder()
	b.Login(login, httptest.NewRequest("GET", "https://ksi.example/auth/login", nil))
	if login.Code != 302 {
		t.Fatalf("login: %d", login.Code)
	}
	location, err := url.Parse(login.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := location.Query().Get("state")
	nonce = location.Query().Get("nonce")
	if location.Query().Get("code_challenge_method") != "S256" || nonce == "" {
		t.Fatal("PKCE/nonce missing")
	}
	r := httptest.NewRequest("GET", "https://ksi.example/auth/callback?code=code&state="+url.QueryEscape(state), nil)
	for _, c := range login.Result().Cookies() {
		r.AddCookie(c)
	}
	callback := httptest.NewRecorder()
	b.Callback(callback, r)
	if callback.Code != 302 {
		t.Fatalf("callback: %d %s", callback.Code, callback.Body.String())
	}
	var session *http.Cookie
	for _, c := range callback.Result().Cookies() {
		if c.Name == "ksi_session" {
			session = c
		}
	}
	if session == nil || !session.HttpOnly || !session.Secure {
		t.Fatal("secure session not issued")
	}
	view := httptest.NewRequest("GET", "https://ksi.example/", nil)
	view.AddCookie(session)
	w := httptest.NewRecorder()
	b.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()).Kind != "human" {
			t.Error("not human")
		}
		w.WriteHeader(204)
	})).ServeHTTP(w, view)
	if w.Code != 204 {
		t.Fatalf("session rejected: %d", w.Code)
	}
	replay := httptest.NewRecorder()
	b.Callback(replay, r)
	if replay.Code != 400 {
		t.Fatal("login state replay accepted")
	}
}
