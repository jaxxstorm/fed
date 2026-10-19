package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
)

// RequirementProgress is current scoped work, not a compliance assessment.
// The same projection accompanies browser and agent guidance responses.
type RequirementProgress struct {
	Binding      WorkBinding           `json:"binding"`
	Capabilities []CapabilityReadiness `json:"capabilities"`
	Duties       []DutyReadiness       `json:"duties"`
	Questions    []PlanQuestion        `json:"questions"`
}

// All work and exact candidate revisions are read once in the caller's snapshot,
// regardless of the number of requirement cards. Never start an independent plan
// transaction here: its scope or source revisions could differ from the caller.
func (s *Store) attachProgress(ctx context.Context, q auth.Querier, service string, p Package, rules []catalog.Resolved, contexts []GuidanceContext, now time.Time) error {
	if len(contexts) == 0 {
		return nil
	}
	selected := p
	selected.Playbooks = nil
	for _, g := range contexts {
		if g.State == "reviewed" && g.Guidance != nil {
			selected.Playbooks = append(selected.Playbooks, g.Guidance.Playbooks...)
		}
	}
	if len(selected.Playbooks) == 0 {
		return nil
	}
	var profileRaw, scopeRaw []byte
	binding := WorkBinding{CatalogID: p.CatalogID, ReleaseRevision: contexts[0].ReleaseRevision, ReleaseHash: contexts[0].ReleaseHash}
	if err := q.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='profile' AND id='profile' ORDER BY revision DESC LIMIT 1`, service).Scan(&binding.ProfileRevision, &profileRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if err := q.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' ORDER BY revision DESC LIMIT 1`, service).Scan(&binding.ScopeRevision, &scopeRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	var profile catalog.Profile
	if err := json.Unmarshal(profileRaw, &profile); err != nil {
		return err
	}
	if profile.CatalogID != binding.CatalogID {
		return fmt.Errorf("%w: guidance/profile catalog mismatch", ErrInvalid)
	}
	scopes, err := ScopeKeys(scopeRaw)
	if err != nil {
		return err
	}
	reader, ok := q.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	})
	if !ok {
		return fmt.Errorf("%w: readiness context requires batch reader", ErrInvalid)
	}
	// Resolve all current requirements even for a single detail request, so a
	// shared instance has identical downstream impact in list, detail and MCP.
	all, err := catalog.Load(ctx, reader, profile.CatalogID)
	if err != nil {
		return err
	}
	resolved, err := catalog.Resolve(all, profile)
	if err != nil {
		return err
	}
	selected.Playbooks = nil
	seen := map[string]bool{}
	projected := map[string]GuidanceContext{}
	for _, g := range contexts {
		projected[g.VariantID] = g
	}
	for _, rule := range resolved {
		variant, err := catalog.Variant(rule)
		if err != nil {
			return err
		}
		g, ok := projected[variant.ID]
		if !ok {
			g, err = s.projectContext(p, GuidanceContext{VariantID: variant.ID}, now)
			if err != nil {
				return err
			}
		}
		if g.State == "reviewed" && g.Guidance != nil {
			for _, book := range g.Guidance.Playbooks {
				if !seen[book.ID] {
					selected.Playbooks = append(selected.Playbooks, book)
					seen[book.ID] = true
				}
			}
		}
	}
	rows, err := reader.Query(ctx, `SELECT DISTINCT ON(id) id,revision,actor_id,created_at,binding,data FROM readiness_work WHERE service_id=$1 ORDER BY id,revision DESC`, service)
	if err != nil {
		return err
	}
	records := []WorkRecord{}
	for rows.Next() {
		r := WorkRecord{ServiceID: service}
		var b, d []byte
		if err = rows.Scan(&r.ID, &r.Revision, &r.ActorID, &r.CreatedAt, &b, &d); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(b, &r.Binding); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(d, &r.Data); err != nil {
			rows.Close()
			return err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	plan, err := BuildPlan(service, binding, selected, profile, resolved, scopes, records)
	if err != nil {
		return err
	}
	if err = projectCandidates(ctx, reader, &plan, now); err != nil {
		return err
	}
	for i := range contexts {
		if contexts[i].State != "reviewed" {
			continue
		}
		rid := rules[i].Requirement.ID
		progress := &RequirementProgress{Binding: binding, Capabilities: []CapabilityReadiness{}, Duties: []DutyReadiness{}, Questions: []PlanQuestion{}}
		for _, c := range plan.Capabilities {
			if slices.Contains(c.RequirementIDs, rid) {
				progress.Capabilities = append(progress.Capabilities, c)
			}
		}
		for _, d := range plan.Duties {
			if d.RequirementID == rid {
				progress.Duties = append(progress.Duties, d)
			}
		}
		for _, question := range plan.Questions {
			if question.RequirementID == rid {
				progress.Questions = append(progress.Questions, question)
			}
		}
		contexts[i].Progress = progress
	}
	return nil
}
