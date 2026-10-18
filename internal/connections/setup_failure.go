package connections

import (
	"context"
	"slices"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func (s *Service) FailSetupConsent(ctx context.Context, a auth.Actor, claim SetupAuthorizationClaim, session, code string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, claim.Setup.ServiceID, "connect"); err != nil {
		return err
	}
	if err = authorizeBrowser(ctx, tx, a, claim.Setup.ServiceID, session); err != nil {
		return err
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_setup_authorizations WHERE service_id=$1 AND setup_id=$2 AND id=$3 AND consumed_at IS NOT NULL AND browser_actor_id=$4 AND browser_session_hash=$5)`, claim.Setup.ServiceID, claim.Setup.ID, claim.AttemptID, a.ID, session).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return s.recordSetupFailure(ctx, a, claim.Setup, code)
}

func (s *Service) recordSetupFailure(ctx context.Context, a auth.Actor, expected Setup, code string) error {
	if !slices.Contains([]string{"authorization_cancelled", "authorization_invalid_code", "authorization_exchange_failed", "authorization_unavailable", "workspace_policy_restricted", "customer_identity_permission_required", "offline_consent_required", "wrong_tenant", "insufficient_permissions", "grant_expired", "installation_removed", "installation_suspended", "administrator_approval_pending", "role_trust_denied", "delegated_role_configuration_required", "fed_runtime_role_mismatch", "fed_role_verification_denied", "profile_sign_in_required", "provider_error"}, code) {
		code = "provider_error"
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, expected.ServiceID, "connect"); err != nil {
		return err
	}
	setup, err := readSetup(ctx, tx, expected.ServiceID, expected.ID)
	if err != nil {
		return err
	}
	if setup.Revision != expected.Revision || setup.Terminal() {
		return ErrConflict
	}
	if err = authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return err
	}
	setup.Step = SetupConfigure
	setup.VerifiedAccountID = ""
	setup.ErrorCode = code
	setup.Revision++
	setup.UpdatedAt = time.Now().UTC()
	if err = writeSetup(ctx, tx, setup); err != nil {
		return err
	}
	return commitSetup(ctx, tx, a, "consent.failed", setup, "failed")
}
