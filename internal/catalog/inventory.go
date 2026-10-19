package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/jaxxstorm/fed/internal/storage"
)

// GuidanceInventory is a reproducible authoring denominator, not a service
// compliance report. It retains unresolved and not-applicable variants too.
type GuidanceInventory struct {
	CatalogID        string             `json:"catalog_id"`
	Commit           string             `json:"commit"`
	RequirementCount int                `json:"requirement_count"`
	ProfileCount     int                `json:"profile_count"`
	VariantCount     int                `json:"variant_count"`
	Dates            []string           `json:"dates"`
	Profiles         []InventoryProfile `json:"profiles"`
	Variants         []GuidanceVariant  `json:"variants"`
}

type InventoryProfile struct {
	ID      string         `json:"id"`
	Profile Profile        `json:"profile"`
	Counts  map[string]int `json:"counts"`
}

type GuidanceVariant struct {
	ID            string   `json:"id"`
	RequirementID string   `json:"requirement_id"`
	Kind          string   `json:"kind"`
	Document      string   `json:"document"`
	Name          string   `json:"name"`
	Statement     string   `json:"statement"`
	Applicability string   `json:"applicability"`
	Force         string   `json:"force"`
	Reason        string   `json:"reason"`
	IncludedBy    string   `json:"included_by,omitempty"`
	Details       Object   `json:"details"`
	Profiles      []string `json:"profiles"`
}

// Inventory enumerates all currently supported categorical profile fields and
// every source date boundary (plus its predecessor). Resolve's date comparisons
// are piecewise constant between those boundaries. nil/false/true assessment
// flags retain unknown transition decisions instead of assuming them away.
func Inventory(s Snapshot) (GuidanceInventory, error) {
	dates := map[string]bool{"0001-01-01": true}
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case string:
			if t, err := time.Parse(time.DateOnly, x); err == nil {
				dates[x] = true
				if before := t.AddDate(0, 0, -1); before.Year() > 0 {
					dates[before.Format(time.DateOnly)] = true
				}
			}
		case Object:
			for _, child := range x {
				visit(child)
			}
		case map[string]any:
			for _, child := range x {
				visit(child)
			}
		case []any:
			for _, child := range x {
				visit(child)
			}
		}
	}
	for _, r := range s.Requirements {
		visit(r.Raw)
		visit(r.Metadata)
	}
	out := GuidanceInventory{CatalogID: s.ID, Commit: s.Commit, RequirementCount: len(s.Requirements), Profiles: []InventoryProfile{}, Variants: []GuidanceVariant{}}
	for date := range dates {
		out.Dates = append(out.Dates, date)
	}
	sort.Strings(out.Dates)
	indexes := map[string]int{}
	no, yes := false, true
	for _, date := range out.Dates {
		for _, path := range []string{"Program", "Agency"} {
			for _, class := range []string{"A", "B", "C", "D"} {
				for _, context := range []string{"initial", "ongoing"} {
					for _, started := range []*bool{nil, &no, &yes} {
						p := Profile{CatalogID: s.ID, Type: "20x", Path: path, Class: class, Date: date, Context: context, NextAssessmentStarted: started}
						raw, err := json.Marshal(p)
						if err != nil {
							return out, err
						}
						profile := InventoryProfile{ID: storage.Hash(raw), Profile: p, Counts: map[string]int{}}
						resolved, err := Resolve(s.Requirements, p)
						if err != nil {
							return out, err
						}
						for _, r := range resolved {
							profile.Counts[r.Applicability]++
							v, err := Variant(r)
							if err != nil {
								return out, err
							}
							index, exists := indexes[v.ID]
							if !exists {
								index = len(out.Variants)
								indexes[v.ID] = index
								out.Variants = append(out.Variants, v)
							}
							out.Variants[index].Profiles = append(out.Variants[index].Profiles, profile.ID)
						}
						out.Profiles = append(out.Profiles, profile)
					}
				}
			}
		}
	}
	sort.Slice(out.Variants, func(i, j int) bool {
		a, b := out.Variants[i], out.Variants[j]
		if a.RequirementID == b.RequirementID {
			return a.ID < b.ID
		}
		return a.RequirementID < b.RequirementID
	})
	out.ProfileCount = len(out.Profiles)
	out.VariantCount = len(out.Variants)
	return out, nil
}

// Variant uses the same semantic identity at authoring and request time.
func Variant(r Resolved) (GuidanceVariant, error) {
	v := GuidanceVariant{RequirementID: r.Requirement.ID, Kind: r.Requirement.Kind, Document: r.Requirement.Document, Name: r.Requirement.Name, Statement: r.Statement, Applicability: r.Applicability, Force: r.Force, Reason: r.Reason, IncludedBy: r.IncludedBy, Details: r.Details}
	encoded, err := json.Marshal(v)
	if err != nil {
		return v, err
	}
	v.ID = storage.Hash(encoded)
	return v, nil
}
