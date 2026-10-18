package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func requestLogging(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		ctx, id := logging.NewRequest(c.Request().Context())
		c.SetRequest(c.Request().WithContext(ctx))
		c.Response().Header().Set("X-Request-ID", id)
		route := c.Path()
		if route == "" {
			route = "unmatched"
		}
		logging.Route(ctx, route)
		start := time.Now()
		defer func() {
			if recover() != nil {
				logging.Emit(ctx, zerolog.ErrorLevel, logging.Fields{Event: "http.request.panic", Component: "http", ErrorCode: "handler_panic"})
				if !c.Response().Committed {
					c.Error(echo.NewHTTPError(500))
				}
				err = nil
			}
			if err != nil {
				c.Error(err)
				err = nil
			}
			status := c.Response().Status
			method := c.Request().Method
			switch method {
			case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
			default:
				method = "OTHER"
			}
			level := logging.Level(status)
			if status < 400 && (route == "/healthz" || route == "/readyz" || strings.HasPrefix(route, "/assets/")) {
				level = zerolog.DebugLevel
			}
			outcome := "success"
			if status >= 400 {
				outcome = "failed"
			}
			if ctx.Err() != nil {
				outcome = "cancelled"
			}
			logging.Emit(ctx, level, logging.Fields{Event: "http.request.completed", Component: "http", Method: method, Status: status, Duration: time.Since(start), Outcome: outcome})
			if route == "/mcp" && status >= 400 {
				logging.Emit(ctx, level, logging.Fields{Event: "mcp.transport.failed", Component: "mcp", Status: status, Outcome: "failed", ErrorCode: "transport_rejected"})
			}
		}()
		return next(c)
	}
}

// Resolve the inner ServeMux route before dispatch. No URL values or request
// body are copied, and the original writer (including Flusher) passes through.
func loggedMux(m *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := m.Handler(r)
		if pattern == "" {
			pattern = "unmatched"
		}
		logging.Route(r.Context(), pattern)
		m.ServeHTTP(w, r)
	})
}

func diagnostic(ctx context.Context, e UIError, status int) {
	logging.Emit(ctx, logging.Level(status), logging.Fields{Event: "browser.request.failed", Component: "web", Reference: e.Reference, ErrorCode: e.Code, Status: status, Outcome: "failed"})
}
