# Finding explanation inventory

Explanation version: `finding-explanations/v1`.

Agent comparison performed against the collector implementations on 2026-10-18.
These explanations describe retained collector facts, not official policy
interpretations or determinations of requirement compliance. The full catalog
source-comparison review remains a separate release gate.

| Schema / condition | Retained fact used | Interpretation boundary |
| --- | --- | --- |
| `kubernetes.network-isolation/v1` / `ingress_isolation` | `ingress_isolated` | Selecting policy configuration, not public exposure or enforced traffic restrictions |
| same / `egress_isolation` | `egress_isolated` | Selecting policy configuration, not observed connection blocking |
| same / `network_policy_enforcement` | Configuration-only inspection | Actual enforcement needs separate traffic-test evidence |
| `aws.rds/v1` / `storage_encryption` | `storage_encrypted` | This resource's configuration, not all copies, key authorization, or application authorization |
| same / `backup_retention` | `backup_owner`, `backup_retention_days` | Zero retention is meaningful on the controlling resource; objectives and successful restores require other records |
| `aws.elasticache/v1` / `transit_encryption` | `transit_encryption_enabled`, `transit_encryption_mode` | Preferred mode permits plaintext clients; it does not mean TLS is disabled. Missing serverless fields remain unknown |
| `aws.eks.endpoint/v1` / `public_endpoint` | `public_access`, `public_cidrs` | Unrestricted source range, not successful access, exploitation, or workload exposure |
| `aws.guardduty.configuration/v1` / `collector_result` | Selected detector/feature comparison outcome | Enabled settings do not prove detection, coverage or response; no individual missing feature is invented |
| `aws.eks.control-plane-logging/v1` / `collector_result` | Selected log-type comparison outcome | Enabled configuration does not prove delivery, retention or review; empty EKS inventory does not cover standalone clusters |
| `github.pulumi.workflow-evidence/v1` / `collector_result` | Case-insensitive search-term comparison outcome | Text matches do not parse behavior or prove execution; missing terms do not prove a capability absent |
| Unknown schemas / conditions | Retained outcome and raw diagnostic | Explicit explanation gap; no invented cause |

Source comparison used `internal/checks/infrastructure.go`,
`internal/checks/kubernetes.go`, `internal/checks/standalone_kubernetes.go`, and
the generic outcome adapter in `internal/checks/runner.go`. New agent-supplied
schemas or condition codes also use the explicit explanation-gap fallback.

## Collection and grouping rules

- Failed reads and incomplete enumeration take precedence over configuration
  interpretation. They cannot establish absence of a policy or control.
- Groups retain schema, condition, outcome, interpreted cause, evidence basis,
  account, region, cluster, namespace and repository boundaries. Unknown causes
  and distinct access errors remain separate.
- Affected counts count distinct exact targets. Individual findings retain
  observation IDs and timestamps, with authorized drill-down to the retained
  submission and artifact hash.
- Evidence-first actions request records and verification. They do not make
  infrastructure changes.

## Verification

`internal/observation/finding_explanations_test.go` checks negative policy wording,
failed-read and partial-enumeration distinctions, unknown codes, namespace/error
group separation, and preferred TLS mode. PostgreSQL-backed assessment tests
verify exact retained encryption facts and observation drill-down. MCP tests
compare browser and MCP evidence coverage including contradictory citations.
The browser guidance suite checks concise groups, hidden raw diagnostics,
candidate-versus-assessed evidence and immutable revision links across three
screen sizes.

Passing and undetermined checks now retain result, freshness, observation time,
target and drill-down provenance. All seven registered collectors have explanations
for their emitted condition codes. Legacy outcome explanations deliberately avoid
naming individual missing settings absent from the normalized finding; separate
observations remain separate groups. Unknown schemas retain an interpretation-gap
fallback. Assessors can record assertion-specific missing,
partial, stale or access-blocked scope/period gaps with reasons and evidence-first
next actions; unresolved gaps prevent a supported or evidence-complete assertion.
These declarations do not automatically infer coverage of unmentioned targets.
Full-catalog comprehension acceptance remains outstanding.
