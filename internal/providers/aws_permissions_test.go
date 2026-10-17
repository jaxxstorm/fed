package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAWSReadPolicyBoundary(t *testing.T) {
	policy, err := AWSReadPolicy("123456789012")
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct{ Statement []awsReadStatement }
	if err = json.Unmarshal(policy, &parsed); err != nil {
		t.Fatal(err)
	}
	// Explicit reviewed action inventory: adding even a read action requires a
	// deliberate permission review rather than accepting arbitrary Get* grants.
	allowed := map[string]bool{"eks:ListClusters": true, "eks:DescribeCluster": true, "guardduty:ListDetectors": true, "guardduty:GetDetector": true, "rds:DescribeDBInstances": true, "rds:DescribeDBClusters": true, "elasticache:DescribeCacheClusters": true, "elasticache:DescribeReplicationGroups": true, "elasticache:DescribeServerlessCaches": true}
	seen := map[string]bool{}
	for _, statement := range parsed.Statement {
		if statement.Effect != "Allow" || statement.Condition["StringEquals"]["aws:RequestedRegion"] != "us-east-1" {
			t.Fatal("unbounded statement", statement)
		}
		for _, action := range statement.Action {
			if !allowed[action] || seen[action] {
				t.Fatal("unexpected/duplicate permission", action)
			}
			seen[action] = true
			for _, resource := range statement.Resource {
				if resource == "*" {
					if action != "eks:ListClusters" && action != "guardduty:ListDetectors" {
						t.Fatal("unscoped resource", action)
					}
				} else if !strings.Contains(resource, ":us-east-1:123456789012:") {
					t.Fatal("wrong account/region", resource)
				}
			}
		}
	}
	if len(seen) != len(allowed) {
		t.Fatal("missing collector permission")
	}
	for _, account := range []string{"", "123", "12345678901*", "１２３４５６７８９０１２"} {
		if _, err := AWSReadPolicy(account); err == nil {
			t.Fatal("invalid account accepted", account)
		}
	}
}
