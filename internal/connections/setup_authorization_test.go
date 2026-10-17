package connections

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestSetupAuthorizationStagesReplayAndRevocation(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if _, err := s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"connect"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	initiator, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	setup, err := s.BeginSetup(ctx, initiator, BeginSetupInput{ServiceID: "svc", Provider: GitHub, IdempotencyKey: "begin", Input: SetupInput{Name: "GitHub"}})
	if err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}
	setup, err = s.UpdateSetup(ctx, initiator, op, setup.Input)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = setup.Revision
	install, err := s.BeginSetupAuthorization(ctx, a, op, "browser", "https://fed.example", "installation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimSetupAuthorization(ctx, a, Workspace, install.State, "browser", install.RedirectURI); err == nil {
		t.Fatal("provider substitution")
	}
	claim, err := s.ClaimSetupAuthorization(ctx, a, GitHub, install.State, "browser", install.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimSetupAuthorization(ctx, a, GitHub, install.State, "browser", install.RedirectURI); err == nil {
		t.Fatal("installation replay")
	}
	if _, err = s.finishSetupAuthorization(ctx, a, claim, "browser", AuthorizedGrant{AccountID: "123", Credentials: NewCredentials(map[string]string{"installation_id": "42"})}); err == nil {
		t.Fatal("installation hint accepted as grant")
	}
	op.ExpectedRevision = claim.Setup.Revision
	oauth, err := s.BeginSetupAuthorization(ctx, a, op, "browser", "https://fed.example", "oauth")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimSetupAuthorization(ctx, a, GitHub, oauth.State, "different", oauth.RedirectURI); err == nil {
		t.Fatal("session substitution")
	}
	claim, err = s.ClaimSetupAuthorization(ctx, a, GitHub, oauth.State, "browser", oauth.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(claim.Verifier.Value("verifier")))
	if base64.RawURLEncoding.EncodeToString(hash[:]) != oauth.PKCEChallenge {
		t.Fatal("PKCE mismatch")
	}
	if _, err = s.DB.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE hash=$1`, initiator.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.finishSetupAuthorization(ctx, a, claim, "browser", AuthorizedGrant{AccountID: "123", Credentials: NewCredentials(map[string]string{"installation_id": "42"})}); err == nil {
		t.Fatal("revoked initiator completed grant")
	}
}

func TestSetupCancellationFencesClaimedGrant(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if _, err := s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	setup, err := s.BeginSetup(ctx, a, BeginSetupInput{ServiceID: "svc", Provider: Workspace, IdempotencyKey: "begin", Input: SetupInput{Name: "Workspace"}})
	if err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}
	setup, err = s.UpdateSetup(ctx, a, op, setup.Input)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = setup.Revision
	challenge, err := s.BeginSetupAuthorization(ctx, a, op, "browser", "https://fed.example", "oauth")
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimSetupAuthorization(ctx, a, Workspace, challenge.State, "browser", challenge.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = claim.Setup.Revision
	op.IdempotencyKey = "cancel"
	if _, err = s.CancelSetup(ctx, a, op); err != nil {
		t.Fatal(err)
	}
	if _, err = s.finishSetupAuthorization(ctx, a, claim, "browser", AuthorizedGrant{AccountID: "customer", Credentials: NewCredentials(map[string]string{"refresh_token": "secret"})}); err == nil {
		t.Fatal("cancelled callback restored access")
	}
}

func TestSetupCallbackExpiryActorAndRevisionFences(t *testing.T) {
	for _, scenario := range []string{"expired", "different-actor", "reconfigured"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, a := setupService(t)
			s.Credentials.Keys, _ = NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
			if _, err := s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
				t.Fatal(err)
			}
			setup, err := s.BeginSetup(ctx, a, BeginSetupInput{ServiceID: "svc", Provider: Workspace, IdempotencyKey: "begin", Input: SetupInput{Name: "Workspace"}})
			if err != nil {
				t.Fatal(err)
			}
			op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}
			setup, err = s.UpdateSetup(ctx, a, op, setup.Input)
			if err != nil {
				t.Fatal(err)
			}
			op.ExpectedRevision = setup.Revision
			challenge, err := s.BeginSetupAuthorization(ctx, a, op, "browser", "https://fed.example", "oauth")
			if err != nil {
				t.Fatal(err)
			}
			actor, session := a, "browser"
			switch scenario {
			case "expired":
				_, err = s.DB.Exec(ctx, `UPDATE connection_setup_authorizations SET expires_at=now()-interval '1 second' WHERE setup_id=$1`, setup.ID)
			case "different-actor":
				actor, err = auth.BootstrapAdmin(ctx, s.DB, "test", "other-browser")
				if err != nil {
					t.Fatal(err)
				}
				session = "other-browser"
				_, err = s.DB.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('other-browser',$1,'csrf',now()+interval '1 hour')`, actor.ID)
			case "reconfigured":
				op.ExpectedRevision++
				op.IdempotencyKey = "edit"
				_, err = s.UpdateSetup(ctx, a, op, SetupInput{Name: "Changed"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ClaimSetupAuthorization(ctx, actor, Workspace, challenge.State, session, challenge.RedirectURI); err == nil {
				t.Fatal("stale or substituted callback accepted")
			}
		})
	}
}
