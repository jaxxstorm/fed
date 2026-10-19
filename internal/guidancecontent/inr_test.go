package guidancecontent

import (
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestINRPreservesDistinctDutiesAndApplicability(t *testing.T) {
	p, err := INR(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	i, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateInventory(p, i); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	excluded := 0
	unresolved := 0
	for _, book := range p.Playbooks {
		if book.Review != nil {
			t.Fatal("draft builder invented a review")
		}
		for _, id := range book.VariantIDs {
			covered[id] = true
		}
		if book.Kind == "excluded" {
			excluded++
		}
		if book.Kind == "needs_decision" {
			unresolved++
		}
		if book.ID == "aar-required" && (len(book.Operate.Items) != 2 || book.Operate.Items[0].Trigger != "event" || book.Operate.Items[1].Trigger != "recurring") {
			t.Fatal("after-action reporting collapsed into persistent lesson incorporation")
		}
		if book.ID == "rir-required" && book.Prerequisites[0].CapabilityID != "incident-procedures" {
			t.Fatal("procedure review incorrectly requires real incident history")
		}
	}
	count := 0
	for _, v := range i.Variants {
		if v.Document == "INR" {
			count++
			if !covered[v.ID] {
				t.Fatal("INR variant missing", v.ID)
			}
		}
	}
	if count != 9 || len(covered) != 9 || excluded != 2 || unresolved != 3 {
		t.Fatal("INR denominator or applicability lost", count, len(covered), excluded, unresolved)
	}
	assertComparisons(t, p, "INR")
}
