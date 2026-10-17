package connections

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
)

type CollectionAccess struct {
	Connection      Connection
	Source          assessment.EvidenceSource
	SourceID        string
	SourceRevision  int
	BindingRevision int
	Resource        ResourceKey
	Credentials     Credentials
}

// CollectorAccess resolves even disabled/retired bindings: bound failures must
// never silently fall through to a global provider credential.
func (s *Service) CollectorAccess(ctx context.Context, a auth.Actor, service, sourceType, locator string, cluster ...string) (CollectionAccess, bool, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "check"); err != nil {
		return CollectionAccess{}, false, err
	}
	clusterName := ""
	if len(cluster) > 0 {
		clusterName = cluster[0]
	}
	rows, err := s.DB.Query(ctx, `SELECT b.source_id,b.connection_id,b.provider,b.tenant,b.resource_kind,b.resource_id,b.revision,b.lifecycle,r.revision,r.data FROM connection_source_bindings b JOIN LATERAL (SELECT revision,data FROM records WHERE service_id=b.service_id AND kind='evidence_source' AND id=b.source_id ORDER BY revision DESC LIMIT 1) r ON true WHERE b.service_id=$1 AND r.data->>'type'=$2 AND r.data->>'locator'=$3 AND ($4='' OR r.data->>'cluster'=$4)`, service, sourceType, locator, clusterName)
	if err != nil {
		return CollectionAccess{}, false, err
	}
	type binding struct {
		Access    CollectionAccess
		Lifecycle string
	}
	bindings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (binding, error) {
		var out binding
		var raw []byte
		err := row.Scan(&out.Access.SourceID, &out.Access.Connection.ID, &out.Access.Resource.Provider, &out.Access.Resource.Tenant, &out.Access.Resource.Kind, &out.Access.Resource.ID, &out.Access.BindingRevision, &out.Lifecycle, &out.Access.SourceRevision, &raw)
		if err == nil {
			err = json.Unmarshal(raw, &out.Access.Source)
		}
		return out, err
	})
	if err != nil {
		return CollectionAccess{}, false, err
	}
	if len(bindings) == 0 {
		return CollectionAccess{}, false, nil
	}
	if len(bindings) != 1 {
		return CollectionAccess{}, true, ErrConflict
	}
	out := bindings[0].Access
	if bindings[0].Lifecycle != string(Enabled) || !out.Source.Enabled {
		return out, true, ErrInactive
	}
	d, err := readConnection(ctx, s.DB, service, out.Connection.ID)
	if err != nil {
		return out, true, err
	}
	out.Connection = d.Connection
	if d.Connection.Lifecycle != Active {
		return out, true, ErrInactive
	}
	if d.Connection.Mode != ServerManaged && d.Connection.Mode != ServerEnvironment {
		return out, true, ErrCredentialUnavailable
	}
	if d.Connection.Mode == ServerManaged {
		out.Credentials, err = s.Credentials.Get(ctx, a, service, d.Connection.ID, "check", d.Connection.Generation)
		if err != nil {
			return out, true, err
		}
	}
	return out, true, nil
}

// CheckCollectionTx is called while the service lock is held at result commit.
func CheckCollectionTx(ctx context.Context, tx pgx.Tx, service string, access CollectionAccess) error {
	var valid bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings b JOIN evidence_connections c ON c.service_id=b.service_id AND c.id=b.connection_id WHERE b.service_id=$1 AND b.source_id=$2 AND b.connection_id=$3 AND b.revision=$4 AND b.lifecycle='enabled' AND c.lifecycle='active' AND c.generation=$5 AND (SELECT max(revision) FROM records r WHERE r.service_id=b.service_id AND r.kind='evidence_source' AND r.id=b.source_id)=$6)`, service, access.SourceID, access.Connection.ID, access.BindingRevision, access.Connection.Generation, access.SourceRevision).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return assessment.ErrConflict
	}
	return nil
}

func CollectionError(err error) string {
	if errors.Is(err, ErrInactive) {
		return "connection_inactive"
	}
	if errors.Is(err, ErrCredentialUnavailable) {
		return "connection_credentials_unavailable"
	}
	if errors.Is(err, ErrConflict) {
		return "connection_binding_conflict"
	}
	return "connection_access_failed"
}
