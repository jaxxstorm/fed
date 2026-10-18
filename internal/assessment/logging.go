package assessment

import (
	"context"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/logging"
)

func commitOperation(ctx context.Context, tx interface{ Commit(context.Context) error }, a auth.Actor, service, event string, records ...Record) error {
	f := logging.Fields{Event: "assessment." + event, Component: "assessment", ActorID: a.ID, ServiceID: service}
	if len(records) > 0 {
		f.RecordID = records[0].ID
		f.Revision = records[0].Revision
	}
	return logging.Commit(ctx, tx, f)
}
