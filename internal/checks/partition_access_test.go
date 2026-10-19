package checks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
)

func TestPartitionAccessDoesNotClaimControlCoverage(t *testing.T) {
	r := Result{Observations: []Observation{partitionAccessObserved("123/us-east-1", "Enumeration completed"), {Scope: "resource", Outcome: "fail"}}}
	r.Summarize()
	if r.Expected != 1 || r.Observed != 1 || r.Outcome != "fail" {
		t.Fatalf("access marker hid finding or inflated coverage: %+v", r)
	}
	r.Observations = r.Observations[:1]
	r.Summarize()
	if r.Expected != 0 || r.Observed != 0 || r.Outcome != "unknown" {
		t.Fatalf("access marker claimed completion: %+v", r)
	}
}

func TestEndpointInventoryOnlyClearsPartitionErrorAfterCompleteEnumeration(t *testing.T) {
	for _, failSecondPage := range []bool{false, true} {
		t.Run(fmt.Sprint(failSecondPage), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/clusters":
					if r.URL.Query().Get("nextToken") == "next" {
						if failSecondPage {
							w.WriteHeader(403)
							fmt.Fprint(w, `{"message":"denied"}`)
						} else {
							fmt.Fprint(w, `{"clusters":[]}`)
						}
						return
					}
					fmt.Fprint(w, `{"clusters":["prod"],"nextToken":"next"}`)
				case "/clusters/prod":
					fmt.Fprint(w, `{"cluster":{"arn":"arn:aws:eks:us-east-1:123456789012:cluster/prod","resourcesVpcConfig":{"endpointPublicAccess":true,"publicAccessCidrs":["0.0.0.0/0"]}}}`)
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			cfg := aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1}
			rows := collectEndpoints(context.Background(), cfg, assessment.Account{ID: "123456789012"}, "us-east-1")
			access, failed, finding := false, false, false
			for _, row := range rows {
				access = access || accessObservation(row)
				failed = failed || row.Error != ""
				finding = finding || len(row.Evaluations) > 0 && row.Evaluations[0].Outcome == "fail"
			}
			if access == failSecondPage || failed != failSecondPage || !finding {
				t.Fatalf("partial inventory erased errors or findings: %+v", rows)
			}
		})
	}
}

func TestStandaloneEKSIdentityMatchRequiresClusterWideObservedSource(t *testing.T) {
	const account = "123456789012"
	endpoint := "https://kubernetes.example"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/clusters/prod" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cluster":{"arn":"arn:aws:eks:us-east-1:%s:cluster/prod","endpoint":%q}}`, account, endpoint)
	}))
	defer srv.Close()
	resolve := func(context.Context, assessment.Account, string) (aws.Config, error) {
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}, nil
	}
	scope := assessment.Scope{Kubernetes: []assessment.KubernetesScope{{ClusterID: "uid", Server: endpoint, ClusterWide: true}}}
	result := Result{Observations: []Observation{{Target: &observation.Target{Kind: "kubernetes", ClusterID: "uid"}, Outcome: "unknown"}}}
	check := func() bool {
		return standaloneObservedEKS(context.Background(), scope, result, assessment.Account{ID: account}, "us-east-1", "prod", resolve)
	}
	if !check() {
		t.Fatal("verified matching cluster was not reused")
	}
	scope.Kubernetes[0].ClusterWide = false
	if check() {
		t.Fatal("namespace-only source concealed cluster gap")
	}
	scope.Kubernetes[0].ClusterWide = true
	result.Observations[0].Error = "forbidden"
	if check() {
		t.Fatal("failed source concealed missing access")
	}
	result.Observations[0].Error = ""
	endpoint = "https://other.example"
	if check() {
		t.Fatal("different EKS identity matched")
	}
}
