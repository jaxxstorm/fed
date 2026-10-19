package guidancecontent

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
)

var authoredBuilders = map[string]func(fs.FS, fs.FS) (readiness.Package, error){
	"CED": CED, "INR": INR, "RPL": RPL,
	"CCM": CCM, "CMU": CMU, "CPO": CPO, "SCN": SCN,
	"AFC": AFC, "AGU": AGU, "MAS": MAS, "MKT": MKT, "REC": REC, "SCG": SCG, "SDR": SDR,
	"CNA": CNA, "IAM": IAM, "MLA": MLA, "SVC": SVC, "CMT": CMT, "SCR": SCR, "PIY": PIY,
	"FRC": FRC, "IEC": IEC, "IVV": IVV,
	"CDS": CDS, "VDR": VDR, "VER": VER,
}

func BuildBatch(domain string, rules, markdown fs.FS) (readiness.Package, error) {
	build, ok := authoredBuilders[domain]
	if !ok {
		return readiness.Package{}, fmt.Errorf("no authored draft exists for batch %s", domain)
	}
	return build(rules, markdown)
}

// DraftBatches loads explicitly authored batches without changing their review
// hashes or treating them as a complete release.
func DraftBatches(rules, markdown fs.FS) ([]readiness.Package, error) {
	out := []readiness.Package{}
	names := []string{}
	for name := range authoredBuilders {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p, err := BuildBatch(name, rules, markdown)
		if err != nil {
			return out, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// RebindBundles prepares comparison inputs for a release without authenticating
// or publishing them. Every assembled playbook must retain a matching comparison.
func RebindBundles(batches []readiness.Package, assembled readiness.Package, bundles []ComparisonBundle) (ComparisonBundle, error) {
	out := ComparisonBundle{RecordingState: "pending_authenticated_service_record", Comparisons: map[string]ComparisonInput{}}
	inputs := map[string]ComparisonInput{}
	for _, bundle := range bundles {
		for id, input := range bundle.Comparisons {
			if _, ok := inputs[id]; ok {
				return out, fmt.Errorf("duplicate comparison %s", id)
			}
			inputs[id] = input
		}
	}
	for _, batch := range batches {
		normalized, err := Assemble([]readiness.Package{batch}, assembled.Revision)
		if err != nil {
			return out, err
		}
		for _, book := range batch.Playbooks {
			input, ok := inputs[book.ID]
			if !ok {
				return out, fmt.Errorf("missing source comparison for %s", book.ID)
			}
			bound, err := rebindComparison(batch, normalized, assembled, book.ID, input)
			if err != nil {
				return out, err
			}
			out.Comparisons[book.ID] = bound
		}
	}
	if len(out.Comparisons) != len(inputs) || len(out.Comparisons) != len(assembled.Playbooks) {
		return out, fmt.Errorf("comparison/playbook coverage mismatch")
	}
	return out, nil
}

type DraftCoverage struct {
	CatalogID string       `json:"catalog_id"`
	CountUnit string       `json:"count_unit"`
	Total     int          `json:"total"`
	Encoded   int          `json:"encoded"`
	Remaining int          `json:"remaining"`
	State     string       `json:"state"`
	Batches   []DraftBatch `json:"batches"`
}

type DraftBatch struct {
	Domain    string   `json:"domain"`
	Encoded   []string `json:"encoded_variant_ids"`
	Remaining []string `json:"remaining_variant_ids"`
}

// Coverage is a local authoring report, deliberately distinct from service
// guidance-quality responses that verify authenticated review receipts.
func Coverage(packages []readiness.Package, inventory catalog.GuidanceInventory) (DraftCoverage, error) {
	out := DraftCoverage{CatalogID: inventory.CatalogID, CountUnit: "catalog variants", Total: inventory.VariantCount, State: "local_drafts_not_published", Batches: []DraftBatch{}}
	encoded := map[string]bool{}
	for _, p := range packages {
		if err := readiness.ValidateDraft(p); err != nil {
			return out, err
		}
		if err := readiness.ValidateInventory(p, inventory); err != nil {
			return out, err
		}
		for _, b := range p.Playbooks {
			for _, id := range b.VariantIDs {
				if encoded[id] {
					return out, fmt.Errorf("duplicate variant across authored batches: %s", id)
				}
				encoded[id] = true
			}
		}
	}
	batches := map[string]*DraftBatch{}
	for _, v := range inventory.Variants {
		b := batches[v.Document]
		if b == nil {
			b = &DraftBatch{Domain: v.Document, Encoded: []string{}, Remaining: []string{}}
			batches[v.Document] = b
		}
		if encoded[v.ID] {
			b.Encoded = append(b.Encoded, v.ID)
			out.Encoded++
		} else {
			b.Remaining = append(b.Remaining, v.ID)
			out.Remaining++
		}
	}
	for _, b := range batches {
		sort.Strings(b.Encoded)
		sort.Strings(b.Remaining)
		out.Batches = append(out.Batches, *b)
	}
	sort.Slice(out.Batches, func(i, j int) bool { return out.Batches[i].Domain < out.Batches[j].Domain })
	return out, nil
}
