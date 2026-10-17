package connections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

type AWSSetupProvider interface {
	SetupPrincipal(context.Context) (string, error)
	SetupMaterial(string, string, string) (SetupMaterial, error)
}

func materialAAD(org string, setup Setup) []byte {
	b, _ := json.Marshal([]string{"aws-setup-material/v1", org, setup.ServiceID, setup.ID, setup.Input.AccountID})
	return b
}

func (s *Service) AWSSetupMaterial(ctx context.Context, a auth.Actor, in SetupOperationInput) (SetupMaterial, error) {
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return SetupMaterial{}, err
	}
	adapter, err := s.Registry.Adapter(AWS)
	if err != nil {
		return SetupMaterial{}, err
	}
	provider, ok := adapter.(AWSSetupProvider)
	if !ok {
		return SetupMaterial{}, ErrInvalid
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err = provider.SetupPrincipal(call); err != nil {
		return SetupMaterial{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return SetupMaterial{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return SetupMaterial{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return SetupMaterial{}, err
	}
	if setup.Provider != AWS || setup.Terminal() || setup.Revision != in.ExpectedRevision || !setupAccount.MatchString(setup.Input.AccountID) {
		return SetupMaterial{}, ErrConflict
	}
	external, err := s.setupExternalID(ctx, tx, a, setup, true)
	if err != nil {
		return SetupMaterial{}, err
	}
	out, err := provider.SetupMaterial(setup.Input.AccountID, external, setup.Input.RoleARN)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) setupExternalID(ctx context.Context, tx pgx.Tx, a auth.Actor, setup Setup, create bool) (string, error) {
	var account, key string
	var nonce, ciphertext []byte
	err := tx.QueryRow(ctx, `SELECT account_id,key_id,nonce,ciphertext FROM connection_setup_material WHERE service_id=$1 AND setup_id=$2`, setup.ServiceID, setup.ID).Scan(&account, &key, &nonce, &ciphertext)
	if errors.Is(err, pgx.ErrNoRows) && create {
		external := auth.Secret()
		if setup.ConnectionID != "" {
			credentials, err := s.Credentials.readTx(ctx, tx, a, setup.ServiceID, setup.ConnectionID, "connect", setup.ExpectedGeneration)
			if err != nil {
				return "", err
			}
			external = credentials.Value("external_id")
			if external == "" {
				return "", ErrCredentialUnavailable
			}
		}
		key, nonce, ciphertext, err = s.Credentials.Keys.seal([]byte(external), materialAAD(a.OrganizationID, setup))
		if err != nil {
			return "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO connection_setup_material(service_id,setup_id,account_id,key_id,nonce,ciphertext) VALUES($1,$2,$3,$4,$5,$6)`, setup.ServiceID, setup.ID, setup.Input.AccountID, key, nonce, ciphertext)
		return external, err
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", SetupError("aws_setup_material_required")
		}
		return "", err
	}
	if account != setup.Input.AccountID {
		return "", ErrConflict
	}
	plain, err := s.Credentials.Keys.open(key, nonce, ciphertext, materialAAD(a.OrganizationID, setup))
	if err != nil {
		return "", err
	}
	defer clear(plain)
	return string(plain), nil
}

func (s *Service) VerifyAWSSetup(ctx context.Context, a auth.Actor, in SetupOperationInput) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return setup, err
	}
	if setup.Provider != AWS || setup.Revision != in.ExpectedRevision || setup.Step != SetupConfigure || setup.Input.RoleARN == "" {
		return setup, ErrConflict
	}
	external, err := s.setupExternalID(ctx, tx, a, setup, false)
	if err != nil {
		return setup, err
	}
	setup, err = setup.Advance(SetupAuthorize, setup.Revision, time.Now())
	if err != nil {
		return setup, err
	}
	if err = writeSetup(ctx, tx, setup); err != nil {
		return setup, err
	}
	if err = tx.Commit(ctx); err != nil {
		return setup, err
	}
	adapter, err := s.Registry.Adapter(AWS)
	if err != nil {
		return setup, err
	}
	credentials := NewCredentials(map[string]string{"external_id": external})
	c := Connection{ID: setup.ID, ServiceID: setup.ServiceID, Provider: AWS, Strategy: DelegatedRole, Mode: ServerManaged, Generation: int64(setup.Revision), Configuration: Configuration{AccountID: setup.Input.AccountID, RoleARN: setup.Input.RoleARN, Region: "us-east-1"}}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var health AccessHealth
	for attempt := 0; attempt < 3; attempt++ {
		health, err = adapter.Test(call, c, credentials)
		if err != nil || health.ErrorCode != "role_trust_denied" || attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 500 * time.Millisecond)
		select {
		case <-call.Done():
			timer.Stop()
			return setup, call.Err()
		case <-timer.C:
		}
	}
	if err != nil {
		_ = s.recordSetupFailure(ctx, a, setup, "provider_error")
		return setup, err
	}
	if health.State != Verified || health.VerifiedAccountID != setup.Input.AccountID {
		if health.ErrorCode == "" {
			health.ErrorCode = "provider_error"
		}
		_ = s.recordSetupFailure(ctx, a, setup, health.ErrorCode)
		return setup, SetupError(health.ErrorCode)
	}
	tx, err = s.DB.Begin(ctx)
	if err != nil {
		return setup, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, setup.ServiceID, "connect"); err != nil {
		return setup, err
	}
	current, err := readSetup(ctx, tx, setup.ServiceID, setup.ID)
	if err != nil {
		return setup, err
	}
	if current.Revision != setup.Revision || current.Step != SetupAuthorize {
		return current, ErrConflict
	}
	current, err = s.stageSetupGrant(ctx, tx, a, current, AuthorizedGrant{AccountID: health.VerifiedAccountID, Credentials: credentials})
	if err != nil {
		return current, err
	}
	return current, tx.Commit(ctx)
}

func (s *Service) RotateAWSSetupExternalID(ctx context.Context, a auth.Actor, in SetupOperationInput) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	id, digest, err := setupReceipt(ctx, tx, a, in.ServiceID, "setup.rotate-external-id", in.IdempotencyKey, in)
	if err != nil {
		return Setup{}, err
	}
	setup, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return setup, err
	}
	if id != "" {
		return setup, tx.Commit(ctx)
	}
	if setup.Provider != AWS || setup.Terminal() || setup.Revision != in.ExpectedRevision {
		return setup, ErrConflict
	}
	if setup.ConnectionID != "" {
		d, err := readConnection(ctx, tx, setup.ServiceID, setup.ConnectionID)
		if err != nil {
			return setup, err
		}
		if err = d.Connection.CheckGeneration(setup.ExpectedGeneration); err != nil {
			return setup, err
		}
		if err = s.Credentials.DeleteTx(ctx, tx, a, setup.ServiceID, setup.ConnectionID); err != nil {
			return setup, err
		}
		setup.ExpectedGeneration++
		_, err = tx.Exec(ctx, `UPDATE evidence_connections SET generation=$3,access_health='{"state":"authorization_required","limitations":["Update role trust with the replacement external ID and verify access."]}',updated_at=now() WHERE service_id=$1 AND id=$2`, setup.ServiceID, setup.ConnectionID, setup.ExpectedGeneration)
		if err != nil {
			return setup, err
		}
		_, err = tx.Exec(ctx, `UPDATE connection_setups SET expected_generation=$3 WHERE service_id=$1 AND id=$2`, setup.ServiceID, setup.ID, setup.ExpectedGeneration)
		if err != nil {
			return setup, err
		}
		if err = audit(ctx, tx, a, d.Connection, "rotate-external-id"); err != nil {
			return setup, err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM connection_setup_material WHERE service_id=$1 AND setup_id=$2`, setup.ServiceID, setup.ID)
	if err != nil {
		return setup, err
	}
	key, nonce, ciphertext, err := s.Credentials.Keys.seal([]byte(auth.Secret()), materialAAD(a.OrganizationID, setup))
	if err != nil {
		return setup, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_setup_material(service_id,setup_id,account_id,key_id,nonce,ciphertext) VALUES($1,$2,$3,$4,$5,$6)`, setup.ServiceID, setup.ID, setup.Input.AccountID, key, nonce, ciphertext)
	if err != nil {
		return setup, err
	}
	setup.Step = SetupConfigure
	setup.VerifiedAccountID = ""
	setup.Revision++
	setup.UpdatedAt = time.Now().UTC()
	if err = writeSetup(ctx, tx, setup); err != nil {
		return setup, err
	}
	if err = saveSetupReceipt(ctx, tx, a, setup.ServiceID, "setup.rotate-external-id", in.IdempotencyKey, digest, setup.ID); err != nil {
		return setup, err
	}
	return setup, tx.Commit(ctx)
}
