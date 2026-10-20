package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
)

// Request/issue documents use the workspace's existing revision-bound editor.
// Read-only users see the summaries; action authorization remains server-side.
func (app *App) assessmentWork(r *http.Request, p *Page, service, requirement string) error {
	a := auth.FromContext(r.Context())
	target, reviewErr := app.Store.ReviewTarget(r.Context(), a, service, requirement)
	if reviewErr != nil && !errors.Is(reviewErr, assessment.ErrNotFound) {
		return reviewErr
	}
	if reviewErr == nil {
		p.Sections = append(p.Sections, Section{"Revision-bound human assessment", "Organizational adoption: " + target.Adoption + "\nInternal review: " + target.InternalReview + "\nIndependent assessment: " + target.IndependentReview})
		for _, reason := range target.Reasons {
			p.Sections = append(p.Sections, Section{"Review context", reason})
		}
		instructions, err := app.Store.RuleInstructions(r.Context(), a, service, requirement)
		if err != nil {
			return err
		}
		p.Forms = append(p.Forms, form(r, base(service)+"/actions/assessment-decision", "Review this exact assessment revision", "Record human decision", hidden("requirement_id", requirement), hidden("assessment", pretty(target.Assessment)), hidden("binding", pretty(instructions.Binding)), hidden("dependency_hash", target.DependencyHash), choice("capacity", "Decision capacity", "internal_reviewer", "organizational_adoption", "internal_reviewer", "independent_assessor"), choice("conclusion", "Decision", "changes_requested", "accepted", "changes_requested", "rejected"), area("procedures", "Review procedures performed", ""), area("sampling", "Sampling and limitations", ""), area("notes", "Decision rationale", "")))
	}
	offset := 0
	if raw := r.URL.Query().Get("work_offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return assessment.ErrInvalid
		}
		offset = n
	}
	requests, err := app.Store.EvidenceRequests(r.Context(), a, assessment.EvidenceRequestQuery{ServiceID: service, RequirementID: requirement, Limit: 25, Offset: offset})
	if err != nil {
		return err
	}
	for _, item := range requests {
		v, err := assessment.Decode[assessment.EvidenceRequest](item.Record)
		if err != nil {
			return err
		}
		owner := v.OwnerID
		if owner == "" {
			owner = "Unassigned"
		}
		summary := v.AcceptanceCriteria + "\nOwner: " + owner + " · State: " + v.State + "\nPeriod: " + v.Period.Start.String() + " – " + v.Period.End.String()
		if item.RevalidationNeeded {
			summary += "\nBindings changed: revalidate before accepting fulfillment."
		}
		p.Sections = append(p.Sections, Section{"Evidence request · " + v.Title, summary})
		if v.State != "accepted" && v.State != "cancelled" {
			p.Forms = append(p.Forms, form(r, base(service)+"/actions/evidence-request", "Evidence request · "+v.Title, "Save request revision", hidden("id", item.Record.ID), hidden("requirement_id", requirement), hidden("expected_revision", strconv.Itoa(item.Record.Revision)), area("payload", "Request and fulfillment (human review required for acceptance)", pretty(v))))
		}
	}
	issues, err := app.Store.List(r.Context(), a, service, "assessment_issue", requirement, 25, offset)
	if err != nil {
		return err
	}
	for _, item := range issues {
		v, err := assessment.Decode[assessment.AssessmentIssue](item)
		if err != nil {
			return err
		}
		owner := v.OwnerID
		if owner == "" {
			owner = "Unassigned"
		}
		p.Sections = append(p.Sections, Section{"Outstanding issue · " + v.State, v.Explanation + "\nOwner: " + owner + "\nDisposition: " + v.Disposition})
		if v.State != "closed" {
			p.Forms = append(p.Forms, form(r, base(service)+"/actions/assessment-issue", "Issue milestones and disposition", "Save issue revision", hidden("id", item.ID), hidden("requirement_id", requirement), hidden("expected_revision", strconv.Itoa(item.Revision)), area("payload", "Issue details (human review required for closure)", pretty(v))))
		}
	}
	if len(requests) == 25 || len(issues) == 25 {
		p.Pagination = append(p.Pagination, Link{"Next evidence requests and issues", base(service) + "/requirements/" + requirement + "?work_offset=" + strconv.Itoa(offset+25)})
	}
	if offset > 0 {
		p.Pagination = append(p.Pagination, Link{"Previous evidence requests and issues", base(service) + "/requirements/" + requirement + "?work_offset=" + strconv.Itoa(max(0, offset-25))})
	}
	return nil
}
