package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestConnectionAPISessionCSRFRevisionAndWriteOnlyInputs(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Connections")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES($1,$2,'csrf',now()+interval '1 hour')`, storage.Hash([]byte("session")), a.ID); err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(nil)
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	handler := (&auth.Browser{DB: db}).Protect((&App{Store: store, Connections: s}).Handler())
	post := func(input any, csrf string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(input)
		r := httptest.NewRequest("POST", "https://app.example/api/ui/v1/services/"+service.ID+"/connections/configure", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "ksi_session", Value: "session"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	in := connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "create", Provider: connections.Workspace, Name: "Directory", Mode: connections.AgentManaged, Configuration: connections.Configuration{AccountID: "C123"}}
	if w := post(in, ""); w.Code != 403 {
		t.Fatal("missing CSRF accepted", w.Code)
	}
	w := post(in, "csrf")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created connections.Detail
	if err = json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	in.ID = created.Connection.ID
	in.IdempotencyKey = "stale"
	if w = post(in, "csrf"); w.Code != 409 {
		t.Fatal("stale revision accepted", w.Code, w.Body.String())
	}
	in.ServiceID = "other"
	if w = post(in, "csrf"); w.Code != 400 {
		t.Fatal("route service mismatch accepted")
	}
	if w = post(map[string]any{"service_id": service.ID, "credentials": "do-not-echo-this"}, "csrf"); w.Code != 400 || strings.Contains(w.Body.String(), "do-not-echo-this") {
		t.Fatal("credential input exposed or accepted")
	}
	got, err := s.Get(ctx, a, service.ID, created.Connection.ID)
	if err != nil || got.Connection.Revision != 1 {
		t.Fatal("browser operation diverged from shared service")
	}
}
