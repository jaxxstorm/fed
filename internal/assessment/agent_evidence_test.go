package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func agentFixture(t *testing.T) (*Store, auth.Actor, string) {
	t.Helper()
	ctx := context.Background()
	db := testutil.Database(t)
	s := &Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := s.CreateService(ctx, a, "Agent workflow")
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
	id, err := catalog.Import(ctx, db, payload, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, a, svc.ID, 0, catalog.Profile{CatalogID: id, Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: "2026-10-16"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveScope(ctx, a, svc.ID, 0, Scope{Description: "Saved boundary", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	return s, a, svc.ID
}

func TestReusableDocumentaryEvidence(t *testing.T) {
	var err error
	s, a, service := agentFixture(t)
	ctx := context.Background()
	source := EvidenceSource{Type: "document", Name: "Security policies", Locator: "https://docs.example/security", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}
	if _, err := s.SaveEvidenceSource(ctx, a, service, "policies", 0, source); err != nil {
		t.Fatal(err)
	}
	in := ArtifactSubmission{ServiceID: service, IdempotencyKey: "policy", Source: Ref{"policies", 1}, ScopeRevision: 1, Locator: "https://docs.example/security/policy", SourceVersion: "v1", MediaType: "text/markdown", Content: "# Security policy\nAll changes require review.\n", ObservedAt: time.Now().UTC().Add(-time.Hour), RetrievedAt: time.Now().UTC(), Method: "Read approved policy", Description: "Policy excerpt"}
	var wg sync.WaitGroup
	records := make(chan Record, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := s.SubmitEvidenceArtifact(ctx, a, in); records <- r; errs <- e }()
	}
	wg.Wait()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	ev := <-records
	if other := <-records; ev.ID != other.ID {
		t.Fatal("duplicate evidence on concurrent retry")
	}
	changed := in
	changed.Content = "Different"
	if _, err := s.SubmitEvidenceArtifact(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	outside := in
	outside.IdempotencyKey = "outside"
	outside.Locator = "https://docs.example/security-other/policy"
	if _, err := s.SubmitEvidenceArtifact(ctx, a, outside); err == nil {
		t.Fatal("source path escaped")
	}
	oversized := in
	oversized.IdempotencyKey = "oversized"
	oversized.Content = strings.Repeat("x", MaxAgentArtifactBytes+1)
	if _, err = s.SubmitEvidenceArtifact(ctx, a, oversized); !errors.Is(err, observation.ErrTooLarge) {
		t.Fatal("size limit not enforced", err)
	}
	for _, rid := range []string{"KSI-SVC-ACM", "KSI-SVC-SIN"} {
		i, err := s.RuleInstructions(ctx, a, service, rid)
		if err != nil {
			t.Fatal(err)
		}
		if len(i.Assertions) != 1 || i.Assertions[0].Statement != i.Rule.Statement {
			t.Fatal("guidance omits official statement")
		}
		if rid == "KSI-SVC-ACM" && i.Binding.CatalogID == MappedCatalog && i.GuidanceKind != "reviewed_application_guidance" {
			t.Fatal("reviewed guidance missing")
		}
		sub := RuleResultSubmission{ServiceID: service, RequirementID: rid, IdempotencyKey: rid, Binding: i.Binding, Explanation: "Policy records the assertion; fixture analysis", RequestedState: "evidence_complete", Assertions: []AssertionResult{{AssertionID: "whole-statement", Outcome: "supported", Rationale: "Cited source supports assertion", Citations: []Citation{{Evidence: Ref{ev.ID, 1}, StartLine: 1, EndLine: 2}}}}}
		missing := sub
		missing.IdempotencyKey += "-missing"
		missing.Assertions = nil
		if _, err = s.SubmitRuleResult(ctx, a, missing); !errors.Is(err, ErrInvalid) {
			t.Fatal("missing assertion completed", err)
		}
		stale := sub
		stale.IdempotencyKey += "-stale"
		stale.Binding.ScopeRevision++
		if _, err = s.SubmitRuleResult(ctx, a, stale); !errors.Is(err, ErrConflict) {
			t.Fatal("obsolete scope completed", err)
		}
		receipt, err := s.SubmitRuleResult(ctx, a, sub)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Work.State != "evidence_complete" || receipt.Review != "pending" {
			t.Fatalf("receipt: %+v", receipt)
		}
		assertPresentationParity(t, s, a, service, rid, time.Now())
		replay, err := s.SubmitRuleResult(ctx, a, sub)
		if err != nil || replay.Result != receipt.Result {
			t.Fatal("receipt retry", err)
		}
		concurrent := sub
		concurrent.IdempotencyKey += "-other-agent"
		if _, err = s.SubmitRuleResult(ctx, a, concurrent); !errors.Is(err, ErrConflict) {
			t.Fatal("concurrent result overwritten", err)
		}
		listed, err := s.List(ctx, a, service, "evidence", rid, 100, 0)
		if err != nil || len(listed) != 1 || listed[0].ID != ev.ID {
			t.Fatal("reused evidence not listed", err)
		}
		status, err := s.Status(ctx, a, service, rid, time.Now())
		if err != nil || status.Evidence != "current" || status.Work.State != "evidence_complete" {
			t.Fatalf("document without measures: %+v %v", status, err)
		}
		draft, err := Decode[Draft](*status.Draft)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.SaveDraft(ctx, a, service, rid, 1, draft); err != nil {
			t.Fatal("legacy draft route rejected explicit link", err)
		}
		if _, err = s.Review(ctx, a, service, rid, Review{Assessment: Ref{rid, 2}}); err != nil {
			t.Fatal(err)
		}
		assertPresentationParity(t, s, a, service, rid, time.Now())
		if rid == "KSI-SVC-ACM" {
			legacy, err := s.AttachEvidence(ctx, a, service, rid, "legacy", Evidence{Source: "external", URL: "https://example.com/report", Description: "Legacy drift report", ScopeRevision: 1, ObservedAt: in.ObservedAt})
			if err != nil {
				t.Fatal(err)
			}
			measure, err := s.SaveMeasure(ctx, a, service, "legacy-freshness", rid, 0, Measure{Name: "Drift history", Rationale: "Legacy freshness policy", ScopeRevision: 1, FreshnessHours: 24})
			if err != nil {
				t.Fatal(err)
			}
			mixed := draft
			mixed.Evidence = append(mixed.Evidence, Ref{legacy.ID, legacy.Revision})
			mixed.Measures = []Ref{{measure.ID, measure.Revision}}
			if _, err = s.SaveDraft(ctx, a, service, rid, 2, mixed); err != nil {
				t.Fatal(err)
			}
			status, err := s.Status(ctx, a, service, rid, time.Now().Add(25*time.Hour))
			if err != nil || status.Evidence != "stale" {
				t.Fatalf("mixed evidence policy: %+v %v", status, err)
			}
		}
		status, err = s.Status(ctx, a, service, rid, time.Now().Add(49*time.Hour))
		assertPresentationParity(t, s, a, service, rid, time.Now().Add(49*time.Hour))
		if err != nil || status.Evidence != "stale" || status.Work.State != "needs_refresh" || status.Review != "review-needed" {
			t.Fatalf("expiry: %+v %v", status, err)
		}
	}
	var count int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind='evidence' AND data ? 'document'`, service).Scan(&count); err != nil || count != 1 {
		t.Fatal("artifact copied per rule", count, err)
	}
	page, err := s.ListOpenRules(ctx, a, OpenRulesInput{ServiceID: service, Limit: 1})
	if err != nil || page.NextCursor == "" {
		t.Fatal("pagination unavailable", err)
	}
	s.clock = func() time.Time { return time.Now().Add(49 * time.Hour) }
	if _, err = s.ListOpenRules(ctx, a, OpenRulesInput{ServiceID: service, Limit: 1, Cursor: page.NextCursor}); !errors.Is(err, ErrViewChanged) {
		t.Fatal("expiry did not invalidate cursor", err)
	}
	s.clock = nil
	all, err := s.Requirements(ctx, a, service)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"KSI", "FRR"} {
		expected := map[string]string{}
		for _, rule := range all {
			if rule.Requirement.Kind == kind && rule.Applicability != "not_applicable" && rule.Requirement.ID != "KSI-SVC-ACM" && rule.Requirement.ID != "KSI-SVC-SIN" {
				expected[rule.Requirement.ID] = rule.Applicability
			}
		}
		cursor := ""
		seen := map[string]bool{}
		for {
			p, e := s.ListOpenRules(ctx, a, OpenRulesInput{ServiceID: service, Kind: kind, Limit: 100, Cursor: cursor})
			if e != nil {
				t.Fatal(e)
			}
			for _, item := range p.Items {
				if seen[item.RequirementID] || expected[item.RequirementID] != item.Applicability {
					t.Fatal("incorrect filter or duplicate", item)
				}
				seen[item.RequirementID] = true
			}
			cursor = p.NextCursor
			if cursor == "" {
				break
			}
		}
		if len(seen) != len(expected) {
			t.Fatal("open rules omitted applicability states", kind, len(seen), len(expected))
		}
	}
	if _, err = s.SaveMeasure(ctx, a, service, "new-finding", "KSI-SVC-SIN", 0, Measure{Name: "Encryption", Rationale: "New observation fixture", ScopeRevision: 1, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	_, err = s.IngestObservation(ctx, a, observation.Submission{ServiceID: service, ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "new-failure", ObservedAt: time.Now().UTC(), Method: "Fixture inspection", ToolVersion: "fixture/v1", Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:rds:us-east-1:123456789012:db:test"}, Coverage: observation.Coverage{Expected: 1, Observed: 1, Complete: true, State: "observed"}, Artifact: observation.Artifact{Schema: "aws.rds/v1", Configuration: map[string]json.RawMessage{"storage_encrypted": json.RawMessage(`false`)}}, Links: []observation.Link{{RequirementID: "KSI-SVC-SIN", Measure: observation.Ref{ID: "new-finding", Revision: 1}, MappingVersion: "fixture", Rationale: "Fixture encryption mapping", Conditions: []string{"storage_encryption"}}}, Evaluations: []observation.Evaluation{{Condition: "storage_encryption", Outcome: "fail", Explanation: "Observed encryption disabled"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ rid, state string }{{"KSI-SVC-SIN", "needs_refresh"}, {"KSI-SVC-ACM", "evidence_complete"}} {
		assertPresentationParity(t, s, a, service, tc.rid, time.Now())
		status, e := s.Status(ctx, a, service, tc.rid, time.Now())
		if e != nil || status.Work.State != tc.state {
			t.Fatal("relevant finding invalidation", tc, status.Work, e)
		}
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-SVC-SIN")
	if err != nil {
		t.Fatal(err)
	}
	coverage := instructions.EvidenceCoverage
	if coverage == nil || len(coverage.FindingGroups) != 1 || len(coverage.Candidates) == 0 {
		t.Fatal("evidence matrix lost candidate artifacts or contradictory findings", coverage)
	}
	group := coverage.FindingGroups[0]
	if !group.Explanation.Known || group.Explanation.Observed != "The database configuration reports storage encryption disabled." || group.AffectedTargets != 1 {
		t.Fatal("finding interpretation lost retained configuration", group)
	}
	for _, assertion := range coverage.Assertions {
		if len(assertion.States) == 1 && assertion.States[0] == "supported" {
			t.Fatal("new contradiction left an unqualified support claim", assertion)
		}
	}
	detail, err := s.ObservationDetail(ctx, a, service, group.Findings[0].ObservationID)
	if err != nil || detail.ArtifactHash == "" || string(detail.Observation.Artifact.Configuration["storage_encrypted"]) != "false" {
		t.Fatal("finding drill-down lost exact observation", detail, err)
	}
	snapshot, err := s.PreparePublication(ctx, a, service, "documentary")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := Decode[Bundle](snapshot)
	if err != nil || len(bundle.Statuses) != len(bundle.Requirements) {
		t.Fatal("snapshot omitted open requirements", err)
	}
	source.FreshnessHours = 72
	if _, err = s.SaveEvidenceSource(ctx, a, service, "policies", 1, source); err != nil {
		t.Fatal(err)
	}
	status, err := s.Status(ctx, a, service, "KSI-SVC-ACM", time.Now())
	if err != nil || status.Work.State != "needs_refresh" {
		t.Fatal("source policy change not reflected", err)
	}
	stored, err := s.Get(ctx, a, service, "snapshot", snapshot.ID)
	var frozen, original any
	decodeErr := json.Unmarshal(stored.Data, &frozen)
	originalErr := json.Unmarshal(snapshot.Data, &original)
	if err != nil || decodeErr != nil || originalErr != nil || !reflect.DeepEqual(frozen, original) {
		t.Fatal("snapshot mutated", err)
	}
	if err = storage.Migrate(ctx, s.DB); err != nil {
		t.Fatal("repeat migration", err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind='evidence'`, service).Scan(&count); err != nil || count != 3 {
		t.Fatal("migration altered existing evidence", err)
	}
}

func TestCitationBoundaries(t *testing.T) {
	root := ""
	good := "/a~1b/0"
	bad := "/a~1b/01"
	invalid := "/a~2b"
	for _, tc := range []struct {
		content, media string
		c              Citation
		valid          bool
	}{
		{`{"a/b":[true]}`, "application/json", Citation{JSONPointer: &good}, true},
		{`{"a/b":[true]}`, "application/json", Citation{JSONPointer: &root}, true},
		{`{"a/b":[true]}`, "application/json", Citation{JSONPointer: &bad}, false},
		{`{}`, "application/json", Citation{JSONPointer: &invalid}, false},
		{"one\ntwo\n", "text/plain", Citation{StartLine: 1, EndLine: 2}, true},
		{"one\ntwo\n", "text/plain", Citation{StartLine: 1, EndLine: 3}, false},
	} {
		if err := validateCitation([]byte(tc.content), tc.media, tc.c); (err == nil) != tc.valid {
			t.Fatalf("citation %+v: %v", tc, err)
		}
	}
}

func TestSourceBoundariesAndArtifactFailures(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	if err := s.ReconcileEvidenceSources(ctx, a, service); err != nil {
		t.Fatal(err)
	}
	sources, err := s.List(ctx, a, service, "evidence_source", "", 100, 0)
	if err != nil || len(sources) != 1 {
		t.Fatal("scope source not seeded", err)
	}
	source, err := Decode[EvidenceSource](sources[0])
	if err != nil {
		t.Fatal(err)
	}
	source.AccessHint = "customer-profile"
	source.FreshnessHours = 96
	if _, err = s.SaveEvidenceSource(ctx, a, service, sources[0].ID, 1, source); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileEvidenceSources(ctx, a, service); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveEvidenceSource(ctx, a, service, sources[0].ID, 1, source); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit accepted", err)
	}
	outside := source
	outside.Region = "eu-west-2"
	outside.Locator = outside.AccountID + "/" + outside.Region
	if _, err = s.SaveEvidenceSource(ctx, a, service, "outside", 0, outside); !errors.Is(err, ErrInvalid) {
		t.Fatal("region expanded", err)
	}
	if _, err = s.SaveScope(ctx, a, service, 1, Scope{Description: "Updated boundary", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileEvidenceSources(ctx, a, service); err != nil {
		t.Fatal(err)
	}
	current, err := s.Get(ctx, a, service, "evidence_source", sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	preserved, err := Decode[EvidenceSource](current)
	if err != nil || preserved.AccessHint != "customer-profile" || preserved.FreshnessHours != 96 || preserved.ScopeRevision != 2 {
		t.Fatal("source metadata overwritten", err)
	}
	in := ArtifactSubmission{ServiceID: service, IdempotencyKey: "failure", Source: Ref{current.ID, current.Revision}, ScopeRevision: 2, Locator: preserved.Locator, SourceVersion: "api-v1", MediaType: "application/json", Content: `{"encrypted":true}`, Description: "Configuration", Method: "Describe", ObservedAt: time.Now().Add(-time.Minute), RetrievedAt: time.Now()}
	file := filepath.Join(t.TempDir(), "not-directory")
	if err = os.WriteFile(file, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	originalStore := s.Artifacts
	s.Artifacts = storage.Files{Directory: file}
	if _, err = s.SubmitEvidenceArtifact(ctx, a, in); err == nil {
		t.Fatal("storage failure ignored")
	}
	s.Artifacts = originalStore
	if _, err = s.DB.Exec(ctx, `CREATE FUNCTION reject_document() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='evidence' THEN RAISE EXCEPTION 'fixture commit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_document BEFORE INSERT ON records FOR EACH ROW EXECUTE FUNCTION reject_document()`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitEvidenceArtifact(ctx, a, in); err == nil {
		t.Fatal("database failure ignored")
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind='evidence'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed upload left evidence", err)
	}
	if _, err = s.DB.Exec(ctx, `DROP TRIGGER reject_document ON records`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitEvidenceArtifact(ctx, a, in); err != nil {
		t.Fatal("failed operation not retryable", err)
	}
}
