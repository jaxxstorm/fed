package readiness

import (
	"fmt"
	"strings"
)

// Validation establishes traceability, not semantic correctness. Reviewers must
// actually compare policy and guidance; the visible caveat preserves that limit.
func ValidateComparison(p Package, id string, comparison SourceComparison) error {
	subset, err := RelevantPackage(p, id)
	if err != nil {
		return err
	}
	return validateSubsetComparison(subset, comparison)
}

func validateSubsetComparison(subset Package, comparison SourceComparison) error {
	available := map[string]bool{}
	used := map[string]bool{}
	for _, c := range subset.Citations {
		available[c.ID] = true
	}
	findings := []ReviewFinding{comparison.Applicability, comparison.Obligations, comparison.Prerequisites, comparison.Evidence, comparison.Timing}
	findings = append(findings, comparison.Uncertainties...)
	for _, f := range findings {
		if strings.TrimSpace(f.Finding) == "" || len(f.Citations) == 0 {
			return fmt.Errorf("source comparison requires findings and citations for every dimension")
		}
		for _, id := range f.Citations {
			if !available[id] {
				return fmt.Errorf("review citation %s is not bound to the playbook", id)
			}
			used[id] = true
		}
	}
	for id := range available {
		if !used[id] {
			return fmt.Errorf("source comparison omitted bound citation %s", id)
		}
	}
	if len(comparison.Limitations) == 0 || comparison.Uncertainties == nil {
		return fmt.Errorf("source comparison requires limitations and explicit uncertainties (empty if none)")
	}
	for _, limit := range comparison.Limitations {
		if strings.TrimSpace(limit) == "" {
			return fmt.Errorf("empty review limitation")
		}
	}
	return nil
}
