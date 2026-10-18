package connections

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func (s *Service) Renew(ctx context.Context, a auth.Actor, in OperationInput) (Operation, error) {
	return s.runOperation(ctx, a, in, "renew", func() (AccessHealth, error) {
		release, err := s.lockRenewal(ctx, in.ServiceID, in.ConnectionID)
		if err != nil {
			return AccessHealth{}, err
		}
		defer release()
		d, err := readConnection(ctx, s.DB, in.ServiceID, in.ConnectionID)
		if err != nil {
			return AccessHealth{}, err
		}
		if err = d.Connection.CheckGeneration(in.Generation); err != nil {
			return AccessHealth{}, err
		}
		if d.Connection.Mode != ServerManaged {
			guide, _ := s.Registry.Provider(d.Connection.Provider)
			return AccessHealth{State: AuthorizationRequired, Generation: in.Generation, ActorID: a.ID, Limitations: append([]string{"Renew access in the indicated execution environment, then test or submit attributed discovery."}, guide.Guide.Steps...)}, nil
		}
		credentials, err := s.Credentials.Get(ctx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation)
		if err != nil {
			return AccessHealth{}, err
		}
		adapter, err := s.Registry.Adapter(d.Connection.Provider)
		if err != nil {
			return AccessHealth{}, err
		}
		call, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		renewed, err := adapter.Renew(call, d.Connection, credentials)
		if err != nil {
			return AccessHealth{}, err
		}
		// OAuth responses commonly omit an unchanged offline refresh token.
		if renewed.Value("refresh_token") == "" && credentials.Value("refresh_token") != "" {
			if renewed.values == nil {
				renewed.values = map[string]string{}
			}
			renewed.values["refresh_token"] = credentials.Value("refresh_token")
		}
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			return AccessHealth{}, err
		}
		defer tx.Rollback(ctx)
		if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation); err != nil {
			return AccessHealth{}, err
		}
		current, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
		if err != nil {
			return AccessHealth{}, err
		}
		c, err := current.Connection.Transition("renew", in.ExpectedRevision, time.Now().UTC())
		if err != nil {
			return AccessHealth{}, err
		}
		health := AccessHealth{State: NotConfigured, Generation: c.Generation, ActorID: a.ID, Limitations: []string{"Credentials renewed; test resource access. Existing evidence freshness is unchanged."}}
		b, _ := json.Marshal(health)
		if _, err = tx.Exec(ctx, `UPDATE evidence_connections SET generation=$3,access_health=$4,updated_at=now() WHERE service_id=$1 AND id=$2`, in.ServiceID, in.ConnectionID, c.Generation, b); err != nil {
			return AccessHealth{}, err
		}
		if err = s.Credentials.PutTx(ctx, tx, a, in.ServiceID, in.ConnectionID, c.Generation, renewed); err != nil {
			return AccessHealth{}, err
		}
		if err = audit(ctx, tx, a, c, "renew"); err != nil {
			return AccessHealth{}, err
		}
		return health, commitConnection(ctx, tx, a, "credentials.renewed", c, "")
	})
}

// A session-level lock spans the remote refresh without holding a transaction.
// Concurrent instances cannot both rotate a single-use provider refresh token.
func (s *Service) lockRenewal(ctx context.Context, service, connection string) (func(), error) {
	conn, err := s.DB.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key, _ := json.Marshal([]string{service, connection})
	var acquired bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,817039))`, string(key)).Scan(&acquired)
	if err != nil || !acquired {
		conn.Release()
		if err != nil {
			return nil, err
		}
		return nil, ErrConflict
	}
	return func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.QueryRow(cleanup, `SELECT pg_advisory_unlock(hashtextextended($1,817039))`, string(key)).Scan(&unlocked)
		if err != nil || !unlocked {
			_ = conn.Conn().Close(cleanup)
		}
		conn.Release()
	}, nil
}
