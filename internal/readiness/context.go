package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
)

// GuidanceContext carries the same source-backed explanation to every transport.
// It is content context, not an assessment of the service's evidence.
type GuidanceContext struct {
	Progress        *RequirementProgress `json:"progress,omitempty"`
	State           string               `json:"state"`
	Reason          string               `json:"reason,omitempty"`
	Label           string               `json:"label,omitempty"`
	Caveat          string               `json:"caveat,omitempty"`
	CatalogID       string               `json:"catalog_id"`
	VariantID       string               `json:"variant_id"`
	ReleaseRevision int                  `json:"release_revision,omitempty"`
	ReleaseHash     string               `json:"release_hash,omitempty"`
	Guidance        *Package             `json:"guidance,omitempty"`
}

func (s *Store) Context(ctx context.Context, q auth.Querier, a auth.Actor, service, catalogID string, rule catalog.Resolved, now time.Time) (GuidanceContext, error) {
	items, err := s.contexts(ctx, q, a, service, catalogID, []catalog.Resolved{rule}, now, "read")
	if err != nil {
		return GuidanceContext{}, err
	}
	return items[0], nil
}

func (s *Store) Contexts(ctx context.Context, q auth.Querier, a auth.Actor, service, catalogID string, rules []catalog.Resolved, now time.Time) ([]GuidanceContext, error) {
	return s.contexts(ctx, q, a, service, catalogID, rules, now, "read")
}

// ExportContexts reads one immutable release inside the caller's snapshot and
// honors export-only tokens without granting them interactive read access.
func (s *Store) ExportContexts(ctx context.Context, q auth.Querier, a auth.Actor, service, catalogID string, rules []catalog.Resolved, now time.Time) ([]GuidanceContext, error) {
	return s.contexts(ctx, q, a, service, catalogID, rules, now, "export")
}

func (s *Store) contexts(ctx context.Context, q auth.Querier, a auth.Actor, service, catalogID string, rules []catalog.Resolved, now time.Time, action string) ([]GuidanceContext, error) {
	if err := auth.Authorize(ctx, q, a, service, action); err != nil {
		return nil, err
	}
	var revision int
	var hash string
	var raw []byte
	err := q.QueryRow(ctx, `SELECT revision,content_hash,payload FROM readiness_service_releases WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 ORDER BY revision DESC LIMIT 1`, a.OrganizationID, service, catalogID).Scan(&revision, &hash, &raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var p Package
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
	}
	out := make([]GuidanceContext, 0, len(rules))
	verifier := newSourceVerifier(s.Rules, s.Markdown)
	for _, rule := range rules {
		v, err := catalog.Variant(rule)
		if err != nil {
			return nil, err
		}
		item := GuidanceContext{State: "missing", CatalogID: catalogID, VariantID: v.ID, ReleaseRevision: revision, ReleaseHash: hash, Reason: "No published source-reviewed guidance covers this requirement variant."}
		item, err = s.projectContext(p, item, now, verifier)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if len(p.Playbooks) > 0 {
		if err := s.attachProgress(ctx, q, service, p, rules, out, now, verifier); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) projectContext(p Package, out GuidanceContext, now time.Time, verifier *sourceVerifier) (GuidanceContext, error) {
	for _, b := range p.Playbooks {
		if !slices.Contains(b.VariantIDs, out.VariantID) {
			continue
		}
		subset, err := RelevantPackage(p, b.ID)
		if err != nil {
			return out, err
		}
		out.Guidance = &subset
		defer func() { subset.Playbooks[0].Review = b.Review }()
		if b.Review == nil {
			out.State = "draft"
			out.Reason = "Guidance has not completed source-comparison review."
			return out, nil
		}
		if b.Review != nil && b.Review.ReviewerType == "agent" {
			out.Caveat = AgentReviewCaveat
		}
		out.State = "needs_revalidation"
		out.Reason = "Guidance source review needs revalidation."
		if s.Rules == nil || s.Markdown == nil {
			out.Reason = "The official source corpus is unavailable for guidance verification."
			return out, nil
		}
		hash, err := reviewSubsetHash(subset)
		if err != nil {
			return out, err
		}
		if b.Review == nil || b.Review.ContentHash != hash || b.Review.ReviewerID == "" || b.Review.ReviewedAt.IsZero() || b.Review.ReviewedAt.After(now) || validateSubsetComparison(subset, b.Review.Comparison) != nil || verifier.verify(subset) != nil {
			return out, nil
		}
		if b.Review.ReviewerType == "agent" {
			out.Label = AgentReviewLabel
			out.Caveat = AgentReviewCaveat
		} else if b.Review.ReviewerType == "human" {
			out.Label = "Human-reviewed guidance"
			out.Caveat = "Checked against cited official FedRAMP sources. Interpretations may be incomplete or incorrect; this is not an official FedRAMP determination."
		} else {
			return out, nil
		}
		out.State = "reviewed"
		out.Reason = ""
		return out, nil
	}
	return out, nil
}
