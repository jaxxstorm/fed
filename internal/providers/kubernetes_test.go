package providers

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/connections"
)

func kubeconfig(server, ca, user string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: selected
clusters:
- name: cluster
  cluster:
    server: %s
    certificate-authority-data: %s
contexts:
- name: selected
  context:
    cluster: cluster
    user: reader
users:
- name: reader
  user:
%s
`, server, ca, user)
}

func TestUploadedKubeconfigRejectsExecutableAndFileCredentials(t *testing.T) {
	for _, user := range []string{
		"    exec:\n      command: malicious\n      apiVersion: client.authentication.k8s.io/v1",
		"    tokenFile: /etc/secret",
		"    client-certificate: /etc/cert\n    client-key: /etc/key",
		"    token: token\n    as: admin",
		"    auth-provider:\n      name: malicious",
	} {
		if _, err := ValidateUploadedKubeconfig([]byte(kubeconfig("https://cluster.example", "", user))); err == nil {
			t.Fatalf("accepted unsafe authentication: %s", user)
		}
	}
	raw := kubeconfig("https://cluster.example", "", "    token: token")
	if _, err := ValidateUploadedKubeconfig([]byte(raw)); err != nil {
		t.Fatal("embedded token rejected", err)
	}
	raw = strings.Replace(raw, "certificate-authority-data:", "insecure-skip-tls-verify: true\n    certificate-authority-data:", 1)
	if _, err := ValidateUploadedKubeconfig([]byte(raw)); err == nil {
		t.Fatal("TLS bypass accepted")
	}
}

func TestKubernetesIdentityPaginationAndNamespaceRecreation(t *testing.T) {
	const cluster = "11111111-1111-4111-8111-111111111111"
	const namespace = "22222222-2222-4222-8222-222222222222"
	uid := namespace
	denied := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer token" {
			t.Error("non-read operation or missing credentials")
		}
		switch r.URL.Path {
		case "/api/v1/namespaces/kube-system":
			fmt.Fprintf(w, `{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"kube-system","uid":%q}}`, cluster)
		case "/api/v1/namespaces":
			if denied {
				w.WriteHeader(403)
				fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`)
				return
			}
			if r.URL.Query().Get("limit") != "1" {
				t.Error("namespace discovery is not bounded")
			}
			fmt.Fprintf(w, `{"kind":"NamespaceList","apiVersion":"v1","metadata":{},"items":[{"metadata":{"name":"production","uid":%q}}]}`, namespace)
		case "/api/v1/namespaces/production":
			fmt.Fprintf(w, `{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"production","uid":%q,"resourceVersion":"7"}}`, uid)
		default:
			t.Errorf("unexpected namespace/resource read %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	ca := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	k := connections.NewCredentials(map[string]string{"kubeconfig": kubeconfig(server.URL, ca, "    token: token")})
	c := connections.Connection{Provider: connections.Kubernetes, Name: "Cluster", Mode: connections.ServerManaged, Configuration: connections.Configuration{AccountID: cluster, ClusterID: cluster, Server: server.URL}}
	p := Kubernetes{AllowedServers: []string{server.URL}}
	ctx := context.Background()
	page, err := p.Discover(ctx, c, k, "", 1)
	if err != nil || len(page.Resources) != 1 || page.NextCursor == "" {
		t.Fatal("cluster page", err)
	}
	page, err = p.Discover(ctx, c, k, page.NextCursor, 1)
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Resource.ID != namespace {
		t.Fatal("namespace page", err)
	}
	resource := page.Resources[0]
	if _, err = p.Read(ctx, c, k, resource); err != nil {
		t.Fatal(err)
	}
	uid = "33333333-3333-4333-8333-333333333333"
	if _, err = p.Read(ctx, c, k, resource); err == nil {
		t.Fatal("recreated namespace accepted")
	}
	denied = true
	page, err = p.Discover(ctx, c, k, "", 2)
	if err != nil || len(page.Resources) != 1 || page.Coverage[1].ErrorCode != "insufficient_permissions" {
		t.Fatal("namespace denial discarded cluster result", err)
	}
	c.Configuration.KnownNamespaces = []string{"production"}
	c.Configuration.Server = server.URL
	page, err = p.Discover(ctx, c, k, "", 2)
	if err != nil || len(page.Resources) != 2 || page.Resources[1].Resource.ID != uid || page.Coverage[1].Authoritative {
		t.Fatal("known namespace fallback or UID recovery failed", err)
	}
	if _, err = p.Renew(ctx, c, k); err != nil {
		t.Fatal("valid credential retest failed", err)
	}
	badTLS := connections.NewCredentials(map[string]string{"kubeconfig": kubeconfig(server.URL, base64.StdEncoding.EncodeToString([]byte("invalid certificate")), "    token: token")})
	if _, err = p.Test(ctx, c, badTLS); err == nil {
		t.Fatal("invalid TLS CA accepted")
	}
	c.Configuration.Server = "https://unapproved.example"
	if _, err = p.Configuration(c, k); err == nil {
		t.Fatal("unapproved endpoint accepted")
	}
}
