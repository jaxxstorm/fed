package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestAuthorizationTransactionsAndTokens(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	s := Store{DB: db}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.CreateService(ctx, a, "Ignis")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateService(ctx, a, "Other")
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, a, v.ID, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.Authorize(ctx, db, agent, other.ID, "read"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("cross-service: %v", err)
	}
	if err = auth.Authorize(ctx, db, agent, v.ID, "review"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("agent review: %v", err)
	}
	r := Record{ServiceID: v.ID, Kind: "gap", ID: "gap", Data: json.RawMessage(`{"explanation":"missing evidence"}`)}
	_, err = s.mutate(ctx, agent, r, 0, "edit", func(tx pgx.Tx) error { return errors.New("validation failed") })
	if err == nil {
		t.Fatal("expected failure")
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM records WHERE id='gap'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("record rollback: %d %v", count, err)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE target='gap'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit rollback: %d %v", count, err)
	}
	if _, err = s.mutate(ctx, agent, r, 0, "edit", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.mutate(ctx, agent, r, 0, "edit", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE target='gap'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit: %d %v", count, err)
	}
	if err = auth.RevokeToken(ctx, db, a, v.ID, storage.Hash([]byte(token))); err != nil {
		t.Fatal(err)
	}
	if err = auth.Authorize(ctx, db, agent, v.ID, "read"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("revocation: %v", err)
	}
}
