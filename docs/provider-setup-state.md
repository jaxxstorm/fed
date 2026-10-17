# Provider setup state

Standard AWS, Workspace and GitHub onboarding uses service-owned temporary
setups. AWS uses `delegated_role`, Workspace uses `workspace_consent`, and
GitHub uses `github_installation`. Existing connections retain `legacy`
credential semantics. Agent-managed, manual, Kubernetes and profile/environment
flows keep their explicit existing access modes; setup state does not promote
an agent observation to verified server access.

Progress is `prerequisites → configure → authorize → confirm → complete`.
Only internal provider verification may advance to confirmation, and completion
requires a resulting connection. Declaration edits discard verification and
invalidate earlier authorization attempts. Cancelled, expired and completed
setups cannot be revived. Setup lifetime is 24 hours; authorization attempts
have their own shorter lifetime. Expected revisions reject concurrent edits.

Browser and MCP operations must call the shared setup service. Mutations require
`connect`; redacted status requires `read`. Idempotency receipts store only a
setup identifier and return current status on replay. Grants, external IDs and
PKCE verifiers belong in encrypted storage, never status or receipt JSON.

## AWS permission manifest

`internal/providers/aws_permissions.go` defines `fed-aws-read/v1`:

| Operations | Consumer |
| --- | --- |
| EKS ListClusters / DescribeCluster | Provider discovery, endpoint/logging collectors, EKS identity verification |
| GuardDuty ListDetectors / GetDetector | GuardDuty configuration collector |
| RDS DescribeDBInstances / DescribeDBClusters | RDS configuration collector |
| ElastiCache DescribeCacheClusters / DescribeReplicationGroups / DescribeServerlessCaches | Transit-encryption collector |

Every statement restricts the requested region to `us-east-1`; resource-scoped
reads use target-account ARNs. Enumeration actions without resource-level
authorization use `*`. STS identity verification requires no explicit Allow.
The Fed application task role separately needs permission to assume approved
customer roles. Kubernetes API reads additionally require read-only Kubernetes
authorization; this IAM manifest does not grant it. The manifest does not grant
infrastructure writes, IAM provisioning, or role chaining from customer roles.

Action/resource support was checked against AWS's machine-readable service
authorization reference at
`https://servicereference.us-east-1.amazonaws.com/v1/{service}/{service}.json`
for `eks`, `rds`, `elasticache` and `guardduty`. The nine manifest actions are
non-write operations; only EKS ListClusters and GuardDuty ListDetectors lack
resource-level authorization.
