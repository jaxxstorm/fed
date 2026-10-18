package connections

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jaxxstorm/fed/internal/logging"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type RefreshSourceInput struct {
	ServiceID        string `json:"service_id"`
	SourceID         string `json:"source_id"`
	ExpectedRevision int    `json:"expected_revision"`
	Generation       int64  `json:"generation"`
	ScopeRevision    int    `json:"scope_revision"`
	IdempotencyKey   string `json:"idempotency_key"`
}

func (s *Service) RefreshSource(ctx context.Context, a auth.Actor, in RefreshSourceInput) (assessment.Record, error) {
	if in.ExpectedRevision < 1 || in.ScopeRevision < 1 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || s.Artifacts == nil {
		return assessment.Record{}, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "check"); err != nil {
		return assessment.Record{}, err
	}
	request, _ := json.Marshal(in)
	digest := storage.Hash(request)
	readReceipt := func(q interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}) (assessment.Record, bool, error) {
		var hash string
		var raw []byte
		err := q.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='source.refresh' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&hash, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return assessment.Record{}, false, nil
		}
		if err != nil {
			return assessment.Record{}, false, err
		}
		if hash != digest {
			return assessment.Record{}, true, ErrConflict
		}
		var out assessment.Record
		err = json.Unmarshal(raw, &out)
		return out, true, err
	}
	if out, found, err := readReceipt(s.DB); found || err != nil {
		return out, err
	}
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT data FROM records WHERE service_id=$1 AND kind='evidence_source' AND id=$2 ORDER BY revision DESC LIMIT 1`, in.ServiceID, in.SourceID).Scan(&raw)
	if err != nil {
		return assessment.Record{}, err
	}
	var source assessment.EvidenceSource
	if json.Unmarshal(raw, &source) != nil {
		return assessment.Record{}, ErrInvalid
	}
	access, bound, err := s.CollectorAccess(ctx, a, in.ServiceID, source.Type, source.Locator)
	if err != nil {
		return assessment.Record{}, err
	}
	if !bound || access.SourceID != in.SourceID || access.SourceRevision != in.ExpectedRevision || access.Connection.Generation != in.Generation {
		return assessment.Record{}, ErrConflict
	}
	adapter, err := s.Registry.Adapter(access.Connection.Provider)
	if err != nil {
		return assessment.Record{}, err
	}
	resource := DiscoveredResource{Resource: access.Resource, Name: source.Name, Locator: source.Locator}
	if source.Type == "kubernetes" {
		resource.Name = source.Namespace
	}
	call, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var snapshot Snapshot
	if access.Connection.Provider == GitHub {
		p, ok := adapter.(interface {
			ReadPinned(context.Context, Connection, Credentials, DiscoveredResource, string, string) (Snapshot, error)
		})
		if !ok || len(source.Paths) == 0 {
			return assessment.Record{}, ErrInvalid
		}
		snapshot, err = repositorySnapshot(source.Commit, source.Paths, func(path string) (Snapshot, error) {
			return p.ReadPinned(call, access.Connection, access.Credentials, resource, source.Commit, path)
		})
	} else {
		snapshot, err = adapter.Read(call, access.Connection, access.Credentials, resource)
	}
	if err != nil {
		return assessment.Record{}, err
	}
	if len(snapshot.Content) > MaxResponseBytes {
		return assessment.Record{}, ErrInvalid
	}
	key, err := s.Artifacts.Put(ctx, snapshot.Content)
	if err != nil {
		return assessment.Record{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return assessment.Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "check"); err != nil {
		return assessment.Record{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return assessment.Record{}, err
	}
	if out, found, err := readReceipt(tx); found || err != nil {
		return out, err
	}
	if err = CheckCollectionTx(ctx, tx, in.ServiceID, access); err != nil {
		return assessment.Record{}, err
	}
	version := storage.Hash(snapshot.Content)
	if source.Type == "repository" {
		version = source.Commit
	}
	out, err := assessment.StageProviderSnapshotTx(ctx, tx, a, in.ServiceID, assessment.Ref{ID: in.SourceID, Revision: in.ExpectedRevision}, in.ScopeRevision, snapshot.Content, snapshot.MediaType, version, snapshot.ETag, key, access.Connection.ID, access.Resource.Tenant, access.Resource.ID, "provider."+string(access.Connection.Provider), snapshot.ObservedAt)
	if err != nil {
		return out, err
	}
	b, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'source.refresh',$3,$4,$5)`, a.ID, in.ServiceID, in.IdempotencyKey, digest, b)
	if err != nil {
		return out, err
	}
	return out, logging.Commit(ctx, tx, logging.Fields{Event: "connection.evidence.refreshed", Component: "connections", ServiceID: in.ServiceID, ActorID: a.ID})
}

// Explicitly selected files are retained together so a partial read cannot be
// mistaken for a complete snapshot. []byte encodes losslessly as base64 in JSON.
func repositorySnapshot(commit string, paths []string, read func(string) (Snapshot, error)) (Snapshot, error) {
	if len(paths) == 1 {
		return read(paths[0])
	}
	if len(paths) == 0 {
		return Snapshot{}, ErrInvalid
	}
	type file struct {
		Path        string    `json:"path"`
		Content     []byte    `json:"content_base64"`
		ContentHash string    `json:"content_hash"`
		MediaType   string    `json:"media_type"`
		ETag        string    `json:"etag"`
		ObservedAt  time.Time `json:"observed_at"`
	}
	files := make([]file, 0, len(paths))
	var observed time.Time
	total := 0
	for _, path := range paths {
		snapshot, err := read(path)
		if err != nil {
			return Snapshot{}, err
		}
		total += len(snapshot.Content)
		if total > MaxResponseBytes {
			return Snapshot{}, ErrInvalid
		}
		if observed.IsZero() || snapshot.ObservedAt.Before(observed) {
			observed = snapshot.ObservedAt
		}
		files = append(files, file{path, snapshot.Content, storage.Hash(snapshot.Content), snapshot.MediaType, snapshot.ETag, snapshot.ObservedAt})
	}
	content, err := json.Marshal(struct {
		Format string `json:"format"`
		Commit string `json:"commit"`
		Files  []file `json:"files"`
	}{"repository-files/v1", commit, files})
	if err != nil {
		return Snapshot{}, err
	}
	if len(content) > MaxResponseBytes || len(content) > assessment.MaxAgentArtifactBytes {
		return Snapshot{}, ErrInvalid
	}
	return Snapshot{Content: content, MediaType: "application/json", ObservedAt: observed}, nil
}
