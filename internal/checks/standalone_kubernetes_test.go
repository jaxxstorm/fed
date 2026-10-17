package checks

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/providers"
	"github.com/jaxxstorm/fed/internal/testutil"
	"k8s.io/client-go/rest"
)

func TestStandaloneCollectionReadsOnlySelectedNamespace(t *testing.T) {
	const cluster = "11111111-1111-4111-8111-111111111111"
	const namespace = "22222222-2222-4222-8222-222222222222"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			t.Error("infrastructure write attempted")
		}
		switch r.URL.Path {
		case "/api/v1/namespaces/kube-system":
			fmt.Fprintf(w, `{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"kube-system","uid":%q}}`, cluster)
		case "/api/v1/namespaces/production":
			fmt.Fprintf(w, `{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"production","uid":%q}}`, namespace)
		case "/apis/networking.k8s.io/v1/namespaces/production/networkpolicies":
			fmt.Fprint(w, `{"kind":"NetworkPolicyList","apiVersion":"networking.k8s.io/v1","items":[{"metadata":{"name":"isolate"},"spec":{"podSelector":{},"policyTypes":["Ingress","Egress"]}}]}`)
		case "/api/v1/namespaces/production/pods":
			fmt.Fprint(w, `{"kind":"PodList","apiVersion":"v1","items":[{"metadata":{"name":"api","uid":"33333333-3333-4333-8333-333333333333"}}]}`)
		default:
			t.Errorf("unrelated namespace or cluster-wide read: %s", r.URL.Path)
			w.WriteHeader(403)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Standalone")
	if err != nil {
		t.Fatal(err)
	}
	provider := providers.Kubernetes{AllowedServers: []string{server.URL}, AllowedContexts: []string{"trusted"}, LoadContext: func(string) (*rest.Config, error) {
		return &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{CAData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}}, nil
	}}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{connections.Kubernetes: provider})
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	d, err := s.Configure(ctx, a, connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "create", Provider: connections.Kubernetes, Name: "Standalone", Mode: connections.ServerEnvironment, Configuration: connections.Configuration{AccountID: cluster, ClusterID: cluster, Server: server.URL, Context: "trusted"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.StartAgentDiscovery(ctx, a, connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "scan"})
	if err != nil {
		t.Fatal(err)
	}
	resource := connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.Kubernetes, Tenant: cluster, Kind: "namespace", ID: namespace}, Name: "production", Locator: "kubernetes://cluster/" + cluster + "/namespace/" + namespace}
	if _, err = s.SubmitAgentDiscovery(ctx, a, connections.DiscoverySubmission{ServiceID: service.ID, SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "kubectl", Resources: []connections.DiscoveredResource{resource}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectSources(ctx, a, connections.SelectionInput{ServiceID: service.ID, IdempotencyKey: "select", AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: service.ID, SourceID: "namespace", ConnectionID: d.Connection.ID, Generation: 1, Resource: resource.Resource, Source: assessment.EvidenceSource{Type: "kubernetes", Name: "Production", Locator: resource.Locator, ClusterID: cluster, Namespace: "production", NamespaceUID: namespace, Enabled: true, FreshnessHours: 24}}}}); err != nil {
		t.Fatal(err)
	}
	scopeRecord, err := store.Get(ctx, a, service.ID, "scope", "scope")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := assessment.Decode[assessment.Scope](scopeRecord)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, Connections: s}
	result, bindings, err := runner.collectConnected(ctx, a, service.ID, Kubernetes{}, scope, assessment.Measure{})
	if err != nil || len(result.Observations) != 1 || len(bindings) != 1 {
		t.Fatalf("collection: %+v %v", result, err)
	}
	observation := result.Observations[0]
	if observation.Target == nil || observation.Target.NamespaceUID != namespace || observation.Target.ClusterID != cluster || observation.Target.AccountID != "" || observation.Outcome == "pass" {
		t.Fatal("scope or enforcement result incorrect", observation)
	}
	if _, err = s.ChangeSource(ctx, a, connections.SourceOperationInput{ServiceID: service.ID, SourceID: "namespace", ExpectedRevision: 1, ExpectedBindingRevision: 1, IdempotencyKey: "retire"}, "retire"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = connections.CheckCollectionTx(ctx, tx, service.ID, bindings[0]); err == nil {
		t.Fatal("retirement failed to fence collected result")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	in := connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "renew"}
	if renewed, err := s.Renew(ctx, a, in); err != nil || renewed.Access == nil || renewed.Access.State != connections.AuthorizationRequired {
		t.Fatal("trusted context renewal guidance missing", err)
	}
	in.IdempotencyKey = "retest"
	if tested, err := s.TestWithReceipt(ctx, a, in); err != nil || tested.State != "succeeded" {
		t.Fatal("trusted context retest failed", err)
	}
	in.IdempotencyKey = "disconnect"
	if _, err = s.ChangeLifecycle(ctx, a, in, "disconnect"); err != nil {
		t.Fatal(err)
	}
	result, _, err = runner.collectConnected(ctx, a, service.ID, Kubernetes{}, scope, assessment.Measure{})
	if err != nil || len(result.Observations) != 1 || result.Observations[0].Error != "connection_inactive" {
		t.Fatal("retired/disconnected namespace collected", err)
	}
}
