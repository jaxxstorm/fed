package assessment

import "fmt"

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
	NextSteps        []RequirementNextStep `json:"next_steps,omitempty"`
	HumanReviewed    bool                  `json:"human_reviewed"`
	Status           string                `json:"status"`
	Label            string                `json:"label"`
	Reason           string                `json:"reason"`
	Action           string                `json:"action"`
	EvidenceComplete bool                  `json:"evidence_complete"`
	ReviewRemaining  bool                  `json:"review_remaining"`
	Secondary        []string              `json:"secondary"`
}

func PresentRequirement(applicability string, s Status, hasActivity bool) RequirementPresentation {
	current := s.Work.State == "evidence_complete" && s.Evidence == "current"
	p := RequirementPresentation{EvidenceComplete: current, ReviewRemaining: current && s.Review != "reviewed", Secondary: []string{}}
	p.HumanReviewed = s.Review == "reviewed"
	groups := map[string]int{}
	for _, r := range s.Collection.Recovery {
		key := r.Code + "/" + r.Environment
		index, ok := groups[key]
		if !ok {
			index = len(p.NextSteps)
			groups[key] = index
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: r.Title, Explanation: r.Explanation, Action: r.Action, Provider: r.ConnectionProvider, Targets: []string{}})
		}
		if r.Target != "" {
			p.NextSteps[index].Targets = append(p.NextSteps[index].Targets, r.Target)
		}
	}
	for _, blocker := range s.Work.Blockers {
		p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Evidence still needed", Explanation: blocker, Action: "Gather the missing records and ask your agent to update the requirement assessment with exact citations.", Targets: []string{}})
	}
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
		for _, finding := range s.Collection.Findings {
			if status != "failing" {
				break
			}
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Investigate failed condition: " + finding.Condition, Explanation: finding.Explanation, Targets: []string{finding.Target.ResourceID}, Action: "Inspect the affected resource against this condition. Record the remediation or justified exception, rerun the check, and update the assessment with the new result."})
		}
		for _, why := range s.Work.Reasons {
			p.NextSteps = append(p.NextSteps, RequirementNextStep{Title: "Assessment needs attention", Explanation: why, Targets: []string{}, Action: "Compare the assessment bindings with the current scope and evidence, then submit an updated assessment with exact citations."})
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
			"failing":        {"A technical finding or assessment contradicts the requirement.", "Inspect findings and assertion rationales below. Address the failed condition, gather proof of the change, rerun affected checks and resubmit the assessment before human review."},
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
		return set("blocked", "Blocked", reason, "How to unblock this requirement")
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
