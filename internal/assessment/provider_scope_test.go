package assessment

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jaxxstorm/fed/internal/observation"
)

func TestProviderScopePreservesLegacyAndEnforcesNamespaceUID(t *testing.T) {
	const old = `{"accounts":[{"id":"123456789012","regions":["us-east-1"],"clusters":["prod"]}],"repositories":[],"description":"Legacy"}`
	var scope Scope
	if err := json.Unmarshal([]byte(old), &scope); err != nil {
		t.Fatal(err)
	}
	if err := validateScope(scope); err != nil {
		t.Fatal(err)
	}
	if !sourceInScope(EvidenceSource{Type: "kubernetes", AccountID: "123456789012", Region: "us-east-1", Cluster: "prod", Locator: "123456789012/us-east-1"}, scope) {
		t.Fatal("legacy cluster scope changed")
	}
	uid := "11111111-1111-4111-8111-111111111111"
	scope.Kubernetes = []KubernetesScope{{ClusterID: "standalone", Server: "https://cluster.example", Namespaces: []NamespaceScope{{Name: "production", UID: uid}}}}
	if err := validateScope(scope); err != nil {
		t.Fatal(err)
	}
	target := observation.Target{Kind: "kubernetes", ClusterID: "standalone", Namespace: "production", NamespaceUID: uid, ResourceKind: "pod", ResourceID: "pod-one", UID: "pod-uid"}
	if err := validateObservationTarget(scope, target); err != nil {
		t.Fatal(err)
	}
	target.NamespaceUID = "22222222-2222-4222-8222-222222222222"
	if err := validateObservationTarget(scope, target); !errors.Is(err, observation.ErrScope) {
		t.Fatal("recreated namespace inherited scope")
	}
	target.NamespaceUID = uid
	target.Namespace = "unrelated"
	if err := validateObservationTarget(scope, target); !errors.Is(err, observation.ErrScope) {
		t.Fatal("unrelated namespace accepted")
	}
	scope.Workspace = []WorkspaceScope{{CustomerID: "customer", GroupIDs: []string{"group-one"}}}
	source := EvidenceSource{Type: "google_workspace", ProviderTenant: "customer", ResourceKind: "group", ResourceID: "group-one", Locator: "google-workspace://customer/group/group-one"}
	if !sourceInScope(source, scope) {
		t.Fatal("selected Workspace group rejected")
	}
	source.ProviderTenant = "different"
	if sourceInScope(source, scope) {
		t.Fatal("cross-customer source accepted")
	}
}
