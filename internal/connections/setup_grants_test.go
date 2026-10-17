package connections

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/storage"
)

func TestSetupGrantFinalizationIsAtomicAndReplayable(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	keys, err := NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	s.Credentials.Keys = keys
	setup, err := s.BeginSetup(ctx, a, BeginSetupInput{ServiceID: "svc", Provider: Workspace, IdempotencyKey: "begin", Input: SetupInput{Name: "Workspace"}})
	if err != nil {
		t.Fatal(err)
	}
	setup, err = s.UpdateSetup(ctx, a, SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}, setup.Input)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, "svc", "connect"); err != nil {
		t.Fatal(err)
	}
	setup, err = setup.Advance(SetupAuthorize, setup.Revision, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = writeSetup(ctx, tx, setup); err != nil {
		t.Fatal(err)
	}
	setup, err = s.stageSetupGrant(ctx, tx, a, setup, AuthorizedGrant{AccountID: "C012345", Credentials: NewCredentials(map[string]string{"access_token": "access-secret", "refresh_token": "refresh-secret"})})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: setup.Revision, IdempotencyKey: "finish"}
	complete, err := s.CompleteSetup(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := s.CompleteSetup(ctx, a, op)
	if err != nil || replayed.ConnectionID != complete.ConnectionID {
		t.Fatal("replay", err)
	}
	d, err := s.Get(ctx, a, "svc", complete.ConnectionID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Connection.Strategy != WorkspaceConsent || d.Connection.Tenant() != "C012345" || d.Access.State != Verified {
		t.Fatal("incorrect verified connection", d)
	}
	credentials, err := s.Credentials.Get(ctx, a, "svc", complete.ConnectionID, "connect", 1)
	if err != nil || credentials.Value("refresh_token") != "refresh-secret" {
		t.Fatal("credential transfer", err)
	}
	var secrets, connections int
	if err = s.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM connection_setup_secrets),(SELECT count(*) FROM evidence_connections)`).Scan(&secrets, &connections); err != nil {
		t.Fatal(err)
	}
	if secrets != 0 || connections != 1 {
		t.Fatal("cleanup/duplicate connection", secrets, connections)
	}
	// Reconnection pins the existing identity and generation; a concurrent
	// disconnect must prevent an already-started grant from restoring access.
	reconnect, err := s.BeginSetup(ctx, a, BeginSetupInput{ServiceID: "svc", Provider: Workspace, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "reconnect", Input: SetupInput{Name: "Workspace"}})
	if err != nil {
		t.Fatal(err)
	}
	reconnect, err = s.UpdateSetup(ctx, a, SetupOperationInput{ServiceID: "svc", SetupID: reconnect.ID, ExpectedRevision: 1, IdempotencyKey: "reconfigure"}, reconnect.Input)
	if err != nil {
		t.Fatal(err)
	}
	reconnect, err = reconnect.Advance(SetupAuthorize, reconnect.Revision, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeLifecycle(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "disconnect"}, "disconnect"); err != nil {
		t.Fatal(err)
	}
	stale, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback(ctx)
	if err = setupLock(ctx, stale, a, "svc", "connect"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.stageSetupGrant(ctx, stale, a, reconnect, AuthorizedGrant{AccountID: "C012345", Credentials: NewCredentials(map[string]string{"refresh_token": "late"})}); err == nil {
		t.Fatal("disconnect resurrected by staged grant")
	}
}

func TestSetupMigrationPreservesLegacyMetadata(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", Provider: AWS, Name: "Legacy", Mode: ServerEnvironment, IdempotencyKey: "legacy", Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "legacy"}})
	if err != nil {
		t.Fatal(err)
	}
	var before, after string
	const digest = `SELECT data::text FROM evidence_connections WHERE service_id='svc' AND id=$1`
	if err = s.DB.QueryRow(ctx, digest, d.Connection.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	// This database is an isolated test schema. Reconstruct the v9 boundary to
	// exercise backfill against an actual pre-existing connection.
	_, err = s.DB.Exec(ctx, `DROP TABLE connection_setup_handoffs,connection_setup_material,connection_setup_authorizations,connection_setup_secrets,connection_setups; ALTER TABLE evidence_connections DROP COLUMN access_strategy; DELETE FROM schema_migrations WHERE name IN ('migrations/010_connection_setups.sql','migrations/011_setup_material.sql','migrations/012_setup_handoffs.sql','migrations/013_setup_errors.sql')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.Migrate(ctx, s.DB); err != nil {
		t.Fatal(err)
	}
	if err = storage.Migrate(ctx, s.DB); err != nil {
		t.Fatal("migration replay", err)
	}
	if err = s.DB.QueryRow(ctx, digest, d.Connection.ID).Scan(&after); err != nil || before != after {
		t.Fatal("metadata rewritten", err)
	}
	retained, err := s.Get(ctx, a, "svc", d.Connection.ID)
	if err != nil || retained.Connection.Strategy != LegacyAccess || retained.Connection.Configuration.Profile != "legacy" {
		t.Fatal("legacy semantics changed", err)
	}
}
