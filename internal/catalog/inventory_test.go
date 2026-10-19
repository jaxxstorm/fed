package catalog

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestInventoryPreservesProfileAndTransitionVariants(t *testing.T) {
	r := Requirement{ID: "KSI-TEST", Kind: "KSI", Track: "20x", Raw: Object{"statement": "Test obligation"}, Metadata: Object{"status": "stable", "effective": Object{"date": Object{"maintain": "2026-01-01", "grace": Object{"until_next_assessment": true}}}}}
	snapshot := Snapshot{ID: "catalog", Commit: PinnedCommit, Requirements: []Requirement{r}}
	inventory, err := Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(inventory.Dates, "2025-12-31") || !slices.Contains(inventory.Dates, "2026-01-01") {
		t.Fatal("missing date boundaries")
	}
	if inventory.ProfileCount != len(inventory.Dates)*48 {
		t.Fatal("missing supported profile combinations")
	}
	states := map[string]bool{}
	profiles := map[string]bool{}
	for _, p := range inventory.Profiles {
		profiles[p.ID] = true
		if p.Counts["required"]+p.Counts["unresolved"]+p.Counts["not_applicable"] != 1 {
			t.Fatal("incorrect denominator")
		}
	}
	for _, v := range inventory.Variants {
		states[v.Applicability] = true
		for _, id := range v.Profiles {
			if !profiles[id] {
				t.Fatal("dangling profile")
			}
		}
	}
	for _, state := range []string{"required", "unresolved", "not_applicable"} {
		if !states[state] {
			t.Fatalf("lost %s variants", state)
		}
	}
	again, err := Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(inventory)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("non-deterministic inventory")
	}
	manifest := Authoring(inventory)
	total := 0
	for _, domain := range manifest.Domains {
		total += len(domain.VariantIDs)
	}
	if total != inventory.VariantCount || manifest.CatalogID != inventory.CatalogID {
		t.Fatal("authoring manifest lost inventory variants")
	}
}
