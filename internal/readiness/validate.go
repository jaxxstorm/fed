package readiness

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// ValidateDraft validates references and structure without approving content.
// Semantic sufficiency and inferred prerequisites still require content review.
func ValidateDraft(p Package) error {
	if p.Format != Format || p.CatalogID == "" || p.Revision < 1 {
		return fmt.Errorf("invalid package identity")
	}
	citations := map[string]bool{}
	excerpts := map[string]string{}
	for _, c := range p.Citations {
		if c.ID == "" || citations[c.ID] {
			return fmt.Errorf("invalid citation ID %q", c.ID)
		}
		citations[c.ID] = true
		excerpts[c.ID] = strings.ToLower(strings.Join(strings.Fields(c.Excerpt), " "))
	}
	// Many playbooks cite the same official timing definitions. Parse each
	// distinct text once per validation, without caching any acceptance decision.
	timingByText := map[string][]string{}
	timings := func(text string) []string {
		if found, ok := timingByText[text]; ok {
			return found
		}
		found := numericTiming.FindAllString(strings.ToLower(strings.Join(strings.Fields(text), " ")), -1)
		timingByText[text] = found
		return found
	}
	statement := func(s Statement) error {
		if strings.TrimSpace(s.Text) == "" || !slices.Contains([]Basis{Official, Recommendation, ServiceChoice}, s.Basis) || len(s.Citations) == 0 {
			return fmt.Errorf("statement needs text, authority and citations")
		}
		for _, id := range s.Citations {
			if !citations[id] {
				return fmt.Errorf("unknown citation %s", id)
			}
		}
		if s.Basis != Official && strings.TrimSpace(s.Rationale) == "" {
			return fmt.Errorf("interpretation needs rationale")
		}
		if s.Basis == Official {
			for _, duration := range timings(s.Text) {
				cited := false
				for _, id := range s.Citations {
					cited = cited || slices.Contains(timings(excerpts[id]), duration)
				}
				if !cited {
					return fmt.Errorf("statement invents uncited numeric timing %q; cite the official duration or label it a service choice", duration)
				}
			}
		}
		for _, generic := range []string{"ask an agent to gather evidence from the configured sources", "inspect the full official statement and applicability", "apply this step to", "inspect the affected resource against this condition"} {
			if strings.Contains(strings.ToLower(s.Text), generic) {
				return fmt.Errorf("generic instruction needs requirement-specific guidance")
			}
		}
		return nil
	}
	steps := func(section Steps) error {
		if (len(section.Items) == 0) == (strings.TrimSpace(section.NoneReason) == "") {
			return fmt.Errorf("section needs steps or an explicit none rationale, not both")
		}
		seen := map[string]bool{}
		for _, step := range section.Items {
			if step.ID == "" || seen[step.ID] || step.OwnerRole == "" || len(step.CompletionCriteria) == 0 || !slices.Contains([]string{"one_time", "recurring", "event"}, step.Trigger) {
				return fmt.Errorf("incomplete step %s", step.ID)
			}
			seen[step.ID] = true
			if err := statement(step.Action); err != nil {
				return err
			}
			if step.Trigger != "one_time" && step.Timing == nil {
				return fmt.Errorf("step %s needs a cited trigger or timing decision", step.ID)
			}
			if step.Timing != nil {
				if err := statement(*step.Timing); err != nil {
					return err
				}
			}
			for _, s := range step.CompletionCriteria {
				if err := statement(s); err != nil {
					return err
				}
			}
		}
		return nil
	}
	caps := map[string]Capability{}
	for _, c := range p.Capabilities {
		if _, ok := caps[c.ID]; ok || c.ID == "" || c.Name == "" || !slices.Contains([]string{"service", "account", "cluster", "namespace", "repository"}, c.ScopeKind) {
			return fmt.Errorf("invalid capability %s", c.ID)
		}
		caps[c.ID] = c
		if err := steps(c.Setup); err != nil {
			return fmt.Errorf("capability %s: %w", c.ID, err)
		}
		seen := map[string]bool{}
		for _, a := range c.Alternatives {
			if a.ID == "" || seen[a.ID] {
				return fmt.Errorf("invalid alternative %s", a.ID)
			}
			seen[a.ID] = true
			if err := statement(a.Explanation); err != nil {
				return err
			}
			if err := steps(a.Steps); err != nil {
				return err
			}
		}
	}
	dependencies := func(deps []Dependency) error {
		seen := map[string]bool{}
		for _, d := range deps {
			if _, ok := caps[d.CapabilityID]; !ok || seen[d.CapabilityID] {
				return fmt.Errorf("missing or duplicate dependency %s", d.CapabilityID)
			}
			seen[d.CapabilityID] = true
			if err := statement(d.Reason); err != nil {
				return err
			}
			if c := d.Condition; c != nil {
				parts := strings.SplitN(c.Fact, ".", 2)
				if len(parts) != 2 || parts[1] == "" || !slices.Contains([]string{"profile", "scope", "decision"}, parts[0]) || c.Equals == "" || c.Question == "" {
					return fmt.Errorf("invalid dependency condition %s", c.Fact)
				}
			}
		}
		return nil
	}
	for _, c := range p.Capabilities {
		if err := dependencies(c.Dependencies); err != nil {
			return err
		}
	}
	visited, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fmt.Errorf("capability dependency cycle at %s", id)
		}
		if visited[id] {
			return nil
		}
		active[id] = true
		for _, d := range caps[id].Dependencies {
			if err := visit(d.CapabilityID); err != nil {
				return err
			}
		}
		active[id] = false
		visited[id] = true
		return nil
	}
	for _, c := range p.Capabilities {
		if err := visit(c.ID); err != nil {
			return err
		}
	}
	books, variants := map[string]bool{}, map[string]bool{}
	for _, b := range p.Playbooks {
		if b.ID == "" || books[b.ID] || b.Revision < 1 || b.RequirementID == "" || len(b.VariantIDs) == 0 || !slices.Contains([]string{"actionable", "excluded", "needs_decision"}, b.Kind) {
			return fmt.Errorf("incomplete playbook %s", b.ID)
		}
		books[b.ID] = true
		for _, id := range b.VariantIDs {
			if id == "" || variants[id] {
				return fmt.Errorf("duplicate or empty variant %s", id)
			}
			variants[id] = true
		}
		for _, s := range []Statement{b.Purpose, b.Applicability} {
			if err := statement(s); err != nil {
				return err
			}
		}
		if b.Kind != "actionable" {
			if len(b.Assertions) > 0 || len(b.Prerequisites) > 0 || len(b.Setup.Items)+len(b.Operate.Items)+len(b.Collect.Items)+len(b.ReviewSteps.Items) > 0 {
				return fmt.Errorf("applicability-only playbook %s cannot introduce obligations", b.ID)
			}
			if b.Kind == "needs_decision" && (strings.TrimSpace(b.DecisionQuestion) == "" || strings.TrimSpace(b.DecisionOwnerRole) == "") {
				return fmt.Errorf("playbook %s needs an applicability question and owner", b.ID)
			}
			if b.Kind == "excluded" && (b.DecisionQuestion != "" || b.DecisionOwnerRole != "") {
				return fmt.Errorf("excluded playbook %s cannot request a decision", b.ID)
			}
			continue
		}
		if b.OfficialStatement == "" || len(b.Assertions) == 0 {
			return fmt.Errorf("actionable playbook %s needs statement and assertions", b.ID)
		}
		if (len(b.Prerequisites) == 0) == (strings.TrimSpace(b.NoPrerequisitesReason) == "") {
			return fmt.Errorf("playbook %s needs prerequisites or none rationale", b.ID)
		}
		if err := dependencies(b.Prerequisites); err != nil {
			return err
		}
		for _, section := range []Steps{b.Setup, b.Operate, b.Collect, b.ReviewSteps} {
			if err := steps(section); err != nil {
				return fmt.Errorf("playbook %s: %w", b.ID, err)
			}
		}
		ids := map[string]bool{}
		covered := make([]bool, len(b.OfficialStatement))
		for _, a := range b.Assertions {
			if a.ID == "" || ids[a.ID] || len(a.Clauses) == 0 {
				return fmt.Errorf("invalid assertion %s", a.ID)
			}
			ids[a.ID] = true
			if err := statement(a.Obligation); err != nil {
				return err
			}
			for _, clause := range a.Clauses {
				if strings.TrimSpace(clause) == "" || !strings.Contains(b.OfficialStatement, clause) {
					return fmt.Errorf("assertion %s refers to absent policy clause", a.ID)
				}
				for offset := 0; offset < len(b.OfficialStatement); {
					index := strings.Index(b.OfficialStatement[offset:], clause)
					if index < 0 {
						break
					}
					start := offset + index
					for i := start; i < start+len(clause); i++ {
						covered[i] = true
					}
					offset = start + len(clause)
				}
			}
			ac := a.Acceptance
			if ac.Fact == "" || len(ac.Alternatives) == 0 || len(ac.Insufficient) == 0 {
				return fmt.Errorf("assertion %s needs evidence acceptance criteria", a.ID)
			}
			for _, s := range []Statement{ac.Scope, ac.TimePeriod, ac.Provenance} {
				if err := statement(s); err != nil {
					return err
				}
			}
			routes := map[string]bool{}
			for _, route := range ac.Alternatives {
				if route.ID == "" || routes[route.ID] || len(route.Artifacts) == 0 || route.Proves == "" || route.Limitations == "" {
					return fmt.Errorf("invalid evidence route %s", route.ID)
				}
				routes[route.ID] = true
				for _, artifact := range route.Artifacts {
					if err := statement(artifact); err != nil {
						return err
					}
				}
			}
		}
		for index, char := range b.OfficialStatement {
			if (unicode.IsLetter(char) || unicode.IsNumber(char)) && !covered[index] {
				return fmt.Errorf("playbook %s leaves policy text uncovered by assertions", b.ID)
			}
		}
	}
	return nil
}

// This catches unsupported numeric timing, not semantic deadline equivalence.
// Written-number timing and interpretation still need source-comparison review.
var numericTiming = regexp.MustCompile(`\b[0-9]+(?:\.[0-9]+)?\s+(?:(?:business|calendar)\s+)?(?:seconds?|minutes?|hours?|days?|weeks?|months?|years?)\b`)
