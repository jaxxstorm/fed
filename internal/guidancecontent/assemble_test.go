package guidancecontent

import (
	"encoding/json"
	"io/fs"
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestAssemblyPreservesIndependentCitationMeaning(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	batches := []readiness.Package{}
	for _, build := range []func(fs.FS, fs.FS) (readiness.Package, error){CED, INR, RPL} {
		p, err := build(rules, markdown)
		if err != nil {
			t.Fatal(err)
		}
		batches = append(batches, p)
	}
	before, _ := json.Marshal(batches)
	p, err := Assemble(batches, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = readiness.VerifySources(p, rules, markdown); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(batches)
	if string(before) != string(after) {
		t.Fatal("assembly mutated authored inputs")
	}
	// CED and INR use class-a for different exact excerpts. Both must survive.
	classACitations := 0
	for _, c := range p.Citations {
		if c.Path == "providers/20x/rules/fedramp-certification.md" {
			classACitations++
		}
	}
	if classACitations != 3 {
		t.Fatal("citation ID collision changed source meaning", classACitations)
	}
	variants := 0
	for _, b := range p.Playbooks {
		variants += len(b.VariantIDs)
		if b.Review != nil {
			t.Fatal("assembly copied review authority")
		}
	}
	if variants != 24 {
		t.Fatal("assembly lost variants", variants)
	}
	bundles := []ComparisonBundle{}
	for _, domain := range []string{"CED", "INR", "RPL"} {
		raw, e := os.ReadFile("../../guidance/reviews/" + domain + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var b ComparisonBundle
		if e = json.Unmarshal(raw, &b); e != nil {
			t.Fatal(e)
		}
		bundles = append(bundles, b)
	}
	rebound, err := RebindBundles(batches, p, bundles)
	if err != nil {
		t.Fatal(err)
	}
	if len(rebound.Comparisons) != len(p.Playbooks) || rebound.RecordingState != "pending_authenticated_service_record" {
		t.Fatal("rebind lost coverage or claimed authentication")
	}
	changed := p
	changed.Playbooks = append([]readiness.Playbook(nil), p.Playbooks...)
	changed.Playbooks[0].Purpose.Text += " A changed interpretation."
	if _, err = RebindBundles(batches, changed, bundles); err == nil {
		t.Fatal("source comparison reused after meaning changed")
	}
	reversed := []readiness.Package{batches[2], batches[1], batches[0]}
	other, err := Assemble(reversed, 1)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(p)
	y, _ := json.Marshal(other)
	if string(x) != string(y) {
		t.Fatal("assembly depends on batch iteration order")
	}
	batches[0].Playbooks[0].Review = &readiness.Review{ReviewerID: "untrusted-input", ReviewerType: "agent", ContentHash: "fabricated"}
	untrusted, err := Assemble(batches, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range untrusted.Playbooks {
		if book.Review != nil {
			t.Fatal("assembly carried caller-supplied review authority")
		}
	}
	batches[1].Capabilities = append(batches[1].Capabilities, batches[0].Capabilities[0])
	batches[1].Capabilities[len(batches[1].Capabilities)-1].Name = "Conflicting shared definition"
	// A self-contained valid conflicting capability must be rejected rather than
	// silently picking a batch. Reuse its own citation to keep it structurally valid.
	cap := &batches[1].Capabilities[len(batches[1].Capabilities)-1]
	cap.Setup = batches[1].Capabilities[0].Setup
	if _, err = Assemble(batches, 1); err == nil {
		t.Fatal("conflicting shared capability accepted")
	}
}
