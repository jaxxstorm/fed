package connections

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type EnvironmentBindings struct {
	AWSProfiles        map[string]string
	KubernetesContexts map[string]string
}

func importedConnection(service string, source assessment.EvidenceSource, bindings EnvironmentBindings) (Connection, error) {
	c := Connection{ServiceID: service, Mode: AgentManaged, Lifecycle: Active, Revision: 1, Generation: 1, Imported: true}
	var group string
	switch source.Type {
	case "repository":
		parts := strings.Split(source.Repository, "/")
		if len(parts) != 2 || parts[0] == "" {
			return c, ErrInvalid
		}
		c.Provider, c.Name, c.Configuration.AccountID = GitHub, parts[0], parts[0]
		group = parts[0]
	case "aws":
		c.Provider, c.Name, c.Configuration.AccountID, c.Configuration.Region = AWS, source.AccountID, source.AccountID, source.Region
		c.Configuration.Profile = bindings.AWSProfiles[source.AccountID]
		if c.Configuration.Profile != "" {
			c.Mode = ServerEnvironment
		}
		group = source.AccountID
	case "kubernetes":
		group = source.AccountID + "/" + source.Region + "/" + source.Cluster
		c.Provider, c.Name, c.Configuration.AccountID, c.Configuration.ClusterID, c.Configuration.Region = Kubernetes, source.Cluster, group, group, source.Region
		c.Configuration.Context = bindings.KubernetesContexts[group]
		if c.Configuration.Context != "" {
			c.Mode = ServerEnvironment
		}
	case "document", "operational_records":
		u, err := url.Parse(source.Locator)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return c, ErrInvalid
		}
		c.Provider, c.Name, c.Configuration.AccountID, c.Mode = Documents, u.Host, u.Host, Manual
		group = u.Host
	default:
		return c, ErrInvalid
	}
	c.ID = "import-" + storage.Hash([]byte(string(c.Provider) + "/" + group))[:24]
	return c, nil
}

// ImportSources creates operational bindings without rewriting any existing
// source, evidence or export revision. Unknown provider IDs retain legacy keys
// until an explicit verified/attributed match replaces them.
func (s *Service) ImportSources(ctx context.Context, a auth.Actor, service string, bindings EnvironmentBindings) (int, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON (id) id,data FROM records r WHERE service_id=$1 AND kind='evidence_source' AND NOT EXISTS(SELECT 1 FROM connection_source_bindings b WHERE b.service_id=r.service_id AND b.source_id=r.id) ORDER BY id,revision DESC`, service)
	if err != nil {
		return 0, err
	}
	type sourceRecord struct {
		ID     string
		Source assessment.EvidenceSource
	}
	sources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sourceRecord, error) {
		var r sourceRecord
		var b []byte
		if err := row.Scan(&r.ID, &b); err != nil {
			return r, err
		}
		return r, json.Unmarshal(b, &r.Source)
	})
	if err != nil {
		return 0, err
	}
	for _, r := range sources {
		c, err := importedConnection(service, r.Source, bindings)
		if err != nil {
			return 0, err
		}
		c.ActorID, c.UpdatedAt = a.ID, time.Now().UTC()
		metadata, _ := json.Marshal(c)
		health, _ := json.Marshal(AccessHealth{State: NotConfigured, Generation: 1, Limitations: []string{"Imported configuration; provider access has not been tested."}})
		tag, err := tx.Exec(ctx, `INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,access_health,actor_id) VALUES($1,$2,$3,1,1,'active',$4,$5,$6) ON CONFLICT(service_id,id) DO NOTHING`, service, c.ID, c.Provider, metadata, health, a.ID)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 1 {
			_, err = tx.Exec(ctx, `INSERT INTO connection_revisions(service_id,connection_id,revision,data,actor_id) VALUES($1,$2,1,$3,$4)`, service, c.ID, metadata, a.ID)
			if err != nil {
				return 0, err
			}
			if err = audit(ctx, tx, a, c, "import"); err != nil {
				return 0, err
			}
		} else {
			existing, err := readConnection(ctx, tx, service, c.ID)
			if err != nil {
				return 0, err
			}
			if existing.Connection.Lifecycle != Active {
				return 0, ErrInactive
			}
		}
		state := Enabled
		if !r.Source.Enabled {
			state = Disabled
		}
		// Source IDs remain distinct even where old declarations overlap. No
		// external resource identity is inferred solely from a display name.
		_, err = tx.Exec(ctx, `INSERT INTO connection_source_bindings(service_id,source_id,connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,1,$9)`, service, r.ID, c.ID, c.Provider, c.Configuration.AccountID, r.Source.Type, "legacy:"+r.ID, state, a.ID)
		if err != nil {
			return 0, err
		}
	}
	return len(sources), tx.Commit(ctx)
}
