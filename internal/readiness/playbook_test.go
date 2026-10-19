package readiness

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jaxxstorm/fed/internal/storage"
)

func draftFixture() Package {
	s := Statement{Text: "Document the monitored systems and review their alerts.", Basis: Official, Citations: []string{"policy"}}
	none := Steps{NoneReason: "This synthetic fixture has no separate step in this section."}
	return Package{Format: Format, CatalogID: "catalog", Revision: 1, Citations: []Citation{{ID: "policy"}}, Capabilities: []Capability{{ID: "monitoring", Name: "Monitoring", ScopeKind: "service", Setup: none}}, Playbooks: []Playbook{{Kind: "actionable", ID: "rule", Revision: 1, RequirementID: "RULE", VariantIDs: []string{"variant"}, OfficialStatement: s.Text, Purpose: s, Applicability: s, Prerequisites: []Dependency{{CapabilityID: "monitoring", Reason: s}}, Setup: none, Operate: none, Collect: none, ReviewSteps: none, Assertions: []Assertion{{ID: "monitored", Obligation: s, Clauses: []string{s.Text}, Acceptance: Acceptance{Fact: "Systems and alert review are recorded", Scope: s, TimePeriod: s, Provenance: s, Alternatives: []EvidenceRoute{{ID: "records", Artifacts: []Statement{s}, Proves: "Records cover systems and review", Limitations: "Not proof of every alert delivery"}}, Insufficient: []string{"An undated dashboard without scope"}}}}}}}
}

func TestDraftValidationRejectsIncompleteAndUnsafePlans(t *testing.T) {
	if err := ValidateDraft(draftFixture()); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		change  func(*Package)
		message string
	}{
		{"missing citation", func(p *Package) { p.Citations = nil }, "unknown citation"},
		{"generic", func(p *Package) {
			p.Playbooks[0].Purpose.Text = "Ask an agent to gather evidence from the configured sources."
		}, "generic instruction"},
		{"missing clause", func(p *Package) { p.Playbooks[0].Assertions[0].Clauses = []string{"Document the monitored systems"} }, "uncovered"},
		{"foreign clause", func(p *Package) { p.Playbooks[0].Assertions[0].Clauses = []string{"Invented policy"} }, "absent policy clause"},
		{"missing criteria", func(p *Package) { p.Playbooks[0].Assertions[0].Acceptance.Alternatives = nil }, "acceptance criteria"},
		{"dangling dependency", func(p *Package) { p.Playbooks[0].Prerequisites[0].CapabilityID = "missing" }, "missing or duplicate dependency"},
		{"cycle", func(p *Package) { p.Capabilities[0].Dependencies = p.Playbooks[0].Prerequisites }, "cycle"},
		{"condition", func(p *Package) {
			p.Playbooks[0].Prerequisites[0].Condition = &Condition{Fact: "execute.script", Equals: "yes", Question: "Run?"}
		}, "invalid dependency condition"},
		{"ambiguous section", func(p *Package) { p.Playbooks[0].Setup = Steps{} }, "section needs"},
		{"duplicate variant", func(p *Package) { p.Playbooks[0].VariantIDs = []string{"variant", "variant"} }, "duplicate or empty variant"},
		{"invented deadline", func(p *Package) {
			s := p.Playbooks[0].Purpose
			timing := s
			timing.Text = "Within 24 hours"
			p.Playbooks[0].Operate = Steps{Items: []Step{{ID: "review", OwnerRole: "Operations", Trigger: "recurring", Action: s, Timing: &timing, CompletionCriteria: []Statement{s}}}}
		}, "uncited numeric timing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := draftFixture()
			tt.change(&p)
			err := ValidateDraft(p)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("got %v, want %s", err, tt.message)
			}
		})
	}
}

func TestSourceVerificationIncludesNarrativeAndDefinitions(t *testing.T) {
	rulesCommit, siteCommit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	rules := fstest.MapFS{"rules.json": {Data: []byte(`{"rule":"Read the definition"}`)}}
	metadata, _ := json.Marshal(map[string]any{"rules": map[string]string{"repo": "FedRAMP/rules", "sha": rulesCommit}, "site": map[string]string{"repo": "FedRAMP/2026", "sha": siteCommit}})
	markdown := fstest.MapFS{"_sources.json": {Data: metadata}, "definitions.md": {Data: []byte("# Promptly\nPublished timing definition.\n")}}
	excerpt := "Published timing definition."
	p := Package{CatalogID: rulesCommit + ":" + storage.Hash(rules["rules.json"].Data), Sources: map[string]Source{"rules": {Repository: "fedramp/rules", Commit: rulesCommit}, "site": {Repository: "fedramp/2026", Commit: siteCommit}}, Citations: []Citation{{ID: "definition", Source: "site", Path: "definitions.md", Anchor: "promptly", Excerpt: excerpt, ContentHash: storage.Hash(markdown["definitions.md"].Data), ExcerptHash: storage.Hash([]byte(excerpt))}}}
	if err := VerifySources(p, rules, markdown); err != nil {
		t.Fatal(err)
	}
	markdown["definitions.md"] = &fstest.MapFile{Data: []byte("# Promptly\nChanged timing definition.\n")}
	if err := VerifySources(p, rules, markdown); err == nil || !strings.Contains(err.Error(), "needs revalidation") {
		t.Fatal("narrative-only change accepted", err)
	}
	p.Citations[0].Path = "../outside"
	if err := VerifySources(p, rules, markdown); err == nil {
		t.Fatal("path traversal accepted")
	}
	p.Citations = nil
	p.CatalogID = "wrong"
	if err := VerifySources(p, rules, markdown); err == nil {
		t.Fatal("mismatched rules accepted")
	}
}
