package assessment

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestAssertionGapsRetainScopePeriodAndBlockCompletion(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	const rid = "KSI-RPL-TRC"
	source := EvidenceSource{Type: "document", Name: "Development recovery report", Locator: "https://docs.example/recovery", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}
	if _, err := s.SaveEvidenceSource(ctx, a, service, "recovery", 0, source); err != nil {
		t.Fatal(err)
	}
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "development-report", Source: Ref{"recovery", 1}, ScopeRevision: 1, Locator: source.Locator, SourceVersion: "v1", MediaType: "text/markdown", Content: "Development restore test executed. Production was not tested.\n", ObservedAt: time.Now().UTC(), RetrievedAt: time.Now().UTC(), Method: "Read retained recovery report", Description: "Development recovery record"})
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.RuleInstructions(ctx, a, service, rid)
	if err != nil {
		t.Fatal(err)
	}
	gap := AssertionGap{Kind: "access_blocked", Scope: "Production recovery records", Period: "Current review period", Reason: "Only development test records were supplied; production records are not accessible to the assessor.", NextAction: "Ask the recovery owner for authorized production test records and objective comparisons."}
	sub := RuleResultSubmission{ServiceID: service, RequirementID: rid, IdempotencyKey: "scoped-gap", Binding: i.Binding, Explanation: "Development execution does not establish production recovery coverage.", RequestedState: "partial", Assertions: []AssertionResult{{AssertionID: i.Assertions[0].ID, Outcome: "supported", Rationale: "Development evidence only; production remains unassessed.", Citations: []Citation{{Evidence: Ref{ev.ID, ev.Revision}, StartLine: 1, EndLine: 1}}, Gaps: []AssertionGap{gap}}}}
	if _, err = s.SubmitRuleResult(ctx, a, sub); !errors.Is(err, ErrInvalid) {
		t.Fatal("supported assertion accepted unresolved gap", err)
	}
	sub.Assertions[0].Outcome = "unknown"
	sub.RequestedState = "evidence_complete"
	if _, err = s.SubmitRuleResult(ctx, a, sub); !errors.Is(err, ErrInvalid) {
		t.Fatal("gap bypassed completion gate", err)
	}
	sub.RequestedState = "partial"
	receipt, err := s.SubmitRuleResult(ctx, a, sub)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Work.Assessment != "unknown" || receipt.Review != "pending" {
		t.Fatal("partial gap changed assessment/review", receipt)
	}
	i, err = s.RuleInstructions(ctx, a, service, rid)
	if err != nil {
		t.Fatal(err)
	}
	x := i.EvidenceCoverage.Assertions[0]
	if !slices.Contains(x.States, "partial") || !slices.Contains(x.States, "access_blocked") || !reflect.DeepEqual(x.Gaps, []AssertionGap{gap}) || len(x.Citations) != 1 {
		t.Fatal("exact scoped gap or partial citation lost", x)
	}
	replay, err := s.SubmitRuleResult(ctx, a, sub)
	if err != nil || replay.Result != receipt.Result {
		t.Fatal("gap retry lost idempotency", err)
	}
	// A contradictory observation and an access gap must coexist, not flatten
	// into an optimistic partial-support state.
	sub.Binding = i.Binding
	sub.ExpectedResultRevision = receipt.Result.Revision
	sub.ExpectedDraftRevision = receipt.Draft.Revision
	sub.IdempotencyKey = "contradiction-and-gap"
	sub.Assertions[0].Outcome = "contradicted"
	if _, err = s.SubmitRuleResult(ctx, a, sub); err != nil {
		t.Fatal(err)
	}
	i, err = s.RuleInstructions(ctx, a, service, rid)
	if err != nil {
		t.Fatal(err)
	}
	x = i.EvidenceCoverage.Assertions[0]
	if !slices.Contains(x.States, "contradicted") || !slices.Contains(x.States, "access_blocked") {
		t.Fatal("contradiction or access gap hidden", x)
	}
}
