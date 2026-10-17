package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/checks"
	"github.com/jaxxstorm/fed/internal/connections"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAgentConnectionDiscoverySelectionAndRetirement(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Connections")
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(nil)
	connectionService := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	server := httptest.NewServer(adapter.Handler(store, &checks.Runner{DB: db}, adapter.ConnectionOptions{Service: connectionService, PublicOrigin: "https://app.example"}))
	defer server.Close()
	token, err := auth.IssueToken(ctx, db, a, service.ID, []string{"read", "edit", "connect"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "connection-agent", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, input any) *sdk.CallToolResult {
		t.Helper()
		out, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: input})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	providers := unpack[[]connections.ProviderDefinition](t, call("list_connection_providers", adapter.ServiceInput{ServiceID: service.ID}))
	if len(providers) != 5 {
		t.Fatal("provider catalog missing")
	}
	d := unpack[connections.Detail](t, call("configure_evidence_connection", connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "create", Provider: connections.GitHub, Name: "Repositories", Mode: connections.AgentManaged, Configuration: connections.Configuration{AccountID: "example"}}))
	discovery := unpack[connections.DiscoverySession](t, call("discover_connection_sources", adapter.DiscoveryStartInput{OperationInput: connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "start"}, Execution: "agent"}))
	resource := connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.GitHub, Tenant: "example", Kind: "repository", ID: "123"}, Name: "example/repo", Locator: "https://github.com/example/repo"}
	unpack[connections.DiscoveryReceipt](t, call("submit_connection_discovery", connections.DiscoverySubmission{ServiceID: service.ID, SessionID: discovery.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "gh api", Resources: []connections.DiscoveredResource{resource}, Complete: true, Coverage: []connections.KindCoverage{{Kind: "repository", Complete: true, Authoritative: true}}}))
	selected := unpack[connections.SelectionReceipt](t, call("select_connection_sources", connections.SelectionInput{ServiceID: service.ID, IdempotencyKey: "select", AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: service.ID, SourceID: "repo", ConnectionID: d.Connection.ID, Generation: 1, Resource: resource.Resource, Source: assessment.EvidenceSource{Type: "repository", Name: "Repo", Repository: "example/repo", Commit: "1111111111111111111111111111111111111111", Locator: resource.Locator, Paths: []string{".github/workflows/ci.yaml"}, Enabled: true, FreshnessHours: 48}}}}))
	if selected.ScopeRevision != 1 || len(selected.Sources) != 1 {
		t.Fatal("agent selection failed")
	}
	page := unpack[connections.InventoryPage](t, call("list_connection_sources", connections.InventoryQuery{ServiceID: service.ID, ConnectionID: d.Connection.ID, Selection: "selected"}))
	if len(page.Resources) != 1 || page.Resources[0].ActorID == a.ID || page.Resources[0].Mode != connections.AgentManaged {
		t.Fatal("agent attribution lost")
	}
	unpack[assessment.Record](t, call("delete_evidence_source", connections.SourceOperationInput{ServiceID: service.ID, SourceID: "repo", ExpectedRevision: 1, ExpectedBindingRevision: 1, IdempotencyKey: "retire"}))
	legacy := unpack[struct {
		Items []assessment.Record `json:"items"`
	}](t, call("list_evidence_sources", adapter.ListInput{ServiceID: service.ID}))
	if len(legacy.Items) != 0 {
		t.Fatal("legacy listing resurrected retired source")
	}
	history := unpack[struct {
		Items []assessment.Record `json:"items"`
	}](t, call("list_evidence_sources", adapter.ListInput{ServiceID: service.ID, IncludeRetired: true}))
	if len(history.Items) != 1 || history.Items[0].ID != "repo" {
		t.Fatal("source history inaccessible")
	}
	browser := (&auth.Browser{DB: db, Dev: &a}).Protect((&web.App{Store: store, Connections: connectionService}).Handler())
	w := httptest.NewRecorder()
	browser.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1/api/ui/v1/services/"+service.ID+"/connections/"+d.Connection.ID, nil))
	var browserDetail connections.Detail
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &browserDetail) != nil || browserDetail.Connection.ID != d.Connection.ID || browserDetail.Connection.ActorID == a.ID {
		t.Fatal("browser did not expose agent-attributed connection")
	}
	if strings.Contains(w.Body.String(), token) || strings.Contains(w.Body.String(), "ciphertext") {
		t.Fatal("browser exposed credentials")
	}
	other, err := store.CreateService(ctx, a, "Other service")
	if err != nil {
		t.Fatal(err)
	}
	if result := call("get_evidence_connection", adapter.GetInput{ServiceID: other.ID, ID: d.Connection.ID}); !result.IsError {
		t.Fatal("service-scoped agent crossed service boundary")
	}
	agent, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	setup := unpack[connections.Setup](t, call("begin_provider_setup", connections.BeginSetupInput{ServiceID: service.ID, Provider: connections.Workspace, IdempotencyKey: "setup", Input: connections.SetupInput{Name: "Workspace"}}))
	handoff := unpack[connections.AuthorizationHandoff](t, call("authorize_provider_setup", connections.SetupOperationInput{ServiceID: service.ID, SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "handoff"}))
	u, err := url.Parse(handoff.URL)
	if err != nil || u.Scheme != "https" || u.Host != "app.example" {
		t.Fatal("handoff escaped configured origin", err)
	}
	w = httptest.NewRecorder()
	browser.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1/api/ui/v1/services/"+service.ID+"/connections/setups/"+setup.ID+"/status", nil))
	var browserSetup connections.Setup
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &browserSetup) != nil || browserSetup.ID != setup.ID || browserSetup.ActorID != agent.ID {
		t.Fatal("browser/MCP setup divergence", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), agent.TokenHash) || strings.Contains(w.Body.String(), "refresh_token") {
		t.Fatal("setup status leaked credentials")
	}
	if result := call("get_provider_setup", adapter.GetInput{ServiceID: other.ID, ID: setup.ID}); !result.IsError {
		t.Fatal("cross-service setup access")
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('setup-browser',$1,'csrf',now()+interval '1 hour')`, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = auth.RevokeToken(ctx, db, a, service.ID, agent.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err = connectionService.ClaimSetupHandoff(ctx, a, u.Query().Get("handoff"), "setup-browser"); err == nil {
		t.Fatal("revoked initiating agent completed handoff")
	}
	// Authorization is checked on each HTTP operation, not just MCP initialize.
	if _, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "list_evidence_connections", Arguments: adapter.ConnectionListInput{ServiceID: service.ID}}); err == nil {
		t.Fatal("revoked bearer retained transport access")
	}
}
