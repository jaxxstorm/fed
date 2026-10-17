package connections

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestCredentialEncryptionBindsContext(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	k, err := NewKeyring("v1", map[string]string{"v1": key})
	if err != nil {
		t.Fatal(err)
	}
	aad := credentialAAD("org", "svc", "conn", "github", 1)
	id, nonce, sealed, err := k.seal([]byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}
	for _, context := range [][]byte{credentialAAD("other", "svc", "conn", "github", 1), credentialAAD("org", "other", "conn", "github", 1), credentialAAD("org", "svc", "other", "github", 1), credentialAAD("org", "svc", "conn", "aws", 1), credentialAAD("org", "svc", "conn", "github", 2)} {
		if _, err := k.open(id, nonce, sealed, context); !errors.Is(err, ErrCredentialUnavailable) {
			t.Fatal("cross-context decryption accepted")
		}
	}
	if plain, err := k.open(id, nonce, sealed, aad); err != nil || string(plain) != "secret" {
		t.Fatal("round trip failed")
	}
	if _, err := NewKeyring("missing", map[string]string{"v1": key}); err == nil {
		t.Fatal("missing active key accepted")
	}
	var absent *Keyring
	if _, err := absent.open(id, nonce, sealed, aad); err == nil {
		t.Fatal("missing keyring accepted")
	}
}

func TestCredentialStorageRotationAndRetirement(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('svc',$1,'Test'),('other',$1,'Other')`, a.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,actor_id) VALUES('svc','conn','github',1,1,'active','{}',$1)`, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), "v2": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))}
	k, _ := NewKeyring("v1", keys)
	s := CredentialStore{DB: db, Keys: k}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = s.PutTx(ctx, tx, a, "svc", "conn", 1, NewCredentials(map[string]string{"token": "unique-secret-value"})); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	credentials, err := s.Get(ctx, a, "svc", "conn", "check", 1)
	if err != nil || credentials.Value("token") != "unique-secret-value" {
		t.Fatalf("read: %v", err)
	}
	if _, err = s.Get(ctx, a, "other", "conn", "check", 1); err == nil {
		t.Fatal("cross-service credential read")
	}
	if _, err = s.Get(ctx, a, "svc", "conn", "read", 1); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("read-only credential access")
	}
	s.Keys, _ = NewKeyring("v2", keys)
	if err = s.RotateKey(ctx, a, "svc", "conn", 1); err != nil {
		t.Fatal(err)
	}
	s.Keys, _ = NewKeyring("v2", map[string]string{"v2": keys["v2"]})
	if _, err = s.Get(ctx, a, "svc", "conn", "check", 1); err != nil {
		t.Fatal("rotation still needs old key")
	}
	var ciphertext []byte
	if err = db.QueryRow(ctx, `SELECT ciphertext FROM connection_credentials`).Scan(&ciphertext); err != nil || bytes.Contains(ciphertext, []byte("unique-secret-value")) {
		t.Fatal("plaintext persisted")
	}
	if _, err = db.Exec(ctx, `UPDATE evidence_connections SET lifecycle='deleted',generation=2 WHERE service_id='svc'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, a, "svc", "conn", "check", 1); !errors.Is(err, ErrInactive) {
		t.Fatal("retired credential read")
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = s.DeleteTx(ctx, tx, a, "svc", "conn"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM connection_credentials`).Scan(&count); err != nil || count != 0 {
		t.Fatal("local credentials retained")
	}
}
