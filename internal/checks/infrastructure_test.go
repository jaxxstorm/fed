package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
)

func TestInfrastructureReadOnlyFixtures(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/clusters/prod" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"cluster":{"arn":"arn:aws:eks:us-east-1:123456789012:cluster/prod","resourcesVpcConfig":{"endpointPublicAccess":true,"endpointPrivateAccess":true,"publicAccessCidrs":["::/0"]}}}`)
			return
		}
		action := r.FormValue("Action")
		w.Header().Set("Content-Type", "text/xml")
		switch action {
		case "DescribeDBInstances":
			pages++
			if r.FormValue("Marker") == "next" {
				fmt.Fprint(w, `<DescribeDBInstancesResponse><DescribeDBInstancesResult><DBInstances><DBInstance><DBInstanceArn>arn:aws:rds:us-east-1:123456789012:db:replica</DBInstanceArn><StorageEncrypted>true</StorageEncrypted><BackupRetentionPeriod>0</BackupRetentionPeriod><ReadReplicaSourceDBInstanceIdentifier>primary</ReadReplicaSourceDBInstanceIdentifier></DBInstance></DBInstances></DescribeDBInstancesResult></DescribeDBInstancesResponse>`)
				return
			}
			fmt.Fprint(w, `<DescribeDBInstancesResponse><DescribeDBInstancesResult><Marker>next</Marker><DBInstances><DBInstance><DBInstanceArn>arn:aws:rds:us-east-1:123456789012:db:primary</DBInstanceArn><StorageEncrypted>false</StorageEncrypted><BackupRetentionPeriod>0</BackupRetentionPeriod></DBInstance></DBInstances></DescribeDBInstancesResult></DescribeDBInstancesResponse>`)
		case "DescribeDBClusters":
			fmt.Fprint(w, `<DescribeDBClustersResponse><DescribeDBClustersResult><DBClusters><DBCluster><DBClusterArn>arn:aws:rds:us-east-1:123456789012:cluster:aurora</DBClusterArn><StorageEncrypted>true</StorageEncrypted><BackupRetentionPeriod>3</BackupRetentionPeriod></DBCluster></DBClusters></DescribeDBClustersResult></DescribeDBClustersResponse>`)
		case "DescribeReplicationGroups":
			fmt.Fprint(w, `<DescribeReplicationGroupsResponse><DescribeReplicationGroupsResult><ReplicationGroups><ReplicationGroup><ARN>arn:aws:elasticache:us-east-1:123456789012:replicationgroup:redis</ARN><TransitEncryptionEnabled>true</TransitEncryptionEnabled><TransitEncryptionMode>preferred</TransitEncryptionMode></ReplicationGroup></ReplicationGroups></DescribeReplicationGroupsResult></DescribeReplicationGroupsResponse>`)
		case "DescribeCacheClusters":
			fmt.Fprint(w, `<DescribeCacheClustersResponse><DescribeCacheClustersResult><CacheClusters/></DescribeCacheClustersResult></DescribeCacheClustersResponse>`)
		case "DescribeServerlessCaches":
			fmt.Fprint(w, `<DescribeServerlessCachesResponse><DescribeServerlessCachesResult><ServerlessCaches/></DescribeServerlessCachesResult></DescribeServerlessCachesResponse>`)
		default:
			t.Errorf("unexpected or non-read operation %s %s", action, r.URL.Path)
			w.WriteHeader(403)
		}
	}))
	defer srv.Close()
	config := func(ctx context.Context, a assessment.Account, region string) (aws.Config, error) {
		if a.ID != "123456789012" || region != "us-east-1" {
			t.Fatalf("outside scope: %s/%s", a.ID, region)
		}
		return aws.Config{Region: region, BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1}, nil
	}
	scope := assessment.Scope{Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}, Clusters: []string{"prod"}}}}
	result, err := (Infrastructure{Kind: "rds", Config: config}).Collect(context.Background(), scope, assessment.Measure{Policy: map[string]json.RawMessage{"minimum_retention_days": json.RawMessage(`7`)}})
	if err != nil || len(result.Observations) != 3 || pages != 2 {
		t.Fatalf("RDS pagination: %+v %v", result, err)
	}
	if result.Observations[0].Evaluations[1].Outcome != "fail" || result.Observations[1].Evaluations[1].Outcome != "unknown" || result.Observations[2].Evaluations[1].Outcome != "fail" {
		t.Fatalf("backup ownership/policy: %+v", result)
	}
	result, err = (Infrastructure{Kind: "elasticache", Config: config}).Collect(context.Background(), scope, assessment.Measure{})
	if err != nil || result.Outcome != "fail" {
		t.Fatalf("preferred TLS passed: %+v %v", result, err)
	}
	result, err = (Infrastructure{Kind: "endpoint", Config: config}).Collect(context.Background(), scope, assessment.Measure{})
	if err != nil || result.Outcome != "fail" {
		t.Fatalf("unrestricted IPv6 passed: %+v %v", result, err)
	}
}

func TestEndpointPolicyAndEmptyInventory(t *testing.T) {
	o := configObservation("123456789012", "us-east-1", "arn:aws:eks:us-east-1:123456789012:cluster:test", "aws.eks.endpoint/v1", map[string]any{"public_cidrs": []string{"10.2.0.0/16"}}, observation.Evaluation{Condition: "public_endpoint", Outcome: "policy_required", Explanation: "Policy missing"})
	r := Result{Observations: []Observation{o}}
	applyInfrastructurePolicy(&r, assessment.Measure{Policy: map[string]json.RawMessage{"approved_cidrs": json.RawMessage(`["10.0.0.0/8"]`)}})
	if r.Observations[0].Outcome != "pass" {
		t.Fatal("allowed subnet rejected")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		if r.FormValue("Action") == "DescribeDBInstances" {
			fmt.Fprint(w, `<DescribeDBInstancesResponse><DescribeDBInstancesResult><DBInstances/></DescribeDBInstancesResult></DescribeDBInstancesResponse>`)
		} else {
			fmt.Fprint(w, `<DescribeDBClustersResponse><DescribeDBClustersResult><DBClusters/></DescribeDBClustersResult></DescribeDBClustersResponse>`)
		}
	}))
	defer srv.Close()
	cfg := aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}
	result := Result{Observations: collectRDS(context.Background(), cfg, "123456789012", "us-east-1")}
	result.Summarize()
	if result.Outcome != "unknown" || len(result.Observations) != 1 {
		t.Fatal("empty inventory passed")
	}
}
