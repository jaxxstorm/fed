package checks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
	"k8s.io/client-go/rest"
)

func TestKubernetesBoundReadOnlyCollection(t *testing.T) {
	var endpoint string
	var ca []byte
	var requests atomic.Int32
	var policiesPresent atomic.Bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			t.Errorf("mutation: %s", r.Method)
		}
		switch r.URL.Path {
		case "/clusters/prod":
			json.NewEncoder(w).Encode(map[string]any{"cluster": map[string]any{"endpoint": endpoint, "certificateAuthority": map[string]string{"data": base64.StdEncoding.EncodeToString(ca)}}})
		case "/api/v1/namespaces":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"NamespaceList","items":[{"metadata":{"name":"production"}}]}`)
		case "/apis/apps/v1/namespaces/kube-system/daemonsets/aws-node":
			w.WriteHeader(403)
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`)
		case "/apis/networking.k8s.io/v1/namespaces/production/networkpolicies":
			if policiesPresent.Load() {
				fmt.Fprint(w, `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","items":[{"metadata":{"name":"isolate"},"spec":{"podSelector":{"matchLabels":{"app":"api"}},"policyTypes":["Ingress","Egress"]}}]}`)
				return
			}
			fmt.Fprint(w, `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","items":[]}`)
		case "/api/v1/namespaces/production/pods":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"api","namespace":"production","uid":"uid","labels":{"app":"api"}}}]}`)
		default:
			t.Errorf("unexpected resource, possibly secret: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	endpoint = srv.URL
	ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	t.Setenv("KSI_KUBERNETES_CONTEXTS", `{"123456789012/us-east-1/prod":"test-context"}`)
	k := Kubernetes{AWSConfig: func(ctx context.Context, a assessment.Account, region string) (aws.Config, error) {
		if region != "us-east-1" {
			t.Fatal("region expanded")
		}
		return aws.Config{Region: region, BaseEndpoint: aws.String(srv.URL), HTTPClient: srv.Client(), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}, nil
	}, ClientConfig: func(name string) (*rest.Config, error) {
		if name != "test-context" {
			t.Fatal("wrong context")
		}
		return &rest.Config{Host: srv.URL, TLSClientConfig: rest.TLSClientConfig{CAData: ca}}, nil
	}}
	scope := assessment.Scope{Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}, Clusters: []string{"prod"}}}}
	result, err := k.Collect(context.Background(), scope, assessment.Measure{})
	if err != nil || len(result.Observations) != 1 || result.Outcome != "fail" {
		t.Fatalf("missing isolation: %+v %v", result, err)
	}
	if len(result.Observations[0].Evaluations) != 3 || result.Observations[0].Evaluations[2].Outcome != "unknown" {
		t.Fatal("enforcement overstated")
	}
	policiesPresent.Store(true)
	result, err = k.Collect(context.Background(), scope, assessment.Measure{})
	if err != nil || result.Outcome != "unknown" {
		t.Fatal("policy existence was treated as enforcement", err)
	}
	result, err = k.Collect(context.Background(), scope, assessment.Measure{Policy: map[string]json.RawMessage{"namespace_exclusions": json.RawMessage(`["production"]`)}})
	if err != nil || len(result.Observations) != 1 || result.Observations[0].Outcome != "unknown" {
		t.Fatal("namespace exclusion not recorded", err)
	}
	before := requests.Load()
	k.ClientConfig = func(string) (*rest.Config, error) {
		return &rest.Config{Host: "https://wrong-cluster", TLSClientConfig: rest.TLSClientConfig{CAData: ca}}, nil
	}
	result, err = k.Collect(context.Background(), scope, assessment.Measure{})
	if err != nil || result.Observations[0].Error != "cluster_binding_identity_mismatch" || requests.Load() != before+1 {
		t.Fatal("wrong cluster inspected", err)
	}
}
