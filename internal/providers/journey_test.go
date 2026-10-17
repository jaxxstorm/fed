package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

// Exercise the real provider adapter through authorization, durable discovery,
// explicit scope selection, immutable snapshot collection and access renewal.
func verifyOAuthJourney(t *testing.T, provider connections.Provider, adapter connections.Adapter, configuration connections.Configuration, grant connections.AuthorizedGrant) {
	t.Helper()
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Provider journey")
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{provider: adapter})
	keys, _ := connections.NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	s := connections.Service{DB: db, Registry: registry, Artifacts: storage.Files{Directory: t.TempDir()}, Credentials: connections.CredentialStore{DB: db, Keys: keys}}
	d, err := s.Configure(ctx, a, connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "configure", Provider: provider, Name: "Provider", Mode: connections.ServerManaged, Configuration: configuration})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	in := connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "authorize"}
	handoff, err := s.BeginAuthorization(ctx, a, in, "https://app.example")
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := s.BindAuthorizationBrowser(ctx, a, service.ID, handoff.OperationID, "browser")
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimAuthorization(ctx, a, provider, challenge.State, "browser", challenge.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteAuthorization(ctx, a, claim, "browser", grant.AccountID, grant.Credentials, grant.AccountHint); err != nil {
		t.Fatal(err)
	}
	in.Generation = 2
	in.IdempotencyKey = "test"
	if result, err := s.TestWithReceipt(ctx, a, in); err != nil || result.Access == nil || result.Access.State != connections.Verified {
		t.Fatalf("access test: %+v %v", result, err)
	}
	in.IdempotencyKey = "discover"
	session, err := s.StartServerDiscovery(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	cursor := ""
	for page := 0; page < 10; page++ {
		receipt, err := s.DiscoverServerPage(ctx, a, connections.ServerDiscoveryInput{ServiceID: service.ID, SessionID: session.ID, IdempotencyKey: "page-" + string(rune('a'+page)), Cursor: cursor, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		cursor = receipt.NextCursor
		if cursor == "" {
			break
		}
	}
	if cursor != "" {
		t.Fatal("discovery did not finish")
	}
	page, err := s.ListInventory(ctx, a, connections.InventoryQuery{ServiceID: service.ID, ConnectionID: d.Connection.ID})
	if err != nil || len(page.Resources) == 0 {
		t.Fatal("inventory empty", err)
	}
	resource := page.Resources[0]
	source := assessment.EvidenceSource{Type: "google_workspace", Name: resource.Name, Locator: resource.Locator, Enabled: true, FreshnessHours: 24}
	if provider == connections.GitHub {
		source.Type = "repository"
		source.Repository = resource.Name
		source.Commit = "1111111111111111111111111111111111111111"
		source.Paths = []string{"ci.yaml"}
	}
	selection, err := s.SelectSources(ctx, a, connections.SelectionInput{ServiceID: service.ID, IdempotencyKey: "select", AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: service.ID, SourceID: "selected", ConnectionID: d.Connection.ID, Generation: 2, Resource: resource.Resource, Source: source}}})
	if err != nil {
		t.Fatal(err)
	}
	refresh := connections.RefreshSourceInput{ServiceID: service.ID, SourceID: "selected", ExpectedRevision: 1, Generation: 2, ScopeRevision: selection.ScopeRevision, IdempotencyKey: "refresh"}
	evidence, err := s.RefreshSource(ctx, a, refresh)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := assessment.Decode[assessment.Evidence](evidence)
	if err != nil || ev.Document == nil || ev.Document.Source.Revision != 1 || ev.Document.ContentHash == "" || ev.Verification != "collector_observation" {
		t.Fatal("provenance incomplete", err)
	}
	in.IdempotencyKey = "renew"
	baseline, err := store.Get(ctx, a, service.ID, "evidence", evidence.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := s.Renew(ctx, a, in); err != nil || result.State != "partial" || result.Access == nil || result.Access.State != connections.NotConfigured || result.Access.Generation != 3 {
		t.Fatalf("renew: %+v %v", result, err)
	}
	current, err := s.Get(ctx, a, service.ID, d.Connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	in.Generation = current.Connection.Generation
	in.IdempotencyKey = "test-renewed"
	if result, err := s.TestWithReceipt(ctx, a, in); err != nil || result.State != "succeeded" {
		t.Fatalf("renewed access test: %+v %v", result, err)
	}
	in.IdempotencyKey = "disconnect"
	if _, err = s.ChangeLifecycle(ctx, a, in, "disconnect"); err != nil {
		t.Fatal(err)
	}
	refresh.IdempotencyKey = "disconnected"
	if _, err = s.RefreshSource(ctx, a, refresh); err == nil {
		t.Fatal("disconnected source collected")
	}
	retained, err := store.Get(ctx, a, service.ID, "evidence", evidence.ID)
	if err != nil || !bytes.Equal(retained.Data, baseline.Data) {
		t.Fatal("renew/disconnect revised evidence")
	}
}
