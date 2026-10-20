package assessment

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

type EvidenceRequest struct {
	Binding            InstructionBinding `json:"binding"`
	AssertionIDs       []string           `json:"assertion_ids"`
	Title              string             `json:"title"`
	AcceptanceCriteria string             `json:"acceptance_criteria"`
	OwnerID            string             `json:"owner_id"`
	SourceHints        []Ref              `json:"source_hints"`
	Period             AssessmentPeriod   `json:"period"`
	DueAt              *time.Time         `json:"due_at,omitempty"`
	ReadinessWork      *Ref               `json:"readiness_work,omitempty"`
	State              string             `json:"state"`
	Fulfillment        []Citation         `json:"fulfillment"`
	DecisionReason     string             `json:"decision_reason"`
}

type EvidenceRequestChange struct {
	ServiceID        string          `json:"service_id"`
	RequirementID    string          `json:"requirement_id"`
	ID               string          `json:"id"`
	ExpectedRevision int             `json:"expected_revision"`
	Request          EvidenceRequest `json:"request"`
}

type EvidenceRequestQuery struct {
	ServiceID     string `json:"service_id"`
	RequirementID string `json:"requirement_id,omitempty"`
	State         string `json:"state,omitempty"`
	OwnerID       string `json:"owner_id,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}

type EvidenceRequestView struct {
	Record             Record `json:"record"`
	RevalidationNeeded bool   `json:"revalidation_needed"`
}

func validRequest(v EvidenceRequest) bool {
	return strings.TrimSpace(v.Title) != "" && len(v.Title) <= 1024 && strings.TrimSpace(v.AcceptanceCriteria) != "" && len(v.AcceptanceCriteria) <= 16384 && len(v.AssertionIDs) > 0 && len(v.AssertionIDs) <= 100 && len(v.SourceHints) <= 100 && len(v.Fulfillment) <= 100 && len(v.DecisionReason) <= 4096 && !v.Period.Start.IsZero() && !v.Period.End.IsZero() && !v.Period.End.Before(v.Period.Start) && slices.Contains([]string{"open", "submitted", "accepted", "cancelled"}, v.State)
}

func (s *Store) SaveEvidenceRequest(ctx context.Context, a auth.Actor, in EvidenceRequestChange) (Record, error) {
	v := in.Request
	if !validRequest(v) {
		return Record{}, ErrInvalid
	}
	action := "edit"
	if v.State == "accepted" || v.State == "cancelled" {
		action = "review"
	}
	return s.mutate(ctx, a, record(in.ServiceID, "evidence_request", in.ID, in.RequirementID, v), in.ExpectedRevision, action, func(tx pgx.Tx) error {
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
		oldRecord, err := latest(ctx, tx, in.ServiceID, "evidence_request", in.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if oldRecord.Revision == 0 {
			if v.State != "open" || len(v.Fulfillment) > 0 {
				return ErrInvalid
			}
		} else {
			old, err := Decode[EvidenceRequest](oldRecord)
			if err != nil {
				return err
			}
			if old.State == "accepted" || old.State == "cancelled" {
				return ErrConflict
			}
			if v.State == "submitted" && old.State != "open" {
				return ErrConflict
			}
			if v.State == "accepted" && old.State != "submitted" {
				return ErrConflict
			}
			if (v.State == "accepted" || v.State == "submitted") && !sameRequestDefinition(old, v) {
				return ErrConflict
			}
			if v.State == "accepted" && string(data(old.Fulfillment)) != string(data(v.Fulfillment)) {
				return ErrConflict
			}
			if old.State == "submitted" && v.State == "open" {
				if err := auth.Authorize(ctx, tx, a, in.ServiceID, "review"); err != nil {
					return err
				}
				if strings.TrimSpace(v.DecisionReason) == "" {
					return ErrInvalid
				}
			}
		}
		if v.State == "accepted" || v.State == "cancelled" {
			if strings.TrimSpace(v.DecisionReason) == "" {
				return ErrInvalid
			}
		}
		if v.State == "open" && len(v.Fulfillment) > 0 {
			return ErrInvalid
		}
		if (v.State == "submitted" || v.State == "accepted") && len(v.Fulfillment) == 0 {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, id := range v.AssertionIDs {
			if seen[id] || !slices.ContainsFunc(instructions.Assertions, func(a Assertion) bool { return a.ID == id }) {
				return ErrInvalid
			}
			seen[id] = true
		}
		for _, hint := range v.SourceHints {
			if !slices.Contains(v.Binding.Sources, hint) {
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
		if v.ReadinessWork != nil {
			var exists bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM readiness_work WHERE service_id=$1 AND id=$2 AND revision=$3)`, in.ServiceID, v.ReadinessWork.ID, v.ReadinessWork.Revision).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrInvalid
			}
			var duplicate bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(id) id,data FROM records WHERE service_id=$1 AND kind='evidence_request' ORDER BY id,revision DESC) r WHERE id<>$2 AND data->>'state' NOT IN ('accepted','cancelled') AND data->'readiness_work'->>'id'=$3)`, in.ServiceID, in.ID, v.ReadinessWork.ID).Scan(&duplicate); err != nil {
				return err
			}
			if duplicate {
				return ErrConflict
			}
		}
		return s.validateRequestCitations(ctx, tx, in.ServiceID, v)
	})
}

func sameRequestDefinition(a, b EvidenceRequest) bool {
	a.State = ""
	b.State = ""
	a.Fulfillment = nil
	b.Fulfillment = nil
	a.DecisionReason = ""
	b.DecisionReason = ""
	return string(data(a)) == string(data(b))
}

func (s *Store) validateRequestCitations(ctx context.Context, tx pgx.Tx, service string, v EvidenceRequest) error {
	for _, c := range v.Fulfillment {
		r, err := reference(ctx, tx, service, "evidence", c.Evidence)
		if err != nil {
			return err
		}
		ev, err := Decode[Evidence](r)
		if err != nil {
			return err
		}
		if ev.ScopeRevision != v.Binding.ScopeRevision {
			return ErrConflict
		}
		media := "application/json"
		if ev.Document != nil {
			media = ev.Document.MediaType
			if !slices.Contains(v.Binding.Sources, ev.Document.Source) {
				return ErrConflict
			}
		}
		if s.Artifacts == nil || ev.ArtifactKey == "" {
			return ErrInvalid
		}
		content, err := s.Artifacts.Get(ctx, ev.ArtifactKey)
		if err != nil {
			return err
		}
		if err = validateCitation(content, media, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EvidenceRequests(ctx context.Context, a auth.Actor, in EvidenceRequestQuery) ([]EvidenceRequestView, error) {
	if in.Limit == 0 {
		in.Limit = 25
	}
	if in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.State != "" && !slices.Contains([]string{"open", "submitted", "accepted", "cancelled"}, in.State) {
		return nil, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.query(), a, in.ServiceID, "read"); err != nil {
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM (SELECT DISTINCT ON(id) * FROM records WHERE service_id=$1 AND kind='evidence_request' ORDER BY id,revision DESC) r WHERE ($2='' OR requirement_id=$2) AND ($3='' OR data->>'state'=$3) AND ($4='' OR data->>'owner_id'=$4) ORDER BY id LIMIT $5 OFFSET $6`, in.ServiceID, in.RequirementID, in.State, in.OwnerID, in.Limit, in.Offset)
	if err != nil {
		return nil, err
	}
	records := []Record{}
	for rows.Next() {
		var r Record
		if err = rows.Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []EvidenceRequestView{}
	for _, r := range records {
		v, err := Decode[EvidenceRequest](r)
		if err != nil {
			return nil, err
		}
		instructions, err := s.RuleInstructions(ctx, a, in.ServiceID, r.RequirementID)
		if err != nil {
			return nil, err
		}
		out = append(out, EvidenceRequestView{Record: r, RevalidationNeeded: string(data(v.Binding)) != string(data(instructions.Binding))})
	}
	return out, nil
}
