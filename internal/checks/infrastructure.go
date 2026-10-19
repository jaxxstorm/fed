package checks

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
)

type Infrastructure struct {
	Kind   string
	Config func(context.Context, assessment.Account, string) (aws.Config, error)
}

func (c Infrastructure) Collect(ctx context.Context, scope assessment.Scope, m assessment.Measure) (Result, error) {
	r := Result{Observations: []Observation{}, Limitations: []string{"Read-only configuration evidence; does not establish full requirement coverage or operating effectiveness."}}
	for _, a := range scope.Accounts {
		for _, region := range a.Regions {
			label := a.ID + "/" + region
			cfg, err := c.Config(ctx, a, region)
			if err != nil {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: collectorErrorCode(err, "credentials_or_account_validation_failed")})
				continue
			}
			switch c.Kind {
			case "rds":
				r.Observations = append(r.Observations, collectRDS(ctx, cfg, a.ID, region)...)
			case "elasticache":
				r.Observations = append(r.Observations, collectRedis(ctx, cfg, a.ID, region)...)
			case "endpoint":
				r.Observations = append(r.Observations, collectEndpoints(ctx, cfg, a, region)...)
			}
		}
	}
	applyInfrastructurePolicy(&r, m)
	r.Summarize()
	return r, nil
}

func applyInfrastructurePolicy(r *Result, m assessment.Measure) {
	var minimum int
	_ = json.Unmarshal(m.Policy["minimum_retention_days"], &minimum)
	var approved []string
	_ = json.Unmarshal(m.Policy["approved_cidrs"], &approved)
	for i := range r.Observations {
		o := &r.Observations[i]
		artifact, ok := o.Data.(observation.Artifact)
		if !ok {
			continue
		}
		for j := range o.Evaluations {
			e := &o.Evaluations[j]
			if e.Condition == "backup_retention" && e.Outcome == "policy_required" && minimum > 0 {
				var days int
				if json.Unmarshal(artifact.Configuration["backup_retention_days"], &days) == nil {
					e.Outcome = "pass"
					if days < minimum {
						e.Outcome = "fail"
					}
					e.Explanation = "Retention compared with the customer-defined minimum; restore capability is not established."
				}
			}
			if e.Condition == "public_endpoint" && e.Outcome == "policy_required" && len(approved) > 0 {
				var observed []string
				if json.Unmarshal(artifact.Configuration["public_cidrs"], &observed) != nil {
					continue
				}
				e.Outcome = "pass"
				if len(observed) == 0 {
					e.Outcome = "unknown"
				}
				for _, raw := range observed {
					prefix, err := netip.ParsePrefix(raw)
					contained := false
					if err == nil {
						for _, allow := range approved {
							p, err := netip.ParsePrefix(allow)
							if err == nil && p.Addr().BitLen() == prefix.Addr().BitLen() && p.Bits() <= prefix.Bits() && p.Contains(prefix.Masked().Addr()) {
								contained = true
							}
						}
					}
					if !contained {
						e.Outcome = "fail"
					}
				}
				e.Explanation = "Public CIDRs compared with customer-approved ranges; this does not establish all network reachability."
			}
		}
		o.Outcome = "pass"
		for _, e := range o.Evaluations {
			if e.Outcome == "fail" {
				o.Outcome = "fail"
			} else if o.Outcome == "pass" && e.Outcome != "pass" {
				o.Outcome = "unknown"
			}
		}
	}
}
func configObservation(account, region, arn, schema string, configuration map[string]any, evals ...observation.Evaluation) Observation {
	fields := map[string]json.RawMessage{}
	for k, v := range configuration {
		b, _ := json.Marshal(v)
		fields[k] = b
	}
	o := Observation{Scope: account + "/" + region + "/" + arn, Outcome: "pass", Target: &observation.Target{Kind: "aws", AccountID: account, Region: region, ResourceID: arn}, Data: observation.Artifact{Schema: schema, Configuration: fields}, Evaluations: evals}
	for _, e := range evals {
		if e.Outcome == "fail" {
			o.Outcome = "fail"
		} else if o.Outcome == "pass" && e.Outcome != "pass" {
			o.Outcome = "unknown"
		}
	}
	return o
}
func condition(id string, value *bool, explanation string) observation.Evaluation {
	state := "unknown"
	if value != nil {
		state = "pass"
		if !*value {
			state = "fail"
		}
	}
	return observation.Evaluation{Condition: id, Outcome: state, Explanation: explanation}
}
func backupCondition(days *int32, owner string) observation.Evaluation {
	e := observation.Evaluation{Condition: "backup_retention", Outcome: "policy_required", Explanation: "Retention must be compared with customer recovery objectives; configuration does not demonstrate a successful restore."}
	if owner != "self" {
		e.Outcome = "unknown"
		e.Explanation = "Backup configuration is controlled by the related cluster/source resource."
	} else if days == nil {
		e.Outcome = "unknown"
	} else if *days == 0 {
		e.Outcome = "fail"
		e.Explanation = "Automated backup retention is disabled on the controlling resource."
	}
	return e
}
func collectRDS(ctx context.Context, cfg aws.Config, account, region string) []Observation {
	client := rds.NewFromConfig(cfg)
	out := []Observation{}
	label := account + "/" + region
	instances := rds.NewDescribeDBInstancesPaginator(client, &rds.DescribeDBInstancesInput{})
	for instances.HasMorePages() {
		page, err := instances.NextPage(ctx)
		if err != nil {
			out = append(out, Observation{Scope: label + "/instances", Outcome: "error", Error: "rds_instance_enumeration_failed"})
			break
		}
		for _, v := range page.DBInstances {
			owner := "self"
			if aws.ToString(v.DBClusterIdentifier) != "" {
				owner = "cluster"
			} else if aws.ToString(v.ReadReplicaSourceDBInstanceIdentifier) != "" {
				owner = "source"
			}
			out = append(out, configObservation(account, region, aws.ToString(v.DBInstanceArn), "aws.rds/v1", map[string]any{"engine": v.Engine, "storage_encrypted": v.StorageEncrypted, "kms_key_id": v.KmsKeyId, "backup_retention_days": v.BackupRetentionPeriod, "source_resource": v.ReadReplicaSourceDBInstanceIdentifier, "cluster_id": v.DBClusterIdentifier, "backup_owner": owner, "resource_type": "instance"}, condition("storage_encryption", v.StorageEncrypted, "Storage encryption configuration; access controls require separate evidence."), backupCondition(v.BackupRetentionPeriod, owner)))
		}
	}
	clusters := rds.NewDescribeDBClustersPaginator(client, &rds.DescribeDBClustersInput{})
	for clusters.HasMorePages() {
		page, err := clusters.NextPage(ctx)
		if err != nil {
			out = append(out, Observation{Scope: label + "/clusters", Outcome: "error", Error: "rds_cluster_enumeration_failed"})
			break
		}
		for _, v := range page.DBClusters {
			owner := "self"
			if aws.ToString(v.ReplicationSourceIdentifier) != "" {
				owner = "source"
			}
			out = append(out, configObservation(account, region, aws.ToString(v.DBClusterArn), "aws.rds/v1", map[string]any{"engine": v.Engine, "storage_encrypted": v.StorageEncrypted, "kms_key_id": v.KmsKeyId, "backup_retention_days": v.BackupRetentionPeriod, "source_resource": v.ReplicationSourceIdentifier, "backup_owner": owner, "resource_type": "cluster"}, condition("storage_encryption", v.StorageEncrypted, "Cluster storage encryption configuration."), backupCondition(v.BackupRetentionPeriod, owner)))
		}
	}
	if len(out) == 0 {
		out = append(out, Observation{Scope: label, Outcome: "unknown", Data: map[string]string{"inventory": "no_resources"}})
	}
	return out
}
func collectRedis(ctx context.Context, cfg aws.Config, account, region string) []Observation {
	client := elasticache.NewFromConfig(cfg)
	out := []Observation{}
	label := account + "/" + region
	groups := elasticache.NewDescribeReplicationGroupsPaginator(client, &elasticache.DescribeReplicationGroupsInput{})
	for groups.HasMorePages() {
		page, err := groups.NextPage(ctx)
		if err != nil {
			out = append(out, Observation{Scope: label, Outcome: "error", Error: "redis_replication_group_enumeration_failed"})
			break
		}
		for _, v := range page.ReplicationGroups {
			e := condition("transit_encryption", v.TransitEncryptionEnabled, "TLS configuration does not establish client certificate verification.")
			if string(v.TransitEncryptionMode) == "preferred" {
				e.Outcome = "fail"
				e.Explanation = "Preferred TLS mode still permits plaintext clients."
			}
			out = append(out, configObservation(account, region, aws.ToString(v.ARN), "aws.elasticache/v1", map[string]any{"transit_encryption_enabled": v.TransitEncryptionEnabled, "transit_encryption_mode": string(v.TransitEncryptionMode), "resource_type": "replication_group"}, e))
		}
	}
	caches := elasticache.NewDescribeCacheClustersPaginator(client, &elasticache.DescribeCacheClustersInput{})
	for caches.HasMorePages() {
		page, err := caches.NextPage(ctx)
		if err != nil {
			out = append(out, Observation{Scope: label, Outcome: "error", Error: "redis_cache_enumeration_failed"})
			break
		}
		for _, v := range page.CacheClusters {
			if aws.ToString(v.ReplicationGroupId) != "" || !strings.Contains(strings.ToLower(aws.ToString(v.Engine)), "redis") && aws.ToString(v.Engine) != "valkey" {
				continue
			}
			out = append(out, configObservation(account, region, aws.ToString(v.ARN), "aws.elasticache/v1", map[string]any{"engine": v.Engine, "engine_version": v.EngineVersion, "transit_encryption_enabled": v.TransitEncryptionEnabled, "resource_type": "cache_cluster"}, condition("transit_encryption", v.TransitEncryptionEnabled, "Standalone cache TLS configuration.")))
		}
	}
	var next *string
	for {
		page, err := client.DescribeServerlessCaches(ctx, &elasticache.DescribeServerlessCachesInput{NextToken: next})
		if err != nil {
			out = append(out, Observation{Scope: label, Outcome: "error", Error: "redis_serverless_enumeration_failed"})
			break
		}
		for _, v := range page.ServerlessCaches {
			if aws.ToString(v.Engine) != "redis" && aws.ToString(v.Engine) != "valkey" {
				continue
			}
			out = append(out, configObservation(account, region, aws.ToString(v.ARN), "aws.elasticache/v1", map[string]any{"engine": v.Engine, "resource_type": "serverless"}, observation.Evaluation{Condition: "transit_encryption", Outcome: "unknown", Explanation: "This response does not expose transit-encryption mode; no inference from an absent field."}))
		}
		next = page.NextToken
		if aws.ToString(next) == "" {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, Observation{Scope: label, Outcome: "unknown", Data: map[string]string{"inventory": "no_resources"}})
	}
	return out
}
func collectEndpoints(ctx context.Context, cfg aws.Config, a assessment.Account, region string) []Observation {
	client := eks.NewFromConfig(cfg)
	out := []Observation{}
	names := append([]string{}, a.Clusters...)
	label := a.ID + "/" + region
	enumerated := true
	if len(names) == 0 {
		pages := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				out = append(out, Observation{Scope: label, Outcome: "error", Error: "cluster_enumeration_failed"})
				enumerated = false
				break
			}
			names = append(names, page.Clusters...)
		}
	}
	for _, name := range names {
		v, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
		if err != nil || v.Cluster == nil || v.Cluster.ResourcesVpcConfig == nil {
			out = append(out, Observation{Scope: label + "/" + name, Outcome: "error", Error: "cluster_endpoint_read_failed"})
			continue
		}
		c := v.Cluster.ResourcesVpcConfig
		e := observation.Evaluation{Condition: "public_endpoint", Outcome: "policy_required", Explanation: "Compare public CIDRs with customer-approved administrative and deployment ranges."}
		if !c.EndpointPublicAccess {
			e.Outcome = "pass"
			e.Explanation = "Public endpoint is disabled; public CIDRs do not apply."
		} else {
			for _, cidr := range c.PublicAccessCidrs {
				if cidr == "0.0.0.0/0" || cidr == "::/0" {
					e.Outcome = "fail"
					e.Explanation = "Public endpoint permits unrestricted source addresses."
				}
			}
		}
		out = append(out, configObservation(a.ID, region, aws.ToString(v.Cluster.Arn), "aws.eks.endpoint/v1", map[string]any{"public_access": c.EndpointPublicAccess, "private_access": c.EndpointPrivateAccess, "public_cidrs": c.PublicAccessCidrs}, e))
	}
	if enumerated && len(names) > 0 {
		out = append(out, partitionAccessObserved(label, "EKS inventory access succeeded; endpoint observations retain their individual results."))
	}
	if len(out) == 0 {
		out = append(out, Observation{Scope: label, Outcome: "unknown", Data: map[string]string{"inventory": "no_resources"}})
	}
	return out
}
