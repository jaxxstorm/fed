package assessment

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type AssessorDesignation struct {
	IdentityID   string    `json:"identity_id"`
	Organization string    `json:"organization"`
	Reason       string    `json:"reason"`
	ExpiresAt    time.Time `json:"expires_at"`
	Revoked      bool      `json:"revoked"`
}

type ReviewDependency struct {
	Kind      string `json:"kind"`
	Reference Ref    `json:"reference"`
}

type AssessmentReviewTarget struct {
	Assessment        Ref                `json:"assessment"`
	DependencyHash    string             `json:"dependency_hash"`
	Dependencies      []ReviewDependency `json:"dependencies"`
	Decisions         []Record           `json:"decisions"`
	Adoption          string             `json:"adoption"`
	Review            string             `json:"review"`
	InternalReview    string             `json:"internal_review"`
	IndependentReview string             `json:"independent_review"`
	Reasons           []string           `json:"reasons"`
}

func (s *Store) SaveAssessorDesignation(ctx context.Context, a auth.Actor, service, id string, expected int, v AssessorDesignation) (Record, error) {
	if strings.TrimSpace(v.Organization) == "" || len(v.Organization) > 1024 || strings.TrimSpace(v.Reason) == "" || len(v.Reason) > 4096 || v.IdentityID == "" || v.ExpiresAt.IsZero() {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "assessor_designation", id, "", v), expected, "admin", func(tx pgx.Tx) error {
		if a.Kind != "human" {
			return auth.ErrForbidden
		}
		var human bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identities i JOIN services s ON i.organization_id=s.organization_id WHERE i.id=$1 AND s.id=$2 AND i.kind='human')`, v.IdentityID, service).Scan(&human); err != nil {
			return err
		}
		if !human {
			return ErrInvalid
		}
		return nil
	})
}

// ReviewTarget fingerprints the current versions of dependencies, not unrelated
// evidence collected elsewhere. Historical decisions retain their old hash.
func (s *Store) ReviewTarget(ctx context.Context, a auth.Actor, service, requirement string) (AssessmentReviewTarget, error) {
	instructions, err := s.RuleInstructions(ctx, a, service, requirement)
	if err != nil {
		return AssessmentReviewTarget{}, err
	}
	if instructions.ReviewTarget == nil {
		return AssessmentReviewTarget{}, ErrNotFound
	}
	return *instructions.ReviewTarget, nil
}

func (s *Store) reviewTarget(ctx context.Context, a auth.Actor, service, requirement string, instructions RuleInstructions) (AssessmentReviewTarget, error) {
	out := AssessmentReviewTarget{Dependencies: []ReviewDependency{}, Decisions: []Record{}, Adoption: "pending", Review: "pending", Reasons: []string{}}
	out.InternalReview = "pending"
	out.IndependentReview = "pending"
	r, err := s.Get(ctx, a, service, "assessment", requirement)
	if err != nil {
		return out, err
	}
	draft, err := Decode[Draft](r)
	if err != nil {
		return out, err
	}
	out.Assessment = Ref{r.ID, r.Revision}
	add := func(kind string, ref Ref) {
		d := ReviewDependency{kind, ref}
		if !slices.Contains(out.Dependencies, d) {
			out.Dependencies = append(out.Dependencies, d)
		}
	}
	add("assessment", out.Assessment)
	for _, kind := range []string{"profile", "scope"} {
		record, err := s.Get(ctx, a, service, kind, kind)
		if err != nil {
			return out, err
		}
		add(kind, Ref{record.ID, record.Revision})
	}
	for _, ref := range draft.Evidence {
		add("evidence", ref)
		latestEvidence, err := s.Get(ctx, a, service, "evidence", ref.ID)
		if err != nil {
			return out, err
		}
		add("evidence", Ref{latestEvidence.ID, latestEvidence.Revision})
		er, err := s.presentationReference(ctx, service, "evidence", ref)
		if err != nil {
			return out, err
		}
		ev, err := Decode[Evidence](er)
		if err != nil {
			return out, err
		}
		if ev.Document != nil {
			source, err := s.Get(ctx, a, service, "evidence_source", ev.Document.Source.ID)
			if err != nil {
				return out, err
			}
			add("evidence_source", Ref{source.ID, source.Revision})
			if !ev.ObservedAt.Add(time.Duration(ev.Document.FreshnessHours) * time.Hour).After(s.now()) {
				out.Reasons = append(out.Reasons, "Current evidence freshness expired; historical-period support is unchanged.")
			}
		}
	}
	for _, kind := range []string{"measure", "assessment_issue"} {
		for offset := 0; ; offset += 100 {
			records, err := s.List(ctx, a, service, kind, requirement, 100, offset)
			if err != nil {
				return out, err
			}
			for _, r := range records {
				add(kind, Ref{r.ID, r.Revision})
			}
			if len(records) < 100 {
				break
			}
		}
	}
	slices.SortFunc(out.Dependencies, func(a, b ReviewDependency) int {
		if order := strings.Compare(a.Kind+"/"+a.Reference.ID, b.Kind+"/"+b.Reference.ID); order != 0 {
			return order
		}
		return a.Reference.Revision - b.Reference.Revision
	})
	out.DependencyHash = storage.Hash(data(struct {
		Dependencies    []ReviewDependency
		AssertionHash   string
		GuidanceVariant string
		GuidanceHash    string
	}{out.Dependencies, instructions.Binding.AssertionHash, instructions.Binding.GuidanceVariant, instructions.Binding.GuidanceHash}))
	// Scan all decision revisions before choosing the newest decision per capacity.
	seen := map[string]bool{}
	for offset := 0; ; offset += 100 {
		records, err := s.List(ctx, a, service, "assessment_decision", requirement, 100, offset)
		if err != nil {
			return out, err
		}
		out.Decisions = append(out.Decisions, records...)
		if len(records) < 100 {
			break
		}
	}
	slices.SortFunc(out.Decisions, func(a, b Record) int { return b.CreatedAt.Compare(a.CreatedAt) })
	for _, record := range out.Decisions {
		d, err := Decode[AssessmentDecision](record)
		if err != nil {
			return out, err
		}
		capacity := d.Decision.Capacity
		if seen[capacity] {
			continue
		}
		seen[capacity] = true
		state := d.Decision.Conclusion
		if d.CurrentFreshness == "current" && instructions.Implementation != nil && instructions.Implementation.CurrentFreshness == "stale" {
			state = "review_needed"
		}
		if d.AssessorDesignation != nil {
			designation, err := s.Get(ctx, a, service, "assessor_designation", d.AssessorDesignation.ID)
			if err != nil {
				return out, err
			}
			v, err := Decode[AssessorDesignation](designation)
			if err != nil {
				return out, err
			}
			if designation.Revision != d.AssessorDesignation.Revision || v.Revoked || !v.ExpiresAt.After(s.now()) {
				state = "review_needed"
				out.Reasons = append(out.Reasons, "Independent-assessor designation changed or expired.")
			}
		}
		if d.Decision.DependencyHash != out.DependencyHash || d.Decision.Assessment != out.Assessment {
			state = "review_needed"
			rows, err := s.query().Query(ctx, `SELECT dependency_kind,dependency_id,dependency_revision FROM assessment_review_dependencies WHERE service_id=$1 AND decision_id=$2 AND decision_revision=$3 ORDER BY dependency_kind,dependency_id,dependency_revision`, service, record.ID, record.Revision)
			if err != nil {
				return out, err
			}
			old := map[string]int{}
			for rows.Next() {
				var kind, id string
				var revision int
				if err = rows.Scan(&kind, &id, &revision); err != nil {
					rows.Close()
					return out, err
				}
				if kind != "assessor_designation" {
					old[kind+"/"+id] = revision
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return out, err
			}
			changed := false
			for _, dependency := range out.Dependencies {
				key := dependency.Kind + "/" + dependency.Reference.ID
				if old[key] != dependency.Reference.Revision {
					out.Reasons = append(out.Reasons, fmt.Sprintf("%s changed from revision %d to %d.", key, old[key], dependency.Reference.Revision))
					changed = true
				}
				delete(old, key)
			}
			removed := make([]string, 0, len(old))
			for key := range old {
				removed = append(removed, key)
			}
			slices.Sort(removed)
			for _, key := range removed {
				out.Reasons = append(out.Reasons, "Dependency removed: "+key)
				changed = true
			}
			if !changed {
				out.Reasons = append(out.Reasons, "Assertion mapping or reviewed guidance changed.")
			}
		}
		if capacity == "organizational_adoption" {
			out.Adoption = state
		} else if out.Review == "pending" {
			out.Review = state
		}
		if capacity == "internal_reviewer" {
			out.InternalReview = state
		}
		if capacity == "independent_assessor" {
			out.IndependentReview = state
		}
	}
	slices.Sort(out.Reasons)
	out.Reasons = slices.Compact(out.Reasons)
	return out, nil
}

func (s *Store) RecordAssessmentDecision(ctx context.Context, a auth.Actor, service, requirement string, in AssessmentDecisionInput) (Record, error) {
	if a.Kind != "human" || a.TokenHash != "" {
		return Record{}, auth.ErrForbidden
	}
	if !slices.Contains([]string{"organizational_adoption", "internal_reviewer", "independent_assessor"}, in.Capacity) || !slices.Contains([]string{"accepted", "changes_requested", "rejected"}, in.Conclusion) || strings.TrimSpace(in.Notes) == "" || len(in.Notes) > 16384 || len(in.Procedures) > 16384 || len(in.Sampling) > 4096 {
		return Record{}, ErrInvalid
	}
	if in.Capacity != "organizational_adoption" && (strings.TrimSpace(in.Procedures) == "" || strings.TrimSpace(in.Sampling) == "") {
		return Record{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "review"); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return Record{}, err
	}
	view := *s
	view.reader = tx
	view.presentationCache = nil
	instructions, err := view.RuleInstructions(ctx, a, service, requirement)
	if err != nil {
		return Record{}, err
	}
	if string(data(instructions.Binding)) != string(data(in.Binding)) {
		return Record{}, ErrConflict
	}
	target, err := view.ReviewTarget(ctx, a, service, requirement)
	if err != nil {
		return Record{}, err
	}
	if target.Assessment != in.Assessment || target.DependencyHash != in.DependencyHash {
		return Record{}, ErrConflict
	}
	if instructions.Implementation == nil || !instructions.Implementation.Readiness.Ready {
		return Record{}, ErrInvalid
	}
	decision := AssessmentDecision{Version: "assessment-decision/v1", Decision: in, ActorID: a.ID, OrganizationID: a.OrganizationID, DecidedAt: s.now(), Evidence: []Ref{}}
	decision.CurrentFreshness = instructions.Implementation.CurrentFreshness
	if in.Capacity == "independent_assessor" {
		var designated Record
		row := tx.QueryRow(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM (SELECT DISTINCT ON(id) * FROM records WHERE service_id=$1 AND kind='assessor_designation' ORDER BY id,revision DESC) r WHERE data->>'identity_id'=$2 AND (data->>'revoked')::boolean=false AND (data->>'expires_at')::timestamptz>now() ORDER BY created_at DESC LIMIT 1`, service, a.ID)
		err = row.Scan(&designated.ServiceID, &designated.Kind, &designated.ID, &designated.Revision, &designated.RequirementID, &designated.ActorID, &designated.Data, &designated.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, auth.ErrForbidden
		}
		if err != nil {
			return Record{}, err
		}
		decision.AssessorDesignation = &Ref{designated.ID, designated.Revision}
		designation, err := Decode[AssessorDesignation](designated)
		if err != nil {
			return Record{}, err
		}
		decision.AssessorOrganization = designation.Organization
		target.Dependencies = append(target.Dependencies, ReviewDependency{"assessor_designation", *decision.AssessorDesignation})
	}
	for _, d := range target.Dependencies {
		if d.Kind == "evidence" {
			decision.Evidence = append(decision.Evidence, d.Reference)
		}
	}
	r, err := insertRecord(ctx, tx, a, record(service, "assessment_decision", uuid.NewString(), requirement, decision), 1)
	if err != nil {
		return Record{}, err
	}
	for _, d := range target.Dependencies {
		if _, err = tx.Exec(ctx, `INSERT INTO assessment_review_dependencies(service_id,decision_id,decision_revision,dependency_kind,dependency_id,dependency_revision) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, service, r.ID, r.Revision, d.Kind, d.Reference.ID, d.Reference.Revision); err != nil {
			return Record{}, err
		}
	}
	return r, commitOperation(ctx, tx, a, service, "assessment.decision_recorded", r)
}
