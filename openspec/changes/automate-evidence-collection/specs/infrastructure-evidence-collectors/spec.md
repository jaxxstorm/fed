## ADDED Requirements

### Requirement: Collect RDS encryption and backup configuration
The system SHALL collect paginated RDS instance and cluster configuration, resource identities, encryption and backup-retention fields, and relationships within the saved boundary. Evaluations SHALL distinguish primary, replica and cluster-owned settings, flag disabled encryption and applicable zero-retention backups, and require customer policy before asserting retention meets an objective. Configuration evidence SHALL NOT assert restore success.

#### Scenario: Unencrypted primary with disabled backups
- **WHEN** a fully observed primary controls its backups, has storage encryption disabled and retention zero
- **THEN** evidence includes the resource and fields and produces distinct encryption and backup findings linked to mapped measures

#### Scenario: Aurora member or replica
- **WHEN** backup behavior is owned by a parent cluster or source relationship
- **THEN** evaluation uses the controlling resource or reports unknown rather than treating an irrelevant instance field as a disabled primary backup

### Requirement: Collect Redis transit-encryption configuration
The system SHALL inspect supported ElastiCache Redis/Valkey resource types and record engine/version, transit-encryption state/mode and resource relationships. It SHALL distinguish disabled TLS, optional/mixed TLS and enforced TLS, and SHALL report unsupported or missing configuration as unknown.

#### Scenario: Optional TLS mode
- **WHEN** transit encryption is enabled but plaintext clients are still permitted by the configured mode
- **THEN** the result does not assert enforced encryption and records the transitional configuration

### Requirement: Evaluate EKS endpoint exposure
The system SHALL collect EKS public/private endpoint settings and public CIDRs, flag unrestricted IPv4/IPv6 public access, and evaluate customer-approved CIDRs only when policy is configured. It SHALL avoid asserting network reachability from endpoint settings alone.

#### Scenario: Unrestricted public endpoint
- **WHEN** public access is enabled with 0.0.0.0/0 or ::/0
- **THEN** a resource-specific exposure finding and supporting configuration artifact are persisted

#### Scenario: Private endpoint only
- **WHEN** public access is disabled
- **THEN** stored public CIDRs do not produce a public exposure finding

### Requirement: Collect Kubernetes isolation and enforcement evidence
The system SHALL inspect explicitly bound clusters using read-only access, collecting namespace/workload metadata, NetworkPolicies and supported CNI enforcement configuration. It SHALL distinguish policy existence, ingress/egress selector coverage and verified configuration-based enforcement; unsupported enforcement detection SHALL remain unknown. Namespace exclusions SHALL be explicit. Collectors MUST NOT retrieve Secret values or mutate workloads.

#### Scenario: Production workloads have no NetworkPolicies
- **WHEN** complete namespace and policy enumeration finds production workloads without isolation policies
- **THEN** evidence and findings identify the affected workloads/namespaces and observed isolation gaps

#### Scenario: Policies exist but enforcement is unknown
- **WHEN** policies select workloads but the collector cannot establish supported CNI enforcement configuration
- **THEN** policy presence is recorded while enforcement remains unknown instead of passing the complete measure

#### Scenario: Cluster access is denied
- **WHEN** the bound Kubernetes identity lacks required read permission
- **THEN** the target is recorded as incomplete with an actionable permission gap, not as having no policies

### Requirement: Preserve coverage and finding history
Collectors SHALL retain per-target success, failure, empty inventory and pagination completion, with observation times, versions and limitations. Empty complete inventories SHALL be distinguishable from failed enumeration and SHALL NOT become a technical pass. Findings SHALL have stable resource/condition identity and append-only lifecycle history; only fresh complete relevant observations SHALL resolve findings.

#### Scenario: Partial multi-account collection
- **WHEN** some accounts succeed and another is denied or unreachable
- **THEN** successful evidence remains available and the missing partition prevents a complete-coverage claim

#### Scenario: Previously failing resource disappears from a failed scan
- **WHEN** a subsequent enumeration fails or is incomplete
- **THEN** the previous finding remains unresolved rather than being closed because the resource was absent from the response

#### Scenario: Fresh observation confirms correction
- **WHEN** a complete relevant observation confirms the previously failing condition is corrected
- **THEN** a resolution event links the new evidence while preserving the original finding and evidence
