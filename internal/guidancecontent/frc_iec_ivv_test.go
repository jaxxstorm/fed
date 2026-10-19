package guidancecontent

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestCertificationBatchesExactSourcesAndInventory(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
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
	inv, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		domain                                 string
		build                                  func(fs.FS, fs.FS) (readiness.Package, error)
		count, actionable, excluded, decisions int
		digest                                 string
	}{
		{"FRC", FRC, 150, 56, 55, 39, "24e830ab6da3657464136e21a9eae8deac6e98c6f026119e80a9b2668f61a430"},
		{"IEC", IEC, 76, 33, 22, 21, "1815a8475a54c3d055eb43d6a443b2e5981044b583ce1e53a7948fe507a90d5a"},
		{"IVV", IVV, 80, 29, 34, 17, "865779cb422abbe1ba57b7ab2d8b781a5c1132c80d87838cf67ca5b171c3884c"},
	} {
		t.Run(test.domain, func(t *testing.T) {
			p, err := test.build(rules, markdown)
			if err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateDraft(p); err != nil {
				t.Fatal(err)
			}
			if err = readiness.VerifySources(p, rules, markdown); err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateInventory(p, inv); err != nil {
				t.Fatal(err)
			}
			covered := map[string]bool{}
			kinds := map[string]int{}
			ids := []string{}
			variants := map[string]catalog.GuidanceVariant{}
			for _, v := range inv.Variants {
				if v.Document == test.domain {
					variants[v.ID] = v
				}
			}
			for _, b := range p.Playbooks {
				if b.Review != nil {
					t.Fatal("fabricated review", b.ID)
				}
				kinds[b.Kind]++
				for _, id := range b.VariantIDs {
					if covered[id] {
						t.Fatal("duplicate variant", id)
					}
					v, ok := variants[id]
					if !ok {
						t.Fatal("foreign variant", id)
					}
					if b.OfficialStatement != v.Statement {
						t.Fatal("changed statement", id)
					}
					covered[id] = true
					ids = append(ids, id)
					if b.Kind != "actionable" {
						continue
					}
					if b.Operate.Items[0].Action.Text != v.Statement {
						t.Fatal("actor/force/condition changed", id)
					}
					if _, ok := v.Details["pain_timeframes"]; ok {
						assertCertificationPAINTiming(t, b)
					}
					for _, key := range []string{"following_information", "following_information_bullets"} {
						if items, ok := v.Details[key].([]any); ok {
							for _, item := range items {
								found := false
								for _, a := range b.Assertions {
									found = found || strings.Contains(a.Obligation.Text, item.(string))
								}
								if !found {
									t.Fatal("enumerated obligation lost", b.RequirementID, item)
								}
							}
						}
					}
				}
			}
			for id := range variants {
				if !covered[id] {
					t.Fatal("missing exact variant", id)
				}
			}
			sort.Strings(ids)
			if len(ids) != test.count || storage.Hash([]byte(strings.Join(ids, "\n"))) != test.digest {
				t.Fatal("exact inventoried denominator changed")
			}
			if kinds["actionable"] != test.actionable || kinds["excluded"] != test.excluded || kinds["needs_decision"] != test.decisions {
				t.Fatal("outcome coverage changed", kinds)
			}
			// Exact source provenance must fail closed on changed policy or narrative.
			changedRules := fstest.MapFS{"rules.json": {Data: append(append([]byte{}, raw...), '\n')}, "schema.json": {Data: schema}}
			if _, err = test.build(changedRules, markdown); err == nil {
				t.Fatal("accepted changed rules")
			}
			changedSchema := fstest.MapFS{"rules.json": {Data: raw}, "schema.json": {Data: append(append([]byte{}, schema...), '\n')}}
			if _, err = test.build(changedSchema, markdown); err == nil {
				t.Fatal("accepted changed pinned schema")
			}
			changedMarkdown := certificationOverlay{FS: markdown, path: "definitions.md", data: []byte("changed definitions")}
			if _, err = test.build(rules, changedMarkdown); err == nil {
				t.Fatal("accepted changed definitions")
			}
			t.Logf("%s: %d exact variants; %v; %d shared capabilities", test.domain, len(ids), kinds, len(p.Capabilities))
		})
	}
}

func TestCertificationReviewCorrections(t *testing.T) {
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"FRC": FRC, "IEC": IEC, "IVV": IVV} {
		p, err := build(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range p.Playbooks {
			if b.Kind == "needs_decision" {
				if strings.Contains(b.Applicability.Text, "condition requiring") && !strings.Contains(b.DecisionQuestion, b.OfficialStatement) {
					t.Fatal("generic decision lost actual conditional boundary", b.ID)
				}
				if strings.Contains(b.Applicability.Text, "conflicts with subset") && !strings.Contains(b.DecisionQuestion, "authoritative resolution") {
					t.Fatal("source conflict treated as ordinary profile choice", b.ID)
				}
			}
			if b.Kind != "actionable" {
				continue
			}
			if b.Assertions[0].Acceptance.Fact == b.Purpose.Text {
				t.Fatal("assessment purpose substituted for observable fact", b.ID)
			}
			if b.RequirementID == "IEC-CSO-EFI" && !strings.Contains(b.ReviewSteps.Items[0].Action.Text, "Unknown adverse customer effect should be treated as debilitating") {
				t.Fatal("lost unknown-effect treatment")
			}
			if b.RequirementID == "FRC-CLA-EAM" || b.RequirementID == "FRC-CLA-ASF" {
				for _, a := range b.Assertions {
					if strings.HasPrefix(a.ID, "following_information-") && !strings.Contains(a.Acceptance.Fact, "Only if this framework is selected") {
						t.Fatal("framework alternatives flattened", b.ID)
					}
				}
			}
			if domain == "IEC" && (b.RequirementID == "IEC-CSO-IIR" || b.RequirementID == "IEC-CSO-OIR" || b.RequirementID == "IEC-CSO-FIR") {
				routes := 0
				for _, a := range b.Assertions {
					if strings.HasPrefix(a.ID, "notification-") {
						routes++
					}
				}
				if routes != 3 {
					t.Fatal("distinct notification routes lost", b.ID, routes)
				}
			}
		}
	}
}

func assertCertificationPAINTiming(t *testing.T, b readiness.Playbook) {
	t.Helper()
	// Independent expectations transcribed from the official per-class tables;
	// test every rating, including business-day versus elapsed-hour boundaries.
	class := ""
	for _, c := range []string{"A", "B", "C", "D"} {
		if strings.Contains(b.OfficialStatement, "Class "+c+" Certifications") {
			class = c
		}
	}
	tables := map[string]map[string][]string{
		"IEC-CSO-IIR": {"A": {"1 business day", "1 business day", "6 hours", "6 hours", "6 hours"}, "B": {"1 business day", "1 business day", "6 hours", "6 hours", "6 hours"}, "C": {"1 business day", "24 hours", "1 hour", "1 hour", "1 hour"}, "D": {"1 hour", "1 hour", "0.25 hours", "0.25 hours", "0.25 hours"}},
		"IEC-CSO-OIR": {"A": {"1 business day", "1 business day", "1 business day", "1 business day", "1 business day"}, "B": {"1 business day", "1 business day", "1 business day", "1 business day", "1 business day"}, "C": {"1 business day", "24 hours", "6 hours", "6 hours", "6 hours"}, "D": {"24 hours", "6 hours", "3 hours", "3 hours", "3 hours"}},
		"IEC-CSO-FIR": {"A": {"3 business days", "3 business days", "3 business days", "3 business days", "3 business days"}, "B": {"3 business days", "3 business days", "3 business days", "3 business days", "3 business days"}, "C": {"1 business day", "1 business day", "6 hours", "6 hours", "6 hours"}, "D": {"24 hours", "6 hours", "3 hours", "3 hours", "3 hours"}},
	}
	names := map[string]string{"IEC-CSO-IIR": "Initial Incident Report", "IEC-CSO-OIR": "Ongoing Incident Report", "IEC-CSO-FIR": "Final Incident Report"}
	values := tables[b.RequirementID][class]
	if len(values) != 5 {
		t.Fatal("missing independent class/table expectation", b.RequirementID, class)
	}
	timing := b.Operate.Items[0].Timing
	if timing == nil || timing.Basis != readiness.Official {
		t.Fatal("report deadline is not official", b.ID)
	}
	for i, want := range values {
		if !strings.Contains(timing.Text, fmt.Sprintf("PAIN-%d %s: %s.", i+1, names[b.RequirementID], want)) {
			t.Fatal("incorrect class/PAIN interval", b.ID, i+1, timing.Text)
		}
	}
}

// Overlay changes one source while preserving the rest of the pinned corpus.
type certificationOverlay struct {
	fs.FS
	path string
	data []byte
}

func (o certificationOverlay) Open(name string) (fs.File, error) {
	if name == o.path {
		return fstest.MapFS{name: {Data: o.data}}.Open(name)
	}
	return o.FS.Open(name)
}
