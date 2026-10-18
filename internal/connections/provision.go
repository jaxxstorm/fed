package connections

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

// ProvisionKubernetes is an operator-only credential import, not an API action.
// Ownership of the connection table and a live administrator are both required.
// Credentials are verified before any change, encrypted at rest, and never put
// into connection metadata, audit payloads or idempotency responses.
func (s *Service) ProvisionKubernetes(ctx context.Context, a auth.Actor, service, name, cluster, server, kubeconfig string) (Detail, error) {
	var owner bool
	if err := s.DB.QueryRow(ctx, `SELECT pg_get_userbyid(relowner)=current_user FROM pg_class WHERE oid='evidence_connections'::regclass`).Scan(&owner); err != nil || !owner {
		return Detail{}, auth.ErrForbidden
	}
	if err := auth.Authorize(ctx, s.DB, a, service, "admin"); err != nil {
		return Detail{}, err
	}
	c := Connection{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte("fed:operator:kubernetes:"+service+":"+cluster)).String(), ServiceID: service, Provider: Kubernetes, Name: name, Mode: ServerManaged, Configuration: Configuration{AccountID: cluster, ClusterID: cluster, Server: server}, Lifecycle: Active, Revision: 1, Generation: 1, ActorID: a.ID, Imported: true}
	if cluster == "" || server == "" || len(kubeconfig) > MaxResponseBytes {
		return Detail{}, ErrInvalid
	}
	if err := validateConfiguration(s.Registry, ConfigureInput{ServiceID: service, Provider: Kubernetes, Name: name, Mode: ServerManaged, Configuration: c.Configuration, IdempotencyKey: "operator"}); err != nil {
		return Detail{}, err
	}
	adapter, err := s.Registry.Adapter(Kubernetes)
	if err != nil {
		return Detail{}, err
	}
	credentials := NewCredentials(map[string]string{"kubeconfig": kubeconfig})
	health, err := adapter.Test(ctx, c, credentials)
	if err != nil || health.State != Verified || health.VerifiedAccountID != cluster {
		return Detail{}, ErrCredentialUnavailable
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "admin"); err != nil {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM evidence_connections WHERE service_id=$1 AND id=$2 FOR UPDATE`, service, c.ID); err != nil {
		return Detail{}, err
	}
	previous, err := readConnection(ctx, tx, service, c.ID)
	if err == nil {
		p := previous.Connection
		if !p.Imported || p.Provider != Kubernetes || p.Mode != ServerManaged || !reflect.DeepEqual(p.Configuration, c.Configuration) {
			return Detail{}, ErrConflict
		}
		if p.Lifecycle != Active {
			return Detail{}, ErrInactive
		}
		old, readErr := s.Credentials.readTx(ctx, tx, a, service, c.ID, "connect", p.Generation)
		if readErr != nil {
			return Detail{}, readErr
		}
		if old.Value("kubeconfig") == kubeconfig && p.Name == name {
			return previous, tx.Commit(ctx)
		}
		c.Revision, c.Generation = p.Revision+1, p.Generation+1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, err
	}
	now := time.Now().UTC()
	until := now.Add(15 * time.Minute)
	c.UpdatedAt, c.VerifiedAccountID = now, cluster
	health.ActorID, health.Generation, health.CheckedAt, health.ValidUntil = a.ID, c.Generation, &now, &until
	metadata, _ := json.Marshal(c)
	access, _ := json.Marshal(health)
	_, err = tx.Exec(ctx, `INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,access_health,actor_id,verified_account_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(service_id,id) DO UPDATE SET revision=excluded.revision,generation=excluded.generation,data=excluded.data,access_health=excluded.access_health,actor_id=excluded.actor_id,verified_account_id=excluded.verified_account_id,updated_at=now()`, service, c.ID, c.Provider, c.Revision, c.Generation, c.Lifecycle, metadata, access, a.ID, cluster)
	if err != nil {
		return Detail{}, err
	}
	if err = s.Credentials.PutTx(ctx, tx, a, service, c.ID, c.Generation, credentials); err != nil {
		return Detail{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO connection_revisions(service_id,connection_id,revision,data,actor_id) VALUES($1,$2,$3,$4,$5)`, service, c.ID, c.Revision, metadata, a.ID); err != nil {
		return Detail{}, err
	}
	if err = audit(ctx, tx, a, c, "operator_provisioned"); err != nil {
		return Detail{}, err
	}
	return Detail{Connection: c, Access: health}, commitConnection(ctx, tx, a, "operator_provisioned", c, "")
}
