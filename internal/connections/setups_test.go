package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestSetupPersistenceAuthorizationAndExpiry(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	in := BeginSetupInput{ServiceID: "svc", Provider: Workspace, IdempotencyKey: "begin", Input: SetupInput{Name: "Workspace"}}
	first, err := s.BeginSetup(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.ConnectionID != "" || first.Strategy != WorkspaceConsent {
		t.Fatalf("bad initial state: %+v", first)
	}
	replay, err := s.BeginSetup(ctx, a, in)
	if err != nil || replay.ID != first.ID {
		t.Fatal("begin replay", err)
	}
	in.Input.Name = "Changed"
	if _, err = s.BeginSetup(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting replay", err)
	}
	if _, err = s.GetSetup(ctx, a, "other", first.ID); err == nil {
		t.Fatal("cross-service read")
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetSetup(ctx, reader, "svc", first.ID); err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: first.ID, ExpectedRevision: 1, IdempotencyKey: "edit"}
	if _, err = s.UpdateSetup(ctx, reader, op, SetupInput{Name: "Forbidden"}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("read token mutation", err)
	}
	updated, err := s.UpdateSetup(ctx, a, op, SetupInput{Name: "Renamed"})
	if err != nil || updated.Revision != 2 || updated.Step != SetupConfigure {
		t.Fatal("update", err)
	}
	replay, err = s.UpdateSetup(ctx, a, op, SetupInput{Name: "Renamed"})
	if err != nil || replay.Revision != 2 {
		t.Fatal("update replay", err)
	}
	op.IdempotencyKey = "stale"
	if _, err = s.CancelSetup(ctx, a, op); !errors.Is(err, ErrConflict) {
		t.Fatal("stale cancel", err)
	}
	_, err = s.DB.Exec(ctx, `UPDATE connection_setups SET expires_at=now()-interval '1 second' WHERE id=$1`, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise cleanup with opaque test ciphertext; no decryption is required.
	_, err = s.DB.Exec(ctx, `INSERT INTO connection_setup_secrets(service_id,setup_id,revision,key_id,nonce,ciphertext) VALUES('svc',$1,2,'test','\x01','\x02')`, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.ExpireSetups(ctx, 25); err != nil || n != 1 {
		t.Fatal("expiry", n, err)
	}
	expired, err := s.GetSetup(ctx, a, "svc", first.ID)
	if err != nil || expired.Step != SetupExpired {
		t.Fatal("expired status", err)
	}
	var secrets, connections int
	if err = s.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM connection_setup_secrets),(SELECT count(*) FROM evidence_connections)`).Scan(&secrets, &connections); err != nil {
		t.Fatal(err)
	}
	if secrets != 0 || connections != 0 {
		t.Fatal("setup left secrets or created connection", secrets, connections)
	}
}

func TestSetupCancellationAndAWSValidation(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	in := BeginSetupInput{ServiceID: "svc", Provider: AWS, IdempotencyKey: "begin", Input: SetupInput{Name: "AWS", AccountID: "123456789012", RoleARN: "arn:aws:iam::999999999999:role/Fed"}}
	if _, err := s.BeginSetup(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong-account role", err)
	}
	in.Input.RoleARN = "arn:aws:iam::123456789012:role/Fed"
	setup, err := s.BeginSetup(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "cancel"}
	cancelled, err := s.CancelSetup(ctx, a, op)
	if err != nil || cancelled.Step != SetupCancelled {
		t.Fatal(err)
	}
	op.ExpectedRevision = 2
	op.IdempotencyKey = "revive"
	if _, err = s.UpdateSetup(ctx, a, op, in.Input); !errors.Is(err, ErrInactive) {
		t.Fatal("cancelled setup revived", err)
	}
}
