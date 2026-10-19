package assessment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"sort"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type ReadinessQueueInput struct {
	ServiceID string `json:"service_id"`
	Group     string `json:"group,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
}

type ReadinessRequirement struct {
	RequirementID string                  `json:"requirement_id"`
	Name          string                  `json:"name"`
	Group         string                  `json:"group"`
	Applicability string                  `json:"applicability"`
	Presentation  RequirementPresentation `json:"presentation"`
}

type ReadinessQueue struct {
	ServiceID  string                 `json:"service_id"`
	Binding    InstructionBinding     `json:"binding"`
	AsOf       time.Time              `json:"as_of"`
	CountUnit  string                 `json:"count_unit"`
	Counts     map[string]int         `json:"counts"`
	Total      int                    `json:"total"`
	Items      []ReadinessRequirement `json:"items"`
	NextCursor string                 `json:"next_cursor,omitempty"`
	ViewHash   string                 `json:"view_hash"`
	Ordering   string                 `json:"ordering"`
}

// ReadinessQueue reuses the batched assessment projection. Its groups do not
// substitute capability declarations for evidence assessment or human review.
func (s *Store) ReadinessQueue(ctx context.Context, a auth.Actor, in ReadinessQueueInput) (ReadinessQueue, error) {
	out := ReadinessQueue{ServiceID: in.ServiceID, CountUnit: "requirements", Counts: map[string]int{"evidence": 0, "review": 0, "scoping": 0, "maintain": 0, "excluded": 0}, Items: []ReadinessRequirement{}, Ordering: "Within each work group, access blockers and contradictory findings come before stale or missing evidence. Human review remains a separate step."}
	if in.Limit == 0 {
		in.Limit = 25
	}
	if in.Limit < 1 || in.Limit > 100 || !slices.Contains([]string{"", "evidence", "review", "scoping", "maintain", "excluded"}, in.Group) {
		return out, ErrInvalid
	}
	snapshot, err := s.Presentation(ctx, a, in.ServiceID)
	if err != nil {
		return out, err
	}
	out.Binding = snapshot.Binding
	out.AsOf = snapshot.AsOf
	items := []ReadinessRequirement{}
	for _, r := range snapshot.Requirements {
		p := snapshot.States[r.Requirement.ID]
		group := "evidence"
		switch p.Status {
		case "not_applicable":
			group = "excluded"
		case "needs_scoping":
			group = "scoping"
		case "in_review":
			group = "review"
		case "passing":
			group = "maintain"
		}
		out.Counts[group]++
		if in.Group != "" && group != in.Group {
			continue
		}
		items = append(items, ReadinessRequirement{RequirementID: r.Requirement.ID, Name: r.Requirement.Name, Group: group, Applicability: r.Applicability, Presentation: p})
	}
	rank := map[string]int{"blocked": 0, "failing": 1, "needs_refresh": 2, "needs_scoping": 3, "needs_evidence": 4, "in_progress": 5, "not_started": 6, "in_review": 7, "passing": 8, "not_applicable": 9}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if rank[a.Presentation.Status] != rank[b.Presentation.Status] {
			return rank[a.Presentation.Status] < rank[b.Presentation.Status]
		}
		return a.RequirementID < b.RequirementID
	})
	out.Total = len(items)
	out.ViewHash = storage.Hash(data(struct {
		Binding InstructionBinding
		Items   []ReadinessRequirement
		Counts  map[string]int
	}{out.Binding, items, out.Counts}))
	filter := storage.Hash([]byte(in.Group))
	offset := 0
	if in.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(in.Cursor)
		var c workCursor
		if e != nil || json.Unmarshal(raw, &c) != nil || c.Offset < 0 || c.Service != in.ServiceID || c.Filter != filter {
			return out, ErrInvalid
		}
		if c.View != out.ViewHash {
			return out, ErrViewChanged
		}
		offset = c.Offset
	}
	if offset > len(items) {
		return out, ErrInvalid
	}
	end := offset + min(in.Limit, len(items)-offset)
	out.Items = items[offset:end]
	if end < len(items) {
		out.NextCursor = base64.RawURLEncoding.EncodeToString(data(workCursor{in.ServiceID, filter, out.ViewHash, end}))
	}
	return out, nil
}
