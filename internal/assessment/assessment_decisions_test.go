package assessment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
)

func TestReviewDecisionBindsExactAssessmentAndAuthority(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	in := ImplementationSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "initial", Binding: instructions.Binding, Assessment: implementationFixture(time.Now().UTC(), instructions.Assertions)}
	if _, err = s.SubmitImplementation(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	target, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `INSERT INTO readiness_content_reviews(organization_id,catalog_id,playbook_id,content_hash,reviewer_id) VALUES($1,$2,'KSI-CNA-RNT','reviewed-content',$3)`, a.OrganizationID, instructions.Binding.CatalogID, a.ID); err != nil {
		t.Fatal(err)
	}
	guidanceOnly, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil || guidanceOnly.Review != "pending" || guidanceOnly.Adoption != "pending" {
		t.Fatalf("guidance review approved service assessment: %+v %v", guidanceOnly, err)
	}
	d := AssessmentDecisionInput{Assessment: target.Assessment, Binding: instructions.Binding, DependencyHash: target.DependencyHash, Capacity: "internal_reviewer", Conclusion: "accepted", Procedures: "Reviewed unknown assertions and missing evidence", Sampling: "No operating records available", Notes: "Accept this as an accurate incomplete report, not compliance"}
	token, err := auth.IssueToken(ctx, s.DB, a, service, []string{"read", "edit", "export"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordAssessmentDecision(ctx, agent, service, "KSI-CNA-RNT", d); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("agent made human decision: %v", err)
	}
	d.Capacity = "independent_assessor"
	if _, err = s.RecordAssessmentDecision(ctx, a, service, "KSI-CNA-RNT", d); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("ordinary reviewer claimed independent capacity: %v", err)
	}
	d.Capacity = "internal_reviewer"
	decision, err := s.RecordAssessmentDecision(ctx, a, service, "KSI-CNA-RNT", d)
	if err != nil {
		t.Fatal(err)
	}
	if decision.ActorID != a.ID {
		t.Fatal("decision attribution missing")
	}
	target, err = s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil || target.Review != "accepted" || target.Adoption != "pending" {
		t.Fatalf("review conflated with adoption: %+v %v", target, err)
	}
	designation := AssessorDesignation{IdentityID: a.ID, Organization: "Independent assessor organization", Reason: "Service owner designation", ExpiresAt: time.Now().Add(time.Hour)}
	granted, err := s.SaveAssessorDesignation(ctx, a, service, "external-assessor", 0, designation)
	if err != nil {
		t.Fatal(err)
	}
	d.Capacity = "independent_assessor"
	if _, err = s.RecordAssessmentDecision(ctx, a, service, "KSI-CNA-RNT", d); err != nil {
		t.Fatal(err)
	}
	designation.Revoked = true
	if _, err = s.SaveAssessorDesignation(ctx, a, service, "external-assessor", granted.Revision, designation); err != nil {
		t.Fatal(err)
	}
	afterRevoke, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil || afterRevoke.IndependentReview != "review_needed" || afterRevoke.InternalReview != "accepted" {
		t.Fatalf("authority capacities confused: %+v %v", afterRevoke, err)
	}
	_, err = s.SaveGap(ctx, a, service, "other", "KSI-CNA-RNT", 0, Gap{Explanation: "Unrelated draft gap", Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil || unchanged.DependencyHash != target.DependencyHash {
		t.Fatalf("unbound change invalidated review: %+v %v", unchanged, err)
	}
	_, err = s.SaveScope(ctx, a, service, 1, Scope{Description: "Boundary changed", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.ReviewTarget(ctx, a, service, "KSI-CNA-RNT")
	if err != nil || changed.Review != "review_needed" {
		t.Fatalf("scope change retained current approval: %+v %v", changed, err)
	}
	if _, err = s.RecordAssessmentDecision(ctx, a, service, "KSI-CNA-RNT", d); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale review accepted: %v", err)
	}
}

func TestReviewTracksPolicyEvidencePeriodAndProfile(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	source := EvidenceSource{Type: "document", Name: "Policy", Locator: "https://docs.example/policy", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}
	if _, err := s.SaveEvidenceSource(ctx, a, service, "policy", 0, source); err != nil {
		t.Fatal(err)
	}
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "policy", Source: Ref{ID: "policy", Revision: 1}, ScopeRevision: 1, Locator: source.Locator, SourceVersion: "v1", MediaType: "text/plain", Content: "Network policy under review", ObservedAt: now, RetrievedAt: now, Method: "Read policy", Description: "Policy"})
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	v := implementationFixture(now, instructions.Assertions)
	for i := range v.Assertions {
		v.Assertions[i].Implementation.Citations = []Citation{{Evidence: Ref{ID: ev.ID, Revision: ev.Revision}, StartLine: 1, EndLine: 1}}
	}
	in := ImplementationSubmission{ServiceID: service, RequirementID: "KSI-CNA-RNT", IdempotencyKey: "assessment", Binding: instructions.Binding, Assessment: v}
	if _, err = s.SubmitImplementation(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	target, err := s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil {
		t.Fatal(err)
	}
	d := AssessmentDecisionInput{Assessment: target.Assessment, Binding: instructions.Binding, DependencyHash: target.DependencyHash, Capacity: "internal_reviewer", Conclusion: "accepted", Procedures: "Reviewed policy limitations", Sampling: "One document", Notes: "Unknown operating support retained"}
	if _, err = s.RecordAssessmentDecision(ctx, a, service, in.RequirementID, d); err != nil {
		t.Fatal(err)
	}
	unrelated := source
	unrelated.Locator = "https://docs.example/other"
	if _, err = s.SaveEvidenceSource(ctx, a, service, "other", 0, unrelated); err != nil {
		t.Fatal(err)
	}
	current, err := s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil || current.DependencyHash != target.DependencyHash || current.InternalReview != "accepted" {
		t.Fatalf("unrelated source invalidated review: %+v %v", current, err)
	}
	evidenceRecord, err := s.Get(ctx, a, service, "evidence", ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.mutate(ctx, a, evidenceRecord, evidenceRecord.Revision, "edit", nil); err != nil {
		t.Fatal(err)
	}
	afterEvidence, err := s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil || afterEvidence.DependencyHash == current.DependencyHash || afterEvidence.InternalReview != "review_needed" {
		t.Fatalf("relevant evidence revision retained approval: %+v %v", afterEvidence, err)
	}
	source.AccessHint = "Policy revision needs investigation"
	if _, err = s.SaveEvidenceSource(ctx, a, service, "policy", 1, source); err != nil {
		t.Fatal(err)
	}
	current, err = s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil || current.DependencyHash == afterEvidence.DependencyHash || current.InternalReview != "review_needed" {
		t.Fatalf("policy change retained approval: %+v %v", current, err)
	}
	// A new reporting period changes the assessment dependency even if its
	// narrative and unknown conclusions are otherwise unchanged.
	instructions, err = s.RuleInstructions(ctx, a, service, in.RequirementID)
	if err != nil {
		t.Fatal(err)
	}
	in.Binding = instructions.Binding
	in.ExpectedDraftRevision = instructions.ExpectedDraftRevision
	in.ExpectedResultRevision = instructions.ExpectedResultRevision
	in.IdempotencyKey = "new-period"
	in.Assessment = implementationFixture(now, instructions.Assertions)
	in.Assessment.Period.Start = now.Add(-48 * time.Hour)
	if _, err = s.SubmitImplementation(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	afterPeriod, err := s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil || afterPeriod.DependencyHash == current.DependencyHash || afterPeriod.InternalReview != "review_needed" {
		t.Fatalf("period change retained approval: %+v %v", afterPeriod, err)
	}
	profileRecord, err := s.Get(ctx, a, service, "profile", "profile")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := Decode[catalog.Profile](profileRecord)
	if err != nil {
		t.Fatal(err)
	}
	profile.Date = "2026-10-17"
	if _, err = s.SaveProfile(ctx, a, service, profileRecord.Revision, profile); err != nil {
		t.Fatal(err)
	}
	afterProfile, err := s.ReviewTarget(ctx, a, service, in.RequirementID)
	if err != nil || afterProfile.DependencyHash == afterPeriod.DependencyHash {
		t.Fatalf("profile revision not tracked: %+v %v", afterProfile, err)
	}
}
