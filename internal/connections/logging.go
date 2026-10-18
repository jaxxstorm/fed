package connections

import (
	"context"
	"errors"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
	"time"
)

func commitConnection(ctx context.Context, tx interface{ Commit(context.Context) error }, a auth.Actor, event string, c Connection, outcome string) error {
	return logging.Commit(ctx, tx, logging.Fields{Event: "connection." + event, Component: "connections", ActorID: a.ID, ServiceID: c.ServiceID, ConnectionID: c.ID, Provider: string(c.Provider), Generation: c.Generation, Revision: c.Revision, Outcome: outcome})
}

func providerResult(ctx context.Context, c Connection, operation string, start time.Time, err error, partial bool) {
	level, code, outcome := zerolog.DebugLevel, "", "success"
	if partial {
		level = zerolog.WarnLevel
		outcome = "partial"
	}
	if err != nil {
		level = zerolog.WarnLevel
		outcome = "failed"
		code = logging.Code(err)
		var recovery interface{ RecoveryCode() string }
		if errors.As(err, &recovery) {
			code = recovery.RecoveryCode()
		}
	}
	logging.Emit(ctx, level, logging.Fields{Event: "provider.operation.completed", Component: "provider", ServiceID: c.ServiceID, ConnectionID: c.ID, Provider: string(c.Provider), Stage: operation, Duration: time.Since(start), Outcome: outcome, ErrorCode: code, Generation: c.Generation})
}
func commitSetup(ctx context.Context, tx interface{ Commit(context.Context) error }, a auth.Actor, event string, s Setup, outcome string) error {
	return logging.Commit(ctx, tx, logging.Fields{Event: "connection.setup." + event, Component: "connections", ActorID: a.ID, ServiceID: s.ServiceID, SetupID: s.ID, ConnectionID: s.ConnectionID, Provider: string(s.Provider), Revision: s.Revision, Stage: string(s.Step), Outcome: outcome, ErrorCode: s.ErrorCode})
}
