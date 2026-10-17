## ADDED Requirements

### Requirement: Distinct connections and evidence sources
The system SHALL represent service-owned provider connections separately from child evidence sources. Connections SHALL identify provider account, access mode, configuration revision and credential reference; sources SHALL identify distinct provider resources, assessment scope and evidence policy. Configuration, access verification, discovery completeness and evidence completion SHALL remain separate states.

#### Scenario: One connection has many sources
- **WHEN** a GitHub connection exposes several repositories
- **THEN** each selected repository is represented as a distinct source under the connection while authorization and renewal remain connection-level operations

#### Scenario: Configuration is not verification
- **WHEN** a connection is saved without a successful provider access check
- **THEN** it is labeled not yet verified or agent-reported as applicable, rather than connected with verified server access

### Requirement: Authorized revision-aware lifecycle
The system SHALL support create, configure, test, renew, disconnect and delete operations for connections and configure, disable, refresh, retire and explicit restore operations for sources. Shared domain services SHALL enforce service authorization, expected revisions, idempotency and audit attribution for both web and MCP callers. Routine agent actions SHALL use the same operations without an additional approval workflow; provider-required consent SHALL remain explicit.

#### Scenario: Conflicting edits
- **WHEN** a user or agent updates a connection using an obsolete revision
- **THEN** the operation returns a conflict with the current revision and leaves the newer configuration intact

#### Scenario: Restricted agent token
- **WHEN** an agent with only existing read/edit permissions attempts connection credential management without the connect capability
- **THEN** the operation is denied without exposing credentials or changing the connection

### Requirement: Renewal preserves evidence age
Credential renewal SHALL refresh access or provide an actionable reauthorization step without rewriting evidence observation times, extending evidence freshness or changing source revisions solely because a credential rotated. Evidence refresh SHALL create new immutable artifacts when supported and SHALL be distinguished from inventory rediscovery.

#### Scenario: Token rotates after review
- **WHEN** a valid connection renews its access token without changing resource or assessment bindings
- **THEN** existing evidence and review history retain their exact revisions and freshness deadlines

#### Scenario: Source evidence has expired
- **WHEN** an operator renews credentials for a source whose evidence is stale
- **THEN** the evidence remains stale until new support is gathered and assessed

### Requirement: Deletion preserves history and prevents resurrection
Connection deletion SHALL retire its local source configuration, disable credential use, fence in-flight operations and remove local connection secrets while preserving non-secret history and citations. Source deletion SHALL create a durable retirement marker. Neither operation SHALL delete provider resources. Reconciliation and discovery SHALL NOT recreate retired configuration without explicit restoration. Remote revocation outcomes SHALL be reported separately from local disconnection.

#### Scenario: Deleted source remains in scope
- **WHEN** a retired repository is still present in the saved scope and a later reconciliation or discovery sees it
- **THEN** it remains retired and its existing artifacts and exports remain retrievable

#### Scenario: Callback arrives after deletion
- **WHEN** a provider callback or discovery page arrives for a deleted or superseded connection generation
- **THEN** it is rejected without restoring access, sources or credentials

#### Scenario: Remote revocation fails
- **WHEN** local disconnection succeeds but provider token revocation is unavailable
- **THEN** local use stays blocked and the result explains the outstanding provider-side action without claiming remote revocation succeeded

### Requirement: Credential isolation
Provider secrets SHALL be encrypted or externally referenced outside versioned evidence records. Application responses, MCP tools, logs, discovery receipts and exports SHALL NOT disclose access tokens, refresh tokens, private keys or kubeconfig credentials. Credentials SHALL be resolved only for the authorized service and active connection generation.

#### Scenario: Metadata inspection
- **WHEN** an agent lists connections or a user downloads an assessment export
- **THEN** only non-secret identity, configuration and provenance metadata is included

### Requirement: Compatible source migration
Migration SHALL preserve existing evidence source IDs and revisions, artifact content, source freshness policies and immutable export contents. Imported sources SHALL be attached to explicitly labeled connections with unknown access until checked. Existing MCP evidence tools and source deep links SHALL remain usable.

#### Scenario: Previously cited source is imported
- **WHEN** an existing source is grouped under an imported connection without changing its evidence semantics
- **THEN** its existing citation resolves to the same revision and artifact and no credential verification is inferred from the grouping
