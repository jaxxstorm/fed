package assessment

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// ImplementationReadiness checks declared structure only. It does not determine
// whether prose is true, whether a sample is representative, or whether cited
// artifacts actually demonstrate the declared claims. Citation resolution and
// current instruction binding checks belong to the submission transaction.
func ImplementationReadiness(v ImplementationAssessment, assertions []Assertion, now time.Time) (StructuralReviewReadiness, error) {
	r := StructuralReviewReadiness{Basis: "structural_only; agent conclusions require human review", Blockers: []AssessmentReadinessBlocker{}}
	invalid := func(reason string) (StructuralReviewReadiness, error) {
		return r, fmt.Errorf("%w: %s", ErrInvalid, reason)
	}
	if v.Version != ImplementationAssessmentVersion || v.Period.Start.IsZero() || v.Period.End.IsZero() || v.Period.End.Before(v.Period.Start) || v.Period.End.After(now) || len(v.Assertions) > 100 {
		return invalid("invalid implementation version, period or assertion count")
	}
	fields := []struct {
		name  string
		value NarrativeField
	}{
		{"description", v.Narrative.Description}, {"responsible_roles", v.Narrative.ResponsibleRoles},
		{"mechanisms", v.Narrative.Mechanisms}, {"procedures", v.Narrative.Procedures},
		{"frequency", v.Narrative.Frequency}, {"boundary", v.Narrative.Boundary},
	}
	for _, f := range fields {
		if !slices.Contains([]string{"known", "unknown", "not_applicable"}, f.value.State) || len(f.value.Text) > 16384 || len(f.value.Reason) > 4096 {
			return invalid("invalid narrative field " + f.name)
		}
		if f.value.State == "known" && strings.TrimSpace(f.value.Text) == "" || f.value.State != "known" && strings.TrimSpace(f.value.Reason) == "" {
			r.Blockers = append(r.Blockers, AssessmentReadinessBlocker{Code: "narrative_incomplete", Path: "narrative." + f.name, Explanation: "Provide the implementation detail or an explicit unknown/non-applicability reason.", NextAction: "Investigate the scoped sources and record the result."})
		}
	}
	expected := map[string]bool{}
	for _, a := range assertions {
		expected[a.ID] = true
	}
	seen := map[string]bool{}
	for _, a := range v.Assertions {
		if !expected[a.AssertionID] || seen[a.AssertionID] || strings.TrimSpace(a.Claim) == "" || len(a.Claim) > 16384 {
			return invalid("unknown, duplicate or incomplete assertion")
		}
		seen[a.AssertionID] = true
		for _, support := range []EvidenceSupport{a.Implementation, a.Operating} {
			if !slices.Contains([]string{"supported", "contradicted", "unknown"}, support.Outcome) || strings.TrimSpace(support.Rationale) == "" || len(support.Rationale) > 16384 || len(support.Citations) > 100 || len(support.Limitations) > 100 {
				return invalid("invalid agent support conclusion")
			}
			if support.Outcome != "unknown" && len(support.Citations) == 0 {
				return invalid("supported or contradicted conclusions require citations")
			}
			for _, limitation := range support.Limitations {
				if strings.TrimSpace(limitation) == "" || len(limitation) > 4096 {
					return invalid("invalid limitation")
				}
			}
		}
		sample := a.Sample
		if strings.TrimSpace(sample.Method) == "" || len(sample.Method) > 4096 || strings.TrimSpace(sample.CoverageReason) == "" || len(sample.CoverageReason) > 4096 || len(sample.PopulationUnknownReason) > 4096 || sample.SampleSize < 0 || len(sample.Events) > 100 || !slices.Contains([]string{"none", "sample", "population"}, sample.Coverage) {
			return invalid("invalid operational sample")
		}
		if sample.Population == nil && strings.TrimSpace(sample.PopulationUnknownReason) == "" || sample.Population != nil && (*sample.Population < 0 || sample.SampleSize > *sample.Population) {
			return invalid("invalid or unexplained sample population")
		}
		if sample.SampleSize != len(sample.Events) {
			return invalid("sample size must match cited event count")
		}
		if sample.Coverage == "none" && sample.SampleSize != 0 || sample.Coverage == "sample" && sample.SampleSize == 0 || sample.Coverage == "population" && (sample.Population == nil || *sample.Population != sample.SampleSize || sample.SampleSize == 0) {
			return invalid("declared coverage does not match sample")
		}
		if a.Operating.Outcome == "supported" && len(sample.Events) == 0 {
			return invalid("operating support requires dated events, not configuration alone")
		}
		events := map[string]bool{}
		for _, event := range sample.Events {
			if event.OccurredAt.Before(v.Period.Start) || event.OccurredAt.After(v.Period.End) || strings.TrimSpace(event.Description) == "" || len(event.Description) > 4096 {
				return invalid("event is outside reporting period or lacks description")
			}
			key := string(data(struct {
				Citation Citation
				At       time.Time
			}{event.Citation, event.OccurredAt.UTC()}))
			if events[key] {
				return invalid("duplicate sampled event")
			}
			events[key] = true
			found := false
			for _, c := range a.Operating.Citations {
				if string(data(c)) == string(data(event.Citation)) {
					found = true
					break
				}
			}
			if !found {
				return invalid("sample event must cite operating evidence")
			}
		}
	}
	for _, a := range assertions {
		if !seen[a.ID] {
			r.Blockers = append(r.Blockers, AssessmentReadinessBlocker{Code: "assertion_missing", Path: "assertions." + a.ID, Explanation: "The selected assertion has no explicit conclusion.", NextAction: "Record supported, contradicted or unknown conclusions and their rationale."})
		}
	}
	r.Ready = len(r.Blockers) == 0
	return r, nil
}
