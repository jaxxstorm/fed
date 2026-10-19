package assessment

import (
	"context"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

func (s *Store) loadGuidanceContexts(ctx context.Context, a auth.Actor, service, catalogID string, all []catalog.Resolved, now time.Time) error {
	if s.Guidance == nil {
		return nil
	}
	items, err := s.Guidance.Contexts(ctx, s.query(), a, service, catalogID, all, now)
	if err != nil {
		return err
	}
	s.guidanceContexts = map[string]readiness.GuidanceContext{}
	for i, g := range items {
		s.guidanceContexts[all[i].Requirement.ID] = g
	}
	return nil
}

func (s *Store) guidanceFor(ctx context.Context, a auth.Actor, service, rid, catalogID string, now time.Time) (*readiness.GuidanceContext, error) {
	if s.Guidance == nil {
		return nil, nil
	}
	if s.guidanceContexts != nil {
		if g, ok := s.guidanceContexts[rid]; ok {
			return &g, nil
		}
		return nil, nil
	}
	all, err := s.Requirements(ctx, a, service)
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		if r.Requirement.ID == rid {
			g, err := s.Guidance.Context(ctx, s.query(), a, service, catalogID, r, now)
			return &g, err
		}
	}
	return nil, ErrNotFound
}

func bindGuidance(binding *InstructionBinding, g *readiness.GuidanceContext) []Assertion {
	if g == nil || g.State != "reviewed" || g.Guidance == nil || len(g.Guidance.Playbooks) != 1 || g.Guidance.Playbooks[0].Kind != "actionable" {
		return nil
	}
	book := g.Guidance.Playbooks[0]
	binding.GuidanceRevision = g.ReleaseRevision
	binding.GuidanceHash = g.ReleaseHash
	binding.GuidanceVariant = g.VariantID
	binding.AssertionHash = storage.Hash(data(book.Assertions))
	out := make([]Assertion, 0, len(book.Assertions))
	for _, a := range book.Assertions {
		out = append(out, Assertion{ID: a.ID, Statement: a.Obligation.Text, Required: true, EvidenceKinds: []string{"document", "repository", "operational_record", "configuration"}})
	}
	return out
}
