package guidancecontent

import (
	"encoding/json"
	"fmt"

	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

type ComparisonInput struct {
	ContentHash string                     `json:"content_hash"`
	Comparison  readiness.SourceComparison `json:"comparison"`
}

type ComparisonBundle struct {
	RecordingState string                     `json:"recording_state"`
	Comparisons    map[string]ComparisonInput `json:"comparisons"`
}

// RebindComparison changes citation identifiers only. It refuses changed
// playbook/dependency/source content, so assembly cannot silently carry a source
// comparison onto a different interpretation. The result still requires an
// authenticated service-scoped RecordContentReview call.
func RebindComparison(original, assembled readiness.Package, id string, input ComparisonInput) (ComparisonInput, error) {
	normalized, err := Assemble([]readiness.Package{original}, assembled.Revision)
	if err != nil {
		return ComparisonInput{}, err
	}
	return rebindComparison(original, normalized, assembled, id, input)
}

func rebindComparison(original, normalized, assembled readiness.Package, id string, input ComparisonInput) (ComparisonInput, error) {
	var out ComparisonInput
	hash, err := readiness.ReviewHash(original, id)
	if err != nil {
		return out, err
	}
	if input.ContentHash != hash {
		return out, fmt.Errorf("source comparison for %s is stale", id)
	}
	if err = readiness.ValidateComparison(original, id, input.Comparison); err != nil {
		return out, err
	}
	expected, err := readiness.ReviewHash(normalized, id)
	if err != nil {
		return out, err
	}
	actual, err := readiness.ReviewHash(assembled, id)
	if err != nil {
		return out, err
	}
	if expected != actual {
		return out, fmt.Errorf("assembled dependency or source closure changed for %s; a new source comparison is required", id)
	}
	ids := map[string]string{}
	for _, c := range original.Citations {
		old := c.ID
		c.ID = ""
		raw, err := json.Marshal(c)
		if err != nil {
			return out, err
		}
		ids[old] = "citation-" + storage.Hash(raw)
	}
	raw, err := json.Marshal(input.Comparison)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out.Comparison); err != nil {
		return out, err
	}
	findings := []*readiness.ReviewFinding{&out.Comparison.Applicability, &out.Comparison.Obligations, &out.Comparison.Prerequisites, &out.Comparison.Evidence, &out.Comparison.Timing}
	for i := range out.Comparison.Uncertainties {
		findings = append(findings, &out.Comparison.Uncertainties[i])
	}
	for _, finding := range findings {
		for i, ref := range finding.Citations {
			finding.Citations[i] = ids[ref]
		}
	}
	out.ContentHash = actual
	return out, readiness.ValidateComparison(assembled, id, out.Comparison)
}
