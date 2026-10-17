package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
)

func (s *Store) SubmitEvidenceArtifact(ctx context.Context, a auth.Actor, in ArtifactSubmission) (Record, error) {
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "edit"); err != nil {
		return Record{}, err
	}
	if len(in.Content) > MaxAgentArtifactBytes {
		return Record{}, observation.ErrTooLarge
	}
	now := time.Now().UTC()
	if len(in.Content) == 0 || !utf8.ValidString(in.Content) || strings.TrimSpace(in.Description) == "" || len(in.Description) > 4096 || in.Method == "" || len(in.Method) > 1024 || in.SourceVersion == "" || len(in.SourceVersion) > 512 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || in.ObservedAt.IsZero() || in.RetrievedAt.IsZero() || in.ObservedAt.After(in.RetrievedAt) || in.RetrievedAt.After(now.Add(time.Minute)) || len(in.Limitations) > 100 {
		return Record{}, ErrInvalid
	}
	if in.MediaType != "text/plain" && in.MediaType != "text/markdown" && in.MediaType != "application/json" {
		return Record{}, ErrInvalid
	}
	if in.MediaType == "application/json" && !json.Valid([]byte(in.Content)) {
		return Record{}, ErrInvalid
	}
	if s.Artifacts == nil {
		return Record{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "edit"); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return Record{}, err
	}
	digest := storage.Hash(data(in))
	var old Record
	if found, err := readOperation(ctx, tx, a, in.ServiceID, "submit_evidence_artifact", in.IdempotencyKey, digest, &old); found || err != nil {
		return old, err
	}
	sr, err := reference(ctx, tx, in.ServiceID, "evidence_source", in.Source)
	if err != nil {
		return Record{}, err
	}
	source, err := Decode[EvidenceSource](sr)
	if err != nil {
		return Record{}, err
	}
	sc, err := reference(ctx, tx, in.ServiceID, "scope", Ref{"scope", in.ScopeRevision})
	if err != nil {
		return Record{}, err
	}
	scope, err := Decode[Scope](sc)
	if err != nil {
		return Record{}, err
	}
	if !source.Enabled || !sourceInScope(source, scope) || !locatorWithin(source, in.Locator) || (source.Type == "repository" && in.SourceVersion != source.Commit) {
		return Record{}, observation.ErrScope
	}
	var inactive bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2 AND lifecycle<>'enabled')`, in.ServiceID, in.Source.ID).Scan(&inactive); err != nil {
		return Record{}, err
	}
	if inactive {
		return Record{}, observation.ErrScope
	}
	key, err := s.Artifacts.Put(ctx, []byte(in.Content))
	if err != nil {
		return Record{}, err
	}
	ev := Evidence{Source: "agent", Description: in.Description, ScopeRevision: in.ScopeRevision, ObservedAt: in.ObservedAt, CollectedAt: now, Verification: "agent_observation", ArtifactKey: key, Limitations: in.Limitations, Document: &DocumentaryProvenance{Source: in.Source, Locator: in.Locator, SourceVersion: in.SourceVersion, MediaType: in.MediaType, RetrievedAt: in.RetrievedAt, Method: in.Method, FreshnessHours: source.FreshnessHours}}
	r, err := insertRecord(ctx, tx, a, record(in.ServiceID, "evidence", uuid.NewString(), "", ev), 1)
	if err != nil {
		return Record{}, err
	}
	if err = saveOperation(ctx, tx, a, in.ServiceID, "submit_evidence_artifact", in.IdempotencyKey, digest, r); err != nil {
		return Record{}, err
	}
	return r, tx.Commit(ctx)
}

func insertRecord(ctx context.Context, tx pgx.Tx, a auth.Actor, r Record, revision int) (Record, error) {
	r.Revision = revision
	r.ActorID = a.ID
	err := tx.QueryRow(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, r.ServiceID, r.Kind, r.ID, r.Revision, r.RequirementID, r.ActorID, r.Data).Scan(&r.CreatedAt)
	if err == nil {
		err = audit(ctx, tx, a, r.ServiceID, r.Kind+".saved", r.ID, r.Revision)
	}
	return r, err
}

func readOperation(ctx context.Context, tx pgx.Tx, a auth.Actor, service, operation, key, digest string, out any) (bool, error) {
	var prior string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation=$3 AND key=$4`, a.ID, service, operation, key).Scan(&prior, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if prior != digest {
		return true, ErrConflict
	}
	return true, json.Unmarshal(raw, out)
}

func saveOperation(ctx context.Context, tx pgx.Tx, a auth.Actor, service, operation, key, digest string, out any) error {
	_, err := tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, service, operation, key, digest, data(out))
	return err
}
