package assessment

import (
	"context"
	"github.com/jaxxstorm/fed/internal/auth"
	"time"
)

type EvidenceFilter struct {
	Search, Source, Freshness, Requirement, Type string
	Offset                                       int
}

func (s *Store) ActivityPage(ctx context.Context, a auth.Actor, service, search string, offset int) ([]Record, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	if offset < 0 || offset > 1000000 {
		return nil, ErrInvalid
	}
	rows, err := s.query().Query(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM (SELECT DISTINCT ON(id) * FROM records WHERE service_id=$1 AND kind='rule_result' ORDER BY id,revision DESC) r WHERE ($2='' OR requirement_id ILIKE '%'||$2||'%' OR data#>>'{submission,explanation}' ILIKE '%'||$2||'%') ORDER BY created_at DESC,id LIMIT 26 OFFSET $3`, service, search, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var v Record
		if err = rows.Scan(&v.ServiceID, &v.Kind, &v.ID, &v.Revision, &v.RequirementID, &v.ActorID, &v.Data, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SourcePage(ctx context.Context, a auth.Actor, service, search string, offset int) ([]Record, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	if offset < 0 || offset > 1000000 {
		return nil, ErrInvalid
	}
	rows, err := s.query().Query(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM (SELECT DISTINCT ON(id) * FROM records WHERE service_id=$1 AND kind='evidence_source' ORDER BY id,revision DESC) r WHERE ($2='' OR data->>'name' ILIKE '%'||$2||'%' OR data->>'locator' ILIKE '%'||$2||'%') ORDER BY data->>'name',id LIMIT 26 OFFSET $3`, service, search, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var v Record
		if err = rows.Scan(&v.ServiceID, &v.Kind, &v.ID, &v.Revision, &v.RequirementID, &v.ActorID, &v.Data, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) EvidenceRevision(ctx context.Context, a auth.Actor, service string, ref Ref) (Record, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return Record{}, err
	}
	return reference(ctx, s.query(), service, "evidence", ref)
}

type EvidenceListItem struct {
	Record
	Freshness          string   `json:"freshness"`
	LinkedRequirements []string `json:"linked_requirements"`
}

func (s *Store) EvidencePage(ctx context.Context, a auth.Actor, service string, f EvidenceFilter, now time.Time) ([]EvidenceListItem, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	if f.Offset < 0 || f.Offset > 1000000 {
		return nil, ErrInvalid
	}
	rows, err := s.query().Query(ctx, `WITH latest AS (
 SELECT DISTINCT ON(id) * FROM records WHERE service_id=$1 AND kind='evidence' ORDER BY id,revision DESC
  ), measures AS (SELECT DISTINCT ON(id) requirement_id,data FROM records WHERE service_id=$1 AND kind='measure' ORDER BY id,revision DESC), policies AS (SELECT requirement_id,min((data->>'freshness_hours')::int) AS hours FROM measures GROUP BY requirement_id), classified AS (
  SELECT r.*, CASE WHEN (r.data->>'scope_revision')::int IS DISTINCT FROM (SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope') THEN 'needs_refresh' WHEN r.data->'document' IS NOT NULL THEN
 CASE WHEN (r.data->>'observed_at')::timestamptz + make_interval(hours => (r.data->'document'->>'freshness_hours')::int) <= $2
 OR (r.data->'document'->'source'->>'revision')::int IS DISTINCT FROM (SELECT max(revision) FROM records WHERE service_id=$1 AND kind='evidence_source' AND id=r.data->'document'->'source'->>'id') THEN 'needs_refresh' ELSE 'current' END
  WHEN p.hours IS NULL THEN 'unknown'
  WHEN (r.data->>'observed_at')::timestamptz + make_interval(hours=>p.hours) <= $2 THEN 'needs_refresh' ELSE 'current' END AS freshness,
 ARRAY(SELECT DISTINCT requirement_id FROM rule_evidence_links WHERE service_id=$1 AND evidence_id=r.id AND evidence_revision=r.revision UNION SELECT r.requirement_id WHERE r.requirement_id<>'') AS linked
  FROM latest r LEFT JOIN policies p ON p.requirement_id=r.requirement_id)
 SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at,freshness,linked FROM classified
 WHERE ($3='' OR data->>'description' ILIKE '%'||$3||'%' OR data->>'source' ILIKE '%'||$3||'%')
  AND ($4='' OR data->>'source'=$4 OR data#>>'{document,source,id}'=$4) AND ($5='' OR freshness=$5)
 AND ($6='' OR $6=ANY(linked)) AND ($7='' OR data->>'verification'=$7)
 ORDER BY created_at DESC,id LIMIT 26 OFFSET $8`, service, now, f.Search, f.Source, f.Freshness, f.Requirement, f.Type, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EvidenceListItem{}
	for rows.Next() {
		var v EvidenceListItem
		if err = rows.Scan(&v.ServiceID, &v.Kind, &v.ID, &v.Revision, &v.RequirementID, &v.ActorID, &v.Data, &v.CreatedAt, &v.Freshness, &v.LinkedRequirements); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ArtifactRevision(ctx context.Context, a auth.Actor, service string, ref Ref) ([]byte, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	r, err := reference(ctx, s.query(), service, "evidence", ref)
	if err != nil {
		return nil, err
	}
	v, err := Decode[Evidence](r)
	if err != nil {
		return nil, err
	}
	if v.ArtifactKey == "" || s.Artifacts == nil {
		return nil, ErrNotFound
	}
	return s.Artifacts.Get(ctx, v.ArtifactKey)
}
