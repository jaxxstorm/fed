package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBrowserCompressionPreservesPayloadAndStreamingRoutes(t *testing.T) {
	e := Bootstrap(func(context.Context) error { return nil })
	payload := strings.Repeat(`{"observation":"retained evidence and exact citations"}`, 1000)
	for _, route := range []string{"/api/ui/v1/example", "/assets/example.js", "/mcp", "/services/example/artifacts/example"} {
		e.GET(route, func(c echo.Context) error { return c.String(http.StatusOK, payload) })
	}
	for _, route := range []string{"/api/ui/v1/example", "/assets/example.js", "/mcp", "/services/example/artifacts/example"} {
		for _, accepts := range []bool{false, true} {
			r := httptest.NewRequest(http.MethodGet, route, nil)
			if accepts {
				r.Header.Set("Accept-Encoding", "gzip")
			}
			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			body := w.Body.Bytes()
			compressed := accepts && (strings.HasPrefix(route, "/api/ui/") || strings.HasPrefix(route, "/assets/"))
			if (w.Header().Get("Content-Encoding") == "gzip") != compressed {
				t.Fatalf("unexpected encoding for %s, accepts=%v", route, accepts)
			}
			if compressed {
				if len(body) >= len(payload)/10 {
					t.Fatal("repetitive evidence payload did not shrink substantially")
				}
				reader, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				body, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				reader.Close()
			}
			if string(body) != payload {
				t.Fatalf("payload changed for %s", route)
			}
		}
	}
}
