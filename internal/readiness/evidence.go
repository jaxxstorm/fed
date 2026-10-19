package readiness

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type CandidateEvidence struct {
	Reference   EvidenceRef `json:"reference"`
	State       string      `json:"state"`
	Reason      string      `json:"reason"`
	ObservedAt  *time.Time  `json:"observed_at,omitempty"`
	Limitations []string    `json:"limitations"`
}

// Candidate freshness is independent of declaration and sufficiency. Query
// exact retained revisions in one batch, including references absent from drafts.
func projectCandidates(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, plan *ServicePlan, now time.Time) error {
	refs := map[EvidenceRef]bool{}
	for _, n := range plan.Capabilities {
		if n.Declaration != nil {
			for _, r := range n.Declaration.Data.Evidence {
				refs[r] = true
			}
		}
	}
	for _, n := range plan.Duties {
		if n.Declaration != nil {
			for _, r := range n.Declaration.Data.Evidence {
				refs[r] = true
			}
		}
	}
	ids := []string{}
	revisions := []int{}
	for r := range refs {
		ids = append(ids, r.ID)
		revisions = append(revisions, r.Revision)
	}
	items := map[EvidenceRef]CandidateEvidence{}
	if len(ids) > 0 {
		rows, err := q.Query(ctx, `SELECT r.id,r.revision,r.data,COALESCE(s.revision,0) FROM records r LEFT JOIN LATERAL (SELECT revision FROM records WHERE service_id=r.service_id AND kind='evidence_source' AND id=r.data->'document'->'source'->>'id' ORDER BY revision DESC LIMIT 1) s ON true WHERE r.service_id=$1 AND r.kind='evidence' AND (r.id,r.revision) IN (SELECT * FROM unnest($2::text[],$3::integer[]))`, plan.ServiceID, ids, revisions)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref EvidenceRef
			var raw []byte
			var sourceRevision int
			if err = rows.Scan(&ref.ID, &ref.Revision, &raw, &sourceRevision); err != nil {
				return err
			}
			var e struct {
				ScopeRevision int       `json:"scope_revision"`
				ObservedAt    time.Time `json:"observed_at"`
				Limitations   []string  `json:"limitations"`
				Document      *struct {
					Source         EvidenceRef `json:"source"`
					FreshnessHours int         `json:"freshness_hours"`
				} `json:"document"`
			}
			if err = json.Unmarshal(raw, &e); err != nil {
				return err
			}
			item := CandidateEvidence{Reference: ref, State: "available_for_assessment", Reason: "This exact artifact revision is a candidate, not assessed support for this capability or duty.", Limitations: e.Limitations}
			if !e.ObservedAt.IsZero() {
				item.ObservedAt = &e.ObservedAt
			}
			if e.ScopeRevision != plan.Binding.ScopeRevision {
				item.State = "stale"
				item.Reason = "The artifact belongs to a different saved service boundary."
			} else if e.Document != nil {
				if e.Document.Source.Revision != sourceRevision {
					item.State = "stale"
					item.Reason = "The evidence source revision changed; recheck scope, access and freshness policy."
				} else if e.Document.FreshnessHours <= 0 || e.ObservedAt.IsZero() || !e.ObservedAt.Add(time.Duration(e.Document.FreshnessHours)*time.Hour).After(now) {
					item.State = "stale"
					item.Reason = "The artifact is outside its saved documentary freshness period or lacks a valid period."
				}
			} else {
				item.Limitations = append(item.Limitations, "Freshness and target coverage must be assessed under the linked requirement's evidence policy.")
			}
			items[ref] = item
		}
		if err = rows.Err(); err != nil {
			return err
		}
	}
	project := func(w *WorkRecord) ([]CandidateEvidence, string) {
		out := []CandidateEvidence{}
		if w == nil || len(w.Data.Evidence) == 0 {
			return out, "missing"
		}
		stale := 0
		for _, r := range w.Data.Evidence {
			item, ok := items[r]
			if !ok {
				item = CandidateEvidence{Reference: r, State: "unavailable", Reason: "The exact artifact revision is unavailable in this service."}
			}
			if item.State != "available_for_assessment" {
				stale++
			}
			out = append(out, item)
		}
		if stale == len(out) {
			return out, "requires_reassessment"
		}
		if stale > 0 {
			return out, "partly_stale"
		}
		return out, "available_for_assessment"
	}
	for i := range plan.Capabilities {
		n := &plan.Capabilities[i]
		n.Evidence, n.EvidenceState = project(n.Declaration)
		if n.NeedsRevalidation {
			n.EvidenceState = "requires_reassessment"
		} else if n.EvidenceState == "requires_reassessment" || n.EvidenceState == "partly_stale" {
			n.NextAction = "Ask the evidence owner to refresh or revalidate the referenced artifacts; preserve their historical revisions."
		}
	}
	for i := range plan.Duties {
		n := &plan.Duties[i]
		n.Evidence, n.EvidenceState = project(n.Declaration)
		if n.NeedsRevalidation {
			n.EvidenceState = "requires_reassessment"
		}
	}
	return nil
}
