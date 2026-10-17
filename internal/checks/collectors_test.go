package checks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
)

func TestAWSCollectionAndPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/detector":
			if r.URL.Query().Get("nextToken") == "page2" {
				fmt.Fprint(w, `{"detectorIds":["second"]}`)
			} else {
				fmt.Fprint(w, `{"detectorIds":["first"],"nextToken":"page2"}`)
			}
		case "/detector/first":
			fmt.Fprint(w, `{"status":"ENABLED","serviceRole":"role","features":[{"name":"EKS_AUDIT_LOGS","status":"ENABLED"}]}`)
		case "/detector/second":
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"denied"}`)
		case "/clusters/test":
			fmt.Fprint(w, `{"cluster":{"name":"test","logging":{"clusterLogging":[{"enabled":true,"types":["api"]}]}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	cfg := func(context.Context, assessment.Account, string) (aws.Config, error) {
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1}, nil
	}
	scope := assessment.Scope{Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}, Clusters: []string{"test"}}}}
	result, err := (AWS{Kind: "guardduty", Config: cfg}).Collect(context.Background(), scope, assessment.Measure{Expected: []string{"detector", "EKS_AUDIT_LOGS"}})
	if err != nil || result.Expected != 2 || result.Observed != 1 || result.Outcome != "error" {
		t.Fatalf("partial: %+v %v", result, err)
	}
	result, err = (AWS{Kind: "eks", Config: cfg}).Collect(context.Background(), scope, assessment.Measure{Expected: []string{"api", "audit"}})
	if err != nil || result.Outcome != "fail" {
		t.Fatalf("logging: %+v %v", result, err)
	}
}

func TestGitHubIntent(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != sha || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing pinned ref or auth")
		}
		json.NewEncoder(w).Encode(map[string]string{"type": "file", "encoding": "base64", "sha": "blob", "content": base64.StdEncoding.EncodeToString([]byte("uses: pulumi/actions@v6"))})
	}))
	defer srv.Close()
	g := GitHub{BaseURL: srv.URL, Client: srv.Client(), Token: "test-token"}
	r, err := g.Collect(context.Background(), assessment.Scope{Repositories: []assessment.Repository{{Name: "org/repo", Commit: sha, Paths: []string{".github/workflows/pulumi.yaml"}}}}, assessment.Measure{Expected: []string{"pulumi"}})
	if err != nil || r.Outcome != "pass" || len(r.Limitations) == 0 {
		t.Fatalf("intent: %+v %v", r, err)
	}
}

func TestMissingCoverageIsUnknown(t *testing.T) {
	r := Result{}
	r.Summarize()
	if r.Outcome != "unknown" || r.Expected != 0 {
		t.Fatal("empty collection passed")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"detectorIds":[]}`)
	}))
	defer srv.Close()
	cfg := aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}
	r.Observations = guardDuty(context.Background(), cfg, "account/region", []string{"detector"})
	r.Summarize()
	if r.Outcome != "unknown" || r.Observed != 0 {
		t.Fatal("missing detector passed")
	}
}
