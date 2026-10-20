package assessment

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

type IssueMilestone struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	TargetAt    *time.Time `json:"target_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Evidence    []Citation `json:"evidence"`
}

type AssessmentIssue struct {
	Binding         InstructionBinding `json:"binding"`
	Explanation     string             `json:"explanation"`
	OwnerID         string             `json:"owner_id"`
	State           string             `json:"state"`
	TargetAt        *time.Time         `json:"target_at,omitempty"`
	Disposition     string             `json:"disposition"`
	Gaps            []Ref              `json:"gaps"`
	FindingIDs      []string           `json:"finding_ids"`
	Requests        []Ref              `json:"requests"`
	Milestones      []IssueMilestone   `json:"milestones"`
	ClosureEvidence []Citation         `json:"closure_evidence"`
}

type AssessmentIssueChange struct {
	ServiceID        string          `json:"service_id"`
	RequirementID    string          `json:"requirement_id"`
	ID               string          `json:"id"`
	ExpectedRevision int             `json:"expected_revision"`
	Issue            AssessmentIssue `json:"issue"`
}

func (s *Store) SaveAssessmentIssue(ctx context.Context, a auth.Actor, in AssessmentIssueChange) (Record, error) {
	v := in.Issue
	if strings.TrimSpace(v.Explanation) == "" || len(v.Explanation) > 16384 || len(v.Disposition) > 4096 || (v.State != "open" && v.State != "closed") || len(v.Gaps) > 100 || len(v.Requests) > 100 || len(v.FindingIDs) > 100 || len(v.Milestones) > 100 || len(v.ClosureEvidence) > 100 {
		return Record{}, ErrInvalid
	}
	action := "edit"
	if v.State == "closed" {
		action = "review"
		if strings.TrimSpace(v.Disposition) == "" {
			return Record{}, ErrInvalid
		}
	}
	if v.State == "open" && len(v.ClosureEvidence) > 0 {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(in.ServiceID, "assessment_issue", in.ID, in.RequirementID, v), in.ExpectedRevision, action, func(tx pgx.Tx) error {
		view := *s
		view.reader = tx
		view.presentationCache = nil
		instructions, err := view.RuleInstructions(ctx, a, in.ServiceID, in.RequirementID)
		if err != nil {
			return err
		}
		if string(data(v.Binding)) != string(data(instructions.Binding)) {
			return ErrConflict
		}
		oldRecord, err := latest(ctx, tx, in.ServiceID, "assessment_issue", in.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if oldRecord.Revision == 0 && v.State == "closed" {
			return ErrInvalid
		}
		if oldRecord.Revision > 0 {
			old, err := Decode[AssessmentIssue](oldRecord)
			if err != nil {
				return err
			}
			if old.State == "closed" {
				return ErrConflict
			}
		}
		if v.OwnerID != "" {
			var active bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships m JOIN services s ON s.organization_id=m.organization_id WHERE s.id=$1 AND m.identity_id=$2 AND m.state='active')`, in.ServiceID, v.OwnerID).Scan(&active); err != nil {
				return err
			}
			if !active {
				return ErrInvalid
			}
		}
		for kind, refs := range map[string][]Ref{"gap": v.Gaps, "evidence_request": v.Requests} {
			for _, ref := range refs {
				r, err := reference(ctx, tx, in.ServiceID, kind, ref)
				if err != nil {
					return err
				}
				if r.RequirementID != in.RequirementID {
					return ErrInvalid
				}
			}
		}
		for _, id := range v.FindingIDs {
			var exists bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finding_events f JOIN observation_links l ON l.service_id=f.service_id AND l.observation_id=f.observation_id WHERE f.service_id=$1 AND f.finding_id=$2 AND l.requirement_id=$3)`, in.ServiceID, id, in.RequirementID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrInvalid
			}
		}
		seen := map[string]bool{}
		for _, m := range v.Milestones {
			if m.ID == "" || seen[m.ID] || strings.TrimSpace(m.Description) == "" || len(m.Description) > 4096 || len(m.Evidence) > 100 {
				return ErrInvalid
			}
			seen[m.ID] = true
			if m.CompletedAt != nil && (m.CompletedAt.After(time.Now()) || len(m.Evidence) == 0) {
				return ErrInvalid
			}
			if err = s.validateRequestCitations(ctx, tx, in.ServiceID, EvidenceRequest{Binding: v.Binding, Fulfillment: m.Evidence}); err != nil {
				return err
			}
		}
		return s.validateRequestCitations(ctx, tx, in.ServiceID, EvidenceRequest{Binding: v.Binding, Fulfillment: v.ClosureEvidence})
	})
}
