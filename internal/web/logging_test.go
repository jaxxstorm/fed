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
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/labstack/echo/v4"
)

func TestRequestLoggingDiagnosticsAndStreaming(t *testing.T) {
	var logs bytes.Buffer
	l, _ := logging.New(&logs, nil, "debug", "json", "server")
	e := Bootstrap(func(context.Context) error { return nil })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ui/v1/items/{id}", func(w http.ResponseWriter, r *http.Request) { uiFailure(w, assessment.ErrNotFound, r.Context()) })
	mux.HandleFunc("GET /api/ui/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("stream"))
	})
	e.Any("/api/ui/v1/*", echo.WrapHandler(loggedMux(mux)))
	e.GET("/panic", func(c echo.Context) error { panic("secret-panic") })
	for _, path := range []string{"/api/ui/v1/items/secret-path?token=secret-query", "/api/ui/v1/stream", "/panic", "/secret-unknown"} {
		req := httptest.NewRequest("GET", path, nil).WithContext(logging.WithLogger(context.Background(), l))
		req.Header.Set("X-Request-ID", "secret-header")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Header().Get("X-Request-ID") == "" || w.Header().Get("X-Request-ID") == "secret-header" {
			t.Fatal("missing server correlation")
		}
		if strings.Contains(path, "items") {
			var out UIError
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if w.Code != 404 || !strings.Contains(logs.String(), out.Reference) {
				t.Fatal("diagnostic missing", w.Body.String())
			}
		}
		if path == "/api/ui/v1/stream" && (!w.Flushed || w.Body.String() != "stream") {
			t.Fatal("stream broken")
		}
		if path == "/panic" && w.Code != 500 {
			t.Fatal("panic status", w.Code)
		}
	}
	for _, secret := range []string{"secret-path", "secret-query", "secret-header", "secret-panic", "secret-unknown"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("secret leaked", secret)
		}
	}
	if strings.Count(logs.String(), `"event":"http.request.completed"`) != 4 {
		t.Fatal(logs.String())
	}
	if !strings.Contains(logs.String(), "GET /api/ui/v1/items/{id}") {
		t.Fatal("inner route absent")
	}
}

func TestHTTPLevelsAndCancellation(t *testing.T) {
	var logs bytes.Buffer
	l, _ := logging.New(&logs, nil, "info", "json", "server")
	root := logging.WithLogger(context.Background(), l)
	e := Bootstrap(func(context.Context) error { return assessment.ErrNotFound })
	e.GET("/forbidden", func(c echo.Context) error { return echo.NewHTTPError(403) })
	e.GET("/conflict", func(c echo.Context) error {
		uiFailure(c.Response(), assessment.ErrConflict, c.Request().Context())
		return nil
	})
	e.GET("/cancelled", func(c echo.Context) error { return c.NoContent(200) })
	for _, path := range []string{"/healthz", "/readyz", "/forbidden", "/conflict", "/cancelled"} {
		ctx := root
		if path == "/cancelled" {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(root)
			cancel()
		}
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil).WithContext(ctx))
	}
	if strings.Contains(logs.String(), `"route":"/healthz"`) {
		t.Fatal("probe noise")
	}
	for _, s := range []string{`"status":503`, `"status":403`, `"status":409`, `"outcome":"cancelled"`} {
		if !strings.Contains(logs.String(), s) {
			t.Fatal("missing outcome", s)
		}
	}
}

func TestAuthenticationAndCSRFAreLoggedBeforeDispatch(t *testing.T) {
	for _, test := range []struct {
		name, method string
		browser      *auth.Browser
		status       int
	}{
		{"session", "GET", &auth.Browser{}, 401},
		{"csrf", "POST", &auth.Browser{Dev: &auth.Actor{ID: "trusted-actor"}}, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			l, _ := logging.New(&logs, nil, "info", "json", "server")
			e := Bootstrap(func(context.Context) error { return nil })
			e.Any("/api/ui/v1/*", echo.WrapHandler(test.browser.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unauthorized handler ran") }))))
			req := httptest.NewRequest(test.method, "/api/ui/v1/protected?secret=secret-query", nil).WithContext(logging.WithLogger(context.Background(), l))
			req.Header.Set("X-CSRF-Token", "secret-csrf")
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
			if w.Code != test.status || !strings.Contains(logs.String(), `"event":"http.request.completed"`) {
				t.Fatal("missing rejected request", w.Code, logs.String())
			}
			if strings.Contains(logs.String(), "secret-") {
				t.Fatal("request input leaked")
			}
		})
	}
}

func BenchmarkHTTPLogging(b *testing.B) {
	for _, level := range []string{"info", "error"} {
		b.Run(level, func(b *testing.B) {
			l, _ := logging.New(nil, nil, level, "json", "server")
			e := Bootstrap(func(context.Context) error { return nil })
			e.GET("/api/test", func(c echo.Context) error { return c.NoContent(204) })
			ctx := logging.WithLogger(context.Background(), l)
			b.ReportAllocs()
			for b.Loop() {
				e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx))
			}
		})
	}
}
