# Automatic evidence collection

Once a service has a saved scope and certification profile, worker/all mode
generates a collection plan, creates managed measures, and queues supported
read-only collectors. No assessment draft or manual measure entry is needed.
The default refresh is **24 hours**, with **48-hour freshness**. These are
configurable application defaults, not FedRAMP thresholds.

Open **Collection** on the service page for the plan, schedule, collect-now,
pause/resume, recent batches, and per-job observations. Requirement details show
evidence, coverage, source attribution and resource findings independently of
drafts. Administrative obligations may require manual evidence; unsupported
requirements are explicitly labeled. A technical result never creates a human
review or establishes full KSI satisfaction.

## Scope and credentials

Only saved account/region combinations are inspected. For Ignis, all AWS
collection stays in **us-east-1**. Empty EKS cluster lists permit enumeration in
that saved account/region; Kubernetes access additionally requires an explicit
operator-owned context binding for each discovered cluster. Other local contexts
and repositories are not automatically included.

Configure local SSO bindings on both the server and worker processes:

```sh
export KSI_AWS_ACCOUNT_PROFILES='{"732360055242":"ignistech-production","967721292901":"ignistech-shared","937801277814":"ignistech-mgmt","186241287477":"ignistech-development"}'
export KSI_KUBERNETES_CONTEXTS='{"732360055242/us-east-1/production-9101335":"ignis-prod","186241287477/us-east-1/development-499f60b":"ignis-development"}'
```

AWS identity is checked with STS. Kubernetes endpoint and certificate authority
must match the scoped EKS cluster before the client reads workloads. The worker
uses its own kubeconfig and configured context, including its normal credential
plugin. A remote deployment needs its own credentials/bindings; access in an
external agent's terminal is not inherited by the server.

Additional saved Audit/Log Archive accounts require their own accessible
profiles or assumed roles. A missing or mismatched identity produces a coverage
error, not a successful empty inventory. GitHub collection uses `GITHUB_TOKEN`
and saved repository/commit/path references.

## Collectors and read permissions

All AWS checks also use STS `GetCallerIdentity`; configured roles require
`sts:AssumeRole` and an appropriate trust policy.

| Collector | Read operations | Evaluation boundary |
| --- | --- | --- |
| GuardDuty configuration | `guardduty:ListDetectors`, `guardduty:GetDetector` | Enabled detector/features, not effective response |
| EKS logging | `eks:ListClusters`, `eks:DescribeCluster` | Enabled control-plane log types, not delivery or review |
| RDS configuration | `rds:DescribeDBInstances`, `rds:DescribeDBClusters` | Encryption and controlling-resource backup retention, not restore success |
| Redis/Valkey TLS | `elasticache:DescribeReplicationGroups`, `elasticache:DescribeCacheClusters`, `elasticache:DescribeServerlessCaches` | Disabled/optional/enforced TLS where exposed; absent serverless mode remains unknown |
| EKS endpoint access | `eks:ListClusters`, `eks:DescribeCluster` | Public/private configuration and IPv4/IPv6 source CIDRs |
| Kubernetes isolation | EKS identity reads plus namespace/pod/NetworkPolicy list and `aws-node` DaemonSet get | Workload selection, ingress/egress isolation and supported CNI configuration; no traffic probes |
| Pulumi workflow evidence | GitHub contents read | Configuration intent at a pinned commit, not deployed state |

Kubernetes collection never reads Secret values. Current CNI detection recognizes
AWS VPC CNI configuration with `ENABLE_NETWORK_POLICY=true` and the network policy
agent container. Unsupported or unreadable enforcement remains unknown, even
when policies exist. This configuration inspection is not proof of runtime
traffic enforcement.

See [collection mappings](collection-mappings.md) for versioned, partial mappings
to official requirements. New catalog snapshots require reviewed mappings;
matching only a rule's name is insufficient.

## Customer policy and revisions

Collection configuration accepts these policy overrides:

```json
{
  "minimum_retention_days": 7,
  "approved_cidrs": ["203.0.113.10/32"],
  "namespace_exclusions": []
}
```

These are examples, not recommended customer values. Without a retention or CIDR
policy, observations still collect; policy-dependent evaluations remain explicit.
Zero retention on a controlling primary and unrestricted public CIDRs are
application-baseline findings. Namespace exclusions must be explicit and remain
visible in evidence. Replica and cluster-controlled backup settings are not
treated as independent disabled-primary backups.

Every configuration edit creates an immutable revision and reconciles a new plan.
Runs freeze scope/profile/configuration/measure versions. Old results remain
historical and do not establish coverage for newer bindings. Managed-measure
overrides survive reconciliation. Pausing prevents scheduled claims; collect-now
is an explicit on-demand request and still works while the schedule is paused.

Schedules and jobs are durable in PostgreSQL, deduplicate concurrent workers,
coalesce missed intervals, and recover expired leases. Transient job failures
have bounded attempt backoff; credential/coverage failures are visible in the
batch. A server-only process requires a separate worker.

## Agent workflow

Invoke the MCP prompt **`gather_evidence`** with `service_id` to direct a connected
LLM agent through the evidence gaps, including entries labeled manual evidence
required. The prompt tells the agent to inspect accessible repositories, cloud
configuration and operational documents, submit evidence, cite it in drafts and
record specific blockers. It is available through MCP `prompts/list` and
`prompts/get`; client interfaces vary in how they expose prompts.

This prompt uses the agent's existing tools and credentials. It does not launch
an LLM from the background scheduler. Agents can now submit documentary text/JSON
directly through `submit_evidence_artifact` and return cited rule results through
`submit_rule_result`. See [external-agent evidence workflow](agent-evidence-workflow.md)
for open-rule discovery, source configuration and completion semantics.

Connect to `/mcp` with the existing scoped bearer-header configuration. Agents can:

1. Call `list_services`, `get_assessment_scope`, and `get_collection_plan`.
2. Page plan entries with `limit` (maximum 100) and `offset` until `next_offset`
   is null. Use the generated requirement and measure references.
3. Call `collect_now` with `service_id` and `idempotency_key`; poll
   `get_collection_batch` using the returned ID.
4. Supplement server collection with `ingest_observation` when the agent has
   direct AWS/Kubernetes/repository access. No external artifact hosting is needed.
5. Retrieve evidence through `get_evidence`, `list_evidence`, and
   `get_evidence_artifact`; cite returned evidence IDs/revisions in drafts.

Example `ingest_observation` arguments (replace identifiers and timestamps with
the actual saved scope/plan and observed resource):

```json
{
  "service_id": "SERVICE_ID",
  "scope_revision": 5,
  "profile_revision": 1,
  "idempotency_key": "rds-observation-unique-run-key",
  "target": {
    "kind": "aws",
    "account_id": "123456789012",
    "region": "us-east-1",
    "resource_id": "arn:aws:rds:us-east-1:123456789012:db:example"
  },
  "observed_at": "2026-10-17T00:00:00Z",
  "method": "aws rds describe-db-instances --profile PROFILE --region us-east-1",
  "tool_version": "aws-cli/2",
  "limitations": ["Configuration observation only"],
  "coverage": {"expected": 1, "observed": 1, "complete": true, "state": "observed"},
  "artifact": {
    "schema": "aws.rds/v1",
    "configuration": {"storage_encrypted": false, "backup_retention_days": 0, "backup_owner": "self", "resource_type": "instance"}
  },
  "evaluations": [{"condition": "storage_encryption", "outcome": "fail", "explanation": "Observed StorageEncrypted=false"}],
  "links": [{
    "requirement_id": "KSI-SVC-SIN",
    "measure": {"id": "MEASURE_ID_FROM_PLAN", "revision": 1},
    "mapping_version": "collection-v1",
    "rationale": "RDS encryption-at-rest configuration provides partial support for securing information",
    "conditions": ["storage_encryption"]
  }]
}
```

The artifact limit is **1 MiB**, within the **2 MiB** MCP request limit. Typed
schemas and allowed configuration fields are defined in
`internal/observation/types.go`. Submit resource-sized observations rather than
entire raw account inventories. A repeated key with identical input returns the
same receipt; different input returns `revision_conflict`. Future timestamps
beyond one minute are rejected. Old observations remain old, not newly observed.

Agent submissions are labeled `agent_observation`; their evaluations are
`agent_assertion`. Server collector results remain `collector_observation` and
`collector_evaluation`. Actors, hashes and receipt times are server-owned.
Agents cannot promote their evidence to server-collected provenance or human
review. They need `edit` for ingestion/configuration and `check` for collect-now;
read-only tokens can inspect authorized evidence but cannot start or change work.

## Upgrade and rollback

1. Back up the database and artifact store. Stop the old worker before migration.
2. Build and run `ksi-service migrate`, then start server/worker or all mode with
   the credential and context bindings. Existing services reconcile at startup.
3. Confirm Collection shows a batch and inspect both successful targets and gaps.
   Do not import findings from an earlier chat summary as fresh observations.
4. Verify evidence appears before drafting and that exports contain observation
   provenance, policy bindings, findings and snapshot-time freshness.

Legacy evidence remains intact. Jobs predating frozen profile/configuration
bindings are not guessed into new current-coverage links; inspect their original
artifacts and collect fresh evidence. Empty complete inventories, denied access,
and unsupported evaluations remain distinct. Partial/failed scans cannot resolve
previous findings; a newer complete observation can record a resolution.

To roll back application code, pause schedules and stop workers first. Deploy a
schema-compatible binary; retain immutable records and artifacts. Schema rollback
requires restoring coordinated backups, not deleting evidence/history.
