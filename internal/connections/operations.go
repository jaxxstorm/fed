package connections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type OperationInput struct {
	ServiceID        string `json:"service_id"`
	ConnectionID     string `json:"connection_id"`
	ExpectedRevision int    `json:"expected_revision"`
	Generation       int64  `json:"generation"`
	IdempotencyKey   string `json:"idempotency_key"`
}
type Operation struct {
	ID           string        `json:"id"`
	ConnectionID string        `json:"connection_id"`
	State        string        `json:"state"`
	Access       *AccessHealth `json:"access,omitempty"`
	ErrorCode    string        `json:"error_code,omitempty"`
}

func (s *Service) GetOperation(ctx context.Context, a auth.Actor, service, id string) (Operation, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return Operation{}, err
	}
	_, err := s.DB.Exec(ctx, `UPDATE connection_operations o SET state='expired',result='{"error_code":"authorization_expired"}',updated_at=now() WHERE o.service_id=$1 AND o.id=$2 AND o.state IN ('pending','running') AND EXISTS(SELECT 1 FROM connection_authorizations a WHERE a.service_id=o.service_id AND a.operation_id=o.id AND a.expires_at<=now())`, service, id)
	if err != nil {
		return Operation{}, err
	}
	return readOperation(ctx, s.DB, service, id)
}
func readOperation(ctx context.Context, q auth.Querier, service, id string) (Operation, error) {
	var out Operation
	var b []byte
	err := q.QueryRow(ctx, `SELECT id,connection_id,state,result FROM connection_operations WHERE service_id=$1 AND id=$2`, service, id).Scan(&out.ID, &out.ConnectionID, &out.State, &b)
	if err != nil {
		return out, err
	}
	var result Operation
	if err = json.Unmarshal(b, &result); err != nil {
		return out, err
	}
	out.Access, out.ErrorCode = result.Access, result.ErrorCode
	return out, nil
}

// TestWithReceipt is the transport-facing access test. A duplicate key returns
// the durable operation, including a running result, without repeating a read.
func (s *Service) TestWithReceipt(ctx context.Context, a auth.Actor, in OperationInput) (Operation, error) {
	return s.runOperation(ctx, a, in, "test", func() (AccessHealth, error) { return s.Test(ctx, a, in.ServiceID, in.ConnectionID, in.Generation) })
}

func (s *Service) runOperation(ctx context.Context, a auth.Actor, in OperationInput, operation string, run func() (AccessHealth, error)) (Operation, error) {
	if in.ExpectedRevision < 1 || in.Generation < 1 || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 256 {
		return Operation{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Operation{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return Operation{}, err
	}
	request, _ := json.Marshal(in)
	digest := storage.Hash(request)
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation=$3 AND key=$4`, a.ID, in.ServiceID, "connection."+operation, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return Operation{}, ErrConflict
		}
		var receipt Operation
		if err = json.Unmarshal(prior, &receipt); err != nil {
			return Operation{}, err
		}
		out, err := readOperation(ctx, tx, in.ServiceID, receipt.ID)
		if err != nil {
			return Operation{}, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Operation{}, err
	}
	if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation); err != nil {
		return Operation{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return Operation{}, err
	}
	if d.Connection.Revision != in.ExpectedRevision {
		return Operation{}, ErrConflict
	}
	out := Operation{ID: uuid.NewString(), ConnectionID: in.ConnectionID, State: "running"}
	var tokenHash any
	if a.TokenHash != "" {
		tokenHash = a.TokenHash
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_operations(service_id,connection_id,id,generation,actor_id,token_hash,operation,state) VALUES($1,$2,$3,$4,$5,$6,$7,'running')`, in.ServiceID, in.ConnectionID, out.ID, in.Generation, a.ID, tokenHash, operation)
	if err != nil {
		return Operation{}, err
	}
	b, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, in.ServiceID, "connection."+operation, in.IdempotencyKey, digest, b)
	if err != nil {
		return Operation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Operation{}, err
	}
	health, err := run()
	if err != nil {
		out.State, out.ErrorCode = "failed", "connection_operation_failed"
		var recovery interface{ RecoveryCode() string }
		if errors.As(err, &recovery) {
			out.ErrorCode = recovery.RecoveryCode()
		}
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrInactive) {
			out.ErrorCode = "connection_changed"
		}
		if errors.Is(err, auth.ErrForbidden) {
			out.ErrorCode = "authorization_revoked"
		}
	} else {
		out.Access = &health
		out.State = "succeeded"
		if health.State != Verified {
			out.State = "partial"
		}
	}
	b, _ = json.Marshal(out)
	// Persist terminal failure even if the request was cancelled. This internal
	// receipt update cannot grant access or change connection/source state.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, saveErr := s.DB.Exec(finish, `UPDATE connection_operations SET state=$3,result=$4,updated_at=now() WHERE service_id=$1 AND id=$2 AND state='running'`, in.ServiceID, out.ID, out.State, b)
	if saveErr != nil {
		return Operation{}, saveErr
	}
	return readOperation(finish, s.DB, in.ServiceID, out.ID)
}
