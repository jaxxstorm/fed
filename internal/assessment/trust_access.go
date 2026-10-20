package assessment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

type TrustAccessRequest struct {
	ID          string    `json:"id"`
	IdentityID  string    `json:"identity_id"`
	ReleaseID   string    `json:"release_id"`
	State       string    `json:"state"`
	RequestedAt time.Time `json:"requested_at"`
}
type TrustGrant struct {
	ID         string     `json:"id"`
	IdentityID string     `json:"identity_id"`
	ReleaseID  string     `json:"release_id"`
	Capacity   string     `json:"capacity"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func trustIdentity(ctx context.Context, q auth.Querier, a auth.Actor) error {
	if a.Kind != "human" || a.TokenHash != "" {
		return auth.ErrForbidden
	}
	_, err := auth.ActiveMember(ctx, q, a)
	if errors.Is(err, auth.ErrMembershipPending) {
		return nil
	}
	return err
}
func (s *Store) TrustService(ctx context.Context, slug string) (string, error) {
	var service string
	err := s.query().QueryRow(ctx, `SELECT service_id FROM trust_centres WHERE slug=$1 AND enabled`, slug).Scan(&service)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return service, err
}

func (s *Store) RequestTrustAccess(ctx context.Context, a auth.Actor, slug, release string) (TrustAccessRequest, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return TrustAccessRequest{}, err
	}
	defer tx.Rollback(ctx)
	if err = trustIdentity(ctx, tx, a); err != nil {
		return TrustAccessRequest{}, err
	}
	var service string
	if err = tx.QueryRow(ctx, `SELECT t.service_id FROM trust_centres t JOIN services s ON s.id=t.service_id WHERE t.slug=$1 AND t.enabled AND s.organization_id=$2 FOR UPDATE OF s`, slug, a.OrganizationID).Scan(&service); err != nil {
		return TrustAccessRequest{}, ErrNotFound
	}
	if release != "" {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assessment_releases WHERE service_id=$1 AND id=$2 AND state='published')`, service, release).Scan(&exists); err != nil {
			return TrustAccessRequest{}, err
		}
		if !exists {
			return TrustAccessRequest{}, ErrNotFound
		}
	}
	out := TrustAccessRequest{ID: uuid.NewString(), IdentityID: a.ID, ReleaseID: release, State: "pending"}
	err = tx.QueryRow(ctx, `SELECT id,requested_at FROM trust_access_requests WHERE service_id=$1 AND identity_id=$2 AND coalesce(release_id,'')=$3 AND state='pending' ORDER BY requested_at DESC LIMIT 1`, service, a.ID, release).Scan(&out.ID, &out.RequestedAt)
	if err == nil {
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO trust_access_requests(id,service_id,identity_id,release_id,state) VALUES($1,$2,$3,nullif($4,''),'pending') RETURNING requested_at`, out.ID, service, a.ID, release).Scan(&out.RequestedAt)
	if err != nil {
		return out, err
	}
	return out, commitOperation(ctx, tx, a, service, "trust.access_requested", Record{ID: out.ID})
}

func (s *Store) TrustRequests(ctx context.Context, a auth.Actor, service string, limit, offset int) ([]TrustAccessRequest, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.query(), a, service, "admin"); err != nil {
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT id,identity_id,coalesce(release_id,''),state,requested_at FROM trust_access_requests WHERE service_id=$1 ORDER BY requested_at DESC,id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrustAccessRequest{}
	for rows.Next() {
		var r TrustAccessRequest
		if err = rows.Scan(&r.ID, &r.IdentityID, &r.ReleaseID, &r.State, &r.RequestedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DecideTrustAccess(ctx context.Context, a auth.Actor, service, request, capacity string, expires time.Time, approve bool) error {
	if a.Kind != "human" || a.TokenHash != "" {
		return auth.ErrForbidden
	}
	if approve && ((capacity != "reader" && capacity != "assessor") || !expires.After(s.now())) {
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
	var identity, release string
	if err = tx.QueryRow(ctx, `SELECT identity_id,coalesce(release_id,'') FROM trust_access_requests WHERE id=$1 AND service_id=$2 AND state='pending' FOR UPDATE`, request, service).Scan(&identity, &release); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	state := "denied"
	if approve {
		state = "approved"
		if _, err = tx.Exec(ctx, `INSERT INTO trust_access_grants(id,service_id,identity_id,release_id,capacity,granted_by,expires_at) VALUES($1,$2,$3,nullif($4,''),$5,$6,$7)`, uuid.NewString(), service, identity, release, capacity, a.ID, expires); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE trust_access_requests SET state=$3,decided_by=$4,decided_at=now() WHERE id=$1 AND service_id=$2`, request, service, state, a.ID); err != nil {
		return err
	}
	return commitOperation(ctx, tx, a, service, "trust.access_"+state, Record{ID: request})
}

func (s *Store) RevokeTrustGrant(ctx context.Context, a auth.Actor, service, id string) error {
	if a.Kind != "human" || a.TokenHash != "" {
		return auth.ErrForbidden
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "admin"); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE trust_access_grants SET revoked_at=coalesce(revoked_at,now()) WHERE service_id=$1 AND id=$2`, service, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return commitOperation(ctx, tx, a, service, "trust.grant_revoked", Record{ID: id})
}
func (s *Store) TrustGrants(ctx context.Context, a auth.Actor, service string, limit, offset int) ([]TrustGrant, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.query(), a, service, "admin"); err != nil {
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT id,identity_id,coalesce(release_id,''),capacity,expires_at,revoked_at FROM trust_access_grants WHERE service_id=$1 ORDER BY granted_at DESC,id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrustGrant{}
	for rows.Next() {
		var v TrustGrant
		if err = rows.Scan(&v.ID, &v.IdentityID, &v.ReleaseID, &v.Capacity, &v.ExpiresAt, &v.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) RestrictedRelease(ctx context.Context, a auth.Actor, slug, id string) (AssessmentRelease, error) {
	if err := trustIdentity(ctx, s.query(), a); err != nil {
		return AssessmentRelease{}, err
	}
	service, err := s.TrustService(ctx, slug)
	if err != nil {
		return AssessmentRelease{}, err
	}
	var allowed bool
	err = s.query().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trust_access_grants g JOIN services s ON s.id=g.service_id JOIN assessment_releases r ON r.service_id=g.service_id AND r.id=$3 WHERE g.service_id=$1 AND g.identity_id=$2 AND s.organization_id=$4 AND (g.release_id IS NULL OR g.release_id=r.id) AND g.revoked_at IS NULL AND g.expires_at>now() AND r.state='published')`, service, a.ID, id, a.OrganizationID).Scan(&allowed)
	if err != nil {
		return AssessmentRelease{}, err
	}
	if !allowed {
		return AssessmentRelease{}, auth.ErrForbidden
	}
	r, err := s.release(ctx, service, id)
	if err != nil {
		return r, err
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'trust.release_accessed',$4)`, a.OrganizationID, service, a.ID, id)
	return r, err
}
