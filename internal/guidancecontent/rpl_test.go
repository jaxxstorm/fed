package guidancecontent

import (
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestRPLPreservesObjectivesAlignmentAndTesting(t *testing.T) {
	p, err := RPL(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
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
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateInventory(p, inventory); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, book := range p.Playbooks {
		if book.Review != nil {
			t.Fatal("draft invented authenticated review")
		}
		for _, id := range book.VariantIDs {
			covered[id] = true
		}
		switch book.ID {
		case "rro-required":
			if len(book.Assertions) != 2 || len(book.Operate.Items) != 1 || book.Operate.Items[0].ID != "objective-alignment-review" {
				t.Fatal("objective definition and review collapsed into testing")
			}
		case "trc-required":
			if len(book.Assertions) != 1 || book.Assertions[0].ID != "tested-recovery-capability" {
				t.Fatal("recovery testing lost")
			}
		}
	}
	count := 0
	for _, v := range inventory.Variants {
		if v.Document == "RPL" {
			count++
			if !covered[v.ID] {
				t.Fatal("missing RPL variant", v.ID)
			}
		}
	}
	if count != 12 || len(covered) != 12 {
		t.Fatal("RPL denominator changed", count, len(covered))
	}
	assertComparisons(t, p, "RPL")
	ced, err := CED(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
	if err != nil {
		t.Fatal(err)
	}
	inr, err := INR(os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown"))
	if err != nil {
		t.Fatal(err)
	}
	packages := []readiness.Package{ced, inr, p}
	report, err := Coverage(packages, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 1340 || report.Encoded != 24 || report.Remaining != 1316 || report.State != "local_drafts_not_published" {
		t.Fatal("draft coverage misrepresented", report)
	}
	if _, err = Coverage(append(packages, p), inventory); err == nil {
		t.Fatal("duplicate batch inflated coverage")
	}
}
