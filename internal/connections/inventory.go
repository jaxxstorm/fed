package connections

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type InventoryQuery struct {
	ServiceID    string `json:"service_id"`
	ConnectionID string `json:"connection_id"`
	Search       string `json:"search"`
	Kind         string `json:"kind"`
	Selection    string `json:"selection"`
	Limit        int    `json:"limit"`
	Cursor       string `json:"cursor"`
}

type InventoryResource struct {
	SourceRevision  int `json:"source_revision"`
	BindingRevision int `json:"binding_revision"`
	DiscoveredResource
	SourceLifecycle   SourceLifecycle `json:"source_lifecycle,omitempty"`
	BoundConnectionID string          `json:"bound_connection_id,omitempty"`
	ActorID           string          `json:"actor_id"`
	Mode              AccessMode      `json:"access_mode"`
	SessionID         string          `json:"session_id"`
}

type InventoryPage struct {
	Resources  []InventoryResource `json:"resources"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Generation int64               `json:"generation"`
}

func (s *Service) ListInventory(ctx context.Context, a auth.Actor, in InventoryQuery) (InventoryPage, error) {
	limit, err := PageSize(in.Limit)
	if err != nil || len(in.Search) > 256 || !slices.Contains([]string{"", "all", "selected", "unselected", "retired"}, in.Selection) {
		return InventoryPage{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return InventoryPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "read"); err != nil {
		return InventoryPage{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return InventoryPage{}, err
	}
	definition, _ := s.Registry.Provider(d.Connection.Provider)
	if in.Kind != "" && !slices.Contains(definition.ResourceKinds, in.Kind) {
		return InventoryPage{}, ErrInvalid
	}
	cursor := in.Cursor
	in.Cursor = ""
	in.Limit = limit
	query, _ := json.Marshal(in)
	digest := storage.Hash(query)
	afterKind, afterID := "", ""
	if cursor != "" {
		err = tx.QueryRow(ctx, `SELECT after_kind,after_id FROM connection_inventory_cursors WHERE id=$1 AND service_id=$2 AND connection_id=$3 AND generation=$4 AND query_hash=$5 AND expires_at>now()`, cursor, in.ServiceID, in.ConnectionID, d.Connection.Generation, digest).Scan(&afterKind, &afterID)
		if err != nil {
			return InventoryPage{}, ErrConflict
		}
	}
	rows, err := tx.Query(ctx, `SELECT i.data,i.available,i.observed_at,COALESCE(b.source_id,''),COALESCE(b.lifecycle,''),COALESCE(b.connection_id,''),session.actor_id,session.mode,i.session_id,COALESCE(b.revision,0),COALESCE((SELECT max(revision) FROM records WHERE service_id=b.service_id AND kind='evidence_source' AND id=b.source_id),0)
FROM connection_inventory i JOIN connection_discovery_sessions session ON session.service_id=i.service_id AND session.id=i.session_id
LEFT JOIN connection_source_bindings b ON b.service_id=i.service_id AND b.provider=$3 AND b.tenant=i.tenant AND b.resource_kind=i.resource_kind AND b.resource_id=i.resource_id
WHERE i.service_id=$1 AND i.connection_id=$2 AND ($4='' OR i.resource_kind=$4) AND ($5='' OR position(lower($5) in lower(i.data->>'name'))>0)
AND ($6 IN ('','all') OR ($6='selected' AND b.source_id IS NOT NULL AND b.lifecycle<>'retired') OR ($6='unselected' AND b.source_id IS NULL) OR ($6='retired' AND b.lifecycle='retired'))
AND (i.resource_kind,i.resource_id)>($7,$8) ORDER BY i.resource_kind,i.resource_id LIMIT $9`, in.ServiceID, in.ConnectionID, d.Connection.Provider, in.Kind, in.Search, in.Selection, afterKind, afterID, limit+1)
	if err != nil {
		return InventoryPage{}, err
	}
	resources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (InventoryResource, error) {
		var out InventoryResource
		var raw []byte
		var available bool
		var observed time.Time
		err := row.Scan(&raw, &available, &observed, &out.SourceID, &out.SourceLifecycle, &out.BoundConnectionID, &out.ActorID, &out.Mode, &out.SessionID, &out.BindingRevision, &out.SourceRevision)
		if err != nil {
			return out, err
		}
		sourceID := out.SourceID
		if err = json.Unmarshal(raw, &out.DiscoveredResource); err != nil {
			return out, err
		}
		out.SourceID, out.Available, out.LastSeenAt = sourceID, available, observed
		return out, nil
	})
	if err != nil {
		return InventoryPage{}, err
	}
	out := InventoryPage{Resources: resources, Generation: d.Connection.Generation}
	if len(resources) > limit {
		out.Resources = resources[:limit]
		last := out.Resources[limit-1].Resource
		out.NextCursor = uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO connection_inventory_cursors(id,service_id,connection_id,generation,query_hash,after_kind,after_id,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, out.NextCursor, in.ServiceID, in.ConnectionID, d.Connection.Generation, digest, last.Kind, last.ID, time.Now().Add(time.Hour))
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}
