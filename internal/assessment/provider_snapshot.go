package assessment

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

// StageProviderSnapshotTx is an internal collector boundary. Transports only
// accept source IDs; content and provider provenance come from trusted adapters.
func StageProviderSnapshotTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service string, sourceRef Ref, scopeRevision int, content []byte, mediaType, version, etag, artifactKey, connection, tenant, resource, method string, observed time.Time) (Record, error) {
	if err := auth.Authorize(ctx, tx, a, service, "check"); err != nil {
		return Record{}, err
	}
	if len(content) == 0 || len(content) > MaxAgentArtifactBytes || !utf8.Valid(content) || version == "" || observed.IsZero() || observed.After(time.Now().Add(time.Minute)) || connection == "" {
		return Record{}, ErrInvalid
	}
	if mediaType != "application/json" && mediaType != "text/plain" {
		return Record{}, ErrInvalid
	}
	if mediaType == "application/json" && !json.Valid(content) {
		return Record{}, ErrInvalid
	}
	sr, err := reference(ctx, tx, service, "evidence_source", sourceRef)
	if err != nil {
		return Record{}, err
	}
	source, err := Decode[EvidenceSource](sr)
	if err != nil {
		return Record{}, err
	}
	sc, err := reference(ctx, tx, service, "scope", Ref{"scope", scopeRevision})
	if err != nil {
		return Record{}, err
	}
	scope, err := Decode[Scope](sc)
	if err != nil {
		return Record{}, err
	}
	if !source.Enabled || !sourceInScope(source, scope) {
		return Record{}, ErrInvalid
	}
	if source.Type == "repository" && version != source.Commit {
		return Record{}, ErrInvalid
	}
	if !strings.HasPrefix(method, "provider.") {
		return Record{}, ErrInvalid
	}
	now := time.Now().UTC()
	ev := Evidence{Source: "collector", Description: source.Name + " provider metadata snapshot", ScopeRevision: scopeRevision, ObservedAt: observed, CollectedAt: now, Verification: "collector_observation", ArtifactKey: artifactKey, Collector: method + "@1", Limitations: []string{"Point-in-time provider metadata; does not establish requirement completion or human approval."}, Document: &DocumentaryProvenance{Source: sourceRef, Locator: source.Locator, SourceVersion: version, MediaType: mediaType, RetrievedAt: now, Method: method, FreshnessHours: source.FreshnessHours, ConnectionID: connection, ProviderTenant: tenant, ResourceID: resource, ContentHash: storage.Hash(content), ETag: etag}}
	return insertRecord(ctx, tx, a, record(service, "evidence", uuid.NewString(), "", ev), 1)
}
