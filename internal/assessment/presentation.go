package assessment

import (
	"fmt"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/readiness"
	"slices"
)

type RequirementNextStep struct {
	Title       string   `json:"title"`
	Explanation string   `json:"explanation"`
	Targets     []string `json:"targets"`
	Action      string   `json:"action"`
	Provider    string   `json:"provider,omitempty"`
}

// RequirementPresentation is the common status vocabulary used by browser
// summaries and details. The source domain states remain available separately.
type RequirementPresentation struct {
	ContextGaps      []RequirementNextStep          `json:"context_gaps,omitempty"`
	Readiness        *readiness.RequirementProgress `json:"readiness,omitempty"`
	GuidanceReason   string                         `json:"guidance_reason,omitempty"`
	GuidanceState    string                         `json:"guidance_state,omitempty"`
	FindingGroups    []observation.FindingGroup     `json:"finding_groups,omitempty"`
	NextSteps        []RequirementNextStep          `json:"next_steps,omitempty"`
	HumanReviewed    bool                           `json:"human_reviewed"`
	Status           string                         `json:"status"`
	Label            string                         `json:"label"`
	Reason           string                         `json:"reason"`
	Action           string                         `json:"action"`
	EvidenceComplete bool                           `json:"evidence_complete"`
	ReviewRemaining  bool                           `json:"review_remaining"`
	Secondary        []string                       `json:"secondary"`
}

// WithGuidanceAction adds source-backed collection actions without changing the
// compliance result or allowing guidance to hide findings and access failures.
func WithGuidanceAction(p RequirementPresentation, g readiness.GuidanceContext) RequirementPresentation {
	p.GuidanceState = g.State
	p.GuidanceReason = g.Reason
	p.Readiness = g.Progress
	if g.State != "reviewed" || g.Guidance == nil || len(g.Guidance.Playbooks) != 1 {
		return p
	}
	book := g.Guidance.Playbooks[0]
	if p.Status == "needs_scoping" && book.Kind == "needs_decision" {
		p.Action = book.DecisionOwnerRole + ": " + book.DecisionQuestion
		p.Reason = book.Applicability.Text
		return p
	}
	if book.Kind != "actionable" || (p.Status != "not_started" && p.Status != "in_progress" && p.Status != "needs_evidence") {
		return p
	}
	setAction := func(title, action, reason string) RequirementPresentation {
		p.Action = action
		p.NextSteps = append([]RequirementNextStep{{Title: title, Explanation: reason, Action: action, Targets: []string{}}}, p.NextSteps...)
		return p
	}
	if g.Progress != nil {
		if len(g.Progress.Questions) > 0 {
			q := g.Progress.Questions[0]
			return setAction("Resolve prerequisite applicability", q.Question, q.Reason+" An unanswered condition does not establish applicability.")
		}
		for _, c := range g.Progress.Capabilities {
			if c.NeedsRevalidation || c.Blocked || c.Implementation != "declared_present" || c.EvidenceState != "available_for_assessment" {
				action := c.Name + ": " + c.NextAction
				if !c.NeedsRevalidation && c.Implementation != "declared_present" && len(c.Steps.Items) > 0 {
					step := c.Steps.Items[0]
					action = step.OwnerRole + ": Confirm whether " + c.Name + " exists and request its records. Record any reported implementation gap; review setup recommendations separately."
				}
				return setAction("Shared prerequisite · "+c.Name, action, c.Rationale+" Check this scoped prerequisite before dependent evidence work; declarations are not verified support.")
			}
		}
		for _, d := range g.Progress.Duties {
			if d.Trigger == "due" || d.NeedsRevalidation {
				return setAction("Operating records · "+d.Step.Action.Text, d.NextAction, "This operating duty needs current records before assessment; reported completion is not proof.")
			}
		}
	}
	if len(book.Collect.Items) == 0 {
		return p
	}
	step := book.Collect.Items[0]
	p.Action = step.OwnerRole + ": " + step.Action.Text
	reason := "Collect and assess this record for the saved service boundary; collecting it alone does not establish the whole requirement."
	if p.Status != "not_started" {
		reason = "Compare existing artifacts with this evidence step and collect any missing scope or operating records."
	}
	return setAction("Requirement-specific evidence action", p.Action, reason)
}

func PresentRequirement(applicability string, s Status, hasActivity bool) RequirementPresentation {
	current := s.Work.State == "evidence_complete" && s.Evidence == "current"
	p := RequirementPresentation{EvidenceComplete: current, ReviewRemaining: current && s.Review != "reviewed", Secondary: []string{}}
	p.FindingGroups = observation.GroupFindings(s.Collection.Findings)
	p.HumanReviewed = s.Review == "reviewed"
	groups := map[string]int{}
	for _, r := range s.Collection.Recovery {
		key := r.Environment + "\x00" + r.ConnectionProvider + "\x00" + r.Title + "\x00" + r.Explanation + "\x00" + r.Action
		index, ok := groups[key]
		if !ok {
			index = len(p.NextSteps)
			groups[key] = index
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: r.Title, Explanation: r.Explanation, Action: r.Action, Provider: r.ConnectionProvider, Targets: []string{}})
		}
		if r.Target != "" && !slices.Contains(p.NextSteps[index].Targets, r.Target) {
			p.NextSteps[index].Targets = append(p.NextSteps[index].Targets, r.Target)
		}
	}
	for _, blocker := range s.Work.Blockers {
		p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Evidence still needed", Explanation: blocker, Action: "Gather the missing records and ask your agent to update the requirement assessment with exact citations.", Targets: []string{}})
	}
	p.ContextGaps = append([]RequirementNextStep(nil), p.NextSteps...)
	if current {
		p.Secondary = append(p.Secondary, "Evidence gathered")
	}
	if p.ReviewRemaining {
		p.Secondary = append(p.Secondary, "Review pending")
	}
	if len(s.Work.Blockers) > 0 || s.Technical == "error" {
		p.Secondary = append(p.Secondary, "Access or evidence blocked")
	}
	if s.Evidence == "stale" || s.Work.State == "needs_refresh" {
		p.Secondary = append(p.Secondary, "Support needs updating")
	}
	set := func(status, label, reason, action string) RequirementPresentation {
		p.Status = status
		p.Label = label
		p.Reason = reason
		p.Action = action
		for _, group := range p.FindingGroups {
			if status != "failing" {
				break
			}
			targets := []string{}
			for _, finding := range group.Findings {
				targets = append(targets, finding.Target.ResourceID)
			}
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: fmt.Sprintf("%s · %d affected targets", group.Explanation.Title, group.AffectedTargets), Explanation: group.Explanation.Observed + " " + group.Explanation.Limitations, Targets: targets, Action: group.Explanation.NextAction})
		}
		for _, why := range s.Work.Reasons {
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Assessment needs attention", Explanation: why, Targets: []string{}, Action: "Compare the assessment bindings with the current scope and evidence, then submit an updated assessment with exact citations."})
		}
		if status == "failing" && len(p.FindingGroups) > 0 {
			finding := p.FindingGroups[0]
			p.Reason = finding.Explanation.Title + ": " + finding.Explanation.Observed
			p.Action = "View findings"
		}
		if status == "blocked" && len(p.NextSteps) > 0 {
			p.Action = p.NextSteps[0].Action
			p.Reason = p.NextSteps[0].Title + ": " + p.NextSteps[0].Explanation
		}
		guidance := map[string][2]string{
			"not_started":    {"No assessment has been recorded for this requirement.", "Follow the requirement-specific evidence plan below. Gather records for the saved scope, cite exact evidence revisions, and submit an assessment for every assertion."},
			"in_progress":    {"Evidence or checks exist, but the investigation is not complete.", "Compare existing evidence with every assertion below. Gather missing operational records and coverage, then submit an updated assessment; successful reads alone do not prove the requirement."},
			"needs_evidence": {"The recorded investigation covers only part of the requirement.", "Complete the missing assertions in the evidence plan below. Use dated operational records as well as configuration where the statement requires demonstrated practice."},
			"needs_refresh":  {"Evidence has expired or the assessment no longer matches its source, scope or profile bindings.", "Refresh the affected evidence for the current scope, update citations and resubmit the assessment. Renewing credentials alone does not refresh evidence."},
			"needs_scoping":  {"Applicability has not been decided for this service.", "Open service settings, inspect the requirement's applicability conditions, and record a scope decision with its rationale. Then gather evidence if it applies."},
			"in_review":      {"Current evidence supports the requirement, but human review is outstanding.", "An authorized reviewer should inspect the cited evidence and assessment, then record a human review with notes. Agent results cannot grant human approval."},
			"passing":        {"The requirement has current supporting evidence and a recorded human review.", "Maintain evidence freshness. Reassess after relevant scope, implementation or evidence changes; retain the reviewed revisions for audit history."},
			"not_applicable": {"The current applicability decision excludes this requirement.", "Inspect the applicability rationale in this page's official-source details. Revisit the decision in service settings if the service boundary or implementation changes."},
			"failing":        {"A technical finding or assessment contradicts the requirement.", "Preserve the contradictory observations, verify affected scope, and collect the missing configuration or operating records. Record any proposed remediation separately from evidence collection."},
			"blocked":        {"An access error or missing input prevents the assessment from finishing.", "Resolve each recorded blocker above. If no detailed error was retained, inspect the latest run in collection activity and retry it to obtain diagnostics. Gather missing inputs and update the assessment; do not infer success from missing diagnostics."},
		}
		if g, ok := guidance[status]; ok {
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Next action · " + label, Explanation: g[0], Action: g[1], Targets: []string{}})
		}
		return p
	}
	if applicability == "not_applicable" {
		p.EvidenceComplete = false
		p.ReviewRemaining = false
		return set("not_applicable", "Not applicable", "Outside this service’s selected requirements.", "View applicability")
	}
	if applicability == "unresolved" {
		return set("needs_scoping", "Needs scoping", "Decide whether this requirement applies to the service.", "Review scope")
	}
	if s.Technical == "fail" || len(s.Collection.Findings) > 0 || s.Work.Assessment == "contradicted" {
		return set("failing", "Failing", "The investigation found issues that need attention.", "View findings")
	}
	if s.Work.State == "blocked" || len(s.Work.Blockers) > 0 || s.Technical == "error" {
		reason := "The latest technical check could not finish. Open collection activity to inspect its error, resolve access, then run collection again."
		if len(p.NextSteps) > 0 {
			reason = fmt.Sprintf("%s. Follow the steps below; existing evidence is retained. A successful connection test alone does not replace the failed collection result.", p.NextSteps[0].Title)
		}
		return set("blocked", "Evidence blocked", reason, "How to unblock this requirement")
	}
	if s.Work.State == "needs_refresh" || s.Evidence == "stale" {
		p.EvidenceComplete = false
		return set("needs_refresh", "Needs refresh", "Supporting evidence or assessment bindings have changed.", "Refresh evidence")
	}
	if s.Work.State == "partial" {
		return set("needs_evidence", "Needs evidence", "Add the remaining evidence identified by the investigation.", "View missing evidence")
	}
	if p.EvidenceComplete && s.Work.Assessment == "supported" && s.Evidence == "current" {
		if s.Review == "reviewed" {
			return set("passing", "Passing", "Evidence is complete, current and reviewed.", "View evidence")
		}
		return set("in_review", "In review", "Evidence is ready for a human review.", "Review evidence")
	}
	if s.Draft != nil || s.Collection.Count > 0 || s.Evidence == "current" || hasActivity {
		return set("in_progress", "In progress", "Evidence work has started; finish the investigation.", "Continue investigation")
	}
	return set("not_started", "Not started", "Ask an agent to gather evidence from the configured sources.", "Get agent instructions")
}

type ProgressCounts struct {
	Required         int            `json:"required"`
	Passing          int            `json:"passing"`
	EvidenceComplete int            `json:"evidence_complete"`
	ReviewsRemaining int            `json:"reviews_remaining"`
	ScopeDecisions   int            `json:"scope_decisions"`
	Recommended      int            `json:"recommended"`
	Optional         int            `json:"optional"`
	NotApplicable    int            `json:"not_applicable"`
	Statuses         map[string]int `json:"statuses"`
}

func (p *ProgressCounts) Add(applicability string, state RequirementPresentation) {
	if p.Statuses == nil {
		p.Statuses = map[string]int{}
	}
	switch applicability {
	case "not_applicable":
		p.NotApplicable++
		return
	case "unresolved":
		p.ScopeDecisions++
		return
	case "recommended":
		p.Recommended++
		return
	case "optional":
		p.Optional++
		return
	case "required":
		p.Required++
	default:
		p.ScopeDecisions++
		return
	}
	p.Statuses[state.Status]++
	if state.Status == "passing" {
		p.Passing++
	}
	if state.EvidenceComplete {
		p.EvidenceComplete++
	}
	if state.ReviewRemaining {
		p.ReviewsRemaining++
	}
}
