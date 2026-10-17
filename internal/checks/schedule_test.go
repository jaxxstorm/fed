package checks

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type observationCollector struct{}

func (observationCollector) Collect(ctx context.Context, s assessment.Scope, m assessment.Measure) (Result, error) {
	return Result{Observations: []Observation{configObservation("123456789012", "us-east-1", "arn:aws:rds:us-east-1:123456789012:db:fixture", "aws.rds/v1", map[string]any{"storage_encrypted": false}, observation.Evaluation{Condition: "storage_encryption", Outcome: "fail", Explanation: "Fixture encryption disabled"})}}, nil
}
func TestAutomaticCollectionToEvidence(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	artifacts := storage.Files{Directory: t.TempDir()}
	s := assessment.Store{DB: db, Artifacts: artifacts}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := s.CreateService(ctx, a, "Test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	id, err := catalog.Import(ctx, db, p, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, a, service.ID, 0, catalog.Profile{CatalogID: id, Type: "20x", Path: "Program", Class: "C", Date: "2026-10-16", Context: "initial"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveScope(ctx, a, service.ID, 0, assessment.Scope{Description: "East only", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	r := Runner{DB: db, Artifacts: artifacts, Registry: map[string]Collector{}}
	for _, template := range assessment.CollectionTemplates() {
		r.Registry[template.ID] = observationCollector{}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- r.Schedule(ctx) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM collection_batches`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate initial batch: %d %v", count, err)
	}
	for i := 0; i < 20; i++ {
		worked, err := r.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	status, err := s.Status(ctx, a, service.ID, "KSI-SVC-SIN", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if status.Draft != nil || status.Review != "pending" || status.Evidence != "current" || status.Collection.Count == 0 || len(status.Collection.Findings) == 0 {
		t.Fatalf("evidence without draft: %+v", status)
	}
	snapshot, err := s.PreparePublication(ctx, a, service.ID, "before-draft")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := assessment.Decode[assessment.Bundle](snapshot)
	if err != nil || len(bundle.Observations) == 0 || len(bundle.CollectionPlans) == 0 || len(bundle.CollectionConfigurations) == 0 {
		t.Fatal("collection missing from draft-free snapshot", err)
	}
	for _, state := range bundle.Statuses {
		if state.Draft != nil || state.Review != "pending" {
			t.Fatal("collection created review or draft")
		}
	}
	c, err := s.CollectionConfiguration(ctx, a, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Paused = true
	if _, err = s.ConfigureCollection(ctx, a, service.ID, c.Revision, c); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE collection_schedules SET next_run_at=now()-interval '3 days'`); err != nil {
		t.Fatal(err)
	}
	if err = r.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM collection_batches`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("paused schedule ran: %d %v", count, err)
	}
	batch, err := r.CollectNow(ctx, a, service.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.CollectNow(ctx, a, service.ID, "manual")
	if err != nil || batch.ID != again.ID {
		t.Fatal("batch retry not idempotent", err)
	}
	if len(batch.Jobs) == 0 || batch.Jobs[0].Input.ConfigurationRevision != c.Revision+1 {
		t.Fatal("batch did not freeze configuration")
	}
	if _, err = s.SaveScope(ctx, a, service.ID, 1, assessment.Scope{Description: "Changed boundary", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, a, service.ID, "KSI-SVC-SIN", time.Now())
	if err != nil || status.Collection.Count != 0 || status.Technical == "pass" {
		t.Fatal("obsolete scope credited as current", err)
	}
	shutdown, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if err = r.Run(shutdown); err != nil {
		t.Fatalf("worker shutdown: %v", err)
	}
}
