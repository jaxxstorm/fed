package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func verifySetupJourney(t *testing.T, provider connections.Provider, adapter connections.Adapter, installation string) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Guided setup")
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{provider: adapter})
	keys, _ := connections.NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	s := connections.Service{DB: db, Registry: registry, Artifacts: storage.Files{Directory: t.TempDir()}, Credentials: connections.CredentialStore{DB: db, Keys: keys}}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	setup, err := s.BeginSetup(ctx, a, connections.BeginSetupInput{ServiceID: service.ID, Provider: provider, IdempotencyKey: "begin", Input: connections.SetupInput{Name: "Provider"}})
	if err != nil {
		t.Fatal(err)
	}
	op := connections.SetupOperationInput{ServiceID: service.ID, SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}
	setup, err = s.UpdateSetup(ctx, a, op, setup.Input)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = setup.Revision
	stage := "oauth"
	if provider == connections.GitHub {
		stage = "installation"
	}
	redirect, err := s.SetupAuthorizationURL(ctx, a, op, "browser", "https://app.example", stage)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(redirect)
	callback := "https://app.example/connections/callback/" + string(provider)
	if stage == "installation" {
		claim, err := s.ClaimSetupAuthorization(ctx, a, provider, u.Query().Get("state"), "browser", "https://app.example/connections/setup/github")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.ReturnSetupInstallation(ctx, a, claim, "browser", installation); err != nil {
			t.Fatal(err)
		}
		op.ExpectedRevision = claim.Setup.Revision
		redirect, err = s.SetupAuthorizationURL(ctx, a, op, "browser", "https://app.example", "oauth")
		if err != nil {
			t.Fatal(err)
		}
		u, _ = url.Parse(redirect)
	}
	claim, err := s.ClaimSetupAuthorization(ctx, a, provider, u.Query().Get("state"), "browser", callback)
	if err != nil {
		t.Fatal(err)
	}
	setup, err = s.ExchangeSetupConsent(ctx, a, claim, "browser", "code")
	if err != nil {
		t.Fatal(err)
	}
	if setup.Step != connections.SetupConfirm || setup.ConnectionID != "" || setup.VerifiedAccountID == "" {
		t.Fatal("consent skipped confirmation", setup)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind='evidence'`, service.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("consent created evidence", err)
	}
	op.ExpectedRevision = setup.Revision
	op.IdempotencyKey = "complete"
	setup, err = s.CompleteSetup(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	in := connections.OperationInput{ServiceID: service.ID, ConnectionID: setup.ConnectionID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "discover"}
	session, err := s.StartServerDiscovery(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	cursor := ""
	for i := 0; i < 10; i++ {
		receipt, err := s.DiscoverServerPage(ctx, a, connections.ServerDiscoveryInput{ServiceID: service.ID, SessionID: session.ID, IdempotencyKey: string(rune('a' + i)), Cursor: cursor, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		cursor = receipt.NextCursor
		if cursor == "" {
			break
		}
	}
	page, err := s.ListInventory(ctx, a, connections.InventoryQuery{ServiceID: service.ID, ConnectionID: setup.ConnectionID})
	if err != nil || len(page.Resources) == 0 {
		t.Fatal("no inventory", err)
	}
	resource := page.Resources[0]
	source := assessment.EvidenceSource{Type: "google_workspace", Name: resource.Name, Locator: resource.Locator, Enabled: true, FreshnessHours: 24}
	if provider == connections.GitHub {
		source.Type = "repository"
		source.Repository = resource.Name
		source.Commit = "1111111111111111111111111111111111111111"
		source.Paths = []string{"ci.yaml"}
	}
	selection, err := s.SelectSources(ctx, a, connections.SelectionInput{ServiceID: service.ID, IdempotencyKey: "select", AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: service.ID, SourceID: "selected", ConnectionID: setup.ConnectionID, Generation: 1, Resource: resource.Resource, Source: source}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RefreshSource(ctx, a, connections.RefreshSourceInput{ServiceID: service.ID, SourceID: "selected", ExpectedRevision: 1, Generation: 1, ScopeRevision: selection.ScopeRevision, IdempotencyKey: "refresh"}); err != nil {
		t.Fatal(err)
	}
}
