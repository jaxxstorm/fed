package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
)

// The SDK rejects malformed params before receiving middleware. Observe only
// the JSON error envelope, never its contents in logs, and never delay writes.
func observeProtocolErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const limit = 64 << 10
		var prefix bytes.Buffer
		oversized := false
		wrapped := httpsnoop.Wrap(w, httpsnoop.Hooks{Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(p []byte) (int, error) {
				n, err := next(p)
				if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") && !oversized {
					if prefix.Len()+n > limit {
						oversized = true
						prefix.Reset()
					} else {
						_, _ = prefix.Write(p[:n])
					}
				}
				return n, err
			}
		}})
		next.ServeHTTP(wrapped, r)
		if oversized {
			logging.Emit(r.Context(), zerolog.DebugLevel, logging.Fields{Event: "mcp.response.uninspected", Component: "mcp", Outcome: "size_limit"})
			return
		}
		var envelope struct {
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if prefix.Len() > 0 && json.Unmarshal(prefix.Bytes(), &envelope) == nil && envelope.Error != nil {
			logging.Emit(r.Context(), zerolog.WarnLevel, logging.Fields{Event: "mcp.protocol.failed", Component: "mcp", Outcome: "failed", ErrorCode: "protocol_error", ProtocolCode: envelope.Error.Code})
		}
	})
}
