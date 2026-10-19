package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
)

// Plan reads profile, scope, guidance and declarations in one repeatable
// snapshot. Pagination is over stable prerequisite-first capability instances.
func (s *Store) Plan(ctx context.Context, a auth.Actor, service string, limit, offset int) (ServicePlan, error) {
	return s.plan(ctx, a, service, limit, offset, "")
}

var ErrNotFound = errors.New("readiness instance not found")

// Capability returns one exact scoped instance, with bounded condition questions.
func (s *Store) Capability(ctx context.Context, a auth.Actor, service, id string, limit, offset int) (ServicePlan, error) {
	if id == "" {
		return ServicePlan{}, ErrInvalid
	}
	return s.plan(ctx, a, service, limit, offset, id)
}

func (s *Store) plan(ctx context.Context, a auth.Actor, service string, limit, offset int, instance string) (ServicePlan, error) {
	out := ServicePlan{ServiceID: service, State: "missing", Capabilities: []CapabilityReadiness{}, Questions: []PlanQuestion{}, CountUnit: "scoped capability instances", Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "read"); err != nil {
		return out, err
	}
	var profileRaw, scopeRaw []byte
	if err = tx.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='profile' AND id='profile' ORDER BY revision DESC LIMIT 1`, service).Scan(&out.Binding.ProfileRevision, &profileRaw); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' ORDER BY revision DESC LIMIT 1`, service).Scan(&out.Binding.ScopeRevision, &scopeRaw); err != nil {
		return out, err
	}
	var profile catalog.Profile
	if err = json.Unmarshal(profileRaw, &profile); err != nil {
		return out, err
	}
	out.Binding.CatalogID = profile.CatalogID
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT revision,content_hash,payload FROM readiness_service_releases WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 ORDER BY revision DESC LIMIT 1`, a.OrganizationID, service, profile.CatalogID).Scan(&out.Binding.ReleaseRevision, &out.Binding.ReleaseHash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		if instance != "" {
			return out, ErrNotFound
		}
		return out, tx.Commit(ctx)
	}
	if err != nil {
		return out, err
	}
	var p Package
	if err = json.Unmarshal(raw, &p); err != nil {
		return out, err
	}
	all, err := catalog.Load(ctx, tx, profile.CatalogID)
	if err != nil {
		return out, err
	}
	resolved, err := catalog.Resolve(all, profile)
	if err != nil {
		return out, err
	}
	scopes, err := ScopeKeys(scopeRaw)
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(id) id,revision,actor_id,created_at,binding,data FROM readiness_work WHERE service_id=$1 ORDER BY id,revision DESC`, service)
	if err != nil {
		return out, err
	}
	records := []WorkRecord{}
	for rows.Next() {
		r := WorkRecord{ServiceID: service}
		var binding, data []byte
		if err = rows.Scan(&r.ID, &r.Revision, &r.ActorID, &r.CreatedAt, &binding, &data); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(binding, &r.Binding); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(data, &r.Data); err != nil {
			rows.Close()
			return out, err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out, err = BuildPlan(service, out.Binding, p, profile, resolved, scopes, records)
	if err != nil {
		return out, err
	}
	if err = projectCandidates(ctx, tx, &out, time.Now().UTC()); err != nil {
		return out, err
	}
	if s.Rules == nil || s.Markdown == nil || VerifySources(p, s.Rules, s.Markdown) != nil {
		out.State = "needs_revalidation"
		for i := range out.Duties {
			d := &out.Duties[i]
			d.NeedsRevalidation = true
			d.Implementation = "unknown"
			d.Trigger = "unknown"
			d.EvidenceState = "requires_reassessment"
			d.NextAction = "Recheck guidance against changed official sources before relying on this operating duty."
		}
		for i := range out.Capabilities {
			n := &out.Capabilities[i]
			n.NeedsRevalidation = true
			n.Implementation = "unknown"
			n.Trigger = "unknown"
			n.EvidenceState = "requires_reassessment"
			n.NextAction = "Recheck guidance against the changed official sources before relying on this plan."
		}
	}
	out.Offset = offset
	if instance != "" {
		out.Duties = []DutyReadiness{}
		out.DutyCount = 0
		var selected *CapabilityReadiness
		for i := range out.Capabilities {
			if out.Capabilities[i].ID == instance {
				selected = &out.Capabilities[i]
				break
			}
		}
		if selected == nil {
			return out, ErrNotFound
		}
		out.Capabilities = []CapabilityReadiness{*selected}
		out.Total = 1
		questions := []PlanQuestion{}
		for _, q := range out.Questions {
			if q.CapabilityID == selected.CapabilityID && (q.Scope == nil || *q.Scope == selected.Scope) {
				questions = append(questions, q)
			}
		}
		out.Questions = questions
	}
	out.QuestionCount = len(out.Questions)
	start := min(offset, len(out.Capabilities))
	if instance != "" {
		start = 0
	}
	end := start + min(limit, len(out.Capabilities)-start)
	out.Capabilities = out.Capabilities[start:end]
	qstart := min(offset, len(out.Questions))
	qend := qstart + min(limit, len(out.Questions)-qstart)
	out.Questions = out.Questions[qstart:qend]
	dstart := min(offset, len(out.Duties))
	dend := dstart + min(limit, len(out.Duties)-dstart)
	out.Duties = out.Duties[dstart:dend]
	if end < out.Total || qend < out.QuestionCount || dend < out.DutyCount {
		next := offset + limit
		out.NextOffset = &next
	}
	return out, tx.Commit(ctx)
}
