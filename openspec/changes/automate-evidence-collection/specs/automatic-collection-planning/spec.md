## ADDED Requirements

### Requirement: Automatically reconcile collection plans
The system SHALL create versioned collection plans and managed measures from the saved scope, pinned profile/catalog, registered templates and integrations without requiring manual measure entry. Mappings SHALL identify versions, rationale, supported requirement portions, policy inputs, and application defaults. Reconciliation SHALL preserve custom measures and overrides, and SHALL classify unsupported, manual, unresolved and not-applicable obligations explicitly.

#### Scenario: Service becomes ready for collection
- **WHEN** a service has both a valid scope and profile
- **THEN** the system durably reconciles a plan and queues initial supported collection without a user creating measures

#### Scenario: Mapping or policy is unavailable
- **WHEN** a catalog requirement has no verified mapping or an evaluation needs customer policy
- **THEN** the plan exposes the missing mapping or policy and still collects supported observations that do not require that policy

#### Scenario: Customized measure survives reconciliation
- **WHEN** scope, profile or template versions change after a customer customizes a measure
- **THEN** reconciliation preserves the customization and records new plan bindings without rewriting historical measures

### Requirement: Respect explicit collection boundaries
The system MUST restrict discovery, collection and refresh to saved account/region, cluster and repository targets. It MUST NOT expand scope based on available AWS regions, accessible profiles, kubeconfig contexts or local directories. Credentials and cluster bindings SHALL be operator-managed and target identities SHALL be verified.

#### Scenario: Ignis scope is us-east-1 only
- **WHEN** an initial, manual or scheduled collection runs against accounts scoped to us-east-1
- **THEN** no API calls target another AWS region and no additional region is saved

#### Scenario: Credential identity mismatch
- **WHEN** a configured profile or Kubernetes binding resolves to another account or cluster
- **THEN** collection for that target is blocked and the mismatch is visible without collecting from the unintended target

### Requirement: Durable scheduled and on-demand collection
The system SHALL provide asynchronous collect-now and durable periodic refresh, with configurable cadence, freshness, pause/resume and bounded backoff. The default cadence SHALL be 24 hours and freshness 48 hours, labeled application defaults. Scheduled execution SHALL use a service-scoped automation identity. Initial triggers and due runs SHALL survive restart and deduplicate across workers and retries.

#### Scenario: Duplicate workers and catch-up
- **WHEN** multiple workers process the same due schedule after a restart with several missed intervals
- **THEN** exactly one logical catch-up run is created for that entry and prior missed slots do not create an unbounded backlog

#### Scenario: Collection requested through MCP
- **WHEN** an authorized agent calls collect_now with an idempotency key
- **THEN** it promptly receives the batch ID and blocked-entry information, and an identical retry returns the same logical batch

#### Scenario: Paused or unauthorized collection
- **WHEN** scheduled collection is paused or an API caller lacks check permission
- **THEN** no new scheduled run or unauthorized requested run is started respectively, and configuration/history remain readable to authorized readers

### Requirement: Revision-safe execution and finalization
The system SHALL freeze scope, profile, measure and template versions per batch, revalidate execution boundaries, and atomically persist evidence links, findings, result state and audit. Retry finalization SHALL NOT duplicate logical observations. Obsolete-scope results SHALL remain historical and SHALL NOT contribute to current coverage.

#### Scenario: Scope changes during a run
- **WHEN** a run started under an older scope finishes after the scope changes
- **THEN** its observations retain their original binding and do not establish current-scope coverage

#### Scenario: Collection configuration changes
- **WHEN** policy inputs, freshness settings, or managed-measure overrides change
- **THEN** a new immutable configuration revision triggers a distinct plan, new runs freeze that revision, and old runs and evidence keep their original policy and measure bindings

#### Scenario: Artifact or transaction failure
- **WHEN** storage or database finalization fails
- **THEN** no published evidence references a missing artifact and a retry can complete without duplicate evidence or findings
