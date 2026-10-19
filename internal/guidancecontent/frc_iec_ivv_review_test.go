package guidancecontent

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestCertificationComparisonBundles(t *testing.T) {
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"FRC": FRC, "IEC": IEC, "IVV": IVV} {
		t.Run(domain, func(t *testing.T) {
			p, err := build(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
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
				t.Fatal("recording state or exact denominator mismatch")
			}
			for _, b := range p.Playbooks {
				input, ok := bundle.Comparisons[b.ID]
				if !ok {
					t.Fatal("missing comparison", b.ID)
				}
				hash, err := readiness.ReviewHash(p, b.ID)
				if err != nil || hash != input.ContentHash {
					t.Fatal("stale comparison", b.ID, err)
				}
				if err = readiness.ValidateComparison(p, b.ID, input.Comparison); err != nil {
					t.Fatal(b.ID, err)
				}
				omitted := input.Comparison
				omitted.Uncertainties = nil
				if readiness.ValidateComparison(p, b.ID, omitted) == nil {
					t.Fatal("accepted omitted bound-definition comparisons", b.ID)
				}
				if !strings.Contains(input.Comparison.Applicability.Finding, b.VariantIDs[0]) {
					t.Fatal("comparison lost exact variant", b.ID)
				}
				if b.Review != nil {
					t.Fatal("fabricated authenticated review", b.ID)
				}
				// Negative checks make citation closure and content binding meaningful.
				changed := p
				changed.Playbooks = append([]readiness.Playbook(nil), p.Playbooks...)
				for i := range changed.Playbooks {
					if changed.Playbooks[i].ID == b.ID {
						changed.Playbooks[i].Purpose.Text += " changed"
					}
				}
				stale, err := readiness.ReviewHash(changed, b.ID)
				if err != nil || stale == input.ContentHash {
					t.Fatal("review hash did not bind content", b.ID)
				}
			}
			t.Logf("%s: all %d hashes, exact IDs and full RelevantPackage citation comparisons validated", domain, len(p.Playbooks))
		})
	}
}
