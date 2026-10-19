package guidancecontent

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/readiness"
)

// These are comparison inputs, not authenticated content reviews. Validate the
// original domain package closure: assembly uses different citation identifiers.
func TestCommunicationSourceComparisonBundles(t *testing.T) {
	total := 0
	kinds := map[string]int{}
	for _, tc := range []struct {
		domain string
		build  func(fs.FS, fs.FS) (readiness.Package, error)
		count  int
	}{
		{"CCM", CCM, 107}, {"CMU", CMU, 27}, {"CPO", CPO, 42}, {"SCN", SCN, 81},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			p, err := tc.build(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile("../../guidance/reviews/" + tc.domain + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var bundle ComparisonBundle
			if err = json.Unmarshal(raw, &bundle); err != nil {
				t.Fatal(err)
			}
			if bundle.RecordingState != "pending_authenticated_service_record" {
				t.Fatal("fabricated recording state")
			}
			if len(bundle.Comparisons) != tc.count || len(p.Playbooks) != tc.count {
				t.Fatal("exact comparison count changed")
			}
			for _, b := range p.Playbooks {
				total++
				kinds[b.Kind]++
				if b.Review != nil {
					t.Fatal("comparison attached review metadata", b.ID)
				}
				input, ok := bundle.Comparisons[b.ID]
				if !ok {
					t.Fatal("missing comparison", b.ID)
				}
				hash, err := readiness.ReviewHash(p, b.ID)
				if err != nil {
					t.Fatal(err)
				}
				if input.ContentHash != hash {
					t.Fatal("stale original-batch hash", b.ID)
				}
				if err = readiness.ValidateComparison(p, b.ID, input.Comparison); err != nil {
					t.Fatalf("%s: %v", b.ID, err)
				}
				if len(input.Comparison.Uncertainties) == 0 {
					t.Fatal("lost explicit source/service uncertainty", b.ID)
				}
				if !strings.Contains(input.Comparison.Applicability.Finding, b.RequirementID) {
					t.Fatal("lost requirement-specific comparison", b.ID)
				}
				if b.Kind != "actionable" {
					continue
				}
				assertions := map[string]readiness.Assertion{}
				for _, a := range b.Assertions {
					assertions[a.ID] = a
				}
				switch b.RequirementID {
				case "CCM-QTR-RTR", "SCN-CSO-MAR", "SCN-CSO-NOM", "CPO-CSX-CPM":
					first, one := assertions["focused-clause-1"]
					second, two := assertions["focused-clause-2"]
					if !one || !two || first.Acceptance.Fact == second.Acceptance.Fact {
						t.Fatal("collapsed independently assessable duties", b.ID)
					}
				case "SCN-ADP-NTF", "SCN-TRF-NIP", "SCN-TRF-NFP", "SCN-TRF-NAF", "SCN-TRF-NAV":
					a, ok := assertions["certification-data-notification"]
					if !ok || a.Obligation.Basis != readiness.Official || !strings.Contains(a.Acceptance.Fact, "Certification Data") {
						t.Fatal("lost structured notification target", b.ID)
					}
					if !strings.Contains(b.Operate.Items[0].Action.Text, "Update FedRAMP Certification Data") {
						t.Fatal("operation omitted notification target", b.ID)
					}
					if b.RequirementID == "SCN-TRF-NIP" || b.RequirementID == "SCN-TRF-NFP" || b.RequirementID == "SCN-TRF-NAF" {
						if _, ok := assertions["focused-clause-1"]; !ok {
							t.Fatal("notice content collapsed into timely delivery", b.ID)
						}
					}
				case "CCM-QTR-MTG":
					if !strings.Contains(b.Operate.Items[0].CompletionCriteria[0].Text, "OCR ordinal recurrence alongside the meeting recurrence") {
						t.Fatal("silently repaired mismatched source artifact", b.ID)
					}
				}
			}
		})
	}
	if total != 257 || kinds["actionable"] != 103 || kinds["excluded"] != 95 || kinds["needs_decision"] != 59 {
		t.Fatalf("unexpected variants: %d %v", total, kinds)
	}
}
