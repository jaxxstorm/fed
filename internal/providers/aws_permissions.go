package providers

import (
	"encoding/json"
	"regexp"

	"github.com/jaxxstorm/fed/internal/connections"
)

const AWSReadPolicyVersion = "fed-aws-read/v1"

type awsReadStatement struct {
	Effect    string                       `json:"Effect"`
	Action    []string                     `json:"Action"`
	Resource  []string                     `json:"Resource"`
	Condition map[string]map[string]string `json:"Condition"`
}

// AWSReadPolicy covers the currently registered RDS, ElastiCache, EKS and
// GuardDuty collectors plus EKS discovery. STS identity needs no Allow grant;
// AssumeRole belongs to the Fed task role, not the customer's read policy.
// Kubernetes API authorization remains a separate explicit RBAC grant.
func AWSReadPolicy(account string) (json.RawMessage, error) {
	if !regexp.MustCompile(`^[0-9]{12}$`).MatchString(account) {
		return nil, connections.ErrInvalid
	}
	arn := func(service, resource string) string {
		return "arn:aws:" + service + ":us-east-1:" + account + ":" + resource
	}
	statements := []awsReadStatement{
		{Action: []string{"eks:ListClusters", "guardduty:ListDetectors"}, Resource: []string{"*"}},
		{Action: []string{"eks:DescribeCluster"}, Resource: []string{arn("eks", "cluster/*")}},
		{Action: []string{"guardduty:GetDetector"}, Resource: []string{arn("guardduty", "detector/*")}},
		{Action: []string{"rds:DescribeDBInstances"}, Resource: []string{arn("rds", "db:*")}},
		{Action: []string{"rds:DescribeDBClusters"}, Resource: []string{arn("rds", "cluster:*")}},
		{Action: []string{"elasticache:DescribeCacheClusters"}, Resource: []string{arn("elasticache", "cluster:*")}},
		{Action: []string{"elasticache:DescribeReplicationGroups"}, Resource: []string{arn("elasticache", "replicationgroup:*")}},
		{Action: []string{"elasticache:DescribeServerlessCaches"}, Resource: []string{arn("elasticache", "serverlesscache:*")}},
	}
	for i := range statements {
		statements[i].Effect = "Allow"
		statements[i].Condition = map[string]map[string]string{"StringEquals": {"aws:RequestedRegion": "us-east-1"}}
	}
	return json.MarshalIndent(struct {
		Version   string             `json:"Version"`
		Statement []awsReadStatement `json:"Statement"`
	}{"2012-10-17", statements}, "", "  ")
}
