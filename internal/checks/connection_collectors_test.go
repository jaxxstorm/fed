package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestBoundGitHubNeverFallsBackToGlobalCredentials(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Test")
	if err != nil {
		t.Fatal(err)
	}
	scope := assessment.Scope{Description: "Repositories", Repositories: []assessment.Repository{{Name: "example/repo", Commit: "1111111111111111111111111111111111111111", Paths: []string{"ci.yaml"}}}}
	if _, err = store.SaveScope(ctx, a, service.ID, 0, scope); err != nil {
		t.Fatal(err)
	}
	if err = store.ReconcileEvidenceSources(ctx, a, service.ID); err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(nil)
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","content":"cHVsdW1p","sha":"blob"}`))
	}))
	defer server.Close()
	collector := GitHub{Token: "global-secret", BaseURL: server.URL, Client: server.Client()}
	runner := &Runner{DB: db, Connections: s}
	if _, _, err = runner.collectConnected(ctx, a, service.ID, collector, scope, assessment.Measure{}); err != nil || calls != 1 {
		t.Fatal("explicit unbound legacy fallback failed", err)
	}
	if _, err = s.ImportSources(ctx, a, service.ID, connections.EnvironmentBindings{}); err != nil {
		t.Fatal(err)
	}
	result, _, err := runner.collectConnected(ctx, a, service.ID, collector, scope, assessment.Measure{})
	if err != nil || calls != 1 || len(result.Observations) != 1 || result.Observations[0].Outcome != "error" {
		t.Fatal("bound connection used global token", err)
	}
	list, err := s.List(ctx, a, service.ID, false)
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	c := list[0].Connection
	if _, err = s.ChangeLifecycle(ctx, a, connections.OperationInput{ServiceID: service.ID, ConnectionID: c.ID, Generation: c.Generation, ExpectedRevision: c.Revision, IdempotencyKey: "disconnect"}, "disconnect"); err != nil {
		t.Fatal(err)
	}
	result, _, err = runner.collectConnected(ctx, a, service.ID, collector, scope, assessment.Measure{})
	if err != nil || calls != 1 || result.Observations[0].Error != "connection_inactive" {
		t.Fatal("disconnected connection fell through", err)
	}
}
