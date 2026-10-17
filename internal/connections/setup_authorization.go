package connections

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type SetupAuthorizationClaim struct {
	Setup       Setup
	AttemptID   string
	Stage       string
	RedirectURI string
	Verifier    Credentials
}

func setupVerifierAAD(org string, setup Setup, id, stage string) []byte {
	b, _ := json.Marshal([]any{"setup-authorization/v1", org, setup.ServiceID, setup.ID, setup.Revision, id, stage})
	return b
}

// BeginSetupAuthorization binds an actual browser session before returning
// provider state. An MCP caller must first hand off to this browser operation.
func (s *Service) BeginSetupAuthorization(ctx context.Context, a auth.Actor, in SetupOperationInput, sessionHash, origin, stage string) (AuthorizationChallenge, error) {
	origin, err := publicOrigin(origin)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return AuthorizationChallenge{}, err
	}
	if err = authorizeBrowser(ctx, tx, a, in.ServiceID, sessionHash); err != nil {
		return AuthorizationChallenge{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	if setup.Step == SetupExpired {
		if err = tx.Commit(ctx); err != nil {
			return AuthorizationChallenge{}, err
		}
		return AuthorizationChallenge{}, ErrInactive
	}
	if err = authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return AuthorizationChallenge{}, err
	}
	if setup.Provider != Workspace && setup.Provider != GitHub || stage != "oauth" && stage != "installation" || stage == "installation" && setup.Provider != GitHub {
		return AuthorizationChallenge{}, ErrInvalid
	}
	if setup.Revision != in.ExpectedRevision {
		return AuthorizationChallenge{}, ErrConflict
	}
	// Returning from installation may start a distinct OAuth attempt. Every
	// new stage/retry advances the fence and consumes earlier stage tokens.
	if setup.Step == SetupAuthorize {
		setup.Revision++
		setup.UpdatedAt = time.Now().UTC()
	} else {
		setup, err = setup.Advance(SetupAuthorize, in.ExpectedRevision, time.Now())
		if err != nil {
			return AuthorizationChallenge{}, err
		}
	}
	if err = writeSetup(ctx, tx, setup); err != nil {
		return AuthorizationChallenge{}, err
	}
	redirect := origin + "/connections/callback/" + string(setup.Provider)
	if stage == "installation" {
		redirect = origin + "/connections/setup/github"
	}
	id, state, verifier := uuid.NewString(), auth.Secret(), auth.Secret()
	key, nonce, ciphertext, err := s.Credentials.Keys.seal([]byte(verifier), setupVerifierAAD(a.OrganizationID, setup, id, stage))
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	expires := time.Now().Add(10 * time.Minute)
	if expires.After(setup.ExpiresAt) {
		expires = setup.ExpiresAt
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_setup_authorizations(service_id,setup_id,id,setup_revision,stage,state_hash,browser_actor_id,browser_session_hash,redirect_uri,verifier_key_id,verifier_nonce,verifier_ciphertext,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, setup.ServiceID, setup.ID, id, setup.Revision, stage, storage.Hash([]byte(state)), a.ID, sessionHash, redirect, key, nonce, ciphertext, expires)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	hash := sha256.Sum256([]byte(verifier))
	out := AuthorizationChallenge{State: state, PKCEChallenge: base64.RawURLEncoding.EncodeToString(hash[:]), RedirectURI: redirect}
	return out, tx.Commit(ctx)
}

func (s *Service) ClaimSetupAuthorization(ctx context.Context, a auth.Actor, provider Provider, state, sessionHash, redirect string) (SetupAuthorizationClaim, error) {
	if len(state) != 64 {
		return SetupAuthorizationClaim{}, ErrInvalid
	}
	// Resolve only a service identifier outside the transaction; all validity
	// checks occur again under the service lock to preserve lock ordering.
	var service string
	err := s.DB.QueryRow(ctx, `SELECT service_id FROM connection_setup_authorizations WHERE state_hash=$1`, storage.Hash([]byte(state))).Scan(&service)
	if err != nil {
		return SetupAuthorizationClaim{}, ErrConflict
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return SetupAuthorizationClaim{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, service, "connect"); err != nil {
		return SetupAuthorizationClaim{}, err
	}
	if err = authorizeBrowser(ctx, tx, a, service, sessionHash); err != nil {
		return SetupAuthorizationClaim{}, err
	}
	var claim SetupAuthorizationClaim
	var setupID, key string
	var revision int
	var nonce, ciphertext []byte
	err = tx.QueryRow(ctx, `SELECT setup_id,id,setup_revision,stage,redirect_uri,verifier_key_id,verifier_nonce,verifier_ciphertext FROM connection_setup_authorizations WHERE service_id=$1 AND state_hash=$2 AND browser_actor_id=$3 AND browser_session_hash=$4 AND redirect_uri=$5 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, service, storage.Hash([]byte(state)), a.ID, sessionHash, redirect).Scan(&setupID, &claim.AttemptID, &revision, &claim.Stage, &claim.RedirectURI, &key, &nonce, &ciphertext)
	if err != nil {
		return claim, ErrConflict
	}
	claim.Setup, err = readSetup(ctx, tx, service, setupID)
	if err != nil {
		return claim, err
	}
	if claim.Setup.Provider != provider || claim.Setup.Step != SetupAuthorize || claim.Setup.Revision != revision {
		return claim, ErrConflict
	}
	if err = authorizeSetupInitiator(ctx, tx, claim.Setup); err != nil {
		return claim, err
	}
	plain, err := s.Credentials.Keys.open(key, nonce, ciphertext, setupVerifierAAD(a.OrganizationID, claim.Setup, claim.AttemptID, claim.Stage))
	if err != nil {
		return claim, err
	}
	defer clear(plain)
	claim.Verifier = NewCredentials(map[string]string{"verifier": string(plain)})
	_, err = tx.Exec(ctx, `UPDATE connection_setup_authorizations SET consumed_at=now(),verifier_key_id=NULL,verifier_nonce=NULL,verifier_ciphertext=NULL WHERE service_id=$1 AND id=$2`, service, claim.AttemptID)
	if err != nil {
		return claim, err
	}
	return claim, tx.Commit(ctx)
}

// finishSetupAuthorization is called only after a provider adapter verifies the
// grant. It rechecks both actors and all revision/session fences after network IO.
func (s *Service) finishSetupAuthorization(ctx context.Context, a auth.Actor, claim SetupAuthorizationClaim, sessionHash string, grant AuthorizedGrant) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, claim.Setup.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	if err = authorizeBrowser(ctx, tx, a, claim.Setup.ServiceID, sessionHash); err != nil {
		return Setup{}, err
	}
	setup, err := readSetup(ctx, tx, claim.Setup.ServiceID, claim.Setup.ID)
	if err != nil {
		return setup, err
	}
	if setup.Revision != claim.Setup.Revision || setup.Step != SetupAuthorize || claim.Stage != "oauth" {
		return setup, ErrConflict
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_setup_authorizations WHERE service_id=$1 AND setup_id=$2 AND id=$3 AND setup_revision=$4 AND stage='oauth' AND consumed_at IS NOT NULL AND expires_at>now() AND browser_actor_id=$5 AND browser_session_hash=$6)`, setup.ServiceID, setup.ID, claim.AttemptID, setup.Revision, a.ID, sessionHash).Scan(&valid)
	if err != nil {
		return setup, err
	}
	if !valid {
		return setup, ErrConflict
	}
	setup, err = s.stageSetupGrant(ctx, tx, a, setup, grant)
	if err != nil {
		return setup, err
	}
	return setup, tx.Commit(ctx)
}
