package web

import (
	"encoding/json"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserAPISessionAndCSRF(t *testing.T) {
	called := false
	b := &auth.Browser{}
	handler := b.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/ui/v1/bootstrap", nil))
	if response.Code != 401 || response.Header().Get("Location") != "" || called {
		t.Fatalf("API auth redirected or ran protected handler: %d", response.Code)
	}
	var result map[string]any
	if json.Unmarshal(response.Body.Bytes(), &result) != nil || result["code"] != "session_expired" {
		t.Fatal("missing structured auth response")
	}
	actor := auth.Actor{}
	b.Dev = &actor
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/ui/v1/services", nil))
	if response.Code != 403 || called {
		t.Fatal("mutation accepted without CSRF")
	}
}
func TestUIConflictRecovery(t *testing.T) {
	w := httptest.NewRecorder()
	uiFailure(w, assessment.ErrConflict)
	var e UIError
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if w.Code != 409 || e.Code != "revision_conflict" || e.Action == "" || e.Reference == "" {
		t.Fatal("conflict recovery information missing")
	}
}
func TestFrontendDoesNotCaptureProtocolRoutes(t *testing.T) {
	app := &App{}
	h := app.Handler()
	for _, path := range []string{"/mcp", "/healthz", "/auth/login", "/api/not-an-endpoint"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 404 {
			t.Fatalf("%s incorrectly captured: %d", path, r.Code)
		}
	}
}
