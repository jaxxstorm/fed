package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func validateCitation(content []byte, media string, c Citation) error {
	if media == "application/json" {
		if c.JSONPointer == nil || c.StartLine != 0 || c.EndLine != 0 {
			return ErrInvalid
		}
		var value any
		if json.Unmarshal(content, &value) != nil {
			return ErrInvalid
		}
		pointer := *c.JSONPointer
		if pointer == "" {
			return nil
		}
		if !strings.HasPrefix(pointer, "/") {
			return ErrInvalid
		}
		for _, token := range strings.Split(pointer[1:], "/") {
			for i := 0; i < len(token); i++ {
				if token[i] == '~' {
					if i+1 >= len(token) || (token[i+1] != '0' && token[i+1] != '1') {
						return ErrInvalid
					}
					i++
				}
			}
			token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			switch v := value.(type) {
			case map[string]any:
				var ok bool
				value, ok = v[token]
				if !ok {
					return ErrInvalid
				}
			case []any:
				n, e := strconv.Atoi(token)
				if e != nil || n < 0 || n >= len(v) || strconv.Itoa(n) != token {
					return ErrInvalid
				}
				value = v[n]
			default:
				return ErrInvalid
			}
		}
		return nil
	}
	if c.JSONPointer != nil || c.StartLine < 1 || c.EndLine < c.StartLine || c.EndLine > len(strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")) {
		return ErrInvalid
	}
	return nil
}

func (s *Store) SubmitRuleResult(ctx context.Context, a auth.Actor, in RuleResultSubmission) (RuleReceipt, error) {
	var receipt RuleReceipt
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "edit"); err != nil {
		return receipt, err
	}
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || in.ExpectedResultRevision < 0 || in.ExpectedDraftRevision < 0 || strings.TrimSpace(in.Explanation) == "" || len(in.Explanation) > 16384 || len(in.Assertions) > 100 || len(in.Blockers) > 100 || !slices.Contains([]string{"evidence_complete", "partial", "blocked"}, in.RequestedState) {
		return receipt, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return receipt, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "edit"); err != nil {
		return receipt, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return receipt, err
	}
	digest := storage.Hash(data(in))
	if found, e := readOperation(ctx, tx, a, in.ServiceID, "submit_rule_result", in.IdempotencyKey, digest, &receipt); found || e != nil {
		return receipt, e
	}
	view := *s
	view.reader = tx
	instructions, err := view.RuleInstructions(ctx, a, in.ServiceID, in.RequirementID)
	if err != nil {
		return receipt, err
	}
	if string(data(instructions.Binding)) != string(data(in.Binding)) || instructions.ExpectedResultRevision != in.ExpectedResultRevision || instructions.ExpectedDraftRevision != in.ExpectedDraftRevision {
		return receipt, ErrConflict
	}
	var structural *StructuralReviewReadiness
	if in.Implementation != nil {
		readiness, err := ImplementationReadiness(*in.Implementation, instructions.Assertions, time.Now())
		if err != nil {
			return receipt, err
		}
		// Derivation is server-side so callers cannot validate one set of
		// citations while persisting a different structured assessment.
		in.Assertions = implementationAssertionResults(*in.Implementation)
		structural = &readiness
	}
	work := EvidenceWork{State: in.RequestedState, Assessment: "supported", MissingAssertions: []string{}, Blockers: in.Blockers, Reasons: []string{}}
	assertions := map[string]Assertion{}
	for _, v := range instructions.Assertions {
		assertions[v.ID] = v
	}
	seen := map[string]bool{}
	evidence := map[string]Ref{}
	currentFreshness := "missing"
	for _, result := range in.Assertions {
		if _, ok := assertions[result.AssertionID]; !ok || seen[result.AssertionID] || strings.TrimSpace(result.Rationale) == "" || len(result.Rationale) > 16384 || len(result.Citations) > 100 || !slices.Contains([]string{"supported", "contradicted", "unknown"}, result.Outcome) {
			return receipt, ErrInvalid
		}
		seen[result.AssertionID] = true
		if err := validateAssertionGaps(result); err != nil {
			return receipt, err
		}
		if result.Outcome == "contradicted" {
			work.Assessment = "contradicted"
		} else if result.Outcome == "unknown" && work.Assessment == "supported" {
			work.Assessment = "unknown"
		}
		if result.Outcome == "unknown" || len(result.Citations) == 0 || len(result.Gaps) > 0 {
			work.MissingAssertions = append(work.MissingAssertions, result.AssertionID)
		}
		for _, citation := range result.Citations {
			if currentFreshness == "missing" {
				currentFreshness = "current"
			}
			er, err := reference(ctx, tx, in.ServiceID, "evidence", citation.Evidence)
			if err != nil {
				return receipt, err
			}
			ev, err := Decode[Evidence](er)
			if err != nil {
				return receipt, err
			}
			if ev.ScopeRevision != in.Binding.ScopeRevision {
				return receipt, ErrConflict
			}
			media := "application/json"
			if ev.Document != nil {
				media = ev.Document.MediaType
				if !slices.Contains(in.Binding.Sources, ev.Document.Source) {
					return receipt, ErrConflict
				}
				if !ev.ObservedAt.Add(time.Duration(ev.Document.FreshnessHours) * time.Hour).After(time.Now()) {
					if in.Implementation == nil {
						return receipt, fmt.Errorf("%w: evidence %s expired", ErrInvalid, citation.Evidence.ID)
					}
					currentFreshness = "stale"
				}
			} else {
				// Legacy records retain their original requirement/measure contract.
				if er.RequirementID != in.RequirementID {
					return receipt, ErrInvalid
				}
				var hours *int
				if err = tx.QueryRow(ctx, `SELECT min((data->>'freshness_hours')::int) FROM (SELECT DISTINCT ON(id) data FROM records WHERE service_id=$1 AND kind='measure' AND requirement_id=$2 ORDER BY id,revision DESC) m`, in.ServiceID, in.RequirementID).Scan(&hours); err != nil {
					return receipt, err
				}
				if hours == nil || !ev.ObservedAt.Add(time.Duration(*hours)*time.Hour).After(time.Now()) {
					if in.Implementation == nil {
						return receipt, ErrInvalid
					}
					currentFreshness = "stale"
				}
			}
			if s.Artifacts == nil || ev.ArtifactKey == "" {
				return receipt, ErrInvalid
			}
			content, err := s.Artifacts.Get(ctx, ev.ArtifactKey)
			if err != nil {
				return receipt, err
			}
			if err = validateCitation(content, media, citation); err != nil {
				return receipt, fmt.Errorf("%w: invalid citation for %s", err, citation.Evidence.ID)
			}
			evidence[citation.Evidence.ID+fmt.Sprint(citation.Evidence.Revision)] = citation.Evidence
		}
	}
	for _, v := range instructions.Assertions {
		if v.Required && !seen[v.ID] {
			work.MissingAssertions = append(work.MissingAssertions, v.ID)
		}
	}
	if len(work.MissingAssertions) > 0 && work.Assessment == "supported" {
		work.Assessment = "unknown"
	}
	if in.RequestedState == "blocked" && len(in.Blockers) == 0 {
		return receipt, ErrInvalid
	}
	for _, blocker := range in.Blockers {
		if strings.TrimSpace(blocker) == "" || len(blocker) > 4096 {
			return receipt, ErrInvalid
		}
	}
	if in.RequestedState == "evidence_complete" && (len(work.MissingAssertions) > 0 || len(in.Blockers) > 0 || !slices.Contains([]string{"required", "recommended", "optional"}, instructions.Rule.Applicability)) {
		return receipt, fmt.Errorf("%w: complete requires cited required assertions, resolved applicability and no blockers; missing=%v", ErrInvalid, work.MissingAssertions)
	}
	receipt = RuleReceipt{Result: Ref{in.RequirementID, in.ExpectedResultRevision + 1}, Draft: Ref{in.RequirementID, in.ExpectedDraftRevision + 1}, Evidence: []Ref{}, Work: work, Review: "pending"}
	receipt.StructuralReadiness = structural
	if structural != nil {
		receipt.CurrentFreshness = currentFreshness
	}
	for _, ref := range evidence {
		receipt.Evidence = append(receipt.Evidence, ref)
	}
	slices.SortFunc(receipt.Evidence, func(a, b Ref) int { return strings.Compare(a.ID, b.ID) })
	var reviewed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM records WHERE service_id=$1 AND kind='review' AND requirement_id=$2)`, in.ServiceID, in.RequirementID).Scan(&reviewed); err != nil {
		return receipt, err
	}
	if reviewed {
		receipt.Review = "review-needed"
	}
	if _, err = insertRecord(ctx, tx, a, record(in.ServiceID, "rule_result", in.RequirementID, in.RequirementID, storedRuleResult{in, receipt}), receipt.Result.Revision); err != nil {
		return receipt, err
	}
	for _, result := range in.Assertions {
		for _, citation := range result.Citations {
			if _, err = tx.Exec(ctx, `INSERT INTO rule_evidence_links(service_id,requirement_id,evidence_id,evidence_revision,result_id,result_revision,assertion_id,data) VALUES($1,$2,$3,$4,$2,$5,$6,$7) ON CONFLICT DO NOTHING`, in.ServiceID, in.RequirementID, citation.Evidence.ID, citation.Evidence.Revision, receipt.Result.Revision, result.AssertionID, data(citation)); err != nil {
				return receipt, err
			}
		}
	}
	draft := Draft{Explanation: in.Explanation, Evidence: receipt.Evidence, Measures: []Ref{}, ProfileRevision: in.Binding.ProfileRevision, ScopeRevision: in.Binding.ScopeRevision}
	draft.Implementation = in.Implementation
	// Preserve legacy measure references for legacy evidence freshness.
	measures, err := view.List(ctx, a, in.ServiceID, "measure", in.RequirementID, 100, 0)
	if err != nil {
		return receipt, err
	}
	for _, m := range measures {
		draft.Measures = append(draft.Measures, Ref{m.ID, m.Revision})
	}
	if _, err = insertRecord(ctx, tx, a, record(in.ServiceID, "assessment", in.RequirementID, in.RequirementID, draft), receipt.Draft.Revision); err != nil {
		return receipt, err
	}
	if err = saveOperation(ctx, tx, a, in.ServiceID, "submit_rule_result", in.IdempotencyKey, digest, receipt); err != nil {
		return receipt, err
	}
	return receipt, commitOperation(ctx, tx, a, in.ServiceID, "rule_result.submitted", Record{ID: receipt.Draft.ID, Revision: receipt.Draft.Revision})
}

func validateAssertionGaps(result AssertionResult) error {
	if len(result.Gaps) > 100 || (result.Outcome == "supported" && len(result.Gaps) > 0) {
		return fmt.Errorf("%w: supported assertions cannot retain unresolved coverage gaps", ErrInvalid)
	}
	seen := map[AssertionGap]bool{}
	for _, gap := range result.Gaps {
		if seen[gap] {
			return fmt.Errorf("%w: duplicate assertion gap", ErrInvalid)
		}
		seen[gap] = true
		if !slices.Contains([]string{"missing", "partial", "stale", "access_blocked"}, gap.Kind) || strings.TrimSpace(gap.Scope) == "" || strings.TrimSpace(gap.Reason) == "" || strings.TrimSpace(gap.NextAction) == "" || len(gap.Scope) > 4096 || len(gap.Period) > 4096 || len(gap.Reason) > 4096 || len(gap.NextAction) > 4096 {
			return fmt.Errorf("%w: coverage gaps require a kind, scope, reason and next action", ErrInvalid)
		}
	}
	return nil
}

// WorkStatus does not require a collection plan or a draft.
func (s *Store) WorkStatus(ctx context.Context, a auth.Actor, service, rid string, now time.Time) (EvidenceWork, error) {
	pr, err := s.Get(ctx, a, service, "profile", "profile")
	if err != nil {
		return EvidenceWork{}, err
	}
	var profile struct {
		CatalogID string `json:"catalog_id"`
	}
	if err = json.Unmarshal(pr.Data, &profile); err != nil {
		return EvidenceWork{}, err
	}
	sr, err := s.Get(ctx, a, service, "scope", "scope")
	if errors.Is(err, ErrNotFound) {
		return EvidenceWork{State: "open", Assessment: "unknown"}, nil
	}
	if err != nil {
		return EvidenceWork{}, err
	}
	return s.ruleWork(ctx, a, service, rid, InstructionBinding{CatalogID: profile.CatalogID, ProfileRevision: pr.Revision, ScopeRevision: sr.Revision, Version: instructionVersion}, now)
}
