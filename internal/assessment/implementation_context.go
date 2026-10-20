package assessment

import (
	"context"
	"strings"
	"time"
)

func (s *Store) implementationContext(ctx context.Context, in RuleInstructions, draft, result Record) (*ImplementationContext, error) {
	out := &ImplementationContext{CurrentFreshness: "missing", Review: "pending", Readiness: StructuralReviewReadiness{Basis: "structural_only; agent conclusions require human review", Blockers: []AssessmentReadinessBlocker{}}}
	block := func(code, explanation, action string) {
		out.Readiness.Ready = false
		out.Readiness.Blockers = append(out.Readiness.Blockers, AssessmentReadinessBlocker{Code: code, Explanation: explanation, NextAction: action})
	}
	if draft.Revision == 0 {
		block("implementation_missing", "No structured implementation assessment has been submitted.", "Investigate the sources and submit an implementation assessment.")
		return out, nil
	}
	v, err := Decode[Draft](draft)
	if err != nil {
		return nil, err
	}
	out.Draft = &Ref{ID: draft.ID, Revision: draft.Revision}
	out.Assessment = v.Implementation
	if v.Implementation == nil {
		block("legacy_draft_enrichment", "This draft has no structured narrative or operational period.", "Enrich the existing draft through submit_implementation_assessment.")
		return out, nil
	}
	r, err := ImplementationReadiness(*v.Implementation, in.Assertions, s.now())
	if err != nil {
		block("assessment_context_changed", "The stored assessment requires revalidation against the current assertions.", "Read current instructions and submit a revised assessment.")
	} else {
		out.Readiness = r
	}
	if result.Revision == 0 {
		block("result_missing", "The structured result is unavailable.", "Submit a revised assessment.")
		return out, nil
	}
	stored, err := Decode[storedRuleResult](result)
	if err != nil {
		return nil, err
	}
	if stored.Receipt.Draft != *out.Draft || string(data(stored.Submission.Binding)) != string(data(in.Binding)) {
		block("binding_changed", "The draft or instruction bindings changed after submission.", "Reassess against the current source, profile and scope revisions.")
	}
	for _, ref := range v.Evidence {
		if out.CurrentFreshness == "missing" {
			out.CurrentFreshness = "current"
		}
		er, err := s.presentationReference(ctx, in.ServiceID, "evidence", ref)
		if err != nil {
			return nil, err
		}
		ev, err := Decode[Evidence](er)
		if err != nil {
			return nil, err
		}
		var hours *int
		if ev.Document != nil {
			hours = &ev.Document.FreshnessHours
		} else {
			err = s.query().QueryRow(ctx, `SELECT min((data->>'freshness_hours')::int) FROM (SELECT DISTINCT ON(id) data FROM records WHERE service_id=$1 AND kind='measure' AND requirement_id=$2 ORDER BY id,revision DESC) m`, in.ServiceID, in.Rule.Requirement.ID).Scan(&hours)
			if err != nil {
				return nil, err
			}
		}
		if hours == nil || !ev.ObservedAt.Add(time.Duration(*hours)*time.Hour).After(s.now()) {
			out.CurrentFreshness = "stale"
		}
		if ev.ScopeRevision != in.Binding.ScopeRevision {
			block("evidence_scope_changed", "Evidence belongs to a different scope revision.", "Revalidate evidence against the selected boundary.")
		}
		if s.Artifacts == nil || ev.ArtifactKey == "" {
			block("artifact_unavailable", "A cited artifact is unavailable.", "Restore access to the exact cited artifact.")
			continue
		}
		if _, err = s.Artifacts.Get(ctx, ev.ArtifactKey); err != nil {
			block("artifact_unavailable", "A cited artifact could not be verified.", "Restore access to the exact cited artifact.")
		}
	}
	// Current freshness does not rewrite the agent's historical-period conclusion.
	// Other working-state reasons remain visible even when structure is reviewable.
	for _, reason := range in.Work.Reasons {
		if !strings.Contains(reason, "expired") {
			block("working_context_changed", reason, "Investigate the changed inputs before review.")
		}
	}
	return out, nil
}
