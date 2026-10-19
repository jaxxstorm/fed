package guidancecontent

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestAdministrativeComparisonsCoverExactSourceClosure(t *testing.T) {
	rules, site := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	total := 0
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"AFC": AFC, "AGU": AGU, "MAS": MAS, "MKT": MKT, "REC": REC, "SCG": SCG, "SDR": SDR} {
		t.Run(domain, func(t *testing.T) {
			p, err := build(rules, site)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile("../../guidance/reviews/" + domain + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var bundle ComparisonBundle
			if err = json.Unmarshal(raw, &bundle); err != nil {
				t.Fatal(err)
			}
			if bundle.RecordingState != "pending_authenticated_service_record" || len(bundle.Comparisons) != len(p.Playbooks) {
				t.Fatal("review state or exact comparison count changed")
			}
			used := map[string]bool{}
			for _, b := range p.Playbooks {
				input, ok := bundle.Comparisons[b.ID]
				if !ok {
					t.Fatal("missing exact playbook", b.ID)
				}
				hash, err := readiness.ReviewHash(p, b.ID)
				if err != nil || input.ContentHash != hash {
					t.Fatal("stale original-batch review hash", b.ID, err)
				}
				if err = readiness.ValidateComparison(p, b.ID, input.Comparison); err != nil {
					t.Fatal(b.ID, err)
				}
				if len(input.Comparison.Uncertainties) == 0 {
					t.Fatal("missing explicit source/factual uncertainty", b.ID)
				}
				if b.Review != nil {
					t.Fatal("fabricated authenticated review", b.ID)
				}
				q, err := readiness.RelevantPackage(p, b.ID)
				if err != nil {
					t.Fatal(err)
				}
				for _, c := range q.Citations {
					used[c.ID] = true
				}
				total++
			}
			for _, c := range p.Citations {
				if !used[c.ID] {
					t.Fatal("unreviewed source citation", c.ID)
				}
			}
			t.Logf("%s: %d exact hashes/comparisons and %d bound source citations verified", domain, len(p.Playbooks), len(used))
		})
	}
	if total != 198 {
		t.Fatal("comparison denominator changed", total)
	}
}

func TestAdministrativeReviewCorrections(t *testing.T) {
	rules, site := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"AFC": AFC, "AGU": AGU, "MAS": MAS, "MKT": MKT, "REC": REC, "SCG": SCG, "SDR": SDR} {
		p, err := build(rules, site)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range p.Playbooks {
			if strings.Contains(b.Applicability.Text, "Source force: .") || strings.Contains(b.Applicability.Text, "before this rule becomes mandatory") {
				t.Fatal("lost force distinction", domain, b.ID)
			}
			if b.RequirementID == "SDR-CSX-KMT" && b.OfficialStatement == "" {
				for _, phrase := range []string{"Class A", "Class B", "Class C", "Class D", "past 30 days", "persistent validation", "Phase 4 Pilot", "not a selected obligation"} {
					if !strings.Contains(b.Purpose.Text, phrase) {
						t.Fatal("class context lost on path exclusion", phrase)
					}
				}
			}
			if b.RequirementID == "SDR-CSX-KMT" && b.Kind != "actionable" && (strings.Contains(b.OfficialStatement, "Class B") || strings.Contains(b.OfficialStatement, "Class C")) {
				if !strings.Contains(b.Purpose.Text, "past 30 days") || !strings.Contains(b.Purpose.Text, "past year (where available)") {
					t.Fatal("selected history qualifiers lost", b.ID)
				}
				if strings.Contains(b.OfficialStatement, "Class C") && !strings.Contains(b.Purpose.Text, "persistent validation") {
					t.Fatal("Class C daily validation context lost", b.ID)
				}
			}
			if b.RequirementID != "SCG-CSO-RSC" || b.Kind != "actionable" {
				continue
			}
			for _, a := range b.Assertions {
				decision := false
				for _, route := range a.Acceptance.Alternatives {
					if route.ID == "documented-decision" {
						decision = true
						if !strings.Contains(route.Limitations, "Does not waive a MUST") {
							t.Fatal("departure waives required topics")
						}
					}
				}
				if decision != strings.HasPrefix(a.Obligation.Text, "Recommended:") {
					t.Fatal("recommended/required topic disposition conflated", a.ID)
				}
			}
		}
	}
}

func TestAdministrativeDraftsExactInventoryAndSources(t *testing.T) {
	rules, site := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := fs.ReadFile(rules, "schema.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                                    string
		build                                   func(fs.FS, fs.FS) (readiness.Package, error)
		total, actionable, excluded, unresolved int
	}{
		{"AFC", AFC, 49, 22, 27, 0}, {"AGU", AGU, 20, 0, 0, 20},
		{"MAS", MAS, 29, 12, 11, 6}, {"MKT", MKT, 14, 2, 12, 0},
		{"REC", REC, 16, 0, 16, 0}, {"SCG", SCG, 36, 18, 18, 0},
		{"SDR", SDR, 34, 13, 13, 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := test.build(rules, site)
			if err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateDraft(p); err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateInventory(p, inventory); err != nil {
				t.Fatal(err)
			}
			if err = readiness.VerifySources(p, rules, site); err != nil {
				t.Fatal(err)
			}
			covered, counts := map[string]bool{}, map[string]int{}
			books := map[string]readiness.Playbook{}
			for _, b := range p.Playbooks {
				if b.Review != nil {
					t.Fatal("draft fabricated authenticated review")
				}
				counts[b.Kind]++
				for _, id := range b.VariantIDs {
					if covered[id] {
						t.Fatal("duplicate variant", id)
					}
					covered[id] = true
					books[id] = b
				}
				if b.Kind == "actionable" {
					if len(b.Prerequisites) == 0 || len(b.Setup.Items) == 0 || len(b.Operate.Items) == 0 || len(b.Collect.Items) == 0 || len(b.ReviewSteps.Items) == 0 {
						t.Fatal("incomplete substantive sections", b.ID)
					}
					if !strings.Contains(b.ReviewSteps.Items[0].CompletionCriteria[0].Text, "unknown") {
						t.Fatal("missing evidence must remain unknown")
					}
				}
			}
			want := 0
			for _, v := range inventory.Variants {
				if v.Document != test.name {
					continue
				}
				want++
				b, ok := books[v.ID]
				if !ok {
					t.Fatal("missing exact variant", v.ID)
				}
				if b.OfficialStatement != v.Statement || !strings.Contains(b.Applicability.Text, v.Reason) || !strings.Contains(b.Applicability.Text, "Source force: "+v.Force) {
					t.Fatal("source force/actor/applicability lost", v.ID)
				}
				if b.Kind == "actionable" {
					following, _ := v.Details["following_information"].([]any)
					if len(b.Assertions) != 1+len(following) {
						t.Fatal("following-information clauses lost", v.RequirementID)
					}
					for i, item := range following {
						if b.Assertions[i+1].Obligation.Text != item.(string) {
							t.Fatal("clause wording lost", v.RequirementID)
						}
					}
				}
			}
			if want != test.total || len(covered) != test.total || counts["actionable"] != test.actionable || counts["excluded"] != test.excluded || counts["needs_decision"] != test.unresolved {
				t.Fatal("inventory denominator changed", want, len(covered), counts)
			}
			t.Logf("%s: %d exact variants; %d actionable, %d excluded, %d unresolved; draft/inventory/source validation passed", test.name, want, test.actionable, test.excluded, test.unresolved)
		})
	}
}

type administrativeOverlay struct {
	fs.FS
	path    string
	content []byte
}

func (o administrativeOverlay) Open(name string) (fs.File, error) {
	if name == o.path {
		return fstest.MapFS{name: &fstest.MapFile{Data: o.content}}.Open(name)
	}
	return o.FS.Open(name)
}

func TestAdministrativeDraftsRejectChangedSources(t *testing.T) {
	rules, site := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte(strings.Replace(string(raw), "Providers MUST immediately notify FedRAMP", "Providers MAY immediately notify FedRAMP", 1))
	if _, err = AFC(administrativeOverlay{rules, "rules.json", changed}, site); err == nil {
		t.Fatal("changed rule accepted")
	}
	page := "reference/addressing-fedramp-communication.md"
	raw, err = fs.ReadFile(site, page)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = AFC(rules, administrativeOverlay{site, page, append(raw, []byte("\nchanged narrative")...)}); err == nil {
		t.Fatal("changed narrative accepted")
	}
	raw, err = fs.ReadFile(site, "definitions.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SCG(rules, administrativeOverlay{site, "definitions.md", append(raw, []byte("\nchanged definition")...)}); err == nil {
		t.Fatal("changed definition accepted")
	}
}

func TestAdministrativeDraftsSemanticBoundaries(t *testing.T) {
	rules, site := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	for _, test := range []struct {
		build                        func(fs.FS, fs.FS) (readiness.Package, error)
		requirement, section, phrase string
	}{
		{AFC, "AFC-CSO-CRA", "review", "estimated resolution time, not universal remediation deadlines"},
		{AFC, "AFC-CSO-NOC", "timing", "Immediately"},
		{AFC, "AFC-CSO-ACK", "timing", "without unnecessary delay"},
		{MAS, "MAS-CSO-TPR", "scope", "ONLY IF"},
		{MAS, "MAS-CSO-SUP", "review", "MUST separation"},
		{SCG, "SCG-CSO-RSC", "review", "recommended rather than promoting"},
		{SCG, "SCG-CSO-SDF", "timing", "initially provisioned"},
		{SDR, "SDR-CSO-FRR", "review", "separately from independent"},
		{SDR, "SDR-CSX-KMT", "timing", "past 30 days"},
		{MKT, "MKT-CSO-PML", "operate", "does not accept listing applications via email"},
	} {
		p, err := test.build(rules, site)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, b := range p.Playbooks {
			if b.RequirementID != test.requirement || b.Kind != "actionable" {
				continue
			}
			found = true
			var text string
			switch test.section {
			case "review":
				text = b.ReviewSteps.Items[0].Action.Text
			case "timing":
				text = b.Operate.Items[0].Timing.Text
			case "scope":
				text = b.Assertions[0].Acceptance.Scope.Text
			case "operate":
				text = b.Operate.Items[0].Action.Text
			}
			if !strings.Contains(text, test.phrase) {
				t.Fatal(test.requirement, "lost semantic distinction", test.phrase)
			}
		}
		if !found {
			t.Fatal("missing actionable recipe", test.requirement)
		}
	}
}
