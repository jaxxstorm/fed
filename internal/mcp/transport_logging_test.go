package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/logging"
)

func TestProtocolObservationPreservesBodiesAndFlushing(t *testing.T) {
	for _, test := range []struct {
		name, content, body string
		event               string
	}{
		{"error", "application/json", `{"id":"secret-id","error":{"code":-32602,"message":"secret-message"}}`, "mcp.protocol.failed"},
		{"large", "application/json", `{"result":"` + strings.Repeat("secret-body", 7000) + `"}`, "mcp.response.uninspected"},
		{"stream", "text/event-stream", "data: secret-stream\n\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			l, _ := logging.New(&logs, nil, "debug", "json", "server")
			h := observeProtocolErrors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.content)
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				_, _ = w.Write([]byte(test.body))
			}))
			req := httptest.NewRequest("POST", "/mcp", nil).WithContext(logging.WithLogger(context.Background(), l))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if !w.Flushed || w.Body.String() != test.body || w.Code != 200 {
				t.Fatal("response changed")
			}
			if strings.Contains(logs.String(), "secret") {
				t.Fatal("body leaked")
			}
			if test.event != "" && !strings.Contains(logs.String(), test.event) {
				t.Fatal("missing event", logs.String())
			}
			if test.event == "" && logs.Len() != 0 {
				t.Fatal("SSE inspected")
			}
		})
	}
}
