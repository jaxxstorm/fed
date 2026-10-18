package connections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

// ChangeLifecycle handles local state atomically. Provider-side revocation is a
// separate operation and can never roll back local disconnection/deletion.
func (s *Service) ChangeLifecycle(ctx context.Context, a auth.Actor, in OperationInput, operation string) (Detail, error) {
	if operation != "disconnect" && operation != "delete" && operation != "restore" {
		return Detail{}, ErrInvalid
	}
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 {
		return Detail{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return Detail{}, err
	}
	request, _ := json.Marshal(in)
	digest := storage.Hash(request)
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation=$3 AND key=$4`, a.ID, in.ServiceID, "connection."+operation, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return Detail{}, ErrConflict
		}
		var out Detail
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, commitConnection(ctx, tx, a, operation, out.Connection, "replayed")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM evidence_connections WHERE service_id=$1 AND id=$2 FOR UPDATE`, in.ServiceID, in.ConnectionID); err != nil {
		return Detail{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return Detail{}, err
	}
	if d.Connection.Generation != in.Generation {
		return Detail{}, ErrConflict
	}
	c, err := d.Connection.Transition(operation, in.ExpectedRevision, time.Now().UTC())
	if err != nil {
		return Detail{}, err
	}
	c.ActorID = a.ID
	if operation == "delete" {
		rows, err := tx.Query(ctx, `SELECT b.source_id, (SELECT max(revision) FROM records r WHERE r.service_id=b.service_id AND r.kind='evidence_source' AND r.id=b.source_id) FROM connection_source_bindings b WHERE b.service_id=$1 AND b.connection_id=$2 AND b.lifecycle <> 'retired' ORDER BY b.source_id`, in.ServiceID, in.ConnectionID)
		if err != nil {
			return Detail{}, err
		}
		type source struct {
			ID       string
			Revision int
		}
		sources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (source, error) {
			var v source
			err := row.Scan(&v.ID, &v.Revision)
			return v, err
		})
		if err != nil {
			return Detail{}, err
		}
		for _, v := range sources {
			if _, err = assessment.ChangeSourceLifecycleTx(ctx, tx, a, in.ServiceID, v.ID, v.Revision, "retired"); err != nil {
				return Detail{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE connection_source_bindings SET lifecycle='retired',revision=revision+1,actor_id=$3,updated_at=now() WHERE service_id=$1 AND connection_id=$2 AND lifecycle <> 'retired'`, in.ServiceID, in.ConnectionID, a.ID); err != nil {
			return Detail{}, err
		}
	}
	if err = s.Credentials.DeleteTx(ctx, tx, a, in.ServiceID, in.ConnectionID); err != nil {
		return Detail{}, err
	}
	out := Detail{Connection: c, Access: AccessHealth{State: AuthorizationRequired, Generation: c.Generation, Limitations: []string{"Local access state only; provider-side revocation has not been performed."}}}
	out.RemoteRevocation = "not_requested"
	metadata, _ := json.Marshal(c)
	health, _ := json.Marshal(out.Access)
	_, err = tx.Exec(ctx, `UPDATE evidence_connections SET revision=$3,generation=$4,lifecycle=$5,data=$6,access_health=$7,actor_id=$8,updated_at=now() WHERE service_id=$1 AND id=$2`, c.ServiceID, c.ID, c.Revision, c.Generation, c.Lifecycle, metadata, health, a.ID)
	if err != nil {
		return Detail{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_revisions(service_id,connection_id,revision,data,actor_id) VALUES($1,$2,$3,$4,$5)`, c.ServiceID, c.ID, c.Revision, metadata, a.ID)
	if err != nil {
		return Detail{}, err
	}
	if err = audit(ctx, tx, a, c, operation); err != nil {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE connection_operations SET state='cancelled',result='{"error_code":"connection_changed"}',updated_at=now() WHERE service_id=$1 AND connection_id=$2 AND state IN ('pending','running')`, c.ServiceID, c.ID); err != nil {
		return Detail{}, err
	}
	response, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, c.ServiceID, "connection."+operation, in.IdempotencyKey, digest, response)
	if err != nil {
		return Detail{}, err
	}
	return out, commitConnection(ctx, tx, a, operation, out.Connection, "")
}
