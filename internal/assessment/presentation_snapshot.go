package assessment

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"time"
)

type PresentationSnapshot struct {
	Binding         InstructionBinding
	EvidenceRecords int
	OpenGaps        map[string]bool
	AsOf            time.Time
	Requirements    []catalog.Resolved
	States          map[string]RequirementPresentation
}

// Presentation reads one repeatable snapshot. Untouched rules are classified in
// memory after a single activity query, avoiding repeated status reads for the
// large catalog's untouched requirements. Active rules retain the shared domain
// projection, including exact citation and review freshness validation.
func (s *Store) Presentation(ctx context.Context, a auth.Actor, service string) (PresentationSnapshot, error) {
	out := PresentationSnapshot{AsOf: s.now(), States: map[string]RequirementPresentation{}}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "read"); err != nil {
		return out, err
	}
	view := *s
	view.reader = tx
	view.presentationCache, err = view.loadPresentationCache(ctx, service, a.ID, out.AsOf)
	if err != nil {
		return out, err
	}
	if pr, e := view.Get(ctx, a, service, "profile", "profile"); e == nil {
		profile, e := Decode[catalog.Profile](pr)
		if e != nil {
			return out, e
		}
		out.Binding.CatalogID = profile.CatalogID
		out.Binding.ProfileRevision = pr.Revision
	} else if !errors.Is(e, ErrNotFound) {
		return out, e
	}
	if sr, e := view.Get(ctx, a, service, "scope", "scope"); e == nil {
		out.Binding.ScopeRevision = sr.Revision
	} else if !errors.Is(e, ErrNotFound) {
		return out, e
	}
	out.OpenGaps = map[string]bool{}
	for _, record := range view.presentationCache.latest {
		if record.Kind == "gap" {
			g, e := Decode[Gap](record)
			if e != nil {
				return out, e
			}
			if g.Status == "open" {
				out.OpenGaps[record.RequirementID] = true
			}
		}
	}
	for _, record := range view.presentationCache.latest {
		if record.Kind == "evidence" {
			out.EvidenceRecords++
		}
	}
	out.Requirements, err = view.Requirements(ctx, a, service)
	if errors.Is(err, ErrNotFound) {
		return out, tx.Commit(ctx)
	}
	if err != nil {
		return out, err
	}
	if view.Guidance != nil {
		pr, err := view.Get(ctx, a, service, "profile", "profile")
		if err != nil {
			return out, err
		}
		profile, err := Decode[catalog.Profile](pr)
		if err != nil {
			return out, err
		}
		if err = view.loadGuidanceContexts(ctx, a, service, profile.CatalogID, out.Requirements, out.AsOf); err != nil {
			return out, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT requirement_id FROM records WHERE service_id=$1 AND kind IN ('assessment','rule_result','evidence') AND requirement_id<>'' UNION SELECT requirement_id FROM jobs WHERE service_id=$1 UNION SELECT requirement_id FROM observation_links WHERE service_id=$1 UNION SELECT requirement_id FROM rule_evidence_links WHERE service_id=$1`, service)
	if err != nil {
		return out, err
	}
	active := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		active[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, req := range out.Requirements {
		status := Status{}
		if active[req.Requirement.ID] {
			status, err = view.Status(ctx, a, service, req.Requirement.ID, out.AsOf)
			if err != nil {
				return out, err
			}
		}
		out.States[req.Requirement.ID] = PresentRequirement(req.Applicability, status, active[req.Requirement.ID])
		if g, ok := view.guidanceContexts[req.Requirement.ID]; ok {
			out.States[req.Requirement.ID] = WithGuidanceAction(out.States[req.Requirement.ID], g)
		}
	}
	return out, tx.Commit(ctx)
}
