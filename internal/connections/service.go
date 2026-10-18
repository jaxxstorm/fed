package connections

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type Service struct {
	SetupReadiness map[Provider]SetupReadiness
	Artifacts      storage.Artifacts
	DB             *pgxpool.Pool
	Registry       *Registry
	Credentials    CredentialStore
}

type ConfigureInput struct {
	ServiceID        string        `json:"service_id"`
	ID               string        `json:"id"`
	ExpectedRevision int           `json:"expected_revision"`
	IdempotencyKey   string        `json:"idempotency_key"`
	Provider         Provider      `json:"provider"`
	Name             string        `json:"name"`
	Mode             AccessMode    `json:"access_mode"`
	Configuration    Configuration `json:"configuration"`
}

type Detail struct {
	DiscoveredCount  int          `json:"discovered_count"`
	SelectedCount    int          `json:"selected_count"`
	Connection       Connection   `json:"connection"`
	Access           AccessHealth `json:"access"`
	RemoteRevocation string       `json:"remote_revocation,omitempty"`
}

func validateConfiguration(r *Registry, in ConfigureInput) error {
	d, ok := r.Provider(in.Provider)
	if !ok || !slices.Contains(d.AccessModes, in.Mode) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 256 || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 256 || in.ExpectedRevision < 0 {
		return ErrInvalid
	}
	c := in.Configuration
	if len(c.KnownNamespaces) > 100 {
		return ErrInvalid
	}
	for _, namespace := range c.KnownNamespaces {
		if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`).MatchString(namespace) {
			return ErrInvalid
		}
	}
	if c.AccountID == "" || len(c.AccountID) > 256 {
		return ErrInvalid
	}
	if in.Provider == AWS {
		if !regexp.MustCompile(`^\d{12}$`).MatchString(c.AccountID) || c.Region != "us-east-1" {
			return ErrInvalid
		}
		if c.RoleARN != "" && !regexp.MustCompile(`^arn:aws:iam::`+c.AccountID+`:role/[A-Za-z0-9+=,.@_/-]+$`).MatchString(c.RoleARN) {
			return ErrInvalid
		}
	}
	for _, symbolic := range []string{c.Profile, c.Context, c.ClusterID, c.InstallationID} {
		if len(symbolic) > 256 || strings.ContainsAny(symbolic, "\r\n\x00") {
			return ErrInvalid
		}
	}
	return nil
}

func readConnection(ctx context.Context, q auth.Querier, service, id string) (Detail, error) {
	var out Detail
	var metadata, health []byte
	var strategy AccessStrategy
	err := q.QueryRow(ctx, `SELECT data,access_health,revision,generation,lifecycle,updated_at,verified_account_id,access_strategy FROM evidence_connections WHERE service_id=$1 AND id=$2`, service, id).Scan(&metadata, &health, &out.Connection.Revision, &out.Connection.Generation, &out.Connection.Lifecycle, &out.Connection.UpdatedAt, &out.Connection.VerifiedAccountID, &strategy)
	if err != nil {
		return out, err
	}
	revision, generation, lifecycle := out.Connection.Revision, out.Connection.Generation, out.Connection.Lifecycle
	updatedAt := out.Connection.UpdatedAt
	verifiedAccount := out.Connection.VerifiedAccountID
	if err = json.Unmarshal(metadata, &out.Connection); err != nil {
		return out, err
	}
	out.Connection.Revision, out.Connection.Generation, out.Connection.Lifecycle = revision, generation, lifecycle
	out.Connection.UpdatedAt = updatedAt
	out.Connection.VerifiedAccountID = verifiedAccount
	out.Connection.Strategy = strategy
	err = json.Unmarshal(health, &out.Access)
	if err == nil {
		err = q.QueryRow(ctx, `SELECT (SELECT count(*) FROM connection_inventory WHERE service_id=$1 AND connection_id=$2 AND available),(SELECT count(*) FROM connection_source_bindings WHERE service_id=$1 AND connection_id=$2 AND lifecycle<>'retired')`, service, id).Scan(&out.DiscoveredCount, &out.SelectedCount)
	}
	return out, err
}

func (s *Service) Get(ctx context.Context, a auth.Actor, service, id string) (Detail, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return Detail{}, err
	}
	return readConnection(ctx, s.DB, service, id)
}

func (s *Service) List(ctx context.Context, a auth.Actor, service string, includeDeleted bool) ([]Detail, error) {
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM evidence_connections WHERE service_id=$1 AND ($2 OR lifecycle <> 'deleted') ORDER BY id`, service, includeDeleted)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(ids))
	for _, id := range ids {
		d, err := readConnection(ctx, tx, service, id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, tx.Commit(ctx)
}

func audit(ctx context.Context, tx pgx.Tx, a auth.Actor, c Connection, operation string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target,revision) VALUES($1,$2,$3,$4,$5,$6)`, a.OrganizationID, c.ServiceID, a.ID, "connection."+operation, c.ID, c.Revision)
	return err
}

func (s *Service) Configure(ctx context.Context, a auth.Actor, in ConfigureInput) (Detail, error) {
	if err := validateConfiguration(s.Registry, in); err != nil {
		return Detail{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Detail{}, err
	}
	// Serialize creation/idempotency and establish consistent lock ordering.
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return Detail{}, err
	}
	request, _ := json.Marshal(in)
	digest := storage.Hash(request)
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='connection.configure' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return Detail{}, ErrConflict
		}
		var out Detail
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, commitConnection(ctx, tx, a, "configured", out.Connection, "replayed")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, err
	}
	now := time.Now().UTC()
	c := Connection{ID: in.ID, ServiceID: in.ServiceID, Provider: in.Provider, Name: in.Name, Mode: in.Mode, Configuration: in.Configuration, Lifecycle: Active, Revision: 1, Generation: 1, ActorID: a.ID, UpdatedAt: now}
	if in.ID == "" {
		if in.ExpectedRevision != 0 {
			return Detail{}, ErrConflict
		}
		c.ID = uuid.NewString()
	} else {
		if _, err = tx.Exec(ctx, `SELECT id FROM evidence_connections WHERE service_id=$1 AND id=$2 FOR UPDATE`, in.ServiceID, in.ID); err != nil {
			return Detail{}, err
		}
		previous, err := readConnection(ctx, tx, in.ServiceID, in.ID)
		if err != nil {
			return Detail{}, err
		}
		// Identity changes require a new connection; labels cannot widen a tenant.
		if previous.Connection.Provider != in.Provider || previous.Connection.Configuration.AccountID != in.Configuration.AccountID {
			return Detail{}, ErrInvalid
		}
		c, err = previous.Connection.Transition("configure", in.ExpectedRevision, now)
		if err != nil {
			return Detail{}, err
		}
		c.Name, c.Mode, c.Configuration, c.ActorID = in.Name, in.Mode, in.Configuration, a.ID
		// Reconfiguration invalidates cached credentials; renew only for access-only rotation.
		if err = s.Credentials.DeleteTx(ctx, tx, a, c.ServiceID, c.ID); err != nil {
			return Detail{}, err
		}
	}
	out := Detail{Connection: c, Access: AccessHealth{State: NotConfigured, Generation: c.Generation, Limitations: []string{}}}
	metadata, _ := json.Marshal(c)
	health, _ := json.Marshal(out.Access)
	_, err = tx.Exec(ctx, `INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,access_health,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(service_id,id) DO UPDATE SET revision=excluded.revision,generation=excluded.generation,lifecycle=excluded.lifecycle,data=excluded.data,access_health=excluded.access_health,actor_id=excluded.actor_id,updated_at=now()`, c.ServiceID, c.ID, c.Provider, c.Revision, c.Generation, c.Lifecycle, metadata, health, a.ID)
	if err != nil {
		return Detail{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_revisions(service_id,connection_id,revision,data,actor_id) VALUES($1,$2,$3,$4,$5)`, c.ServiceID, c.ID, c.Revision, metadata, a.ID)
	if err != nil {
		return Detail{}, err
	}
	if err = audit(ctx, tx, a, c, "configure"); err != nil {
		return Detail{}, err
	}
	response, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'connection.configure',$3,$4,$5)`, a.ID, c.ServiceID, in.IdempotencyKey, digest, response)
	if err != nil {
		return Detail{}, err
	}
	return out, commitConnection(ctx, tx, a, "configured", out.Connection, "")
}

func (s *Service) Test(ctx context.Context, a auth.Actor, service, id string, generation int64) (AccessHealth, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "connect"); err != nil {
		return AccessHealth{}, err
	}
	d, err := readConnection(ctx, s.DB, service, id)
	if err != nil {
		return AccessHealth{}, err
	}
	if err = d.Connection.CheckGeneration(generation); err != nil {
		return AccessHealth{}, err
	}
	if d.Connection.Mode == AgentManaged || d.Connection.Mode == Manual {
		return AccessHealth{}, errors.New("access must be reported through attributed agent discovery")
	}
	adapter, err := s.Registry.Adapter(d.Connection.Provider)
	if err != nil {
		return AccessHealth{}, err
	}
	var credentials Credentials
	if d.Connection.Mode == ServerManaged {
		credentials, err = s.Credentials.Get(ctx, a, service, id, "connect", generation)
		if err != nil {
			return AccessHealth{}, err
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	health, callErr := adapter.Test(callCtx, d.Connection, credentials)
	providerResult(ctx, d.Connection, "verify_access", start, callErr, health.State != Verified)
	if callErr != nil {
		health = AccessHealth{State: AccessError, ErrorCode: "provider_error", Limitations: []string{}}
		var recovery interface{ RecoveryCode() string }
		if errors.As(callErr, &recovery) {
			health.ErrorCode = recovery.RecoveryCode()
		}
		if health.ErrorCode == "grant_expired" {
			health.State = Expired
		}
		if health.ErrorCode == "wrong_tenant" || health.ErrorCode == "insufficient_permissions" {
			health.State = Denied
		}
	}
	if health.State == Verified && health.VerifiedAccountID != d.Connection.Tenant() {
		health = AccessHealth{State: Denied, ErrorCode: "wrong_tenant", Limitations: []string{}}
	}
	now, until := time.Now().UTC(), time.Now().UTC().Add(15*time.Minute)
	health.ActorID, health.Generation, health.CheckedAt, health.ValidUntil = a.ID, generation, &now, &until
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AccessHealth{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockCredentialConnection(ctx, tx, a, service, id, "connect", generation); err != nil {
		return AccessHealth{}, err
	}
	b, _ := json.Marshal(health)
	if _, err = tx.Exec(ctx, `UPDATE evidence_connections SET access_health=$3 WHERE service_id=$1 AND id=$2`, service, id, b); err != nil {
		return AccessHealth{}, err
	}
	if err = audit(ctx, tx, a, d.Connection, "test"); err != nil {
		return AccessHealth{}, err
	}
	return health, commitConnection(ctx, tx, a, "access.tested", d.Connection, string(health.State))
}
