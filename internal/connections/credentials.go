package connections

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
)

var ErrCredentialUnavailable = errors.New("connection credentials unavailable")

// Keyring is loaded from deployment configuration, never persisted in PostgreSQL.
type Keyring struct {
	active string
	keys   map[string]cipher.AEAD
}

func LoadKeyring() (*Keyring, error) {
	var encoded map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("KSI_CONNECTION_KEYS")), &encoded); err != nil {
		return nil, ErrCredentialUnavailable
	}
	return NewKeyring(os.Getenv("KSI_CONNECTION_KEY_ID"), encoded)
}

func NewKeyring(active string, encoded map[string]string) (*Keyring, error) {
	k := &Keyring{active: active, keys: map[string]cipher.AEAD{}}
	for id, value := range encoded {
		raw, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(raw) != 32 || id == "" {
			return nil, ErrCredentialUnavailable
		}
		block, err := aes.NewCipher(raw)
		if err != nil {
			return nil, ErrCredentialUnavailable
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrCredentialUnavailable
		}
		k.keys[id] = aead
	}
	if k.keys[active] == nil {
		return nil, ErrCredentialUnavailable
	}
	return k, nil
}

type CredentialStore struct {
	DB   *pgxpool.Pool
	Keys *Keyring
}

func credentialAAD(org, service, connection, provider string, generation int64) []byte {
	b, _ := json.Marshal([]any{"connection-credential-v1", org, service, connection, provider, generation})
	return b
}

func (k *Keyring) seal(plain, aad []byte) (string, []byte, []byte, error) {
	if k == nil || k.keys[k.active] == nil {
		return "", nil, nil, ErrCredentialUnavailable
	}
	aead := k.keys[k.active]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, nil, ErrCredentialUnavailable
	}
	return k.active, nonce, aead.Seal(nil, nonce, plain, aad), nil
}

func (k *Keyring) open(id string, nonce, ciphertext, aad []byte) ([]byte, error) {
	if k == nil || k.keys[id] == nil {
		return nil, ErrCredentialUnavailable
	}
	aead := k.keys[id]
	if len(nonce) != aead.NonceSize() {
		return nil, ErrCredentialUnavailable
	}
	b, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrCredentialUnavailable
	}
	return b, nil
}

func lockCredentialConnection(ctx context.Context, tx pgx.Tx, a auth.Actor, service, connection, action string, expected int64) (string, error) {
	if action != "connect" && action != "check" {
		return "", auth.ErrForbidden
	}
	if err := auth.Authorize(ctx, tx, a, service, action); err != nil {
		return "", err
	}
	var provider, lifecycle string
	var generation int64
	err := tx.QueryRow(ctx, `SELECT provider,lifecycle,generation FROM evidence_connections WHERE service_id=$1 AND id=$2 FOR UPDATE`, service, connection).Scan(&provider, &lifecycle, &generation)
	if err != nil {
		return "", ErrCredentialUnavailable
	}
	if lifecycle != string(Active) {
		return "", ErrInactive
	}
	if generation != expected {
		return "", ErrConflict
	}
	return provider, nil
}

// Put requires an active generation; callers replacing grants must update the
// connection generation in the same transaction via PutTx.
func (s CredentialStore) PutTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, connection string, generation int64, credentials Credentials) error {
	provider, err := lockCredentialConnection(ctx, tx, a, service, connection, "connect", generation)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(credentials.values)
	if err != nil || len(plain) > MaxResponseBytes || len(credentials.values) == 0 {
		return ErrInvalid
	}
	defer clear(plain)
	id, nonce, ciphertext, err := s.Keys.seal(plain, credentialAAD(a.OrganizationID, service, connection, provider, generation))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_credentials(service_id,connection_id,generation,key_id,nonce,ciphertext) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(service_id,connection_id) DO UPDATE SET generation=excluded.generation,key_id=excluded.key_id,nonce=excluded.nonce,ciphertext=excluded.ciphertext,updated_at=now()`, service, connection, generation, id, nonce, ciphertext)
	return err
}

func (s CredentialStore) readTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, connection, action string, generation int64) (Credentials, error) {
	provider, err := lockCredentialConnection(ctx, tx, a, service, connection, action, generation)
	if err != nil {
		return Credentials{}, err
	}
	var id string
	var nonce, ciphertext []byte
	err = tx.QueryRow(ctx, `SELECT key_id,nonce,ciphertext FROM connection_credentials WHERE service_id=$1 AND connection_id=$2 AND generation=$3`, service, connection, generation).Scan(&id, &nonce, &ciphertext)
	if err != nil {
		return Credentials{}, ErrCredentialUnavailable
	}
	plain, err := s.Keys.open(id, nonce, ciphertext, credentialAAD(a.OrganizationID, service, connection, provider, generation))
	if err != nil {
		return Credentials{}, err
	}
	defer clear(plain)
	var values map[string]string
	if json.Unmarshal(plain, &values) != nil {
		return Credentials{}, ErrCredentialUnavailable
	}
	return NewCredentials(values), nil
}

func (s CredentialStore) Get(ctx context.Context, a auth.Actor, service, connection, action string, generation int64) (Credentials, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Credentials{}, err
	}
	defer tx.Rollback(ctx)
	credentials, err := s.readTx(ctx, tx, a, service, connection, action, generation)
	if err != nil {
		return Credentials{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

// RotateKey re-encrypts the same grant without changing configuration or evidence.
func (s CredentialStore) RotateKey(ctx context.Context, a auth.Actor, service, connection string, generation int64) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	credentials, err := s.readTx(ctx, tx, a, service, connection, "connect", generation)
	if err != nil {
		return err
	}
	if err = s.PutTx(ctx, tx, a, service, connection, generation, credentials); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteTx can run after local retirement in the same transaction. Remote
// revocation must never prevent removing the local credential.
func (s CredentialStore) DeleteTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, connection string) error {
	if err := auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM evidence_connections WHERE service_id=$1 AND id=$2 FOR UPDATE`, service, connection); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM connection_credentials WHERE service_id=$1 AND connection_id=$2`, service, connection)
	return err
}
