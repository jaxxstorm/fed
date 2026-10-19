package assessment

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/readiness"
)

type EvidenceCandidate struct {
	Evidence    Ref                 `json:"evidence"`
	Description string              `json:"description"`
	ObservedAt  time.Time           `json:"observed_at"`
	State       string              `json:"state"`
	Reason      string              `json:"reason"`
	Target      *observation.Target `json:"target,omitempty"`
	Limitations []string            `json:"limitations"`
}

type AssertionEvidence struct {
	Gaps        []AssertionGap        `json:"gaps,omitempty"`
	AssertionID string                `json:"assertion_id"`
	Obligation  string                `json:"obligation"`
	Acceptance  *readiness.Acceptance `json:"acceptance,omitempty"`
	States      []string              `json:"states"`
	Rationale   string                `json:"rationale"`
	Citations   []Citation            `json:"citations"`
	Result      *Ref                  `json:"result,omitempty"`
}

type RequirementEvidenceContext struct {
	CheckGroups   []observation.FindingGroup `json:"check_groups"`
	Blockers      []string                   `json:"blockers"`
	Assertions    []AssertionEvidence        `json:"assertions"`
	Candidates    []EvidenceCandidate        `json:"candidates"`
	FindingGroups []observation.FindingGroup `json:"finding_groups"`
	Collection    ObservationStatus          `json:"collection"`
	Limitations   []string                   `json:"limitations"`
}

func (s *Store) evidenceCoverage(ctx context.Context, a auth.Actor, in RuleInstructions, now time.Time) (RequirementEvidenceContext, error) {
	out := RequirementEvidenceContext{Blockers: in.Work.Blockers, Assertions: []AssertionEvidence{}, Candidates: []EvidenceCandidate{}, FindingGroups: []observation.FindingGroup{}, Limitations: []string{"Relevant artifacts are candidates until an assertion assessment cites and evaluates them. A passing configuration check is not a whole-requirement compliance decision."}}
	for _, record := range in.ExistingEvidence {
		e, err := Decode[Evidence](record)
		if err != nil {
			return out, err
		}
		candidate := EvidenceCandidate{Evidence: Ref{ID: record.ID, Revision: record.Revision}, Description: e.Description, ObservedAt: e.ObservedAt, Target: e.Target, State: "candidate", Reason: "Available for assertion assessment; relevance alone does not establish sufficiency.", Limitations: e.Limitations}
		if e.ScopeRevision != in.Binding.ScopeRevision {
			candidate.State = "stale"
			candidate.Reason = "The artifact is bound to an earlier service scope."
		} else if e.Document != nil && !slices.Contains(in.Binding.Sources, e.Document.Source) {
			candidate.State = "stale"
			candidate.Reason = "The artifact's exact evidence-source revision is no longer in the current instruction binding; recheck access, scope and freshness policy."
		} else if e.Document != nil && (e.Document.FreshnessHours <= 0 || !e.ObservedAt.Add(time.Duration(e.Document.FreshnessHours)*time.Hour).After(now)) {
			candidate.State = "stale"
			candidate.Reason = "The artifact is outside its source's saved freshness period."
		}
		out.Candidates = append(out.Candidates, candidate)
	}
	var stored storedRuleResult
	var resultRef *Ref
	r, err := s.Get(ctx, a, in.ServiceID, "rule_result", in.Rule.Requirement.ID)
	if err == nil {
		stored, err = Decode[storedRuleResult](r)
		if err != nil {
			return out, err
		}
		resultRef = &Ref{ID: r.ID, Revision: r.Revision}
	} else if !errors.Is(err, ErrNotFound) {
		return out, err
	}
	assessed := map[string]AssertionResult{}
	for _, assertion := range stored.Submission.Assertions {
		assessed[assertion.AssertionID] = assertion
	}
	criteria := map[string]readiness.Acceptance{}
	if in.Guidance != nil && in.Guidance.Guidance != nil {
		for _, book := range in.Guidance.Guidance.Playbooks {
			for _, assertion := range book.Assertions {
				criteria[assertion.ID] = assertion.Acceptance
			}
		}
	}
	for _, assertion := range in.Assertions {
		x := AssertionEvidence{AssertionID: assertion.ID, Obligation: assertion.Statement, States: []string{"missing"}, Rationale: "No current cited assessment demonstrates this obligation.", Citations: []Citation{}}
		if ac, ok := criteria[assertion.ID]; ok {
			x.Acceptance = &ac
		}
		if result, ok := assessed[assertion.ID]; ok {
			x.Gaps = result.Gaps
			x.Result = resultRef
			x.Citations = result.Citations
			x.Rationale = result.Rationale
			switch result.Outcome {
			case "supported":
				x.States = []string{"supported"}
			case "contradicted":
				x.States = []string{"contradicted"}
			default:
				x.States = []string{"missing"}
				if len(result.Citations) > 0 {
					x.States = []string{"partial"}
				}
			}
			if in.Work.State == "needs_refresh" || stored.Submission.Binding.AssertionHash != in.Binding.AssertionHash || stored.Submission.Binding.GuidanceVariant != in.Binding.GuidanceVariant {
				x.States = []string{"stale"}
				if result.Outcome == "contradicted" {
					x.States = append(x.States, "contradicted")
				}
				x.Rationale = "This assessment needs revalidation: " + result.Rationale
			}
			for _, gap := range result.Gaps {
				if !slices.Contains(x.States, gap.Kind) {
					x.States = append(x.States, gap.Kind)
				}
			}
		}
		out.Assertions = append(out.Assertions, x)
	}
	out.Collection, err = s.observationStatus(ctx, in.ServiceID, in.Rule.Requirement.ID, now)
	if err != nil {
		return out, err
	}
	out.FindingGroups = observation.GroupFindings(out.Collection.Findings)
	checks := []observation.Finding{}
	for _, check := range out.Collection.Checks {
		if check.Outcome != "fail" {
			checks = append(checks, check)
		}
	}
	out.CheckGroups = observation.GroupFindings(checks)
	if len(out.FindingGroups) > 0 {
		out.Limitations = append(out.Limitations, "Contradictory technical observations remain visible alongside assertion assessments. Their mapping to an individual assertion has not been inferred.")
	}
	if out.Collection.Expected > out.Collection.Observed {
		out.Limitations = append(out.Limitations, "Collection is incomplete for the expected target set; observations from covered targets do not establish coverage of the missing targets.")
	}
	if len(out.Collection.Errors) > 0 {
		out.Limitations = append(out.Limitations, "Collection errors are access or observation gaps, not proof that the requested controls are absent.")
	}
	return out, nil
}
