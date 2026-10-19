package assessment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

const instructionVersion = "agent-evidence/v1"

// RuleDetail projects instructions and status in one authorized snapshot. Both
// need the same guidance, so reuse the request-local projection rather than
// decoding and source-validating the full release twice for a browser page.
func (s *Store) RuleDetail(ctx context.Context, a auth.Actor, service, rid string) (RuleInstructions, Status, error) {
	if err := s.ReconcileEvidenceSources(ctx, a, service); err != nil {
		return RuleInstructions{}, Status{}, err
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return RuleInstructions{}, Status{}, err
	}
	defer tx.Rollback(ctx)
	view := *s
	view.reader = tx
	instructions, err := view.RuleInstructions(ctx, a, service, rid)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return instructions, Status{}, err
		}
		// A service without saved scope can still display its status, but has
		// no actionable instruction binding yet.
		instructions = RuleInstructions{}
	}
	status, err := view.Status(ctx, a, service, rid, view.now())
	if err != nil {
		return instructions, status, err
	}
	return instructions, status, tx.Commit(ctx)
}

func (s *Store) RuleInstructions(ctx context.Context, a auth.Actor, service, rid string) (RuleInstructions, error) {
	if s.reader == nil {
		if err := s.ReconcileEvidenceSources(ctx, a, service); err != nil {
			return RuleInstructions{}, err
		}
		tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return RuleInstructions{}, err
		}
		defer tx.Rollback(ctx)
		view := *s
		view.reader = tx
		out, err := view.RuleInstructions(ctx, a, service, rid)
		if err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	out := RuleInstructions{ServiceID: service, GuidanceKind: "generic", Assertions: []Assertion{}, Sources: []Record{}}
	all, err := s.Requirements(ctx, a, service)
	if err != nil {
		return out, err
	}
	found := false
	for _, r := range all {
		if r.Requirement.ID == rid {
			out.Rule = r
			found = true
			break
		}
	}
	if !found {
		return out, ErrNotFound
	}
	pr, err := s.Get(ctx, a, service, "profile", "profile")
	if err != nil {
		return out, err
	}
	p, err := Decode[catalog.Profile](pr)
	if err != nil {
		return out, err
	}
	sr, err := s.Get(ctx, a, service, "scope", "scope")
	if err != nil {
		return out, err
	}
	scope, err := Decode[Scope](sr)
	if err != nil {
		return out, err
	}
	out.Binding = InstructionBinding{CatalogID: p.CatalogID, ProfileRevision: pr.Revision, ScopeRevision: sr.Revision, Version: instructionVersion, Sources: []Ref{}}
	for offset := 0; ; offset += 100 {
		sources, err := s.List(ctx, a, service, "evidence_source", "", 100, offset)
		if err != nil {
			return out, err
		}
		for _, r := range sources {
			v, err := Decode[EvidenceSource](r)
			if err != nil {
				return out, err
			}
			if !v.Enabled || !sourceInScope(v, scope) || (len(v.RequirementIDs) > 0 && !slices.Contains(v.RequirementIDs, rid)) {
				continue
			}
			out.Sources = append(out.Sources, r)
			out.Binding.Sources = append(out.Binding.Sources, Ref{r.ID, r.Revision})
		}
		if len(sources) < 100 {
			break
		}
	}
	out.Assertions = []Assertion{{ID: "whole-statement", Statement: out.Rule.Statement, Required: true, EvidenceKinds: []string{"document", "repository", "operational_record", "configuration"}}}
	out.Steps = []string{"Inspect the full official statement and applicability; identify every clause requiring evidence.", "Read the configured sources using your own tools and credentials, staying within saved boundaries.", "Gather source material and operational examples; distinguish documented intent from observed practice.", "Submit bounded text/JSON artifacts, cite exact line ranges or JSON pointers, and assess the entire statement with explicit limitations.", "Submit a rule result with supported, contradicted or unknown assertions; record specific unavailable inputs as blockers."}
	if p.CatalogID == MappedCatalog {
		if guidance, ok := reviewedInstructions[rid]; ok && guidance.Statement == out.Rule.Statement {
			out.GuidanceKind = "reviewed_application_guidance"
			out.Steps = append(append([]string{}, guidance.Steps...), out.Steps...)
		}
	}
	out.ExistingEvidence, err = s.List(ctx, a, service, "evidence", rid, 100, 0)
	if err != nil {
		return out, err
	}
	result, err := s.Get(ctx, a, service, "rule_result", rid)
	if err == nil {
		out.ExpectedResultRevision = result.Revision
	} else if !errors.Is(err, ErrNotFound) {
		return out, err
	}
	draft, err := s.Get(ctx, a, service, "assessment", rid)
	if err == nil {
		out.ExpectedDraftRevision = draft.Revision
	} else if !errors.Is(err, ErrNotFound) {
		return out, err
	}
	if s.Guidance != nil {
		if s.guidanceContexts == nil {
			if err := s.loadGuidanceContexts(ctx, a, service, p.CatalogID, all, s.now()); err != nil {
				return out, err
			}
		}
		guidance, e := s.guidanceFor(ctx, a, service, rid, p.CatalogID, s.now())
		if e != nil {
			return out, e
		}
		out.Guidance = guidance
		if assertions := bindGuidance(&out.Binding, guidance); len(assertions) > 0 {
			out.Assertions = assertions
			out.GuidanceKind = "source_reviewed_playbook"
			out.Steps = []string{}
			book := guidance.Guidance.Playbooks[0]
			for _, step := range book.Collect.Items {
				out.Steps = append(out.Steps, step.Action.Text)
			}
			for _, step := range book.ReviewSteps.Items {
				out.Steps = append(out.Steps, step.Action.Text)
			}
		}
	}
	out.Work, err = s.ruleWork(ctx, a, service, rid, out.Binding, s.now())
	if err == nil {
		coverage, e := s.evidenceCoverage(ctx, a, out, s.now())
		if e != nil {
			return out, e
		}
		out.EvidenceCoverage = &coverage
	}
	return out, err
}

// These guides were checked against the pinned official statements. They keep
// the full statement as the required assertion so a technical subset cannot
// silently stand in for the complete obligation.
var reviewedInstructions = map[string]struct {
	Statement string
	Steps     []string
}{
	"KSI-SVC-ACM": {
		Statement: "The configuration of machine-based information resources is managed using automation and persistently reviewed for drift.",
		Steps:     []string{"Inspect pinned IaC and deployment workflows for automation covering the scoped machine-based resources.", "Find drift detection configuration and dated execution/review records. Repository intent alone does not show persistent review.", "Cite both automation and drift-review evidence; describe uncovered resources and gaps rather than concluding from one workflow file."},
	},
	"KSI-RPL-RRO": {
		Statement: "The desired Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO) are defined and persistently reviewed for alignment with the provider's business needs and capabilities.",
		Steps:     []string{"Find approved RTO and RPO definitions for the scoped service in configured policies, recovery plans or operational records.", "Collect dated reviews tying those objectives to business needs and demonstrated recovery capabilities.", "Do not invent customer recovery objectives or equate backup retention with an approved RPO; report missing definitions or review records explicitly."},
	},
}

type storedRuleResult struct {
	Submission RuleResultSubmission `json:"submission"`
	Receipt    RuleReceipt          `json:"receipt"`
}

func (s *Store) ruleWork(ctx context.Context, a auth.Actor, service, rid string, binding InstructionBinding, now time.Time) (EvidenceWork, error) {
	out := EvidenceWork{State: "open", Assessment: "unknown", MissingAssertions: []string{"whole-statement"}, Blockers: []string{}, Reasons: []string{}}
	g, err := s.guidanceFor(ctx, a, service, rid, binding.CatalogID, now)
	if err != nil {
		return out, err
	}
	assertions := bindGuidance(&binding, g)
	if len(assertions) > 0 {
		out.MissingAssertions = []string{}
		for _, assertion := range assertions {
			out.MissingAssertions = append(out.MissingAssertions, assertion.ID)
		}
	}
	r, err := s.Get(ctx, a, service, "rule_result", rid)
	if errors.Is(err, ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	stored, err := Decode[storedRuleResult](r)
	if err != nil {
		return out, err
	}
	out = stored.Receipt.Work
	old := stored.Submission.Binding
	if old.AssertionHash != binding.AssertionHash || old.GuidanceVariant != binding.GuidanceVariant || (g != nil && g.State == "needs_revalidation") {
		out.Reasons = append(out.Reasons, "guidance or assertion coverage changed; reassess against current sources")
		out.MissingAssertions = []string{}
		for _, assertion := range assertions {
			out.MissingAssertions = append(out.MissingAssertions, assertion.ID)
		}
	}
	if old.CatalogID != binding.CatalogID || old.ProfileRevision != binding.ProfileRevision || old.ScopeRevision != binding.ScopeRevision || old.Version != binding.Version {
		out.Reasons = append(out.Reasons, "rule or boundary changed")
	}
	for _, ref := range stored.Receipt.Evidence {
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
			if source.Revision != ev.Document.Source.Revision {
				out.Reasons = append(out.Reasons, "source changed")
			}
			if !ev.ObservedAt.Add(time.Duration(ev.Document.FreshnessHours) * time.Hour).After(now) {
				out.Reasons = append(out.Reasons, "evidence expired")
			}
		} else {
			var hours *int
			if s.presentationCache != nil {
				hours = s.presentationCache.measureHours[rid]
			} else {
				err = s.query().QueryRow(ctx, `SELECT min((data->>'freshness_hours')::int) FROM (SELECT DISTINCT ON(id) data FROM records WHERE service_id=$1 AND kind='measure' AND requirement_id=$2 ORDER BY id,revision DESC) m`, service, rid).Scan(&hours)
			}
			if err != nil {
				return out, err
			}
			if hours == nil || !ev.ObservedAt.Add(time.Duration(*hours)*time.Hour).After(now) {
				out.Reasons = append(out.Reasons, "legacy evidence expired or missing freshness policy")
			}
		}
	}
	if out.Assessment == "supported" {
		// Whole-statement assertions include every condition explicitly mapped to
		// this requirement, never findings belonging to unrelated rules.
		var contradictory bool
		if s.presentationCache != nil {
			contradictory = s.presentationCache.contradictions[rid].After(r.CreatedAt)
		} else {
			err = s.query().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finding_events f JOIN observations o ON o.service_id=f.service_id AND o.id=f.observation_id JOIN observation_links l ON l.service_id=o.service_id AND l.observation_id=o.id WHERE f.service_id=$1 AND l.requirement_id=$2 AND o.received_at>$3 AND o.scope_revision=$4 AND o.profile_revision=$5 AND f.outcome='fail' AND (coalesce(jsonb_array_length(l.data->'conditions'),0)=0 OR (l.data->'conditions') ? f.condition))`, service, rid, r.CreatedAt, binding.ScopeRevision, binding.ProfileRevision).Scan(&contradictory)
		}
		if err != nil {
			return out, err
		}
		if contradictory {
			out.Reasons = append(out.Reasons, "new observation contradicts whole-statement support")
		}
	}
	if len(out.Reasons) > 0 {
		out.State = "needs_refresh"
	}
	return out, nil
}

type OpenRulesInput struct {
	ServiceID       string `json:"service_id"`
	Limit           int    `json:"limit,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	Kind            string `json:"kind,omitempty"`
	IncludeComplete bool   `json:"include_complete,omitempty"`
}
type OpenRule struct {
	RequirementID string       `json:"requirement_id"`
	Name          string       `json:"name"`
	Applicability string       `json:"applicability"`
	Work          EvidenceWork `json:"work"`
}
type OpenRulesPage struct {
	Items      []OpenRule `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}
type workCursor struct {
	Service string `json:"service"`
	Filter  string `json:"filter"`
	View    string `json:"view"`
	Offset  int    `json:"offset"`
}

func (s *Store) ListOpenRules(ctx context.Context, a auth.Actor, in OpenRulesInput) (OpenRulesPage, error) {
	out := OpenRulesPage{Items: []OpenRule{}}
	if in.Limit == 0 {
		in.Limit = 25
	}
	if in.Limit < 1 || in.Limit > 100 || (in.Kind != "" && in.Kind != "KSI" && in.Kind != "FRR") {
		return out, ErrInvalid
	}
	if err := s.ReconcileEvidenceSources(ctx, a, in.ServiceID); err != nil {
		return out, err
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	view := *s
	view.reader = tx
	all, err := view.Requirements(ctx, a, in.ServiceID)
	if err != nil {
		return out, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Requirement.ID < all[j].Requirement.ID })
	items := []OpenRule{}
	pr, err := view.Get(ctx, a, in.ServiceID, "profile", "profile")
	if err != nil {
		return out, err
	}
	profile, err := Decode[catalog.Profile](pr)
	if err != nil {
		return out, err
	}
	sr, err := view.Get(ctx, a, in.ServiceID, "scope", "scope")
	if err != nil {
		return out, err
	}
	binding := InstructionBinding{CatalogID: profile.CatalogID, ProfileRevision: pr.Revision, ScopeRevision: sr.Revision, Version: instructionVersion}
	now := s.now()
	if err = view.loadGuidanceContexts(ctx, a, in.ServiceID, profile.CatalogID, all, now); err != nil {
		return out, err
	}
	for _, r := range all {
		if in.Kind != "" && r.Requirement.Kind != in.Kind {
			continue
		}
		work, err := view.ruleWork(ctx, a, in.ServiceID, r.Requirement.ID, binding, now)
		if err != nil {
			return out, err
		}
		if !in.IncludeComplete && (r.Applicability == "not_applicable" || r.Applicability == "not-applicable" || work.State == "evidence_complete") {
			continue
		}
		items = append(items, OpenRule{r.Requirement.ID, r.Requirement.Name, r.Applicability, work})
	}
	var revision int64
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT revision FROM evidence_work_versions WHERE service_id=$1),0)`, in.ServiceID).Scan(&revision); err != nil {
		return out, err
	}
	digest := storage.Hash(data(struct {
		Revision int64
		Items    []OpenRule
	}{revision, items}))
	filter := storage.Hash(data(struct {
		Kind     string
		Complete bool
	}{in.Kind, in.IncludeComplete}))
	offset := 0
	if in.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(in.Cursor)
		var c workCursor
		if e != nil || json.Unmarshal(raw, &c) != nil || c.Offset < 0 || c.Service != in.ServiceID || c.Filter != filter {
			return out, ErrInvalid
		}
		if c.View != digest {
			return out, ErrViewChanged
		}
		offset = c.Offset
	}
	if offset > len(items) {
		return out, ErrInvalid
	}
	end := min(offset+in.Limit, len(items))
	out.Items = items[offset:end]
	if end < len(items) {
		out.NextCursor = base64.RawURLEncoding.EncodeToString(data(workCursor{in.ServiceID, filter, digest, end}))
	}
	return out, tx.Commit(ctx)
}
