## ADDED Requirements

### Requirement: Bounded attributed discovery
The system SHALL support resumable, paginated server-run and agent-submitted discovery sessions bound to connection generation, actor, provider identity and resource-kind coverage. Results SHALL expose timestamps, execution mode, next cursor, partial errors and limitations. Default pages SHALL contain at most 25 resources and requested pages SHALL be capped at 100.

#### Scenario: Provider has hundreds of resources
- **WHEN** discovery spans multiple pages
- **THEN** UI and MCP can resume using scoped cursors without loading every resource or restarting completed pages

#### Scenario: Agent discovers resources with its own tools
- **WHEN** an authorized agent submits valid pages with method, identity and observation metadata
- **THEN** resources appear as agent-reported inventory with durable receipts and are not mislabeled as server-verified access

### Requirement: Stable identity and idempotent reconciliation
Discovery SHALL use stable provider resource identities rather than display names where available. Repeated unchanged results SHALL NOT create duplicate sources or source revisions. Page replay SHALL be idempotent; reuse of a page idempotency key with different content SHALL return a conflict. Rebinding a resource to another connection SHALL be explicit.

#### Scenario: GitHub repository is renamed
- **WHEN** discovery returns the same repository ID under a new name
- **THEN** the existing source identity is retained and the new display metadata does not create a second source

#### Scenario: Same resource is found through another connection
- **WHEN** a second connection discovers a resource already selected in the service
- **THEN** the result identifies the existing source and does not silently change its credential path

### Requirement: Discovery does not silently widen assessment scope
Discovering a resource SHALL NOT automatically select it, expand scope or complete evidence work. Selection SHALL validate resource identity, provider boundaries, connection/source revisions and expected scope revision. Authorized actors with scope-edit permission SHALL be able to explicitly apply the requested scope additions atomically with source selection, receiving the scope delta and resulting revisions. AWS restrictions and GitHub commit/path pinning SHALL remain enforced.

#### Scenario: Select discovered repositories
- **WHEN** a user or agent selects discovered repositories with full commit hashes and paths and explicitly requests permitted scope additions
- **THEN** selection and scope changes commit together or return a revision/validation error without partial activation

#### Scenario: Discovery exceeds allowed scope
- **WHEN** a provider lists resources outside the current scope and the caller has no scope-edit capability
- **THEN** those resources are shown as outside scope and cannot become active evidence sources through discovery alone

#### Scenario: Workspace and standalone Kubernetes sources
- **WHEN** an authorized actor selects supported Workspace customer resources or standalone cluster/namespace resources
- **THEN** additive scope entries bind the actual provider identity without inventing repository or AWS account identifiers

### Requirement: Partial results do not remove successful sources
Discovery SHALL retain successful pages and last-known inventory when an API call is denied, throttled or interrupted. Only complete authoritative enumeration of a resource kind SHALL mark unseen resources unavailable, and unavailable resources SHALL retain history. Retired resources SHALL remain suppressed until explicit restoration.

#### Scenario: Throttling interrupts repository enumeration
- **WHEN** some repository pages succeed and a later page is rate-limited
- **THEN** discovery reports partial coverage and retry guidance without marking unvisited repositories deleted or unavailable

#### Scenario: Complete scan no longer sees a source
- **WHEN** a complete authoritative scan no longer includes a previously selected resource
- **THEN** the source is labeled unavailable with its last-seen time, and historical artifacts remain readable

### Requirement: Freshness and discovery are independent
Inventory updates SHALL NOT reset evidence freshness, assessment review status or source policy. Selecting or editing a source SHALL apply existing immutable source and scope revision semantics; access-only observations SHALL remain operational metadata.

#### Scenario: Rediscover stale source
- **WHEN** inventory confirms that a source still exists but its supporting evidence is expired
- **THEN** its inventory check is updated while its evidence continues to require refresh
