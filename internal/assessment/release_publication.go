package assessment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type ReleaseTransition struct {
	ServiceID        string `json:"service_id"`
	ID               string `json:"id"`
	ExpectedRevision int    `json:"expected_revision"`
	ManifestHash     string `json:"manifest_hash"`
	AudienceHash     string `json:"audience_hash"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	IdempotencyKey   string `json:"idempotency_key"`
}
type PublicReleaseView struct {
	ID         string           `json:"id"`
	State      string           `json:"state"`
	PreparedAt time.Time        `json:"prepared_at"`
	Period     AssessmentPeriod `json:"period"`
	Incomplete bool             `json:"incomplete"`
	Label      string           `json:"label"`
	Public     PublicRelease    `json:"public"`
}

func (s *Store) ConfigureTrustCentre(ctx context.Context, a auth.Actor, service, slug string, enabled bool) error {
	if a.Kind != "human" || a.TokenHash != "" {
		return auth.ErrForbidden
	}
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(slug) || len(slug) > 100 {
		return ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "admin"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO trust_centres(service_id,slug,enabled) VALUES($1,$2,$3) ON CONFLICT(service_id) DO UPDATE SET slug=excluded.slug,enabled=excluded.enabled`, service, slug, enabled); err != nil {
		return err
	}
	return commitOperation(ctx, tx, a, service, "trust.centre_configured", Record{ID: slug})
}

func (s *Store) TransitionRelease(ctx context.Context, a auth.Actor, in ReleaseTransition) (AssessmentRelease, error) {
	if a.Kind != "human" || a.TokenHash != "" {
		return AssessmentRelease{}, auth.ErrForbidden
	}
	if in.IdempotencyKey == "" || in.Reason == "" || len(in.Reason) > 4096 {
		return AssessmentRelease{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AssessmentRelease{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "export"); err != nil {
		return AssessmentRelease{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return AssessmentRelease{}, err
	}
	view := *s
	view.reader = tx
	view.presentationCache = nil
	digest := storage.Hash(data(in))
	var saved struct {
		ID string `json:"id"`
	}
	if found, e := readOperation(ctx, tx, a, in.ServiceID, "transition_assessment_release", in.IdempotencyKey, digest, &saved); found || e != nil {
		if e != nil {
			return AssessmentRelease{}, e
		}
		return view.release(ctx, in.ServiceID, saved.ID)
	}
	r, err := view.release(ctx, in.ServiceID, in.ID)
	if err != nil {
		return r, err
	}
	if r.Revision != in.ExpectedRevision || r.ManifestHash != in.ManifestHash || r.AudienceHash != in.AudienceHash {
		return r, ErrConflict
	}
	state := ""
	switch in.Action {
	case "approve":
		if r.State != "prepared" {
			return r, ErrConflict
		}
		if len(r.Manifest.Blockers) > 0 {
			return r, ErrInvalid
		}
		state = "approved"
	case "publish":
		if r.State != "approved" {
			return r, ErrConflict
		}
		var enabled bool
		if err = tx.QueryRow(ctx, `SELECT enabled FROM trust_centres WHERE service_id=$1`, in.ServiceID).Scan(&enabled); err != nil {
			return r, err
		}
		if !enabled {
			return r, ErrInvalid
		}
		state = "published"
	case "withdraw":
		if r.State != "published" {
			return r, ErrConflict
		}
		state = "withdrawn"
	default:
		return r, ErrInvalid
	}
	if state == "approved" || state == "published" {
		hash := sha256.New()
		if err = s.WriteReleaseArchive(ctx, hash, r.Manifest); err != nil {
			return r, err
		}
		if hex.EncodeToString(hash.Sum(nil)) != r.ArchiveHash {
			return r, ErrConflict
		}
	}
	now := s.now().UTC()
	receipt := map[string]any{"destination": "first_party", "release_id": r.ID, "manifest_hash": r.ManifestHash, "audience_hash": r.AudienceHash, "published_at": now}
	if _, err = tx.Exec(ctx, `UPDATE assessment_releases SET state=$3,revision=revision+1,published_at=CASE WHEN $3='published' THEN $4 ELSE published_at END,withdrawn_at=CASE WHEN $3='withdrawn' THEN $4 ELSE withdrawn_at END,receipt=CASE WHEN $3='published' THEN $5 ELSE receipt END WHERE service_id=$1 AND id=$2`, in.ServiceID, r.ID, state, now, data(receipt)); err != nil {
		return r, err
	}
	if state == "published" {
		if _, err = tx.Exec(ctx, `UPDATE trust_centres SET latest_release_id=$2 WHERE service_id=$1`, in.ServiceID, r.ID); err != nil {
			return r, err
		}
	}
	if state == "withdrawn" {
		if _, err = tx.Exec(ctx, `UPDATE trust_centres SET latest_release_id=(SELECT id FROM assessment_releases WHERE service_id=$1 AND state='published' ORDER BY published_at DESC,id DESC LIMIT 1) WHERE service_id=$1 AND latest_release_id=$2`, in.ServiceID, r.ID); err != nil {
			return r, err
		}
	}
	if _, err = insertRecord(ctx, tx, a, record(in.ServiceID, "assessment_release_event", uuid.NewString(), "", in), 1); err != nil {
		return r, err
	}
	saved.ID = r.ID
	if err = saveOperation(ctx, tx, a, in.ServiceID, "transition_assessment_release", in.IdempotencyKey, digest, saved); err != nil {
		return r, err
	}
	r.State = state
	r.Revision++
	return r, commitOperation(ctx, tx, a, in.ServiceID, "assessment.release_"+state, Record{ID: r.ID, Revision: r.Revision})
}

// PublicReleaseIndex uses an explicit projection. No internal manifest, evidence
// metadata, reviewer identity, or raw artifact URL is serialized here.
func (s *Store) PublicReleaseIndex(ctx context.Context, slug string, limit, offset int) ([]PublicReleaseView, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	var service string
	if err := s.query().QueryRow(ctx, `SELECT service_id FROM trust_centres WHERE slug=$1 AND enabled`, slug).Scan(&service); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT r.id,r.state,d.data->'public',d.data->'period',d.data->>'prepared_at',(d.data->>'incomplete')::boolean FROM assessment_releases r JOIN records d ON d.service_id=r.service_id AND d.kind='assessment_release' AND d.id=r.id AND d.revision=1 WHERE r.service_id=$1 AND r.state IN ('published','withdrawn') ORDER BY r.published_at DESC,r.id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicReleaseView{}
	for rows.Next() {
		var v PublicReleaseView
		var public, period []byte
		var prepared string
		if err = rows.Scan(&v.ID, &v.State, &public, &period, &prepared, &v.Incomplete); err != nil {
			return nil, err
		}
		if v.State == "withdrawn" {
			out = append(out, PublicReleaseView{ID: v.ID, State: "withdrawn", Label: "This release has been withdrawn."})
			continue
		}
		if err = json.Unmarshal(public, &v.Public); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(period, &v.Period); err != nil {
			return nil, err
		}
		v.PreparedAt, err = time.Parse(time.RFC3339Nano, prepared)
		if err != nil {
			return nil, err
		}
		v.Label = "Reviewed Fed assessment report; not an authorization claim. Detailed evidence requires approved access."
		out = append(out, v)
	}
	return out, rows.Err()
}
