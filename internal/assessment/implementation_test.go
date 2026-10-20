package assessment

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHistoricalImplementationEvidenceSeparatesCurrentFreshness(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	eventTime := now.Add(-30 * 24 * time.Hour)
	_, err := s.SaveEvidenceSource(ctx, a, service, "exercises", 0, EvidenceSource{Type: "operational_records", Name: "Exercise records", Locator: "https://docs.example/exercises", ScopeRevision: 1, Enabled: true, FreshnessHours: 48})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "historical-exercise", Source: Ref{ID: "exercises", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/exercises/restore", SourceVersion: "approved-v1", MediaType: "text/plain", Content: "Restore exercise completed; reviewer recorded the outcome.", ObservedAt: eventTime, RetrievedAt: now, Method: "Read archived exercise record", Description: "Historical exercise"})
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	citation := Citation{Evidence: Ref{ID: evidence.ID, Revision: evidence.Revision}, StartLine: 1, EndLine: 1}
	v := implementationFixture(now, instructions.Assertions)
	v.Period = AssessmentPeriod{Start: eventTime.Add(-time.Hour), End: eventTime.Add(time.Hour)}
	for i := range v.Assertions {
		v.Assertions[i].Implementation = EvidenceSupport{Outcome: "unknown", Rationale: "Exercise alone does not describe network implementation", Citations: []Citation{citation}}
		v.Assertions[i].Operating = EvidenceSupport{Outcome: "unknown", Rationale: "Historical record sampled; does not establish full requirement", Citations: []Citation{citation}}
		v.Assertions[i].Sample = OperationalSample{Method: "Read one archived exercise", SampleSize: 1, Coverage: "sample", CoverageReason: "Only one dated event", PopulationUnknownReason: "Full population unknown", Events: []OperatingEvent{{Citation: citation, OccurredAt: eventTime, Description: "Exercise performed"}}}
	}
	legacy := RuleResultSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "legacy-historical", Binding: instructions.Binding, Assertions: implementationAssertionResults(v), Explanation: "Archived exercise", RequestedState: "partial"}
	if _, err = s.SubmitRuleResult(ctx, a, legacy); !errors.Is(err, ErrInvalid) {
		t.Fatalf("legacy freshness check changed: %v", err)
	}
	in := ImplementationSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "period-historical", Binding: instructions.Binding, Assessment: v}
	receipt, err := s.SubmitImplementation(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.CurrentFreshness != "stale" || !receipt.Readiness.Ready || receipt.Period != v.Period {
		t.Fatalf("period/current conflated: %+v", receipt)
	}
	current, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	if current.Implementation == nil || current.Implementation.CurrentFreshness != "stale" || !current.Implementation.Readiness.Ready || current.Work.State != "needs_refresh" {
		t.Fatalf("historical support refreshed current work: %+v", current)
	}
	in.Binding = current.Binding
	in.ExpectedResultRevision = current.ExpectedResultRevision
	in.ExpectedDraftRevision = current.ExpectedDraftRevision
	in.IdempotencyKey = "invalid-selector"
	in.Assessment.Assertions[0].Implementation.Citations[0].EndLine = 99
	if _, err = s.SubmitImplementation(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("historical citation bypassed selector validation: %v", err)
	}
}

func implementationFixture(now time.Time, assertions []Assertion) ImplementationAssessment {
	unknown := NarrativeField{State: "unknown", Reason: "No organizational records supplied yet."}
	v := ImplementationAssessment{Version: ImplementationAssessmentVersion, Period: AssessmentPeriod{Start: now.Add(-24 * time.Hour), End: now}, Narrative: ImplementationNarrative{Description: unknown, ResponsibleRoles: unknown, Mechanisms: unknown, Procedures: unknown, Frequency: unknown, Boundary: unknown}}
	for _, a := range assertions {
		support := EvidenceSupport{Outcome: "unknown", Rationale: "No evidence retrieved", Citations: []Citation{}, Limitations: []string{"Operating records unavailable"}}
		v.Assertions = append(v.Assertions, ImplementationAssertion{AssertionID: a.ID, Claim: "Implementation is not yet established by evidence.", Implementation: support, Operating: support, Sample: OperationalSample{Method: "No records available", PopulationUnknownReason: "Population not established", Coverage: "none", CoverageReason: "No sampled events", Events: []OperatingEvent{}}})
	}
	return v
}

func TestStructuralReadinessDoesNotApproveUnknownClaims(t *testing.T) {
	now := time.Now().UTC()
	assertions := []Assertion{{ID: "a", Required: true}}
	v := implementationFixture(now, assertions)
	r, err := ImplementationReadiness(v, assertions, now)
	if err != nil || !r.Ready || r.Basis == "" {
		t.Fatalf("explicit unknown not reviewable: %+v %v", r, err)
	}
	if v.Assertions[0].Operating.Outcome != "unknown" {
		t.Fatal("readiness changed agent conclusion")
	}
	v.Assertions[0].Operating.Outcome = "supported"
	if _, err = ImplementationReadiness(v, assertions, now); err == nil {
		t.Fatal("configuration-free assertion of operating support accepted without events/citations")
	}
	v = implementationFixture(now, assertions)
	v.Assertions = nil
	r, err = ImplementationReadiness(v, assertions, now)
	if err != nil || r.Ready || len(r.Blockers) != 1 {
		t.Fatalf("missing assertion not blocked: %+v %v", r, err)
	}
}

func TestOperatingSampleMustMatchPeriodAndPopulation(t *testing.T) {
	now := time.Now().UTC()
	assertions := []Assertion{{ID: "a"}}
	v := implementationFixture(now, assertions)
	c := Citation{Evidence: Ref{ID: "e", Revision: 1}, StartLine: 1, EndLine: 1}
	v.Assertions[0].Operating = EvidenceSupport{Outcome: "supported", Rationale: "Dated review record", Citations: []Citation{c}}
	v.Assertions[0].Sample = OperationalSample{Method: "Read selected review", SampleSize: 1, Coverage: "sample", CoverageReason: "One review only", PopulationUnknownReason: "Total not known", Events: []OperatingEvent{{Citation: c, OccurredAt: now.Add(-time.Hour), Description: "Review completed"}}}
	if _, err := ImplementationReadiness(v, assertions, now); err != nil {
		t.Fatal(err)
	}
	v.Assertions[0].Sample.Coverage = "population"
	if _, err := ImplementationReadiness(v, assertions, now); err == nil {
		t.Fatal("unknown population reported as complete")
	}
	v.Assertions[0].Sample.Coverage = "sample"
	v.Assertions[0].Sample.Events[0].OccurredAt = now.Add(-48 * time.Hour)
	if _, err := ImplementationReadiness(v, assertions, now); err == nil {
		t.Fatal("event outside period accepted")
	}
}

func TestImplementationSubmissionRetainsAtomicDraftChain(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	in := ImplementationSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "structured", Binding: instructions.Binding, ExpectedResultRevision: instructions.ExpectedResultRevision, ExpectedDraftRevision: instructions.ExpectedDraftRevision, Assessment: implementationFixture(time.Now().UTC(), instructions.Assertions)}
	r, err := s.SubmitImplementation(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Readiness.Ready {
		t.Fatalf("explicitly unknown draft not reviewable: %+v", r)
	}
	retry, err := s.SubmitImplementation(ctx, a, in)
	if err != nil || retry.Draft != r.Draft {
		t.Fatalf("retry lost receipt: %+v %v", retry, err)
	}
	changed := in
	changed.Assessment.Narrative.Description.Reason = "Different investigation result"
	if _, err := s.SubmitImplementation(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting idempotency payload accepted: %v", err)
	}
	record, err := s.Get(ctx, a, service, "assessment", "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := Decode[Draft](record)
	if err != nil || draft.Implementation == nil {
		t.Fatalf("structured payload lost: %v", err)
	}
	work, err := s.WorkStatus(ctx, a, service, "KSI-CNA-RNT", time.Now())
	if err != nil || work.Assessment == "supported" {
		t.Fatalf("unknown was approved: %+v %v", work, err)
	}
	in.IdempotencyKey = "stale"
	if _, err = s.SubmitImplementation(ctx, a, in); err != ErrConflict {
		t.Fatalf("stale write accepted: %v", err)
	}
	in.ExpectedResultRevision = r.Result.Revision
	in.ExpectedDraftRevision = r.Draft.Revision
	in.Binding.ScopeRevision++
	if _, err = s.SubmitImplementation(ctx, a, in); err != ErrConflict {
		t.Fatalf("wrong scope accepted: %v", err)
	}
}

func TestImplementationContradictionIsNotHiddenByUnknownOperation(t *testing.T) {
	v := ImplementationAssessment{Assertions: []ImplementationAssertion{{AssertionID: "a", Claim: "Configuration contradicts intended access", Implementation: EvidenceSupport{Outcome: "contradicted"}, Operating: EvidenceSupport{Outcome: "unknown"}}}}
	results := implementationAssertionResults(v)
	if len(results) != 1 || results[0].Outcome != "contradicted" {
		t.Fatalf("contradiction lost: %+v", results)
	}
}
