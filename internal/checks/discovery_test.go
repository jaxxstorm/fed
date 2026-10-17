package checks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
)

func TestBoundaryDiscoveryCoverage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/clusters" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"clusters":["production"]}`)
			return
		}
		t.Error("discovery must not enumerate regions outside saved scope")
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><regionInfo><item><regionName>us-east-1</regionName></item></regionInfo></DescribeRegionsResponse>`)
	}))
	defer srv.Close()
	config := func(ctx context.Context, a assessment.Account, region string) (aws.Config, error) {
		if a.ID == "222222222222" {
			return aws.Config{}, errors.New("denied")
		}
		return aws.Config{Region: region, BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}, nil
	}
	scope := assessment.Scope{Description: "Boundary", Accounts: []assessment.Account{{ID: "111111111111", Regions: []string{"us-east-1"}}, {ID: "222222222222", Regions: []string{"us-west-2"}, Clusters: []string{"keep-me"}}}, Repositories: []assessment.Repository{{Name: "org/repo"}}}
	result := discoverBoundary(context.Background(), scope, 3, config)
	if result.ExpectedRevision != 3 || len(result.Scope.Accounts) != 2 || len(result.Coverage) != 2 {
		t.Fatalf("unexpected discovery: %+v", result)
	}
	if result.Scope.Accounts[0].Clusters[0] != "production" || result.Scope.Accounts[1].Clusters[0] != "keep-me" || result.Coverage[1].Outcome != "error" || len(result.Scope.Repositories) != 1 {
		t.Fatalf("lost coverage or scope: %+v", result)
	}
}
