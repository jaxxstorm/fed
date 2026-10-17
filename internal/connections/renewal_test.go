package connections

import (
	"bytes"
	"context"
	"encoding/base64"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRenewalPreservesRevisionRefreshTokenAndReceipt(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: Workspace, Name: "Directory", Mode: ServerManaged, Configuration: Configuration{AccountID: "customer-id"}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = s.Credentials.PutTx(ctx, tx, a, "svc", d.Connection.ID, 1, NewCredentials(map[string]string{"access_token": "old", "refresh_token": "offline"})); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.Registry, _ = NewRegistry(map[Provider]Adapter{Workspace: testAdapter{renew: func(context.Context, Connection, Credentials) (Credentials, error) {
		calls++
		return NewCredentials(map[string]string{"access_token": "new"}), nil
	}}})
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "renew"}
	first, err := s.Renew(ctx, a, in)
	if err != nil || first.State != "partial" {
		t.Fatalf("renew: %+v %v", first, err)
	}
	replay, err := s.Renew(ctx, a, in)
	if err != nil || replay.ID != first.ID || calls != 1 {
		t.Fatalf("renew replay: %+v %v", replay, err)
	}
	current, err := s.Get(ctx, a, "svc", d.Connection.ID)
	if err != nil || current.Connection.Revision != 1 || current.Connection.Generation != 2 {
		t.Fatal("renewal changed config revision")
	}
	credentials, err := s.Credentials.Get(ctx, a, "svc", d.Connection.ID, "check", 2)
	if err != nil || credentials.Value("refresh_token") != "offline" || credentials.Value("access_token") != "new" {
		t.Fatal("refresh grant lost")
	}
	var revisions int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_revisions`).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatal("renewal rewrote immutable metadata")
	}
}

func TestConcurrentRenewalAndDisconnectFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: Workspace, Name: "Directory", Mode: ServerManaged, Configuration: Configuration{AccountID: "customer"}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = s.Credentials.PutTx(ctx, tx, a, "svc", d.Connection.ID, 1, NewCredentials(map[string]string{"refresh_token": "offline"})); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var calls atomic.Int32
	s.Registry, _ = NewRegistry(map[Provider]Adapter{Workspace: testAdapter{renew: func(ctx context.Context, _ Connection, _ Credentials) (Credentials, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return NewCredentials(map[string]string{"access_token": "new"}), nil
		case <-ctx.Done():
			return Credentials{}, ctx.Err()
		}
	}}})
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "first"}
	type result struct {
		operation Operation
		err       error
	}
	done := make(chan result, 1)
	go func() { op, err := s.Renew(ctx, a, in); done <- result{op, err} }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("renewal did not start")
	}
	second := in
	second.IdempotencyKey = "second"
	op, err := s.Renew(ctx, a, second)
	if err != nil || op.State != "failed" || calls.Load() != 1 {
		t.Fatalf("parallel refresh escaped lock: %+v %v calls=%d", op, err, calls.Load())
	}
	disconnect := in
	disconnect.IdempotencyKey = "disconnect"
	if _, err = s.ChangeLifecycle(ctx, a, disconnect, "disconnect"); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	first := <-done
	if first.err != nil || first.operation.State != "cancelled" {
		t.Fatalf("late renewal: %+v %v", first.operation, first.err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_credentials`).Scan(&count); err != nil || count != 0 {
		t.Fatal("renewal restored disconnected credentials")
	}
}
