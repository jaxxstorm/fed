package guidancecontent

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestCEDSourceAndExactVariantCoverage(t *testing.T) {
	rules := os.DirFS("../../testdata/fedramp")
	markdown := os.DirFS("../../2026-markdown")
	p, err := CED(rules, markdown)
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
	for _, book := range p.Playbooks {
		if book.Review != nil {
			t.Fatal("draft builder invented a review")
		}
		for _, id := range book.VariantIDs {
			covered[id] = true
		}
	}
	count := 0
	for _, v := range i.Variants {
		if v.RequirementID == "KSI-CED-RAT" {
			count++
			if !covered[v.ID] {
				t.Fatal("CED variant missing", v.ID)
			}
		}
	}
	if count != 3 || len(covered) != 3 || len(p.Playbooks[0].Assertions) != 5 {
		t.Fatal("CED denominator or assertion population changed")
	}
	assertComparisons(t, p, "CED")
}

func assertComparisons(t *testing.T, p readiness.Package, batch string) {
	t.Helper()
	reviewRaw, err := os.ReadFile("../../guidance/reviews/" + batch + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var reviews struct {
		RecordingState string `json:"recording_state"`
		Comparisons    map[string]struct {
			ContentHash string                     `json:"content_hash"`
			Comparison  readiness.SourceComparison `json:"comparison"`
		} `json:"comparisons"`
	}
	if err = json.Unmarshal(reviewRaw, &reviews); err != nil {
		t.Fatal(err)
	}
	if reviews.RecordingState != "pending_authenticated_service_record" {
		t.Fatal("file-based comparison must not impersonate an authenticated review")
	}
	for _, book := range p.Playbooks {
		review, ok := reviews.Comparisons[book.ID]
		if !ok {
			t.Fatal("source comparison missing", book.ID)
		}
		hash, err := readiness.ReviewHash(p, book.ID)
		if err != nil || hash != review.ContentHash {
			t.Fatal("source comparison needs revalidation", book.ID, err)
		}
		if err = readiness.ValidateComparison(p, book.ID, review.Comparison); err != nil {
			t.Fatal(err)
		}
	}
}
