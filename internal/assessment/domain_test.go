package assessment

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestAssessmentLifecycle(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	s := Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := s.CreateService(ctx, a, "Ignis")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	catalogID, err := catalog.Import(ctx, db, payload, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	profile := catalog.Profile{CatalogID: catalogID, Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: "2026-10-16"}
	if _, err = s.SaveScope(ctx, a, service.ID, 0, Scope{Description: "Production accounts"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, a, service.ID, 0, profile); err != nil {
		t.Fatal(err)
	}
	measure, err := s.SaveMeasure(ctx, a, service.ID, "m", "KSI-SVC-ACM", 0, Measure{Name: "Drift evidence", Rationale: "Review drift history", FreshnessHours: 24, ScopeRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ev := Evidence{Source: "trust-centre", URL: "https://trust.example/report", Description: "Drift report", ScopeRevision: 1, ObservedAt: now}
	evidence, err := s.AttachEvidence(ctx, a, service.ID, "KSI-SVC-ACM", "retry-key", ev)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.AttachEvidence(ctx, a, service.ID, "KSI-SVC-ACM", "retry-key", ev)
	if err != nil || replay.ID != evidence.ID {
		t.Fatalf("replay: %v", err)
	}
	draft := Draft{Explanation: "Configuration is managed with Pulumi", Evidence: []Ref{{evidence.ID, 1}}, Measures: []Ref{{measure.ID, 1}}, ProfileRevision: 1, ScopeRevision: 1}
	if _, err = s.SaveDraft(ctx, a, service.ID, "KSI-SVC-ACM", 0, draft); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveDraft(ctx, a, service.ID, "KSI-SVC-ACM", 0, draft); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	draft.Evidence = []Ref{{"not-real", 1}}
	if _, err = s.SaveDraft(ctx, a, service.ID, "KSI-SVC-ACM", 1, draft); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad citation: %v", err)
	}
	status, err := s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now)
	if err != nil || status.Review != "pending" {
		t.Fatalf("status: %+v %v", status, err)
	}
	_, err = db.Exec(ctx, `INSERT INTO jobs(id,service_id,actor_id,requirement_id,check_id,input,state,deadline,result) VALUES('test-run',$1,$2,'KSI-SVC-ACM','fixture','{}','succeeded',now()+interval '1 hour','{"outcome":"pass"}')`, service.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now)
	if err != nil || status.Technical != "pass" || status.Review != "pending" {
		t.Fatalf("pass implies review: %+v %v", status, err)
	}
	if _, err = s.Review(ctx, a, service.ID, "KSI-SVC-ACM", Review{Assessment: Ref{"KSI-SVC-ACM", 1}}); err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now)
	if err != nil || status.Review != "reviewed" {
		t.Fatalf("review: %+v %v", status, err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now.Add(25*time.Hour))
	if err != nil || status.Review != "review-needed" || status.Evidence != "stale" {
		t.Fatalf("expiry: %+v %v", status, err)
	}
	_, err = db.Exec(ctx, `UPDATE jobs SET result='{"outcome":"fail"}',updated_at=now()+interval '1 second' WHERE id='test-run'`)
	if err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now)
	if err != nil || status.Review != "review-needed" || status.Technical != "fail" {
		t.Fatalf("new failure: %+v %v", status, err)
	}
	profile.Class = "C"
	diff, err := s.PreviewProfile(ctx, a, service.ID, profile)
	if err != nil || len(diff) == 0 {
		t.Fatalf("preview: %v", err)
	}
	if _, err = s.SaveProfile(ctx, a, service.ID, 1, profile); err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-ACM", now)
	if err != nil || status.Review != "review-needed" {
		t.Fatalf("profile change: %+v %v", status, err)
	}
	var revision int
	if err = db.QueryRow(ctx, `SELECT (data->>'profile_revision')::int FROM records WHERE kind='assessment' AND service_id=$1`, service.ID).Scan(&revision); err != nil || revision != 1 {
		t.Fatal("historical profile overwritten")
	}
	other, err := s.CreateService(ctx, a, "Other")
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, a, other.ID, []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Artifact(ctx, actor, service.ID, evidence.ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("artifact permission: %v", err)
	}
}
