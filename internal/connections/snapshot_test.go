package connections

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestProviderSnapshotImmutableProvenanceReplayAndDisconnectFence(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Artifacts = storage.Files{Directory: t.TempDir()}
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: Workspace, Name: "Directory", Mode: ServerManaged, Configuration: Configuration{AccountID: "C123"}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = s.Credentials.PutTx(ctx, tx, a, "svc", d.Connection.ID, 1, NewCredentials(map[string]string{"access_token": "private"})); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	session, err := s.StartAgentDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "discover"})
	if err != nil {
		t.Fatal(err)
	}
	resource := DiscoveredResource{Resource: ResourceKey{Provider: Workspace, Tenant: "C123", Kind: "group", ID: "g1"}, Name: "Security", Locator: "google-workspace://C123/group/g1"}
	if _, err = s.SubmitAgentDiscovery(ctx, a, DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", Method: "directory", ObservedAt: time.Now().UTC(), Resources: []DiscoveredResource{resource}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectSources(ctx, a, SelectionInput{ServiceID: "svc", IdempotencyKey: "select", AddToScope: true, Sources: []SourceConfigurationInput{{ServiceID: "svc", SourceID: "group", ConnectionID: d.Connection.ID, Generation: 1, Resource: resource.Resource, Source: assessment.EvidenceSource{Type: "google_workspace", Name: resource.Name, Locator: resource.Locator, Enabled: true, FreshnessHours: 24}}}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	disconnect := false
	s.Registry, _ = NewRegistry(map[Provider]Adapter{Workspace: testAdapter{read: func(context.Context, Connection, Credentials, DiscoveredResource) (Snapshot, error) {
		calls++
		if disconnect {
			if _, err := s.ChangeLifecycle(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "disconnect"}, "disconnect"); err != nil {
				t.Fatal(err)
			}
		}
		return Snapshot{Content: []byte(`{"id":"g1","name":"Security"}`), MediaType: "application/json", ObservedAt: time.Now().UTC(), ETag: "version1"}, nil
	}}})
	in := RefreshSourceInput{ServiceID: "svc", SourceID: "group", ExpectedRevision: 1, ScopeRevision: 1, Generation: 1, IdempotencyKey: "refresh"}
	record, err := s.RefreshSource(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := assessment.Decode[assessment.Evidence](record)
	if err != nil || evidence.Verification != "collector_observation" || evidence.Document.Source.ID != "group" || evidence.Document.ConnectionID != d.Connection.ID || evidence.Document.ContentHash == "" || evidence.Document.ETag != "version1" {
		t.Fatalf("snapshot provenance: %+v %v", evidence, err)
	}
	replay, err := s.RefreshSource(ctx, a, in)
	if err != nil || replay.ID != record.ID || calls != 1 {
		t.Fatal("snapshot retry repeated provider work")
	}
	disconnect = true
	in.IdempotencyKey = "late-refresh"
	if _, err = s.RefreshSource(ctx, a, in); !errors.Is(err, assessment.ErrConflict) {
		t.Fatal("late snapshot committed", err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind='evidence'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("late snapshot changed evidence history")
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind IN ('review','agent_result')`).Scan(&count); err != nil || count != 0 {
		t.Fatal("snapshot changed review or assessment")
	}
}
