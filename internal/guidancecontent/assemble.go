package guidancecontent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

// Assemble canonicalizes citation identities across independently authored
// batches, preserving shared capability IDs only when their definitions agree.
// Reviews must be recorded against the assembled hashes, never copied from a
// batch whose identifiers or dependency closure changed during assembly.
func Assemble(batches []readiness.Package, revision int) (readiness.Package, error) {
	out := readiness.Package{Format: readiness.Format, Revision: revision, Sources: map[string]readiness.Source{}}
	if revision < 1 || len(batches) == 0 {
		return out, fmt.Errorf("assembly requires batches and a positive revision")
	}
	citations := map[string]readiness.Citation{}
	capabilities := map[string]readiness.Capability{}
	for _, original := range batches {
		if err := readiness.ValidateDraft(original); err != nil {
			return out, err
		}
		if out.CatalogID == "" {
			out.CatalogID = original.CatalogID
		}
		if original.CatalogID != out.CatalogID {
			return out, fmt.Errorf("batch catalog mismatch")
		}
		for key, source := range original.Sources {
			if previous, ok := out.Sources[key]; ok && previous != source {
				return out, fmt.Errorf("conflicting source binding %s", key)
			}
			out.Sources[key] = source
		}
		ids := map[string]string{}
		for _, c := range original.Citations {
			old := c.ID
			c.ID = ""
			raw, err := json.Marshal(c)
			if err != nil {
				return out, err
			}
			c.ID = "citation-" + storage.Hash(raw)
			ids[old] = c.ID
			citations[c.ID] = c
		}
		// Decode a private copy so assembling does not rewrite authored batches.
		raw, err := json.Marshal(original)
		if err != nil {
			return out, err
		}
		var value map[string]any
		if err = json.Unmarshal(raw, &value); err != nil {
			return out, err
		}
		var remap func(any)
		remap = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for key, child := range x {
					if key == "citations" {
						if refs, ok := child.([]any); ok {
							for i, ref := range refs {
								if id, ok := ref.(string); ok {
									refs[i] = ids[id]
								}
							}
						}
					} else {
						remap(child)
					}
				}
			case []any:
				for _, child := range x {
					remap(child)
				}
			}
		}
		// Top-level citations are replaced below; only statement references change.
		delete(value, "citations")
		remap(value)
		raw, err = json.Marshal(value)
		if err != nil {
			return out, err
		}
		var p readiness.Package
		if err = json.Unmarshal(raw, &p); err != nil {
			return out, err
		}
		for _, c := range p.Capabilities {
			if previous, ok := capabilities[c.ID]; ok && !reflect.DeepEqual(previous, c) {
				return out, fmt.Errorf("conflicting shared capability %s; reconcile its source-backed definition before assembly", c.ID)
			}
			capabilities[c.ID] = c
		}
		for _, book := range p.Playbooks {
			book.Review = nil
			out.Playbooks = append(out.Playbooks, book)
		}
	}
	for _, c := range citations {
		out.Citations = append(out.Citations, c)
	}
	for _, c := range capabilities {
		out.Capabilities = append(out.Capabilities, c)
	}
	sort.Slice(out.Citations, func(i, j int) bool { return out.Citations[i].ID < out.Citations[j].ID })
	sort.Slice(out.Capabilities, func(i, j int) bool { return out.Capabilities[i].ID < out.Capabilities[j].ID })
	sort.Slice(out.Playbooks, func(i, j int) bool { return out.Playbooks[i].ID < out.Playbooks[j].ID })
	return out, readiness.ValidateDraft(out)
}
