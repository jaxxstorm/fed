package assessment

import (
	"context"
	"github.com/jaxxstorm/fed/internal/auth"
)

func implementationAssertionResults(v ImplementationAssessment) []AssertionResult {
	out := make([]AssertionResult, 0, len(v.Assertions))
	for _, a := range v.Assertions {
		outcome := "supported"
		if a.Implementation.Outcome == "unknown" || a.Operating.Outcome == "unknown" {
			outcome = "unknown"
		}
		if a.Implementation.Outcome == "contradicted" || a.Operating.Outcome == "contradicted" {
			outcome = "contradicted"
		}
		citations := append([]Citation{}, a.Implementation.Citations...)
		citations = append(citations, a.Operating.Citations...)
		out = append(out, AssertionResult{AssertionID: a.AssertionID, Outcome: outcome, Rationale: a.Claim, Citations: citations})
	}
	return out
}

// SubmitImplementation preserves the existing atomic result/draft revision and
// citation-validation transaction. It never performs a human review.
func (s *Store) SubmitImplementation(ctx context.Context, a auth.Actor, in ImplementationSubmission) (ImplementationReceipt, error) {
	explanation := in.Assessment.Narrative.Description.Text
	if explanation == "" {
		explanation = in.Assessment.Narrative.Description.Reason
	}
	if explanation == "" {
		explanation = "Implementation narrative incomplete; agent investigation requires review."
	}
	r, err := s.SubmitRuleResult(ctx, a, RuleResultSubmission{Implementation: &in.Assessment, ServiceID: in.ServiceID, RequirementID: in.RequirementID, IdempotencyKey: in.IdempotencyKey, Binding: in.Binding, ExpectedResultRevision: in.ExpectedResultRevision, ExpectedDraftRevision: in.ExpectedDraftRevision, Explanation: explanation, RequestedState: "partial"})
	if err != nil {
		return ImplementationReceipt{}, err
	}
	return ImplementationReceipt{Result: r.Result, Draft: r.Draft, Readiness: *r.StructuralReadiness, CurrentFreshness: r.CurrentFreshness, Period: in.Assessment.Period}, nil
}
