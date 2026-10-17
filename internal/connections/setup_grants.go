package connections

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

func setupAAD(org string, setup Setup) []byte {
	// Separate namespace prevents a staged grant from being interpreted as a
	// final connection grant even if an identifier is accidentally reused.
	b, _ := json.Marshal([]any{"connection-setup/v1", org, setup.ServiceID, setup.ID, setup.Provider, setup.Revision})
	return b
}

func authorizeSetupInitiator(ctx context.Context, tx pgx.Tx, setup Setup) error {
	a := auth.Actor{ID: setup.ActorID, TokenHash: setup.TokenHash}
	if err := tx.QueryRow(ctx, `SELECT organization_id,kind FROM identities WHERE id=$1`, a.ID).Scan(&a.OrganizationID, &a.Kind); err != nil {
		return auth.ErrForbidden
	}
	return auth.Authorize(ctx, tx, a, setup.ServiceID, "connect")
}

// stageSetupGrant is an internal provider-completion operation, never a raw
// browser/MCP mutation accepting a caller's assertion of verified identity.
func (s *Service) stageSetupGrant(ctx context.Context, tx pgx.Tx, a auth.Actor, setup Setup, grant AuthorizedGrant) (Setup, error) {
	if grant.AccountID == "" || len(grant.Credentials.values) == 0 {
		return setup, ErrInvalid
	}
	if err := authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return setup, err
	}
	if setup.ConnectionID != "" {
		d, err := readConnection(ctx, tx, setup.ServiceID, setup.ConnectionID)
		if err != nil {
			return setup, err
		}
		if err = d.Connection.CheckGeneration(setup.ExpectedGeneration); err != nil {
			return setup, err
		}
		if d.Connection.Tenant() != grant.AccountID {
			return setup, ErrInvalid
		}
	}
	setup.VerifiedAccountID = grant.AccountID
	next, err := setup.Advance(SetupConfirm, setup.Revision, time.Now())
	if err != nil {
		return setup, err
	}
	plain, err := json.Marshal(grant.Credentials.values)
	if err != nil {
		return setup, err
	}
	defer clear(plain)
	key, nonce, ciphertext, err := s.Credentials.Keys.seal(plain, setupAAD(a.OrganizationID, next))
	if err != nil {
		return setup, err
	}
	if err = writeSetup(ctx, tx, next); err != nil {
		return setup, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_setup_secrets(service_id,setup_id,revision,key_id,nonce,ciphertext) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(service_id,setup_id) DO UPDATE SET revision=excluded.revision,key_id=excluded.key_id,nonce=excluded.nonce,ciphertext=excluded.ciphertext`, next.ServiceID, next.ID, next.Revision, key, nonce, ciphertext)
	return next, err
}

func (s *Service) CompleteSetup(ctx context.Context, a auth.Actor, in SetupOperationInput) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	id, digest, err := setupReceipt(ctx, tx, a, in.ServiceID, "setup.complete", in.IdempotencyKey, in)
	if err != nil {
		return Setup{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return setup, err
	}
	if err = authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return setup, err
	}
	if id != "" {
		return setup, tx.Commit(ctx)
	}
	if setup.Step == SetupExpired {
		if err = tx.Commit(ctx); err != nil {
			return setup, err
		}
		return setup, ErrInactive
	}
	if setup.Step != SetupConfirm || setup.Revision != in.ExpectedRevision {
		return setup, ErrConflict
	}
	var key string
	var nonce, ciphertext []byte
	err = tx.QueryRow(ctx, `SELECT key_id,nonce,ciphertext FROM connection_setup_secrets WHERE service_id=$1 AND setup_id=$2 AND revision=$3`, setup.ServiceID, setup.ID, setup.Revision).Scan(&key, &nonce, &ciphertext)
	if err != nil {
		return setup, err
	}
	plain, err := s.Credentials.Keys.open(key, nonce, ciphertext, setupAAD(a.OrganizationID, setup))
	if err != nil {
		return setup, err
	}
	defer clear(plain)
	var values map[string]string
	if err = json.Unmarshal(plain, &values); err != nil {
		return setup, err
	}
	now := time.Now().UTC()
	c := Connection{ID: setup.ConnectionID, ServiceID: setup.ServiceID, Provider: setup.Provider, Strategy: setup.Strategy, Name: setup.Input.Name, Mode: ServerManaged, Configuration: Configuration{AccountID: setup.VerifiedAccountID}, VerifiedAccountID: setup.VerifiedAccountID, Lifecycle: Active, Revision: 1, Generation: 1, ActorID: a.ID, UpdatedAt: now}
	if c.ID != "" {
		d, err := readConnection(ctx, tx, c.ServiceID, c.ID)
		if err != nil {
			return setup, err
		}
		if err = d.Connection.CheckGeneration(setup.ExpectedGeneration); err != nil {
			return setup, err
		}
		if d.Connection.Provider != c.Provider || d.Connection.Tenant() != setup.VerifiedAccountID {
			return setup, ErrInvalid
		}
		c = d.Connection
		c.Generation++
		c.ActorID = a.ID
		c.UpdatedAt = now
	} else {
		c.ID = uuid.NewString()
	}
	if c.Provider == AWS {
		if setup.ConnectionID != "" && (c.Configuration.RoleARN != setup.Input.RoleARN || c.Configuration.AccountID != setup.Input.AccountID) {
			return setup, ErrInvalid
		}
		c.Configuration.RoleARN = setup.Input.RoleARN
		c.Configuration.Region = "us-east-1"
		if setup.Input.AccountID != setup.VerifiedAccountID {
			return setup, ErrInvalid
		}
	}
	if c.Provider == GitHub {
		if setup.ConnectionID != "" && c.Configuration.InstallationID != values["installation_id"] {
			return setup, ErrInvalid
		}
		c.Configuration.InstallationID = values["installation_id"]
		if c.Configuration.InstallationID == "" {
			return setup, ErrInvalid
		}
	}
	metadata, _ := json.Marshal(c)
	until := now.Add(15 * time.Minute)
	health, _ := json.Marshal(AccessHealth{State: Verified, VerifiedAccountID: c.Tenant(), Generation: c.Generation, ActorID: a.ID, CheckedAt: &now, ValidUntil: &until, Limitations: []string{"Access verified; discovery and evidence collection are separate operations."}})
	if setup.ConnectionID == "" {
		_, err = tx.Exec(ctx, `INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,access_health,actor_id,verified_account_id,access_strategy) VALUES($1,$2,$3,1,1,'active',$4,$5,$6,$7,$8)`, c.ServiceID, c.ID, c.Provider, metadata, health, a.ID, c.Tenant(), c.Strategy)
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO connection_revisions(service_id,connection_id,revision,data,actor_id) VALUES($1,$2,1,$3,$4)`, c.ServiceID, c.ID, metadata, a.ID)
		}
	} else {
		_, err = tx.Exec(ctx, `UPDATE evidence_connections SET generation=$3,access_health=$4,actor_id=$5,updated_at=$6 WHERE service_id=$1 AND id=$2`, c.ServiceID, c.ID, c.Generation, health, a.ID, now)
	}
	if err != nil {
		return setup, err
	}
	if err = s.Credentials.PutTx(ctx, tx, a, c.ServiceID, c.ID, c.Generation, NewCredentials(values)); err != nil {
		return setup, err
	}
	setup.ConnectionID = c.ID
	setup, err = setup.Advance(SetupComplete, setup.Revision, now)
	if err != nil {
		return setup, err
	}
	// Set the connection before the complete-state constraint is evaluated.
	_, err = tx.Exec(ctx, `UPDATE connection_setups SET connection_id=$3 WHERE service_id=$1 AND id=$2`, setup.ServiceID, setup.ID, c.ID)
	if err != nil {
		return setup, err
	}
	if err = writeSetup(ctx, tx, setup); err != nil {
		return setup, err
	}
	if err = audit(ctx, tx, a, c, "setup.complete"); err != nil {
		return setup, err
	}
	if err = saveSetupReceipt(ctx, tx, a, setup.ServiceID, "setup.complete", in.IdempotencyKey, digest, setup.ID); err != nil {
		return setup, err
	}
	return setup, tx.Commit(ctx)
}
