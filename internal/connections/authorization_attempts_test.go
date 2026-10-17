package connections

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestAuthorizationBrowserBindingReplayAndGeneration(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: Workspace, Name: "Directory", Mode: ServerManaged, Configuration: Configuration{AccountID: "customer"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "authorize"}
	handoff, err := s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil || replay.OperationID != handoff.OperationID {
		t.Fatal("handoff replay changed attempt")
	}
	if _, err = s.BindAuthorizationBrowser(ctx, a, "svc", handoff.OperationID, "wrong-session"); err == nil {
		t.Fatal("unbound browser accepted")
	}
	challenge, err := s.BindAuthorizationBrowser(ctx, a, "svc", handoff.OperationID, "browser")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindAuthorizationBrowser(ctx, a, "svc", handoff.OperationID, "browser"); err == nil {
		t.Fatal("browser binding replayed")
	}
	if _, err = s.ClaimAuthorization(ctx, a, GitHub, challenge.State, "browser", challenge.RedirectURI); err == nil {
		t.Fatal("wrong provider accepted")
	}
	claim, err := s.ClaimAuthorization(ctx, a, Workspace, challenge.State, "browser", challenge.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(claim.Verifier.Value("verifier")))
	if base64.RawURLEncoding.EncodeToString(hash[:]) != challenge.PKCEChallenge {
		t.Fatal("PKCE mismatch")
	}
	if _, err = s.ClaimAuthorization(ctx, a, Workspace, challenge.State, "browser", challenge.RedirectURI); err == nil {
		t.Fatal("state replay accepted")
	}
	if _, err = s.CompleteAuthorization(ctx, a, claim, "browser", "other-customer", NewCredentials(map[string]string{"token": "secret"})); err == nil {
		t.Fatal("wrong tenant grant accepted")
	}
	out, err := s.CompleteAuthorization(ctx, a, claim, "browser", "customer", NewCredentials(map[string]string{"token": "secret"}))
	if err != nil || out.State != "succeeded" {
		t.Fatalf("complete: %v", err)
	}
	if _, err = s.CompleteAuthorization(ctx, a, claim, "browser", "customer", NewCredentials(map[string]string{"token": "overwrite"})); err == nil {
		t.Fatal("grant completion replay accepted")
	}
	in.Generation, in.IdempotencyKey = 2, "late"
	late, err := s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	lateChallenge, err := s.BindAuthorizationBrowser(ctx, a, "svc", late.OperationID, "browser")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeLifecycle(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 2, IdempotencyKey: "delete"}, "delete"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimAuthorization(ctx, a, Workspace, lateChallenge.State, "browser", lateChallenge.RedirectURI); err == nil {
		t.Fatal("deleted connection accepted callback")
	}
}

func TestAuthorizationRechecksInitiatorAndExpiry(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: Workspace, Name: "Directory", Mode: ServerManaged, Configuration: Configuration{AccountID: "customer"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"connect"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "agent"}
	handoff, err := s.BeginAuthorization(ctx, agent, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.RevokeToken(ctx, s.DB, a, "svc", agent.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindAuthorizationBrowser(ctx, a, "svc", handoff.OperationID, "browser"); err == nil {
		t.Fatal("revoked initiator completed handoff")
	}
	in.IdempotencyKey = "expired"
	handoff, err = s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE connection_authorizations SET expires_at=now()-interval '1 second' WHERE operation_id=$1`, handoff.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindAuthorizationBrowser(ctx, a, "svc", handoff.OperationID, "browser"); err == nil {
		t.Fatal("expired attempt accepted")
	}
}
