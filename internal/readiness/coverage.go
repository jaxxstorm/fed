package readiness

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

// ValidateInventory prevents an explanation-only plan from covering an active
// obligation, and prevents a package from silently inventing its denominator.
func ValidateInventory(p Package, inventory catalog.GuidanceInventory) error {
	if p.CatalogID != inventory.CatalogID {
		return fmt.Errorf("guidance/catalog identity mismatch")
	}
	variants := map[string]catalog.GuidanceVariant{}
	for _, v := range inventory.Variants {
		variants[v.ID] = v
	}
	for _, b := range p.Playbooks {
		for _, id := range b.VariantIDs {
			v, ok := variants[id]
			if !ok || v.RequirementID != b.RequirementID {
				return fmt.Errorf("playbook %s references unknown or foreign variant %s", b.ID, id)
			}
			expected := "actionable"
			if v.Applicability == "not_applicable" {
				expected = "excluded"
			} else if v.Applicability == "unresolved" {
				expected = "needs_decision"
			}
			if b.Kind != expected {
				return fmt.Errorf("playbook %s kind %s cannot cover %s variant", b.ID, b.Kind, v.Applicability)
			}
			if b.OfficialStatement != v.Statement {
				return fmt.Errorf("playbook %s selected statement differs from catalog", b.ID)
			}
		}
	}
	return nil
}

type CoverageEntry struct {
	VariantID     string `json:"variant_id"`
	RequirementID string `json:"requirement_id"`
	Document      string `json:"document"`
	Applicability string `json:"applicability"`
	PlaybookID    string `json:"playbook_id,omitempty"`
	State         string `json:"state"`
	Reason        string `json:"reason,omitempty"`
}

type CoverageReport struct {
	CatalogID    string                    `json:"catalog_id"`
	VariantCount int                       `json:"variant_count"`
	Counts       map[string]int            `json:"counts"`
	Profiles     map[string]map[string]int `json:"profiles"`
	Entries      []CoverageEntry           `json:"entries"`
	Complete     bool                      `json:"complete"`
}

// RelevantPackage closes over every prerequisite and its citations, so changing
// shared capability guidance invalidates exactly the dependent book reviews.
func RelevantPackage(p Package, id string) (Package, error) {
	out := Package{Format: p.Format, CatalogID: p.CatalogID, Revision: p.Revision, Sources: p.Sources}
	var book Playbook
	found := false
	for _, b := range p.Playbooks {
		if b.ID == id {
			book = b
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("unknown playbook %s", id)
	}
	book.Review = nil
	out.Playbooks = []Playbook{book}
	caps := map[string]Capability{}
	for _, c := range p.Capabilities {
		caps[c.ID] = c
	}
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if seen[id] {
			return nil
		}
		seen[id] = true
		c, ok := caps[id]
		if !ok {
			return fmt.Errorf("missing capability %s", id)
		}
		out.Capabilities = append(out.Capabilities, c)
		for _, d := range c.Dependencies {
			if err := visit(d.CapabilityID); err != nil {
				return err
			}
		}
		return nil
	}
	for _, d := range book.Prerequisites {
		if err := visit(d.CapabilityID); err != nil {
			return out, err
		}
	}
	sort.Slice(out.Capabilities, func(i, j int) bool { return out.Capabilities[i].ID < out.Capabilities[j].ID })
	// Walk only typed book/capability payloads to collect Statement.Citations.
	raw, err := json.Marshal(struct {
		Book         Playbook
		Capabilities []Capability
	}{book, out.Capabilities})
	if err != nil {
		return out, err
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		return out, err
	}
	refs := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for key, child := range x {
				if key == "citations" {
					if ids, ok := child.([]any); ok {
						for _, id := range ids {
							if text, ok := id.(string); ok {
								refs[text] = true
							}
						}
					}
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(value)
	for _, c := range p.Citations {
		if refs[c.ID] {
			out.Citations = append(out.Citations, c)
		}
	}
	sort.Slice(out.Citations, func(i, j int) bool { return out.Citations[i].ID < out.Citations[j].ID })
	return out, nil
}

func ReviewHash(p Package, id string) (string, error) {
	subset, err := RelevantPackage(p, id)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(subset)
	if err != nil {
		return "", err
	}
	return storage.Hash(raw), nil
}

// Coverage reports reviewed metadata only after its bound content hash matches.
// Publication must additionally authenticate review records; untrusted JSON is
// never itself authorization to publish.
func Coverage(p Package, inventory catalog.GuidanceInventory, rules, markdown fs.FS, now time.Time) (CoverageReport, error) {
	out := CoverageReport{CatalogID: inventory.CatalogID, VariantCount: len(inventory.Variants), Counts: map[string]int{"missing": 0, "draft": 0, "reviewed": 0, "needs_revalidation": 0}, Profiles: map[string]map[string]int{}, Entries: []CoverageEntry{}}
	if err := ValidateDraft(p); err != nil {
		return out, err
	}
	if err := ValidateInventory(p, inventory); err != nil {
		return out, err
	}
	states, reasons, books := map[string]string{}, map[string]string{}, map[string]string{}
	for _, b := range p.Playbooks {
		subset, err := RelevantPackage(p, b.ID)
		if err != nil {
			return out, err
		}
		state, reason := "draft", "Source-comparison review has not been recorded."
		if err = VerifySources(subset, rules, markdown); err != nil {
			state, reason = "needs_revalidation", err.Error()
		} else if b.Review != nil {
			hash, err := ReviewHash(p, b.ID)
			if err != nil {
				return out, err
			}
			if b.Review.ContentHash != hash || b.Review.ReviewerID == "" || (b.Review.ReviewerType != "agent" && b.Review.ReviewerType != "human") || b.Review.ReviewedAt.IsZero() || b.Review.ReviewedAt.After(now) || ValidateComparison(p, b.ID, b.Review.Comparison) != nil {
				state, reason = "needs_revalidation", "Review does not bind current guidance."
			} else {
				state, reason = "reviewed", ""
			}
		}
		for _, id := range b.VariantIDs {
			states[id], reasons[id], books[id] = state, reason, b.ID
		}
	}
	for _, v := range inventory.Variants {
		state, reason := states[v.ID], reasons[v.ID]
		if state == "" {
			state, reason = "missing", "No playbook covers this catalog variant."
		}
		out.Counts[state]++
		out.Entries = append(out.Entries, CoverageEntry{VariantID: v.ID, RequirementID: v.RequirementID, Document: v.Document, Applicability: v.Applicability, PlaybookID: books[v.ID], State: state, Reason: reason})
		for _, id := range v.Profiles {
			if out.Profiles[id] == nil {
				out.Profiles[id] = map[string]int{}
			}
			out.Profiles[id][state]++
		}
	}
	out.Complete = out.VariantCount > 0 && out.Counts["reviewed"] == out.VariantCount
	return out, nil
}
