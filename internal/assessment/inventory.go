package assessment

import (
	"context"
	"github.com/jaxxstorm/fed/internal/observation"
	"time"
)

// InventorySnapshot is emitted only by an identity-verified server collector.
// Membership is a list of stable target identities, never workload display names.
type InventorySnapshot struct {
	Partition       string               `json:"partition"`
	ConnectionID    string               `json:"connection_id"`
	Generation      int64                `json:"generation"`
	SourceID        string               `json:"source_id"`
	SourceRevision  int                  `json:"source_revision"`
	BindingRevision int                  `json:"binding_revision"`
	Complete        bool                 `json:"complete"`
	Targets         []observation.Target `json:"targets"`
}

type ResourceInventorySummary struct {
	CurrentResources  int       `json:"current_resources"`
	RetiredResources  int       `json:"retired_resources"`
	UnfencedResources int       `json:"unfenced_resources"`
	FirstObservedAt   time.Time `json:"first_observed_at"`
	LastObservedAt    time.Time `json:"last_observed_at"`
	Definition        string    `json:"definition"`
	Limitations       string    `json:"limitations"`
}

func (s *Store) inventorySummaries(ctx context.Context, service string) (map[string]ResourceInventorySummary, error) {
	rows, err := s.query().Query(ctx, `WITH latest AS (
 SELECT DISTINCT ON(l.requirement_id,o.target_key) l.requirement_id,o.*
 FROM observations o JOIN observation_links l ON l.service_id=o.service_id AND l.observation_id=o.id
 WHERE o.service_id=$1 AND o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope')
 AND o.profile_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile')
 AND o.data->'target'->>'kind'<>'collector_partition'
 ORDER BY l.requirement_id,o.target_key,o.observed_at DESC,o.received_at DESC
 ) SELECT l.requirement_id,
 count(*) FILTER(WHERE c.id IS NOT NULL),count(*) FILTER(WHERE c.id IS NULL),
 count(*) FILTER(WHERE c.id IS NOT NULL AND b.observation_id IS NULL),min(l.observed_at),max(l.observed_at)
 FROM latest l LEFT JOIN current_resource_observations c ON c.service_id=l.service_id AND c.id=l.id
 LEFT JOIN observation_inventory_bindings b ON b.service_id=l.service_id AND b.observation_id=l.id
 GROUP BY l.requirement_id`, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ResourceInventorySummary{}
	for rows.Next() {
		var id string
		var v ResourceInventorySummary
		if err = rows.Scan(&id, &v.CurrentResources, &v.RetiredResources, &v.UnfencedResources, &v.FirstObservedAt, &v.LastObservedAt); err != nil {
			return nil, err
		}
		v.Definition = "Distinct stable resource targets in the current scope/profile; collector partitions excluded. Retirement requires a newer complete compatible server inventory."
		v.Limitations = "Retained resources without inventory provenance remain uncertain. Partial discovery cannot prove absence. Counts do not establish full-boundary coverage or compliance."
		out[id] = v
	}
	return out, rows.Err()
}
