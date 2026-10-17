# Collection mappings v1

Reviewed against `testdata/fedramp/rules.json`, Rules commit
`a89d78477ddea98dc57eca24f5007794c2b4eeb6` (payload SHA-256
`fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4`).
Mappings require this exact catalog snapshot; an unrecognized snapshot is
`unsupported_automation`. Profile resolution still controls applicability.

| Collector / measure | Requirement | Partial support and limitations | Policy inputs |
| --- | --- | --- | --- |
| `aws.guardduty.configuration` | KSI-CNA-RVP | Detector configuration supports review of protection against unwanted activity. Does not demonstrate effective response, DDoS protection, or continuous review. | Expected detector/features; detector enabled is an application baseline |
| `aws.eks.control-plane-logging` | KSI-MLA-LET | Enabled EKS log types support maintaining and reviewing an event logging inventory. Does not prove delivery, retention, SIEM review or completeness across the offering. | Expected log types; all five EKS control-plane types are an application default |
| `github.pulumi.workflow-evidence` | KSI-SVC-ACM | Pinned workflow declarations support configuration automation. They do not prove executed deployments, drift monitoring or the live deployed commit. | Expected workflow terms; `pulumi` is an application default, not an effectiveness test |
| `aws.rds.configuration` / encryption | KSI-SVC-SIN | Storage encryption supports protection of information at rest, not access control or encryption of all data. | Encryption enabled is an application baseline |
| `aws.rds.configuration` / backups | KSI-RPL-ABO | Retention and source/cluster relationships support review of backups against recovery objectives. Does not establish RTO/RPO or successful restore. | Zero primary retention is a baseline finding; minimum retention requires customer input |
| `aws.elasticache.transit-encryption` | KSI-SVC-SIN | TLS enforcement supports protection of information in transit. Does not prove client certificate verification or cryptographic configuration of every connection. | Enforced TLS is an application baseline; mixed/preferred mode is transitional |
| `aws.eks.endpoint-access` | KSI-CNA-RNT, KSI-CNA-MAT | Endpoint CIDRs support review of inbound restrictions and attack surface. They do not describe all traffic paths or prove reachability. | Unrestricted public IPv4/IPv6 is a baseline finding; approved CIDRs require customer input |
| `kubernetes.network-isolation` | KSI-CNA-RNT, KSI-CNA-ULN, KSI-CNA-MAT | Policies, workload selectors, ingress/egress isolation and supported CNI configuration support traffic-flow and lateral-movement review. Policy existence is not proof of enforcement. | Explicit namespace exclusions; unsupported enforcement remains unknown |

Default refresh is 24 hours and freshness 48 hours, both configurable application
defaults. Missing policy blocks only the relevant evaluation, not observation
collection. Evidence for one collector can link to several measures without
rescanning the same resource. All links carry mapping version and rationale.

## Manual and unsupported obligations

The `AFC-CSO-*` administrative communication obligations (including
`AFC-CSO-INB`, maintaining a FedRAMP security inbox) require administrative evidence;
AWS resource configuration is not evidence of those duties. Human attestations,
documented routing/ownership, and operational records can be referenced separately.
No automated assessment is inferred for other requirements without an explicit
reviewed mapping. Unresolved applicability stays unresolved; profile-excluded
requirements are not applicable rather than missing evidence.

All collectors respect saved accounts/regions/cluster/repository bindings. For the
current Ignis service this means **us-east-1 only**. Accessible profiles, contexts,
and repositories are not automatic authorization to enlarge the scope.
