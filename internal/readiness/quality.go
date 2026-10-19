package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

// QualityPage reports published content quality across the complete catalog,
// never a service compliance percentage. Draft files are not published releases.
type QualityPage struct {
	ServiceID       string          `json:"service_id"`
	CatalogID       string          `json:"catalog_id"`
	ReleaseRevision int             `json:"release_revision"`
	ReleaseHash     string          `json:"release_hash"`
	CountUnit       string          `json:"count_unit"`
	VariantCount    int             `json:"variant_count"`
	Counts          map[string]int  `json:"counts"`
	Complete        bool            `json:"complete"`
	Entries         []CoverageEntry `json:"entries"`
	Offset          int             `json:"offset"`
	NextOffset      *int            `json:"next_offset,omitempty"`
}

func (s *Store) Quality(ctx context.Context, a auth.Actor, service string, limit, offset int) (QualityPage, error) {
	out := QualityPage{ServiceID: service, CountUnit: "catalog requirement variants", Entries: []CoverageEntry{}, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "read"); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT data->>'catalog_id' FROM records WHERE service_id=$1 AND kind='profile' AND id='profile' ORDER BY revision DESC LIMIT 1`, service).Scan(&out.CatalogID); err != nil {
		return out, err
	}
	i, err := inventoryFrom(ctx, tx, out.CatalogID)
	if err != nil {
		return out, err
	}
	p := Package{Format: Format, CatalogID: out.CatalogID, Revision: 1}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT revision,content_hash,payload FROM readiness_service_releases WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 ORDER BY revision DESC LIMIT 1`, a.OrganizationID, service, out.CatalogID).Scan(&out.ReleaseRevision, &out.ReleaseHash, &raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if err == nil {
		if err = json.Unmarshal(raw, &p); err != nil {
			return out, err
		}
	}
	report, err := Coverage(p, i, s.Rules, s.Markdown, time.Now().UTC())
	if err != nil {
		return out, err
	}
	out.VariantCount = report.VariantCount
	out.Counts = report.Counts
	out.Complete = report.Complete
	// Clamp before addition to avoid overflow on untrusted offsets.
	start := min(offset, len(report.Entries))
	end := start + min(limit, len(report.Entries)-start)
	out.Entries = report.Entries[start:end]
	if end < len(report.Entries) {
		out.NextOffset = &end
	}
	return out, tx.Commit(ctx)
}
