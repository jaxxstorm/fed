// Package assessment owns authorized, versioned assessment mutations. All
// adapters call these services rather than writing directly to domain tables.
package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jaxxstorm/fed/internal/logging"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

var ErrConflict = errors.New("revision_conflict")
var ErrNotFound = errors.New("not_found")
var ErrInvalid = errors.New("invalid_input")

type Store struct {
	DB                *pgxpool.Pool
	Artifacts         storage.Artifacts
	Guidance          *readiness.Store
	guidanceContexts  map[string]readiness.GuidanceContext
	reader            recordReader
	clock             func() time.Time
	presentationCache *presentationCache
}

func (s *Store) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

type recordReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s *Store) query() recordReader {
	if s.reader != nil {
		return s.reader
	}
	return s.DB
}

type Record struct {
	ServiceID     string          `json:"service_id"`
	Kind          string          `json:"kind"`
	ID            string          `json:"id"`
	Revision      int             `json:"revision"`
	RequirementID string          `json:"requirement_id"`
	ActorID       string          `json:"actor_id"`
	Data          json.RawMessage `json:"data"`
	CreatedAt     time.Time       `json:"created_at"`
}
type Service struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	OrganizationID string `json:"organization_id"`
}

func (s *Store) CreateService(ctx context.Context, a auth.Actor, name string) (Service, error) {
	if name == "" {
		return Service{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Service{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, "", "admin"); err != nil {
		return Service{}, err
	}
	v := Service{ID: uuid.NewString(), Name: name, OrganizationID: a.OrganizationID}
	if _, err = tx.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES($1,$2,$3)`, v.ID, v.OrganizationID, v.Name); err != nil {
		return Service{}, err
	}
	if err = audit(ctx, tx, a, v.ID, "service.created", v.ID, 1); err != nil {
		return Service{}, err
	}
	return v, logging.Commit(ctx, tx, logging.Fields{Event: "assessment.service.created", Component: "assessment", ActorID: a.ID, ServiceID: v.ID})
}

func (s *Store) Services(ctx context.Context, a auth.Actor) ([]Service, error) {
	if a.Kind == "human" {
		if _, err := auth.ActiveMember(ctx, s.DB, a); err != nil {
			return nil, err
		}
	}
	rows, err := s.DB.Query(ctx, `SELECT id,name,organization_id FROM services WHERE organization_id=$1 ORDER BY name,id`, a.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []Service
	for rows.Next() {
		var v Service
		if err = rows.Scan(&v.ID, &v.Name, &v.OrganizationID); err != nil {
			return nil, err
		}
		candidates = append(candidates, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	result := []Service{}
	for _, v := range candidates {
		if auth.Authorize(ctx, s.DB, a, v.ID, "read") == nil {
			result = append(result, v)
		}
	}
	return result, nil
}

func audit(ctx context.Context, tx pgx.Tx, a auth.Actor, service, action, target string, revision int) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target,revision) VALUES($1,$2,$3,$4,$5,$6)`, a.OrganizationID, service, a.ID, action, target, revision)
	return err
}

func latest(ctx context.Context, q auth.Querier, service, kind, id string) (Record, error) {
	var r Record
	err := q.QueryRow(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM records WHERE service_id=$1 AND kind=$2 AND id=$3 ORDER BY revision DESC LIMIT 1`, service, kind, id).Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return r, err
}

func (s *Store) Get(ctx context.Context, a auth.Actor, service, kind, id string) (Record, error) {
	if c := s.presentationCache; c != nil && c.service == service && c.actor == a.ID {
		if r, ok := c.latest[kind+"/"+id]; ok {
			return r, nil
		}
		return Record{}, ErrNotFound
	}
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return Record{}, err
	}
	return latest(ctx, s.query(), service, kind, id)
}

func (s *Store) List(ctx context.Context, a auth.Actor, service, kind, requirement string, limit, offset int, includeRetired ...bool) ([]Record, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	history := len(includeRetired) > 0 && includeRetired[0]
	rows, err := s.query().Query(ctx, `SELECT * FROM (SELECT DISTINCT ON(id) service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM records r WHERE service_id=$1 AND kind=$2 AND ($2<>'evidence_source' OR $6 OR NOT EXISTS(SELECT 1 FROM connection_source_bindings b WHERE b.service_id=r.service_id AND b.source_id=r.id AND b.lifecycle='retired')) AND ($3='' OR requirement_id=$3 OR ($2='evidence' AND EXISTS(SELECT 1 FROM rule_evidence_links l WHERE l.service_id=r.service_id AND l.requirement_id=$3 AND l.evidence_id=r.id AND l.evidence_revision=r.revision))) ORDER BY id,revision DESC) r ORDER BY CASE WHEN $2='evidence' THEN created_at END DESC,id LIMIT $4 OFFSET $5`, service, kind, requirement, limit, offset, history)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Record{}
	for rows.Next() {
		var r Record
		if err = rows.Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// mutate is private so callers cannot bypass typed payload validation.
func (s *Store) mutate(ctx context.Context, a auth.Actor, r Record, expected int, action string, validate func(pgx.Tx) error) (Record, error) {
	if r.ServiceID == "" || r.ID == "" || expected < 0 || !json.Valid(r.Data) {
		return Record{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, r.ServiceID, action); err != nil {
		return Record{}, err
	}
	// Serialize writes within a service, including creation of absent records.
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, r.ServiceID); err != nil {
		return Record{}, err
	}
	old, err := latest(ctx, tx, r.ServiceID, r.Kind, r.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	if old.Revision != expected {
		return Record{}, ErrConflict
	}
	if old.Revision > 0 && old.RequirementID != r.RequirementID {
		return Record{}, ErrInvalid
	}
	if validate != nil {
		if err = validate(tx); err != nil {
			return Record{}, err
		}
	}
	r.Revision = expected + 1
	r.ActorID = a.ID
	err = tx.QueryRow(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, r.ServiceID, r.Kind, r.ID, r.Revision, r.RequirementID, r.ActorID, r.Data).Scan(&r.CreatedAt)
	if err != nil {
		return Record{}, err
	}
	if err = audit(ctx, tx, a, r.ServiceID, r.Kind+".saved", r.ID, r.Revision); err != nil {
		return Record{}, err
	}
	return r, logging.Commit(ctx, tx, logging.Fields{Event: "assessment.record.committed", Component: "assessment", ActorID: a.ID, ServiceID: r.ServiceID, RecordID: r.ID, Stage: r.Kind, Revision: r.Revision})
}
