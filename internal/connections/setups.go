package connections

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

var setupAccount = regexp.MustCompile(`^[0-9]{12}$`)
var setupRole = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):role/[A-Za-z0-9_+=,.@/-]+$`)

func validateSetupInput(provider Provider, in SetupInput) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || strings.ContainsAny(in.Name, "\x00\r\n") {
		return ErrInvalid
	}
	if provider != AWS {
		if in.AccountID != "" || in.RoleARN != "" {
			return ErrInvalid
		}
		return nil
	}
	if in.AccountID != "" && !setupAccount.MatchString(in.AccountID) {
		return ErrInvalid
	}
	if in.RoleARN != "" {
		match := setupRole.FindStringSubmatch(in.RoleARN)
		if len(in.RoleARN) > 2048 || match == nil || match[1] != in.AccountID {
			return ErrInvalid
		}
	}
	return nil
}

const setupColumns = `id,service_id,provider,strategy,step,revision,input,verified_account_id,COALESCE(connection_id,''),COALESCE(expected_generation,0),actor_id,COALESCE(token_hash,''),expires_at,updated_at,error_code`

func scanSetup(row pgx.Row) (Setup, error) {
	var out Setup
	var input []byte
	err := row.Scan(&out.ID, &out.ServiceID, &out.Provider, &out.Strategy, &out.Step, &out.Revision, &input, &out.VerifiedAccountID, &out.ConnectionID, &out.ExpectedGeneration, &out.ActorID, &out.TokenHash, &out.ExpiresAt, &out.UpdatedAt, &out.ErrorCode)
	if err == nil {
		err = json.Unmarshal(input, &out.Input)
	}
	return out, err
}

// Setup receipts contain an identifier only. Replays read current redacted
// state, never resurrect stale secrets or return obsolete authorization links.
func setupReceipt(ctx context.Context, tx pgx.Tx, a auth.Actor, service, operation, key string, request any) (string, string, error) {
	if key == "" || len(key) > 256 {
		return "", "", ErrInvalid
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", "", err
	}
	digest := storage.Hash(encoded)
	var oldHash string
	var old []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation=$3 AND key=$4`, a.ID, service, operation, key).Scan(&oldHash, &old)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", digest, nil
	}
	if err != nil {
		return "", "", err
	}
	if oldHash != digest {
		return "", "", ErrConflict
	}
	var id string
	if err = json.Unmarshal(old, &id); err != nil {
		return "", "", err
	}
	return id, digest, nil
}

func saveSetupReceipt(ctx context.Context, tx pgx.Tx, a auth.Actor, service, operation, key, digest, id string) error {
	response, _ := json.Marshal(id)
	_, err := tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, service, operation, key, digest, response)
	return err
}

func setupLock(ctx context.Context, tx pgx.Tx, a auth.Actor, service, action string) error {
	if err := auth.Authorize(ctx, tx, a, service, action); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service)
	return err
}

func (s *Service) BeginSetup(ctx context.Context, a auth.Actor, in BeginSetupInput) (Setup, error) {
	strategy, err := StandardStrategy(in.Provider)
	if err != nil {
		return Setup{}, err
	}
	if err = validateSetupInput(in.Provider, in.Input); err != nil {
		return Setup{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	id, digest, err := setupReceipt(ctx, tx, a, in.ServiceID, "setup.begin", in.IdempotencyKey, in)
	if err != nil {
		return Setup{}, err
	}
	if id != "" {
		out, err := readSetup(ctx, tx, in.ServiceID, id)
		if err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	now := time.Now().UTC()
	if in.ConnectionID != "" {
		d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
		if err != nil {
			return Setup{}, err
		}
		if err = d.Connection.CheckGeneration(in.Generation); err != nil {
			return Setup{}, err
		}
		if d.Connection.Provider != in.Provider || d.Connection.Revision != in.ExpectedRevision {
			return Setup{}, ErrConflict
		}
		if d.Connection.Mode != ServerManaged {
			return Setup{}, ErrInvalid
		}
		// Reauthorization cannot silently convert an environment/profile AWS
		// binding into a delegated connection or change its trust target.
		if in.Provider == AWS && (d.Connection.Strategy != DelegatedRole || d.Connection.Configuration.AccountID != in.Input.AccountID || d.Connection.Configuration.RoleARN != in.Input.RoleARN) {
			return Setup{}, ErrInvalid
		}
	} else if in.ExpectedRevision != 0 || in.Generation != 0 {
		return Setup{}, ErrInvalid
	}
	id = uuid.NewString()
	input, _ := json.Marshal(in.Input)
	_, err = tx.Exec(ctx, `INSERT INTO connection_setups(service_id,id,provider,strategy,step,revision,input,actor_id,token_hash,expires_at,updated_at) VALUES($1,$2,$3,$4,'prerequisites',1,$5,$6,NULLIF($7,''),$8,$9)`, in.ServiceID, id, in.Provider, strategy, input, a.ID, a.TokenHash, now.Add(SetupLifetime), now)
	if err != nil {
		return Setup{}, err
	}
	if in.ConnectionID != "" {
		if _, err = tx.Exec(ctx, `UPDATE connection_setups SET connection_id=$3,expected_generation=$4 WHERE service_id=$1 AND id=$2`, in.ServiceID, id, in.ConnectionID, in.Generation); err != nil {
			return Setup{}, err
		}
	}
	if err = saveSetupReceipt(ctx, tx, a, in.ServiceID, "setup.begin", in.IdempotencyKey, digest, id); err != nil {
		return Setup{}, err
	}
	out, err := readSetup(ctx, tx, in.ServiceID, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// readSetup lazily expires a setup and destroys staged secrets/PKCE values.
// Callers hold the service lock; background expiry uses the same lock order.
func readSetup(ctx context.Context, tx pgx.Tx, service, id string) (Setup, error) {
	out, err := scanSetup(tx.QueryRow(ctx, `SELECT `+setupColumns+` FROM connection_setups WHERE service_id=$1 AND id=$2 FOR UPDATE`, service, id))
	if err != nil {
		return out, err
	}
	if !out.Terminal() && !time.Now().Before(out.ExpiresAt) {
		out, err = out.Advance(SetupExpired, out.Revision, time.Now())
		if err == nil {
			err = writeSetup(ctx, tx, out)
		}
	}
	return out, err
}

func writeSetup(ctx context.Context, tx pgx.Tx, out Setup) error {
	input, _ := json.Marshal(out.Input)
	_, err := tx.Exec(ctx, `UPDATE connection_setups SET step=$3,revision=$4,input=$5,verified_account_id=$6,updated_at=$7,error_code=$8 WHERE service_id=$1 AND id=$2`, out.ServiceID, out.ID, out.Step, out.Revision, input, out.VerifiedAccountID, out.UpdatedAt, out.ErrorCode)
	if err != nil {
		return err
	}
	if out.Terminal() || out.Step == SetupConfigure {
		if _, err = tx.Exec(ctx, `DELETE FROM connection_setup_secrets WHERE service_id=$1 AND setup_id=$2`, out.ServiceID, out.ID); err != nil {
			return err
		}
	}
	if out.Terminal() {
		if _, err = tx.Exec(ctx, `DELETE FROM connection_setup_material WHERE service_id=$1 AND setup_id=$2`, out.ServiceID, out.ID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE connection_setup_authorizations SET consumed_at=COALESCE(consumed_at,now()),verifier_key_id=NULL,verifier_nonce=NULL,verifier_ciphertext=NULL WHERE service_id=$1 AND setup_id=$2 AND (setup_revision<>$3 OR $4)`, out.ServiceID, out.ID, out.Revision, out.Terminal())
	return err
}

func (s *Service) GetSetup(ctx context.Context, a auth.Actor, service, id string) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, service, "read"); err != nil {
		return Setup{}, err
	}
	out, err := readSetup(ctx, tx, service, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) UpdateSetup(ctx context.Context, a auth.Actor, in SetupOperationInput, input SetupInput) (Setup, error) {
	return s.changeSetup(ctx, a, in, &input)
}

func (s *Service) CancelSetup(ctx context.Context, a auth.Actor, in SetupOperationInput) (Setup, error) {
	return s.changeSetup(ctx, a, in, nil)
}

func (s *Service) changeSetup(ctx context.Context, a auth.Actor, in SetupOperationInput, input *SetupInput) (Setup, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	if err = setupLock(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return Setup{}, err
	}
	operation := "setup.cancel"
	if input != nil {
		operation = "setup.update"
	}
	id, digest, err := setupReceipt(ctx, tx, a, in.ServiceID, operation, in.IdempotencyKey, struct {
		Operation SetupOperationInput
		Input     *SetupInput
	}{in, input})
	if err != nil {
		return Setup{}, err
	}
	out, err := readSetup(ctx, tx, in.ServiceID, in.SetupID)
	if err != nil {
		return out, err
	}
	if id != "" {
		return out, tx.Commit(ctx)
	}
	if out.Step == SetupExpired {
		if err = tx.Commit(ctx); err != nil {
			return out, err
		}
		return out, ErrInactive
	}
	if out.Revision != in.ExpectedRevision {
		return out, ErrConflict
	}
	if out.Terminal() {
		return out, ErrInactive
	}
	if input == nil {
		out, err = out.Advance(SetupCancelled, in.ExpectedRevision, time.Now())
	} else {
		if err = validateSetupInput(out.Provider, *input); err != nil {
			return out, err
		}
		if out.Provider == AWS && input.AccountID != out.Input.AccountID {
			var issued bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_setup_material WHERE service_id=$1 AND setup_id=$2)`, out.ServiceID, out.ID).Scan(&issued); err != nil {
				return out, err
			}
			if issued {
				return out, SetupError("setup_account_locked")
			}
		}
		// Editing declarations always discards verification and prior attempts.
		out.Input, out.VerifiedAccountID = *input, ""
		out.ErrorCode = ""
		out.Step, out.Revision, out.UpdatedAt = SetupConfigure, out.Revision+1, time.Now().UTC()
	}
	if err != nil {
		return out, err
	}
	if err = writeSetup(ctx, tx, out); err != nil {
		return out, err
	}
	if err = saveSetupReceipt(ctx, tx, a, in.ServiceID, operation, in.IdempotencyKey, digest, out.ID); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// ExpireSetups is bounded for periodic worker use. It never returns grant data.
func (s *Service) ExpireSetups(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalid
	}
	rows, err := s.DB.Query(ctx, `SELECT service_id,id FROM connection_setups WHERE expires_at<=now() AND step NOT IN ('complete','cancelled','expired') ORDER BY expires_at LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type key struct{ service, id string }
	keys := []key{}
	for rows.Next() {
		var k key
		if err = rows.Scan(&k.service, &k.id); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, k := range keys {
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			return count, err
		}
		_, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, k.service)
		if err == nil {
			_, err = readSetup(ctx, tx, k.service, k.id)
		}
		if err != nil {
			tx.Rollback(ctx)
			return count, err
		}
		if err = tx.Commit(ctx); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// MaintainSetups removes abandoned grants even when nobody revisits the setup.
// Every process may run it: service locks serialize cleanup across replicas.
func (s *Service) MaintainSetups(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		call, cancel := context.WithTimeout(ctx, 20*time.Second)
		_, err := s.ExpireSetups(call, 100)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("provider setup cleanup failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
