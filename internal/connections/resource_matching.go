package connections

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

// Upgrade a single unambiguous imported identity without changing its source
// revision or credential path. Retired bindings retain their tombstone state.
func matchImportedResource(ctx context.Context, tx pgx.Tx, a auth.Actor, service string, r DiscoveredResource, declaredTenant string) error {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings WHERE service_id=$1 AND provider=$2 AND tenant=$3 AND resource_kind=$4 AND resource_id=$5)`, service, r.Resource.Provider, r.Resource.Tenant, r.Resource.Kind, r.Resource.ID).Scan(&exists)
	if err != nil || exists {
		return err
	}
	sourceType := r.Resource.Kind
	switch r.Resource.Kind {
	case "account_region":
		sourceType = "aws"
	case "eks_cluster":
		sourceType = "kubernetes"
	}
	rows, err := tx.Query(ctx, `SELECT b.source_id FROM connection_source_bindings b JOIN LATERAL (SELECT data FROM records WHERE service_id=b.service_id AND kind='evidence_source' AND id=b.source_id ORDER BY revision DESC LIMIT 1) source ON true WHERE b.service_id=$1 AND b.provider=$2 AND b.tenant IN ($3,$8) AND b.resource_id LIKE 'legacy:%' AND source.data->>'type'=$4 AND source.data->>'locator'=$5 AND ($6<>'eks_cluster' OR source.data->>'cluster'=$7) ORDER BY b.source_id`, service, r.Resource.Provider, r.Resource.Tenant, sourceType, r.Locator, r.Resource.Kind, r.Name, declaredTenant)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(ids) != 1 {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE connection_source_bindings SET resource_kind=$3,resource_id=$4,revision=revision+1,actor_id=$5,tenant=$6,updated_at=now() WHERE service_id=$1 AND source_id=$2`, service, ids[0], r.Resource.Kind, r.Resource.ID, a.ID, r.Resource.Tenant)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'source.identity_matched',$4)`, a.OrganizationID, service, a.ID, ids[0])
	return err
}
