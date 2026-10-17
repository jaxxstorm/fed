package connections

import (
	"context"
	"net/url"
	"regexp"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

// SetupHandoff is usable by agents without leaking provider state or credentials.
func (s *Service) SetupHandoff(ctx context.Context, a auth.Actor, in SetupOperationInput, origin string) (AuthorizationHandoff, error) {
	origin, err := publicOrigin(origin)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	if err = auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return AuthorizationHandoff{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return AuthorizationHandoff{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	if setup.Terminal() || setup.Revision != in.ExpectedRevision || setup.Provider == AWS {
		return AuthorizationHandoff{}, ErrConflict
	}
	expires := time.Now().Add(10 * time.Minute)
	if expires.After(setup.ExpiresAt) {
		expires = setup.ExpiresAt
	}
	token := auth.Secret()
	_, err = tx.Exec(ctx, `INSERT INTO connection_setup_handoffs(hash,service_id,setup_id,setup_revision,expires_at) VALUES($1,$2,$3,$4,$5)`, storage.Hash([]byte(token)), setup.ServiceID, setup.ID, setup.Revision, expires)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	out := AuthorizationHandoff{OperationID: setup.ID, URL: origin + "/connections/setup/authorize?handoff=" + url.QueryEscape(token), ExpiresAt: expires}
	return out, tx.Commit(ctx)
}

func (s *Service) ClaimSetupHandoff(ctx context.Context, a auth.Actor, token, session string) (Setup, error) {
	if len(token) != 64 {
		return Setup{}, ErrInvalid
	}
	var service, id string
	var revision int
	err := s.DB.QueryRow(ctx, `SELECT service_id,setup_id,setup_revision FROM connection_setup_handoffs WHERE hash=$1 AND consumed_at IS NULL AND expires_at>now()`, storage.Hash([]byte(token))).Scan(&service, &id, &revision)
	if err != nil {
		return Setup{}, ErrConflict
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, service, "connect"); err != nil {
		return Setup{}, err
	}
	if err = authorizeBrowser(ctx, tx, a, service, session); err != nil {
		return Setup{}, err
	}
	setup, err := readSetup(ctx, tx, service, id)
	if err != nil {
		return setup, err
	}
	if setup.Terminal() || setup.Revision != revision {
		return setup, ErrConflict
	}
	if err = authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return setup, err
	}
	tag, err := tx.Exec(ctx, `UPDATE connection_setup_handoffs SET consumed_at=now() WHERE hash=$1 AND consumed_at IS NULL AND expires_at>now()`, storage.Hash([]byte(token)))
	if err != nil {
		return setup, err
	}
	if tag.RowsAffected() != 1 {
		return setup, ErrConflict
	}
	return setup, tx.Commit(ctx)
}

func (s *Service) IsSetupAuthorizationState(ctx context.Context, state string) bool {
	if len(state) != 64 {
		return false
	}
	var exists bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_setup_authorizations WHERE state_hash=$1)`, storage.Hash([]byte(state))).Scan(&exists)
	return err == nil && exists
}

// Recovery exposes only the originating setup to the same authorized browser;
// it never consumes or revives an expired/replayed attempt.
func (s *Service) SetupAuthorizationRecovery(ctx context.Context, a auth.Actor, state, session string) (Setup, error) {
	if len(state) != 64 {
		return Setup{}, ErrInvalid
	}
	var service, id string
	err := s.DB.QueryRow(ctx, `SELECT service_id,setup_id FROM connection_setup_authorizations WHERE state_hash=$1 AND browser_actor_id=$2 AND browser_session_hash=$3`, storage.Hash([]byte(state)), a.ID, session).Scan(&service, &id)
	if err != nil {
		return Setup{}, ErrConflict
	}
	return s.GetSetup(ctx, a, service, id)
}

func (s *Service) SetupAuthorizationURL(ctx context.Context, a auth.Actor, in SetupOperationInput, session, origin, stage string) (string, error) {
	setup, err := s.GetSetup(ctx, a, in.ServiceID, in.SetupID)
	if err != nil {
		return "", err
	}
	adapter, err := s.Registry.Adapter(setup.Provider)
	if err != nil {
		return "", err
	}
	authorizer, ok := adapter.(Authorizer)
	if !ok {
		return "", ErrInvalid
	}
	challenge, err := s.BeginSetupAuthorization(ctx, a, in, session, origin, stage)
	if err != nil {
		return "", err
	}
	if stage == "installation" {
		installer, ok := adapter.(interface{ InstallationURL(string) string })
		if !ok {
			return "", ErrInvalid
		}
		return installer.InstallationURL(challenge.State), nil
	}
	return authorizer.AuthorizationURL(Connection{Provider: setup.Provider, Strategy: setup.Strategy}, challenge)
}

func (s *Service) ReturnSetupInstallation(ctx context.Context, a auth.Actor, claim SetupAuthorizationClaim, session, installation string) error {
	if claim.Stage != "installation" || claim.Setup.Provider != GitHub || !regexp.MustCompile(`^[1-9][0-9]{0,19}$`).MatchString(installation) {
		return ErrInvalid
	}
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
	setup, err := readSetup(ctx, tx, claim.Setup.ServiceID, claim.Setup.ID)
	if err != nil {
		return err
	}
	if setup.Revision != claim.Setup.Revision || setup.Step != SetupAuthorize {
		return ErrConflict
	}
	if err = authorizeSetupInitiator(ctx, tx, setup); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE connection_setup_authorizations SET installation_hint=$3 WHERE service_id=$1 AND id=$2 AND stage='installation' AND consumed_at IS NOT NULL AND expires_at>now() AND browser_actor_id=$4 AND browser_session_hash=$5 AND installation_hint=''`, setup.ServiceID, claim.AttemptID, installation, a.ID, session)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func (s *Service) ExchangeSetupConsent(ctx context.Context, a auth.Actor, claim SetupAuthorizationClaim, session, code string) (Setup, error) {
	if claim.Stage != "oauth" || code == "" {
		return claim.Setup, ErrInvalid
	}
	adapter, err := s.Registry.Adapter(claim.Setup.Provider)
	if err != nil {
		return claim.Setup, err
	}
	authorizer, ok := adapter.(Authorizer)
	if !ok {
		return claim.Setup, ErrInvalid
	}
	c := Connection{Provider: claim.Setup.Provider, Strategy: claim.Setup.Strategy}
	if claim.Setup.ConnectionID != "" {
		d, err := s.Get(ctx, a, claim.Setup.ServiceID, claim.Setup.ConnectionID)
		if err != nil {
			return claim.Setup, err
		}
		c = d.Connection
	}
	if c.Provider == GitHub {
		var hint string
		err = s.DB.QueryRow(ctx, `SELECT installation_hint FROM connection_setup_authorizations WHERE service_id=$1 AND setup_id=$2 AND stage='installation' AND setup_revision=$3 AND consumed_at IS NOT NULL AND installation_hint<>'' AND browser_actor_id=$4 AND browser_session_hash=$5`, claim.Setup.ServiceID, claim.Setup.ID, claim.Setup.Revision-1, a.ID, session).Scan(&hint)
		if err != nil {
			return claim.Setup, ErrConflict
		}
		if c.Configuration.InstallationID != "" && c.Configuration.InstallationID != hint {
			return claim.Setup, ErrInvalid
		}
		c.Configuration.InstallationID = hint
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	grant, err := authorizer.ExchangeAuthorization(call, c, AuthorizationClaim{RedirectURI: claim.RedirectURI, Verifier: claim.Verifier}, AuthorizationResponse{Code: code, InstallationID: c.Configuration.InstallationID})
	if err != nil {
		return claim.Setup, err
	}
	return s.finishSetupAuthorization(ctx, a, claim, session, grant)
}
