package assessment

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jaxxstorm/fed/internal/readiness"
	"testing"
	"time"
)

// This fixture pins the pre-readiness contract: documentary evidence and its
// assessment/review remain immutable, and an export never follows live scope.
func TestReadinessMigrationHistoricalBaseline(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	source := EvidenceSource{Type: "document", Name: "Change procedure", Locator: "https://docs.example/changes", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}
	if _, err := s.SaveEvidenceSource(ctx, a, service, "procedure", 0, source); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "baseline-evidence", Source: Ref{"procedure", 1}, ScopeRevision: 1, Locator: source.Locator, SourceVersion: "v1", MediaType: "text/plain", Content: "Changes are reviewed before deployment.\n", ObservedAt: now, RetrievedAt: now, Method: "Fixture document read", Description: "Historical baseline fixture"})
	if err != nil {
		t.Fatal(err)
	}
	rid := "KSI-SVC-ACM"
	instructions, err := s.RuleInstructions(ctx, a, service, rid)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.SubmitRuleResult(ctx, a, RuleResultSubmission{ServiceID: service, RequirementID: rid, IdempotencyKey: "baseline-result", Binding: instructions.Binding, Explanation: "Synthetic baseline, not production compliance evidence", RequestedState: "evidence_complete", Assertions: []AssertionResult{{AssertionID: "whole-statement", Outcome: "supported", Rationale: "Synthetic fixture supports the assertion", Citations: []Citation{{Evidence: Ref{ev.ID, 1}, StartLine: 1, EndLine: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(ctx, a, service, rid, Review{Assessment: receipt.Draft}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.PreparePublication(ctx, a, service, "before-readiness")
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := s.Export(ctx, a, service, snapshot.ID, "json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline Bundle
	if err = json.Unmarshal(before, &baseline); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"scope", "evidence", "assessment", "review", "rule_result"} {
		found := false
		for _, r := range baseline.Records {
			if r.Kind == kind {
				found = true
			}
		}
		if !found {
			t.Fatalf("baseline missing %s", kind)
		}
	}
	if baseline.Format != "ksi-assessment-bundle/v1" {
		t.Fatal("legacy export format changed")
	}
	if _, err = s.SaveScope(ctx, a, service, 1, Scope{Description: "Changed live boundary", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1", "us-west-2"}}}}); err != nil {
		t.Fatal(err)
	}
	after, _, err := s.Export(ctx, a, service, snapshot.ID, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("historical export followed live scope")
	}
	for _, r := range baseline.Records {
		if r.Kind != "evidence" && r.Kind != "review" && r.Kind != "assessment" {
			continue
		}
		var historical json.RawMessage
		if err := s.DB.QueryRow(ctx, `SELECT data FROM records WHERE service_id=$1 AND kind=$2 AND id=$3 AND revision=$4`, service, r.Kind, r.ID, r.Revision).Scan(&historical); err != nil {
			t.Fatal(err)
		}
		var saved, exported bytes.Buffer
		if err := json.Compact(&saved, historical); err != nil {
			t.Fatal(err)
		}
		if err := json.Compact(&exported, r.Data); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved.Bytes(), exported.Bytes()) {
			t.Fatalf("historical %s changed", r.Kind)
		}
	}
	// A historical whole-statement approval cannot satisfy newly split assertions.
	s.Guidance = &readiness.Store{DB: s.DB}
	s.guidanceContexts = map[string]readiness.GuidanceContext{
		rid: {
			State: "reviewed", VariantID: "new-variant", ReleaseRevision: 1, ReleaseHash: "new-release",
			Guidance: &readiness.Package{Playbooks: []readiness.Playbook{
				{Kind: "actionable", Assertions: []readiness.Assertion{
					{ID: "automation", Obligation: readiness.Statement{Text: "Demonstrate automation"}},
					{ID: "drift-operation", Obligation: readiness.Statement{Text: "Demonstrate repeated drift review"}},
				}},
			}},
		},
	}
	work, err := s.ruleWork(ctx, a, service, rid, instructions.Binding, time.Now().UTC())
	if err != nil || work.State != "needs_refresh" || len(work.MissingAssertions) != 2 {
		t.Fatal("historical whole-statement review satisfied changed assertions", work, err)
	}
	frozen, _, err := s.Export(ctx, a, service, snapshot.ID, "json")
	if err != nil || !bytes.Equal(before, frozen) {
		t.Fatal("new guidance rewrote historical export", err)
	}
}
