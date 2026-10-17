package connections

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type SourceOperationInput struct {
	ServiceID               string `json:"service_id"`
	SourceID                string `json:"source_id"`
	ExpectedRevision        int    `json:"expected_revision"`
	ExpectedBindingRevision int    `json:"expected_binding_revision"`
	IdempotencyKey          string `json:"idempotency_key"`
	// Rebind is explicit and retains the resource's canonical identity.
	TargetConnectionID string `json:"target_connection_id,omitempty"`
	TargetGeneration   int64  `json:"target_generation,omitempty"`
}

func (s *Service) ChangeSource(ctx context.Context, a auth.Actor, in SourceOperationInput, operation string) (assessment.Record, error) {
	if operation != "disable" && operation != "retire" && operation != "restore" && operation != "rebind" {
		return assessment.Record{}, ErrInvalid
	}
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || in.ExpectedRevision < 1 || in.ExpectedBindingRevision < 1 {
		return assessment.Record{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return assessment.Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return assessment.Record{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return assessment.Record{}, err
	}
	b, _ := json.Marshal(in)
	digest := storage.Hash(b)
	var previousHash string
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation=$3 AND key=$4`, a.ID, in.ServiceID, "source."+operation, in.IdempotencyKey).Scan(&previousHash, &previous)
	if err == nil {
		if previousHash != digest {
			return assessment.Record{}, ErrConflict
		}
		var out assessment.Record
		if err = json.Unmarshal(previous, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return assessment.Record{}, err
	}
	var binding SourceBinding
	err = tx.QueryRow(ctx, `SELECT connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2 FOR UPDATE`, in.ServiceID, in.SourceID).Scan(&binding.ConnectionID, &binding.Resource.Provider, &binding.Resource.Tenant, &binding.Resource.Kind, &binding.Resource.ID, &binding.Lifecycle, &binding.Revision)
	if err != nil {
		return assessment.Record{}, err
	}
	if binding.Revision != in.ExpectedBindingRevision {
		return assessment.Record{}, ErrConflict
	}
	var out assessment.Record
	if operation == "rebind" {
		if binding.Lifecycle == Retired || in.TargetConnectionID == "" || in.TargetConnectionID == binding.ConnectionID {
			return out, ErrInvalid
		}
		if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, in.TargetConnectionID, "connect", in.TargetGeneration); err != nil {
			return out, err
		}
		target, err := readConnection(ctx, tx, in.ServiceID, in.TargetConnectionID)
		if err != nil {
			return out, err
		}
		if target.Connection.Provider != binding.Resource.Provider || target.Connection.Tenant() != binding.Resource.Tenant {
			return out, ErrInvalid
		}
		var found bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_inventory WHERE service_id=$1 AND connection_id=$2 AND tenant=$3 AND resource_kind=$4 AND resource_id=$5 AND available)`, in.ServiceID, in.TargetConnectionID, binding.Resource.Tenant, binding.Resource.Kind, binding.Resource.ID).Scan(&found)
		if err != nil {
			return out, err
		}
		if !found {
			return out, ErrInvalid
		}
		out, err = assessment.BindSourceTx(ctx, tx, a, in.ServiceID, in.SourceID, in.ExpectedRevision, in.TargetConnectionID, binding.Resource.Tenant, binding.Resource.Kind, binding.Resource.ID)
		binding.ConnectionID = in.TargetConnectionID
	} else {
		state := Disabled
		if operation == "retire" {
			state = Retired
		}
		if operation == "restore" {
			if binding.Lifecycle != Retired && binding.Lifecycle != Disabled {
				return out, ErrInvalid
			}
			connection, err := readConnection(ctx, tx, in.ServiceID, binding.ConnectionID)
			if err != nil {
				return out, err
			}
			if connection.Connection.Lifecycle != Active {
				return out, ErrInactive
			}
			state = Enabled
		} else if binding.Lifecycle == Retired {
			return out, ErrInactive
		}
		out, err = assessment.ChangeSourceLifecycleTx(ctx, tx, a, in.ServiceID, in.SourceID, in.ExpectedRevision, string(state))
		binding.Lifecycle = state
	}
	if err != nil {
		return out, err
	}
	_, err = tx.Exec(ctx, `UPDATE connection_source_bindings SET connection_id=$3,lifecycle=$4,revision=revision+1,actor_id=$5,updated_at=now() WHERE service_id=$1 AND source_id=$2`, in.ServiceID, in.SourceID, binding.ConnectionID, binding.Lifecycle, a.ID)
	if err != nil {
		return out, err
	}
	b, _ = json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, in.ServiceID, "source."+operation, in.IdempotencyKey, digest, b)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
