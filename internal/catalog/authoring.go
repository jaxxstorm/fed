package catalog

import "sort"

type AuthoringDomain struct {
	ID                  string         `json:"id"`
	RequirementIDs      []string       `json:"requirement_ids"`
	VariantIDs          []string       `json:"variant_ids"`
	ApplicabilityCounts map[string]int `json:"applicability_counts"`
}

type AuthoringManifest struct {
	CatalogID        string            `json:"catalog_id"`
	RequirementCount int               `json:"requirement_count"`
	VariantCount     int               `json:"variant_count"`
	Domains          []AuthoringDomain `json:"domains"`
}

// Authoring groups every inventory entry, including applicability explanations,
// into bounded domains. Review assignments live outside generated manifests.
func Authoring(inventory GuidanceInventory) AuthoringManifest {
	out := AuthoringManifest{CatalogID: inventory.CatalogID, RequirementCount: inventory.RequirementCount, VariantCount: inventory.VariantCount, Domains: []AuthoringDomain{}}
	domains := map[string]*AuthoringDomain{}
	seen := map[string]map[string]bool{}
	for _, v := range inventory.Variants {
		d, ok := domains[v.Document]
		if !ok {
			d = &AuthoringDomain{ID: v.Document, RequirementIDs: []string{}, VariantIDs: []string{}, ApplicabilityCounts: map[string]int{}}
			domains[v.Document] = d
			seen[v.Document] = map[string]bool{}
		}
		if !seen[v.Document][v.RequirementID] {
			d.RequirementIDs = append(d.RequirementIDs, v.RequirementID)
			seen[v.Document][v.RequirementID] = true
		}
		d.VariantIDs = append(d.VariantIDs, v.ID)
		d.ApplicabilityCounts[v.Applicability]++
	}
	for _, d := range domains {
		sort.Strings(d.RequirementIDs)
		sort.Strings(d.VariantIDs)
		out.Domains = append(out.Domains, *d)
	}
	sort.Slice(out.Domains, func(i, j int) bool { return out.Domains[i].ID < out.Domains[j].ID })
	return out
}
