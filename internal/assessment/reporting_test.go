package assessment

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReportingCandidatesRetryWithoutPublishing(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	end := time.Now().UTC().Add(-time.Minute)
	in := PrepareReleaseInput{ServiceID: service, IdempotencyKey: "on-demand", Period: AssessmentPeriod{Start: end.Add(-time.Hour), End: end}, Public: PublicRelease{ServiceName: "Candidate service", Contacts: []string{}, Requirements: []PublicRequirement{}}}
	id, err := s.QueueReportingCandidate(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.QueueReportingCandidate(ctx, a, in)
	if err != nil || again != id {
		t.Fatalf("queue retry: %s %v", again, err)
	}
	changed := in
	changed.Public.Summary = "changed"
	if _, err = s.QueueReportingCandidate(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("queue conflict: %v", err)
	}
	// Missing service context produces durable failure and owner notification.
	if err = s.RunReportingOnce(ctx); err != nil {
		t.Fatal(err)
	}
	runs, err := s.ReportingRuns(ctx, a, service, 10, 0)
	if err != nil || len(runs) != 1 || runs[0].State != "failed" || runs[0].Notification == "" {
		t.Fatalf("failed candidate: %+v %v", runs, err)
	}
	if _, err = s.SaveServiceContext(ctx, a, service, 0, ServiceContext{Description: "Service", Boundary: "Boundary", Responsibilities: "Provider", Contacts: []string{"owner@example.com"}, Deliverables: []ReleaseDeliverable{}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveEvidenceSource(ctx, a, service, "operations", 0, EvidenceSource{Type: "operational_records", Name: "Operations", Locator: "https://docs.example/operations", ScopeRevision: 1, Enabled: true, FreshnessHours: 1}); err != nil {
		t.Fatal(err)
	}
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "operations", Source: Ref{ID: "operations", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/operations", SourceVersion: "v1", MediaType: "text/plain", Content: "Dated operational sample", ObservedAt: end.Add(-10 * time.Minute), RetrievedAt: end, Method: "Read operational sample", Description: "Operations"})
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	vAssessment := implementationFixture(end, instructions.Assertions)
	vAssessment.Period = in.Period
	for i := range vAssessment.Assertions {
		vAssessment.Assertions[i].Implementation.Citations = []Citation{{Evidence: Ref{ID: ev.ID, Revision: ev.Revision}, StartLine: 1, EndLine: 1}}
	}
	if _, err = s.SubmitImplementation(ctx, a, ImplementationSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "operations-assessment", Binding: instructions.Binding, Assessment: vAssessment}); err != nil {
		t.Fatal(err)
	}
	target, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	for _, capacity := range []string{"organizational_adoption", "internal_reviewer"} {
		if _, err = s.RecordAssessmentDecision(ctx, a, service, "KSI-CNA-RNT", AssessmentDecisionInput{Assessment: target.Assessment, Binding: instructions.Binding, DependencyHash: target.DependencyHash, Capacity: capacity, Conclusion: "accepted", Procedures: "Reviewed dated sample and unknown conclusions", Sampling: "One record", Notes: "Reviewed incomplete report, not compliance"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DB.Exec(ctx, `UPDATE assessment_reporting_runs SET retry_at=now() WHERE service_id=$1`, service); err != nil {
		t.Fatal(err)
	}
	if err = s.RunReportingOnce(ctx); err != nil {
		t.Fatal(err)
	}
	runs, err = s.ReportingRuns(ctx, a, service, 10, 0)
	if err != nil || runs[0].State != "succeeded" || runs[0].Attempts != 2 {
		t.Fatalf("retry: %+v %v", runs, err)
	}
	candidate, err := s.GetRelease(ctx, a, service, runs[0].CandidateID)
	if err != nil || candidate.State != "prepared" {
		t.Fatalf("candidate auto approved: %+v %v", candidate, err)
	}
	if err = s.ConfigureTrustCentre(ctx, a, service, "reporting-service", true); err != nil {
		t.Fatal(err)
	}
	transition := ReleaseTransition{ServiceID: service, ID: candidate.ID, ExpectedRevision: 1, ManifestHash: candidate.ManifestHash, AudienceHash: candidate.AudienceHash, Action: "approve", Reason: "Reviewed incomplete report", IdempotencyKey: "approve"}
	if _, err = s.TransitionRelease(ctx, a, transition); err != nil {
		t.Fatal(err)
	}
	transition.Action = "publish"
	transition.ExpectedRevision = 2
	transition.IdempotencyKey = "publish"
	if _, err = s.TransitionRelease(ctx, a, transition); err != nil {
		t.Fatal(err)
	}
	v := ReportingSchedule{Enabled: true, IntervalHours: 24, NextPeriodStart: end.Add(-2 * time.Hour), NextRunAt: end, Configuration: in}
	v.Configuration.PreviousReleaseID = candidate.ID
	s.clock = func() time.Time { return end.Add(3 * time.Hour) }
	if err = s.SaveReportingSchedule(ctx, a, service, 0, v); err != nil {
		t.Fatal(err)
	}
	if err = s.RunReportingOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.RunReportingOnce(ctx); err != nil {
		t.Fatal(err)
	}
	runs, err = s.ReportingRuns(ctx, a, service, 10, 0)
	if err != nil || len(runs) != 2 {
		t.Fatalf("duplicate schedule: %+v %v", runs, err)
	}
	newCandidate, err := s.GetRelease(ctx, a, service, runs[0].CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	stale := false
	for _, requirement := range newCandidate.Manifest.Requirements {
		if requirement.Implementation != nil && requirement.Implementation.CurrentFreshness == "stale" {
			stale = true
		}
	}
	if !stale || len(newCandidate.Manifest.Changes) == 0 {
		t.Fatal("new reporting cycle did not surface expired support")
	}
	original, err := s.GetRelease(ctx, a, service, candidate.ID)
	if err != nil || original.ManifestHash != candidate.ManifestHash || original.State != "published" {
		t.Fatalf("previous candidate changed: %v", err)
	}
}
