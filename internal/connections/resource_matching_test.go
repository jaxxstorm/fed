package connections

import (
	"context"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
)

func TestDiscoveryMatchesImportedSourceAndPreservesRenamesAndRetirement(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	store := assessment.Store{DB: s.DB}
	if _, err := store.SaveScope(ctx, a, "svc", 0, assessment.Scope{Description: "Repositories", Repositories: []assessment.Repository{{Name: "example/one", Commit: "1111111111111111111111111111111111111111", Paths: []string{"."}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	var sourceID string
	var original []byte
	if err := s.DB.QueryRow(ctx, `SELECT id,data FROM records WHERE kind='evidence_source'`).Scan(&sourceID, &original); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportSources(ctx, a, "svc", EnvironmentBindings{}); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, a, "svc", false)
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	connection := list[0].Connection.ID
	discover := func(connection, key, name string) {
		t.Helper()
		session, err := s.StartAgentDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: connection, ExpectedRevision: 1, Generation: 1, IdempotencyKey: key})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.SubmitAgentDiscovery(ctx, a, DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "gh api", Complete: true, Coverage: []KindCoverage{{Kind: "repository", Complete: true, Authoritative: true}}, Resources: []DiscoveredResource{{Resource: ResourceKey{Provider: GitHub, Tenant: "example", Kind: "repository", ID: "123"}, Name: name, Locator: "https://github.com/" + name}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	discover(connection, "initial", "example/one")
	discover(connection, "rename", "example/renamed")
	page, err := s.ListInventory(ctx, a, InventoryQuery{ServiceID: "svc", ConnectionID: connection})
	if err != nil || len(page.Resources) != 1 || page.Resources[0].SourceID != sourceID || page.Resources[0].Name != "example/renamed" {
		t.Fatal("rename lost source identity", err)
	}
	var current []byte
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT data FROM records WHERE kind='evidence_source'`).Scan(&current); err != nil || string(current) != string(original) {
		t.Fatal("discovery changed evidence source revision")
	}
	other, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "other", Provider: GitHub, Name: "Other access", Mode: AgentManaged, Configuration: Configuration{AccountID: "example"}})
	if err != nil {
		t.Fatal(err)
	}
	discover(other.Connection.ID, "other-scan", "example/renamed")
	page, err = s.ListInventory(ctx, a, InventoryQuery{ServiceID: "svc", ConnectionID: other.Connection.ID})
	if err != nil || page.Resources[0].BoundConnectionID != connection {
		t.Fatal("discovery silently rebound source")
	}
	if _, err = s.ChangeSource(ctx, a, SourceOperationInput{ServiceID: "svc", SourceID: sourceID, ExpectedRevision: 1, ExpectedBindingRevision: 2, IdempotencyKey: "retire"}, "retire"); err != nil {
		t.Fatal(err)
	}
	discover(connection, "after-retire", "example/renamed")
	page, err = s.ListInventory(ctx, a, InventoryQuery{ServiceID: "svc", ConnectionID: connection, Selection: "retired"})
	if err != nil || len(page.Resources) != 1 || page.Resources[0].SourceLifecycle != Retired {
		t.Fatal("retirement was lost")
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind='evidence_source'`).Scan(&count); err != nil || count != 2 {
		t.Fatal("rediscovery revised retired source")
	}
}
