package mcp

import (
	"context"
	"time"

	"github.com/jaxxstorm/fed/internal/logging"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

func methodLogging(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (result sdk.Result, err error) {
		// Middleware receives only methods registered with the SDK, never arbitrary
		// unknown method strings. The transport reports those as HTTP rejections.
		ctx = logging.Operation(ctx)
		start := time.Now()
		logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "mcp.method.started", Component: "mcp", Method: method})
		defer func() {
			outcome, code, level := "success", "", zerolog.InfoLevel
			if err != nil {
				outcome = "failed"
				code = "protocol_error"
				level = zerolog.WarnLevel
			}
			if v, ok := result.(*sdk.CallToolResult); ok && v.IsError {
				outcome = "failed"
				code = "tool_error"
				level = zerolog.WarnLevel
			}
			logging.Emit(ctx, level, logging.Fields{Event: "mcp.method.completed", Component: "mcp", Method: method, Duration: time.Since(start), Outcome: outcome, ErrorCode: code})
		}()
		return next(ctx, method, req)
	}
}
