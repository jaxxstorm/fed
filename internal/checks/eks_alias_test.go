package checks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
)

func TestStandaloneEKSRequiresObservedClusterWideEndpointIdentity(t *testing.T) {
	for _, tt := range []struct {
		name           string
		wide, observed bool
		endpoint, arn  string
		status         int
		want           bool
	}{
		{"same cluster", true, true, "https://cluster.example", "arn:aws:eks:us-east-1:123456789012:cluster/production", 200, true},
		{"namespace subset", false, true, "https://cluster.example", "arn:aws:eks:us-east-1:123456789012:cluster/production", 200, false},
		{"no successful observation", true, false, "https://cluster.example", "arn:aws:eks:us-east-1:123456789012:cluster/production", 200, false},
		{"different endpoint", true, true, "https://other.example", "arn:aws:eks:us-east-1:123456789012:cluster/production", 200, false},
		{"wrong AWS account", true, true, "https://cluster.example", "arn:aws:eks:us-east-1:999999999999:cluster/production", 200, false},
		{"AWS permission denied", true, true, "https://cluster.example", "", 403, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"cluster": map[string]string{"name": "production", "arn": tt.arn, "endpoint": tt.endpoint}})
			}))
			defer server.Close()
			resolve := func(context.Context, assessment.Account, string) (aws.Config, error) {
				return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
					return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
				})}, nil
			}
			scope := assessment.Scope{Kubernetes: []assessment.KubernetesScope{{ClusterID: "uid", Server: "https://cluster.example", ClusterWide: tt.wide}}}
			result := Result{Observations: []Observation{{Scope: "uid/failed", Error: "workload_enumeration_failed"}}}
			if tt.observed {
				result.Observations = append(result.Observations, Observation{Target: &observation.Target{Kind: "kubernetes", ClusterID: "uid", ResourceID: "pod"}})
			}
			if got := standaloneObservedEKS(context.Background(), scope, result, assessment.Account{ID: "123456789012"}, "us-east-1", "production", resolve); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			if (!tt.observed || !tt.wide) && calls != 0 {
				t.Fatal("unnecessary AWS call")
			}
			if len(result.Observations) == 0 || result.Observations[0].Error != "workload_enumeration_failed" {
				t.Fatal("existing coverage error removed")
			}
		})
	}
}
