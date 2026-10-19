package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

var ErrConflict = errors.New("guidance revision conflict")
var ErrReviewRequired = errors.New("complete source-reviewed guidance coverage required")
var ErrInvalid = errors.New("invalid guidance")

type Store struct {
	DB       *pgxpool.Pool
	Rules    fs.FS
	Markdown fs.FS
}

func (s *Store) inventory(ctx context.Context, p Package) (catalog.GuidanceInventory, error) {
	return inventoryFrom(ctx, s.DB, p.CatalogID)
}

// Catalog identities include the immutable payload hash. Cache only catalog
// inventory, never tenant data, reviews, source freshness or coverage decisions.
// Bound retention because a full profile inventory is large.
var inventoryCache = struct {
	sync.Mutex
	items map[string]catalog.GuidanceInventory
	order []string
}{items: map[string]catalog.GuidanceInventory{}}

func inventoryFrom(ctx context.Context, q auth.Querier, catalogID string) (catalog.GuidanceInventory, error) {
	var payload, schema []byte
	var commit string
	if err := q.QueryRow(ctx, `SELECT payload,schema_payload,commit_sha FROM catalogs WHERE id=$1`, catalogID).Scan(&payload, &schema, &commit); err != nil {
		return catalog.GuidanceInventory{}, err
	}
	// Check the content identity even on cache hits, rather than trusting a row ID.
	if commit+":"+storage.Hash(payload) != catalogID {
		return catalog.GuidanceInventory{}, fmt.Errorf("stored catalog identity mismatch")
	}
	inventoryCache.Lock()
	defer inventoryCache.Unlock()
	if err := ctx.Err(); err != nil {
		return catalog.GuidanceInventory{}, err
	}
	cacheKey := catalogID + ":" + storage.Hash(schema)
	if cached, ok := inventoryCache.items[cacheKey]; ok {
		return cached, nil
	}
	snapshot, err := catalog.Parse(payload, schema, commit)
	if err != nil {
		return catalog.GuidanceInventory{}, err
	}
	if snapshot.ID != catalogID {
		return catalog.GuidanceInventory{}, fmt.Errorf("stored catalog identity mismatch")
	}
	i, err := catalog.Inventory(snapshot)
	if err != nil {
		return i, err
	}
	if len(inventoryCache.order) == 2 {
		delete(inventoryCache.items, inventoryCache.order[0])
		inventoryCache.order = inventoryCache.order[1:]
	}
	inventoryCache.items[cacheKey] = i
	inventoryCache.order = append(inventoryCache.order, cacheKey)
	return i, nil
}

func (s *Store) validate(ctx context.Context, p Package) (catalog.GuidanceInventory, error) {
	if err := ValidateDraft(p); err != nil {
		return catalog.GuidanceInventory{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	i, err := s.inventory(ctx, p)
	if err != nil {
		return i, err
	}
	if err = ValidateInventory(p, i); err != nil {
		return i, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if s.Rules == nil || s.Markdown == nil {
		return i, fmt.Errorf("%w: official source corpus unavailable", ErrInvalid)
	}
	if err = VerifySources(p, s.Rules, s.Markdown); err != nil {
		return i, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return i, nil
}

// RecordContentReview requires service edit authorization, rechecked
// inside the transaction. Review metadata in submitted JSON is never trusted.
func (s *Store) RecordContentReview(ctx context.Context, a auth.Actor, service string, p Package, id string, comparison SourceComparison) (Review, error) {
	if service == "" {
		return Review{}, auth.ErrForbidden
	}
	if err := auth.Authorize(ctx, s.DB, a, service, "edit"); err != nil {
		return Review{}, err
	}
	if _, err := s.validate(ctx, p); err != nil {
		return Review{}, err
	}
	hash, err := ReviewHash(p, id)
	if err != nil {
		return Review{}, err
	}
	if err = ValidateComparison(p, id, comparison); err != nil {
		return Review{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	raw, err := json.Marshal(comparison)
	if err != nil {
		return Review{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "edit"); err != nil {
		return Review{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO readiness_agent_reviews(organization_id,service_id,catalog_id,playbook_id,content_hash,reviewer_id,reviewer_type,comparison) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, a.OrganizationID, service, p.CatalogID, id, hash, a.ID, a.Kind, raw)
	if err != nil {
		return Review{}, err
	}
	var out Review
	var saved []byte
	if err = tx.QueryRow(ctx, `SELECT reviewer_id,reviewer_type,reviewed_at,content_hash,comparison FROM readiness_agent_reviews WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 AND playbook_id=$4 AND content_hash=$5`, a.OrganizationID, service, p.CatalogID, id, hash).Scan(&out.ReviewerID, &out.ReviewerType, &out.ReviewedAt, &out.ContentHash, &saved); err != nil {
		return out, err
	}
	if err = json.Unmarshal(saved, &out.Comparison); err != nil {
		return out, err
	}
	canonical, err := json.Marshal(out.Comparison)
	if err != nil {
		return out, err
	}
	if string(canonical) != string(raw) {
		return out, ErrConflict
	}
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target,revision) VALUES($1,$2,$3,'readiness.content.reviewed',$4,$5)`, a.OrganizationID, service, a.ID, id, p.Revision); err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}

// Publish stores an immutable package only when every inventory variant is
// covered by current, authenticated review records. It does not activate it or
// change service compliance state.
func (s *Store) Publish(ctx context.Context, a auth.Actor, service string, p Package) (string, error) {
	if service == "" {
		return "", auth.ErrForbidden
	}
	if err := auth.Authorize(ctx, s.DB, a, service, "edit"); err != nil {
		return "", err
	}
	i, err := s.validate(ctx, p)
	if err != nil {
		return "", err
	}
	// Own the slice before replacing caller-supplied review metadata.
	p.Playbooks = append([]Playbook(nil), p.Playbooks...)
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "edit"); err != nil {
		return "", err
	}
	for index, b := range p.Playbooks {
		hash, err := ReviewHash(p, b.ID)
		if err != nil {
			return "", err
		}
		var review Review
		var comparison []byte
		err = tx.QueryRow(ctx, `SELECT reviewer_id,reviewer_type,reviewed_at,content_hash,comparison FROM readiness_agent_reviews WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 AND playbook_id=$4 AND content_hash=$5`, a.OrganizationID, service, p.CatalogID, b.ID, hash).Scan(&review.ReviewerID, &review.ReviewerType, &review.ReviewedAt, &review.ContentHash, &comparison)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrReviewRequired
		}
		if err != nil {
			return "", err
		}
		if err = json.Unmarshal(comparison, &review.Comparison); err != nil {
			return "", err
		}
		p.Playbooks[index].Review = &review
	}
	coverage, err := Coverage(p, i, s.Rules, s.Markdown, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if !coverage.Complete {
		return "", ErrReviewRequired
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	hash := storage.Hash(raw)
	tag, err := tx.Exec(ctx, `INSERT INTO readiness_service_releases(organization_id,service_id,catalog_id,revision,content_hash,payload,publisher_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, a.OrganizationID, service, p.CatalogID, p.Revision, hash, raw, a.ID)
	if err != nil {
		return "", err
	}
	var saved string
	if err = tx.QueryRow(ctx, `SELECT content_hash FROM readiness_service_releases WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 AND revision=$4`, a.OrganizationID, service, p.CatalogID, p.Revision).Scan(&saved); err != nil {
		return "", err
	}
	if saved != hash {
		return "", ErrConflict
	}
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target,revision) VALUES($1,$2,$3,'readiness.guidance.published',$4,$5)`, a.OrganizationID, service, a.ID, p.CatalogID, p.Revision); err != nil {
			return "", err
		}
	}
	return hash, tx.Commit(ctx)
}
