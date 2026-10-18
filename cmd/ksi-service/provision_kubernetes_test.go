package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/config"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestProvisionKubernetesSelectsAndReplays(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "https://issuer.test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('svc',$1,'Test');`, a.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('svc','scope','scope',1,$1,'{"description":"Test scope","accounts":[{"id":"123456789012","regions":["us-east-1"]}]}')`, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/namespaces/kube-system" {
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system","uid":"35187fad-51b6-4f55-bfcf-19347ae92a92"}}`))
			return
		}
		if r.URL.Path == "/api/v1/namespaces" {
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"NamespaceList","items":[{"metadata":{"name":"kube-system","uid":"35187fad-51b6-4f55-bfcf-19347ae92a92"}}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	kubeconfig, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "Config", "current-context": "c", "clusters": []any{map[string]any{"name": "c", "cluster": map[string]any{"server": server.URL, "certificate-authority-data": base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))}}}, "contexts": []any{map[string]any{"name": "c", "context": map[string]string{"cluster": "c", "user": "u"}}}, "users": []any{map[string]any{"name": "u", "user": map[string]string{"token": "test-token"}}}})
	input, _ := json.Marshal([]provisionedCluster{{ServiceID: "svc", Name: "Cluster", ClusterID: "35187fad-51b6-4f55-bfcf-19347ae92a92", Server: server.URL, Kubeconfig: string(kubeconfig), SelectCluster: true}})
	allow, _ := json.Marshal([]string{server.URL})
	keys, _ := json.Marshal(map[string]string{"v1": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	t.Setenv("KSI_PROVISION_KUBERNETES", string(input))
	t.Setenv("KSI_KUBERNETES_ALLOWED_SERVERS", string(allow))
	t.Setenv("KSI_CONNECTION_KEYS", string(keys))
	t.Setenv("KSI_CONNECTION_KEY_ID", "v1")
	for range 2 {
		if err := provisionKubernetes(ctx, db, config.Config{OIDCIssuer: "https://issuer.test", AdminSubject: "admin"}); err != nil {
			t.Fatal(err)
		}
	}
	var revision, connections, sources int
	var scope string
	if err = db.QueryRow(ctx, `SELECT revision,data::text FROM records WHERE kind='scope' ORDER BY revision DESC LIMIT 1`).Scan(&revision, &scope); err != nil {
		t.Fatal(err)
	}
	if revision != 2 {
		t.Fatal("scope changed on replay", revision)
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(scope), &parsed) != nil || len(parsed["accounts"].([]any)) != 1 || len(parsed["kubernetes"].([]any)) != 1 {
		t.Fatal("scope was not preserved and extended", scope)
	}
	if err = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM evidence_connections),(SELECT count(*) FROM connection_source_bindings)`).Scan(&connections, &sources); err != nil || connections != 1 || sources != 1 {
		t.Fatal("provisioning duplicated records", connections, sources, err)
	}
}
