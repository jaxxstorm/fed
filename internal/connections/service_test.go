package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type testAdapter struct {
	read     func(context.Context, Connection, Credentials, DiscoveredResource) (Snapshot, error)
	test     func(context.Context, Connection, Credentials) (AccessHealth, error)
	renew    func(context.Context, Connection, Credentials) (Credentials, error)
	discover func(context.Context, Connection, Credentials, string, int) (DiscoveryPage, error)
}

func (a testAdapter) Test(ctx context.Context, c Connection, k Credentials) (AccessHealth, error) {
	return a.test(ctx, c, k)
}
func (a testAdapter) Discover(ctx context.Context, c Connection, k Credentials, cursor string, limit int) (DiscoveryPage, error) {
	if a.discover == nil {
		return DiscoveryPage{}, ErrInvalid
	}
	return a.discover(ctx, c, k, cursor, limit)
}
func (a testAdapter) Renew(ctx context.Context, c Connection, k Credentials) (Credentials, error) {
	if a.renew == nil {
		return Credentials{}, ErrInvalid
	}
	return a.renew(ctx, c, k)
}
func (a testAdapter) Read(ctx context.Context, c Connection, k Credentials, r DiscoveredResource) (Snapshot, error) {
	if a.read == nil {
		return Snapshot{}, ErrInvalid
	}
	return a.read(ctx, c, k, r)
}

func setupService(t *testing.T) (*Service, auth.Actor) {
	t.Helper()
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('svc',$1,'Test'),('other',$1,'Other')`, admin.OrganizationID); err != nil {
		t.Fatal(err)
	}
	r, _ := NewRegistry(nil)
	return &Service{DB: db, Registry: r, Credentials: CredentialStore{DB: db}}, admin
}

func TestConfigureIdempotencyRevisionsAndIsolation(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	in := ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Production", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "prod"}}
	first, err := s.Configure(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Access.State != NotConfigured || first.Connection.Revision != 1 {
		t.Fatal("configuration implied verification")
	}
	replay, err := s.Configure(ctx, a, in)
	if err != nil || replay.Connection.ID != first.Connection.ID {
		t.Fatalf("retry: %v", err)
	}
	in.Name = "Different"
	if _, err = s.Configure(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("idempotency mismatch accepted")
	}
	in.ID, in.ExpectedRevision, in.IdempotencyKey = first.Connection.ID, 1, "edit"
	updated, err := s.Configure(ctx, a, in)
	if err != nil || updated.Connection.Revision != 2 || updated.Connection.Generation != 2 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	in.IdempotencyKey = "stale"
	if _, err = s.Configure(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit accepted")
	}
	in.ExpectedRevision, in.Configuration.AccountID = 2, "999999999999"
	if _, err = s.Configure(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("tenant widened")
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Configure(ctx, agent, in); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("legacy token configured connection")
	}
	if _, err = s.Get(ctx, agent, "other", first.Connection.ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("cross-service metadata read")
	}
	list, err := s.List(ctx, agent, "svc", false)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v", err)
	}
	var revisions, audits int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_revisions`).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='connection.configure'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if revisions != 2 || audits != 2 {
		t.Fatalf("replays created history: %d %d", revisions, audits)
	}
}

func TestAccessCheckRejectsLateResult(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	in := ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Production", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "prod"}}
	d, err := s.Configure(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	s.Registry, _ = NewRegistry(map[Provider]Adapter{AWS: testAdapter{test: func(context.Context, Connection, Credentials) (AccessHealth, error) {
		in.ID, in.ExpectedRevision, in.IdempotencyKey = d.Connection.ID, 1, "reconfigure"
		_, err := s.Configure(ctx, a, in)
		return AccessHealth{State: Verified, VerifiedAccountID: "123456789012"}, err
	}}})
	if _, err = s.Test(ctx, a, "svc", d.Connection.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("late result accepted: %v", err)
	}
	current, err := s.Get(ctx, a, "svc", d.Connection.ID)
	if err != nil || current.Access.State != NotConfigured {
		t.Fatal("late test overwrote health")
	}
}

func TestAccessReceiptReplaysWithoutProviderRead(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Production", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "prod"}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.Registry, _ = NewRegistry(map[Provider]Adapter{AWS: testAdapter{test: func(context.Context, Connection, Credentials) (AccessHealth, error) {
		calls++
		return AccessHealth{State: Verified, VerifiedAccountID: "123456789012"}, nil
	}}})
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "test"}
	first, err := s.TestWithReceipt(ctx, a, in)
	if err != nil || first.State != "succeeded" {
		t.Fatalf("test: %+v %v", first, err)
	}
	replay, err := s.TestWithReceipt(ctx, a, in)
	if err != nil || replay.ID != first.ID || calls != 1 {
		t.Fatalf("replay repeated read: %+v %v", replay, err)
	}
	in.Generation++
	if _, err = s.TestWithReceipt(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("changed test replay accepted")
	}
}
