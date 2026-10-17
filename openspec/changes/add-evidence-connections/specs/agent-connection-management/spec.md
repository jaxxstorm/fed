## ADDED Requirements

### Requirement: Discoverable provider setup instructions
MCP SHALL expose provider catalogs and versioned setup instructions for GitHub, Google Workspace, AWS and Kubernetes, including supported access modes, resource types, permission requirements and available lifecycle/discovery tools. Instructions SHALL distinguish server-executed operations from agent-owned tool use and provider-required browser consent.

#### Scenario: Agent is asked to connect AWS
- **WHEN** the agent requests AWS setup instructions
- **THEN** it receives the profile/role or agent-managed options, account verification steps, us-east-1 restriction and tools needed to configure, test, discover and select sources

### Requirement: Web and agent management parity
Authorized agents SHALL be able to list, configure, test, renew, disconnect and delete connections, discover/select resources, configure/retire sources and request evidence refresh through shared domain services. Mutations SHALL enforce the same service boundary, capabilities, expected revisions and idempotency as the browser and SHALL return durable operation status and resulting revisions.

#### Scenario: Agent creates and selects sources
- **WHEN** a connect/edit-scoped agent configures a permitted connection and selects discovered resources
- **THEN** the resulting connection, source configuration and scope revisions appear immediately in the web workspace with agent attribution

#### Scenario: Token is revoked during discovery
- **WHEN** an agent loses authorization before submitting or completing discovery work
- **THEN** the operation fails authorization and does not activate sources or persist credentials under the revoked grant

### Requirement: Provider authorization handoff
For providers requiring interactive consent, MCP SHALL return a short-lived same-origin authorization handoff and an operation ID. Completion SHALL require an authorized browser actor and validated provider callback. Agents SHALL be able to query completion and continue discovery without receiving provider secrets or treating an unfinished handoff as connected.

#### Scenario: Agent requests GitHub authorization
- **WHEN** the agent starts GitHub connection authorization
- **THEN** it receives instructions and a handoff URL, waits for a recorded authorization outcome, and can discover sources only with the resulting access mode and permissions

### Requirement: External discovery preserves provenance
An agent SHALL be able to use its own tools and credentials to submit bounded discovered-resource pages with stable identities, observation time, method and limitations. The system SHALL validate schemas and scope, return idempotent receipts and distinguish agent-reported access from server verification. Raw credentials SHALL NOT be required in discovery submissions.

#### Scenario: Agent enumerates a namespace-restricted cluster
- **WHEN** the agent submits readable namespaces and reports that cluster-wide listing is denied
- **THEN** the inventory retains those attributed results as partial and does not infer access to other namespaces

### Requirement: Existing agent evidence workflow remains compatible
Existing source-listing, rule-instruction, evidence-artifact and rule-result tools SHALL retain their established fields and immutable citation behavior. Connection information SHALL be additive. Lifecycle-aware listing/reconciliation SHALL honor retired sources while preserving exact historical references. Agents using their own credentials SHALL remain able to submit scoped evidence without a server-managed provider grant.

#### Scenario: Old client submits documentary evidence
- **WHEN** an existing client submits valid evidence against an active legacy source using the current artifact contract
- **THEN** the artifact is accepted with existing provenance/freshness semantics and its source is discoverable in the new connection hierarchy

#### Scenario: Agent requests evidence refresh without a server collector
- **WHEN** refresh is requested for an agent-managed source
- **THEN** the response provides a scoped investigation instruction and required bindings rather than claiming server collection or an agent session has started
