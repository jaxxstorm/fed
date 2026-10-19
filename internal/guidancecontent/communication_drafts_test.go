package guidancecontent

import (
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

func TestCommunicationDraftsExactInventoryAndSources(t *testing.T) {
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
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		domain                                string
		build                                 func(fs.FS, fs.FS) (readiness.Package, error)
		count, actionable, excluded, decision int
		digest                                string
	}{
		{"CCM", CCM, 107, 41, 40, 26, "7c30036dfc64023d7216855f38f17d3deff65ff7d1e8a23b4f11d6d4fc5abece"},
		{"CMU", CMU, 27, 12, 8, 7, "bb122271f1365b6b292ec2ca4a64e73611a340456574567556fbb40458700f0f"},
		{"CPO", CPO, 42, 18, 14, 10, "7ab899ba6f9c8fce556f9d278274794270d320679b22b95ace5ee61e3216af1e"},
		{"SCN", SCN, 81, 32, 33, 16, "bbe8d6d2c7d9674663d192595645b1638aade4ecb04ebbe6e515038a4b103935"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			p, err := tc.build(rules, markdown)
			if err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateDraft(p); err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateInventory(p, inventory); err != nil {
				t.Fatal(err)
			}
			if err = readiness.VerifySources(p, rules, markdown); err != nil {
				t.Fatal(err)
			}
			covered, kinds := map[string]bool{}, map[string]int{}
			ids := []string{}
			for _, b := range p.Playbooks {
				if b.Review != nil {
					t.Fatal("draft invented authenticated review", b.ID)
				}
				kinds[b.Kind]++
				for _, id := range b.VariantIDs {
					if covered[id] {
						t.Fatal("duplicate variant", id)
					}
					covered[id] = true
					ids = append(ids, id)
				}
				if b.Kind != "actionable" {
					if len(b.Assertions)+len(b.Prerequisites)+len(b.Operate.Items) != 0 {
						t.Fatal("explanation fabricated duties", b.ID)
					}
					continue
				}
				for _, a := range b.Assertions {
					if len(a.Acceptance.Alternatives) < 2 {
						t.Fatal("missing alternative evidence route", b.ID)
					}
				}
				// These list-bearing rules would superficially validate with just the
				// introductory sentence; protect their actual substantive denominator.
				min := map[string]int{"CCM-OCR-AVL": 9, "CPO-CSO-MTD": 5, "CPO-CSO-OVR": 10, "SCN-CSO-EVA": 6, "SCN-CSO-INF": 11, "SCN-ADP-NTF": 2, "SCN-TRF-NAV": 4, "SCN-CSO-EMG": 2}[b.RequirementID]
				if len(b.Assertions) < min {
					t.Fatalf("%s lost listed requirements: %d < %d", b.RequirementID, len(b.Assertions), min)
				}
				if b.RequirementID == "SCN-CSO-EMG" && !strings.Contains(b.Assertions[1].Obligation.Text, "MUST") {
					t.Fatal("emergency permission lost mandatory follow-through")
				}
				if b.RequirementID == "CMU-CSO-UVM" && !strings.Contains(b.Operate.Items[0].Action.Text, "previously unvalidated algorithms") {
					t.Fatal("update-stream limitation missing")
				}
			}
			for _, v := range inventory.Variants {
				if v.Document == tc.domain && !covered[v.ID] {
					t.Fatal("missing variant", v.ID)
				}
				if v.Document != tc.domain && covered[v.ID] {
					t.Fatal("foreign variant", v.ID)
				}
			}
			sort.Strings(ids)
			if len(ids) != tc.count || storage.Hash([]byte(strings.Join(ids, "\n"))) != tc.digest {
				t.Fatal("exact authored inventory changed")
			}
			if kinds["actionable"] != tc.actionable || kinds["excluded"] != tc.excluded || kinds["needs_decision"] != tc.decision {
				t.Fatalf("unexpected disposition counts: %v", kinds)
			}
			for _, c := range p.Capabilities {
				if c.ScopeKind != "service" {
					t.Fatal("non-service capability", c.ID)
				}
			}
			// A changed excerpt must fail verification even if its hash is recomputed.
			altered := p
			altered.Citations = append([]readiness.Citation(nil), p.Citations...)
			altered.Citations[0].Excerpt += " invented source text"
			altered.Citations[0].ExcerptHash = storage.Hash([]byte(altered.Citations[0].Excerpt))
			if readiness.VerifySources(altered, rules, markdown) == nil {
				t.Fatal("accepted invented citation")
			}
		})
	}
}

func TestCommunicationDraftsRejectSourceDrift(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	changed := fstest.MapFS{"rules.json": &fstest.MapFile{Data: append(raw, '\n')}}
	for _, build := range []func(fs.FS, fs.FS) (readiness.Package, error){CCM, CMU, CPO, SCN} {
		if _, err := build(changed, markdown); err == nil {
			t.Fatal("changed rules silently reused authored guidance")
		}
	}
	page := "reference/cryptographic-module-use.md"
	pageRaw, err := fs.ReadFile(markdown, page)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CMU(rules, fstest.MapFS{page: &fstest.MapFile{Data: append(pageRaw, '\n')}}); err == nil {
		t.Fatal("changed Markdown silently rebound citations")
	}
}
