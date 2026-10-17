package checks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type fixtureCollector struct{}

func (fixtureCollector) Collect(context.Context, assessment.Scope, assessment.Measure) (Result, error) {
	return Result{Observations: []Observation{{Scope: "account/region-a", Outcome: "pass"}, {Scope: "account/region-b", Outcome: "error", Error: "access_denied"}}}, nil
}

func TestDurableRunner(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, a, service.ID, 0, assessment.Scope{Description: "Fixture scope"}); err != nil {
		t.Fatal(err)
	}
	m, _ := json.Marshal(assessment.Measure{CheckID: "fixture", ScopeRevision: 1, Name: "fixture", FreshnessHours: 24})
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'measure','m',1,'KSI-SVC-ACM',$2,$3)`, service.ID, a.ID, m); err != nil {
		t.Fatal(err)
	}
	r := Runner{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}, Registry: map[string]Collector{"fixture": fixtureCollector{}}}
	input := Input{ScopeRevision: 1, Measure: assessment.Ref{ID: "m", Revision: 1}}
	j, err := r.Enqueue(ctx, a, service.ID, "KSI-SVC-ACM", "fixture", "key", input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := r.Enqueue(ctx, a, service.ID, "KSI-SVC-ACM", "fixture", "key", input)
	if err != nil || replay.ID != j.ID {
		t.Fatalf("retry: %v", err)
	}
	if _, err = r.Enqueue(ctx, a, service.ID, "KSI-SVC-ACM", "shell", "key", input); !errors.Is(err, assessment.ErrInvalid) {
		t.Fatal("unknown check accepted")
	}
	claimed, err := r.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.claim(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("claimed a leased job twice")
	}
	if _, err = db.Exec(ctx, `UPDATE jobs SET lease_until=now()-interval '1 second' WHERE id=$1`, claimed.ID); err != nil {
		t.Fatal(err)
	}
	worked, err := r.RunOnce(ctx)
	if err != nil || !worked {
		t.Fatalf("recovery: %v", err)
	}
	got, err := r.Get(ctx, a, service.ID, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "succeeded" || got.Attempts != 2 || got.Result.Outcome != "error" || got.Result.Observed != 1 || got.Result.Expected != 2 || len(got.Result.EvidenceIDs) != 1 {
		t.Fatalf("result: %+v %+v", got, got.Result)
	}
	var outcome string
	if err = db.QueryRow(ctx, `SELECT outcome FROM job_attempts WHERE job_id=$1 AND attempt=1`, j.ID).Scan(&outcome); err != nil || outcome != "lease_expired" {
		t.Fatalf("attempt: %s %v", outcome, err)
	}
	_, err = r.Enqueue(ctx, a, service.ID, "KSI-SVC-ACM", "fixture", "timeout", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE jobs SET deadline=now()-interval '1 second' WHERE state='queued'`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.claim(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("expired job claimed")
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE state='timed-out'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("timeout persistence: %d %v", count, err)
	}
	_, err = r.Enqueue(ctx, a, service.ID, "KSI-SVC-ACM", "fixture", "backoff", input)
	if err != nil {
		t.Fatal(err)
	}
	retryJob, err := r.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.retry(ctx, retryJob, errors.New("transient fixture failure")); err != nil {
		t.Fatal(err)
	}
	if _, err = r.claim(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("retry did not back off", err)
	}
	if _, err = db.Exec(ctx, `UPDATE jobs SET available_at=now()-interval '1 second' WHERE id=$1`, retryJob.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.claim(ctx); err != nil {
		t.Fatal("backed-off job not recoverable", err)
	}
}
