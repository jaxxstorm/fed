package providers

import (
	"context"
	"errors"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
	"time"
)

func observeOperation(ctx context.Context, c connections.Connection, operation string) func(error) {
	start := time.Now()
	return func(err error) {
		level, outcome, code := zerolog.DebugLevel, "success", ""
		if err != nil {
			level = zerolog.WarnLevel
			outcome = "failed"
			code = logging.Code(err)
			var recovery interface{ RecoveryCode() string }
			if errors.As(err, &recovery) {
				code = recovery.RecoveryCode()
			}
		}
		logging.Emit(ctx, level, logging.Fields{Event: "provider.operation.completed", Component: "provider", Provider: string(c.Provider), ServiceID: c.ServiceID, ConnectionID: c.ID, Stage: operation, Duration: time.Since(start), Outcome: outcome, ErrorCode: code, Generation: c.Generation})
	}
}
