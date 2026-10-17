## ADDED Requirements

### Requirement: Shared versioned source configuration
The system SHALL store service-owned versioned sources for repositories, AWS, Kubernetes, documents and operational-record systems. Configuration SHALL include locators, boundaries, non-secret access hints, enabled state, freshness policy and optional rule associations. The configuration UI and paginated `list_evidence_sources` MCP tool SHALL expose the same authorized records. Writes SHALL require edit permission and expected revisions.

#### Scenario: Configure a document source
- **WHEN** an editor saves a document collection in the configuration UI
- **THEN** an authorized agent can discover the same source ID, revision, locator and access hints over MCP without a browser session

#### Scenario: Concurrent configuration edit
- **WHEN** an editor saves an outdated source revision
- **THEN** the service returns a revision conflict and preserves the newer source configuration

### Requirement: Source boundaries and access declarations
Sources SHALL NOT expand saved AWS, Kubernetes or repository scope. Document and record sources SHALL have explicit configured boundaries. Access hints SHALL exclude credentials and credential-bearing URLs, and configured availability SHALL NOT be represented as verified agent access.

#### Scenario: Region outside saved scope
- **WHEN** a source edit names an AWS region outside the saved service boundary
- **THEN** the service rejects the edit and preserves the configured boundary

#### Scenario: Agent cannot access a source
- **WHEN** an agent discovers a configured source but its own credentials cannot read it
- **THEN** the workflow allows a specific blocked result without treating source configuration as successful collection

### Requirement: Existing scope becomes discoverable sources
The system SHALL idempotently expose existing scoped accounts, clusters and repositories as sources while preserving edited source metadata and all historical references. Disabled or narrowed sources SHALL remain available as historical provenance and cease authorizing new current-scope collection outside their revised boundaries.

#### Scenario: Reconcile existing service twice
- **WHEN** source reconciliation runs repeatedly for the same saved scope
- **THEN** it produces stable source identities without duplicates or overwriting customer access hints

#### Scenario: Disable a cited source
- **WHEN** a currently cited source is disabled
- **THEN** its historical artifact remains readable to authorized users and affected current evidence work is marked for refresh
