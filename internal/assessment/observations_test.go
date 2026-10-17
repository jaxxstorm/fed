package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestObservationIngestion(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	s := Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := s.CreateService(ctx, a, "Test")
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
	cid, err := catalog.Import(ctx, db, p, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, a, svc.ID, 0, catalog.Profile{CatalogID: cid, Type: "20x", Path: "Program", Class: "C", Date: "2026-10-16", Context: "initial"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveScope(ctx, a, svc.ID, 0, Scope{Description: "East only", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}, Clusters: []string{"prod"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveMeasure(ctx, a, svc.ID, "encryption", "KSI-SVC-SIN", 0, Measure{Name: "Encryption", Rationale: "At rest", FreshnessHours: 48, ScopeRevision: 1}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, a, svc.ID, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	in := observation.Submission{ServiceID: svc.ID, ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "test", Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:rds:us-east-1:123456789012:db:test"}, ObservedAt: time.Now().UTC(), Method: "DescribeDBInstances", ToolVersion: "aws-cli/2", Coverage: observation.Coverage{Expected: 1, Observed: 1, Complete: true, State: "observed"}, Artifact: observation.Artifact{Schema: "aws.rds/v1", Configuration: map[string]json.RawMessage{"storage_encrypted": json.RawMessage(`false`)}}, Links: []observation.Link{{RequirementID: "KSI-SVC-SIN", Measure: observation.Ref{ID: "encryption", Revision: 1}, MappingVersion: "v1", Rationale: "At rest"}}, Evaluations: []observation.Evaluation{{Condition: "encryption", Outcome: "fail", Explanation: "Storage encryption disabled"}}}
	var wg sync.WaitGroup
	results := make(chan observation.Receipt, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out, err := s.IngestObservation(ctx, agent, in); results <- out; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	var receipt observation.Receipt
	for out := range results {
		if id != "" && id != out.ID {
			t.Fatal("duplicate ingestion")
		}
		id = out.ID
		receipt = out
	}
	if receipt.SourceKind != "agent_observation" || receipt.ActorID != agent.ID || len(receipt.Evidence) != 1 {
		t.Fatalf("provenance: %+v", receipt)
	}
	artifact, err := s.Artifact(ctx, agent, svc.ID, receipt.Evidence[0].ID)
	if err != nil || storage.Hash(artifact) != receipt.ArtifactHash {
		t.Fatalf("artifact: %v", err)
	}
	changed := in
	changed.Method = "different"
	if _, err = s.IngestObservation(ctx, agent, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("key conflict: %v", err)
	}
	bad := in
	bad.IdempotencyKey = "outside"
	bad.Target.Region = "eu-west-2"
	if _, err = s.IngestObservation(ctx, agent, bad); !errors.Is(err, observation.ErrScope) {
		t.Fatalf("scope: %v", err)
	}
	bad = in
	bad.IdempotencyKey = "invalid-link"
	bad.Links = []observation.Link{{RequirementID: "KSI-SVC-SIN", Measure: observation.Ref{ID: "other", Revision: 1}, MappingVersion: "v1", Rationale: "bad"}}
	if _, err = s.IngestObservation(ctx, agent, bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reference: %v", err)
	}
	var n int
	partial := in
	partial.IdempotencyKey = "partial"
	partial.Coverage = observation.Coverage{Expected: 2, Observed: 1, State: "partial"}
	partial.Evaluations = []observation.Evaluation{{Condition: "encryption", Outcome: "pass", Explanation: "Incomplete scan"}}
	if _, err = s.IngestObservation(ctx, agent, partial); err != nil {
		t.Fatal(err)
	}
	status, err := s.Status(ctx, agent, svc.ID, "KSI-SVC-SIN", time.Now())
	if err != nil || len(status.Collection.Findings) != 1 {
		t.Fatal("partial scan closed finding", err)
	}
	oldPass := in
	oldPass.IdempotencyKey = "old-pass"
	oldPass.ObservedAt = in.ObservedAt.Add(-time.Hour)
	oldPass.Evaluations = partial.Evaluations
	if _, err = s.IngestObservation(ctx, agent, oldPass); err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, agent, svc.ID, "KSI-SVC-SIN", time.Now())
	if err != nil || len(status.Collection.Findings) != 1 {
		t.Fatal("old observation closed newer finding", err)
	}
	freshPass := in
	freshPass.IdempotencyKey = "fresh-pass"
	freshPass.ObservedAt = in.ObservedAt.Add(time.Second)
	freshPass.Evaluations = partial.Evaluations
	if _, err = s.IngestObservation(ctx, agent, freshPass); err != nil {
		t.Fatal(err)
	}
	status, err = s.Status(ctx, agent, svc.ID, "KSI-SVC-SIN", time.Now())
	if err != nil || len(status.Collection.Findings) != 0 {
		t.Fatal("fresh complete scan did not resolve finding", err)
	}
	broken := s
	broken.Artifacts = storage.Files{Directory: "/dev/null/artifacts"}
	failure := in
	failure.IdempotencyKey = "storage-failure"
	if _, err = broken.IngestObservation(ctx, agent, failure); err == nil {
		t.Fatal("storage failure accepted")
	}
	if _, err = db.Exec(ctx, `CREATE FUNCTION reject_observation_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='observation.ingested' THEN RAISE EXCEPTION 'fixture audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_observation_audit()`); err != nil {
		t.Fatal(err)
	}
	failure.IdempotencyKey = "transaction-failure"
	if _, err = s.IngestObservation(ctx, agent, failure); err == nil {
		t.Fatal("audit failure accepted")
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM observations`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("partial mutation: %d %v", n, err)
	}
	if err = auth.RevokeToken(ctx, db, a, svc.ID, storage.Hash([]byte(token))); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Artifact(ctx, agent, svc.ID, receipt.Evidence[0].ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("revoked artifact access: %v", err)
	}
}
