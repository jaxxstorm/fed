package assessment

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

// ChangeSourceLifecycleTx appends immutable source history while the caller
// holds the service and connection locks. Scope may have changed since the last
// source revision: retiring configuration must still be possible in that case.
func ChangeSourceLifecycleTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, id string, expected int, lifecycle string) (Record, error) {
	if err := auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return Record{}, err
	}
	if lifecycle != "enabled" && lifecycle != "disabled" && lifecycle != "retired" {
		return Record{}, ErrInvalid
	}
	previous, err := latest(ctx, tx, service, "evidence_source", id)
	if err != nil {
		return Record{}, err
	}
	if previous.Revision != expected {
		return Record{}, ErrConflict
	}
	v, err := Decode[EvidenceSource](previous)
	if err != nil {
		return Record{}, err
	}
	if lifecycle == "enabled" {
		scopeRecord, err := latest(ctx, tx, service, "scope", "scope")
		if err != nil {
			return Record{}, err
		}
		scope, err := Decode[Scope](scopeRecord)
		if err != nil {
			return Record{}, err
		}
		if !sourceInScope(v, scope) {
			return Record{}, ErrConflict
		}
		v.ScopeRevision = scopeRecord.Revision
	}
	v.Lifecycle, v.Enabled = lifecycle, lifecycle == "enabled"
	return insertRecord(ctx, tx, a, record(service, "evidence_source", id, "", v), previous.Revision+1)
}

// BindSourceTx changes only the explicit credential path of an existing source.
// The connection service verifies canonical resource matching before calling it.
func BindSourceTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, id string, expected int, connection, tenant, kind, resource string) (Record, error) {
	if err := auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return Record{}, err
	}
	previous, err := latest(ctx, tx, service, "evidence_source", id)
	if err != nil {
		return Record{}, err
	}
	if previous.Revision != expected {
		return Record{}, ErrConflict
	}
	v, err := Decode[EvidenceSource](previous)
	if err != nil {
		return Record{}, err
	}
	v.ConnectionID, v.ProviderTenant, v.ResourceKind, v.ResourceID = connection, tenant, kind, resource
	return insertRecord(ctx, tx, a, record(service, "evidence_source", id, "", v), previous.Revision+1)
}
