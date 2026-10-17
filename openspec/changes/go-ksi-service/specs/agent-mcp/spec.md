## ADDED Requirements

### Requirement: Authenticated MCP transport
The service SHALL expose MCP Streamable HTTP at `/mcp` using scoped, revocable bearer tokens stored as hashes. It SHALL apply the same domain authorization as the frontend and document bearer-header client configuration.

#### Scenario: Revoked token
- **WHEN** an MCP client connects with a revoked token
- **THEN** the service denies protected resource and tool access

### Requirement: Discoverable structured resources and tools
The MCP server SHALL expose catalog requirement and service requirement resources and tools named `list_requirements`, `get_requirement_status`, `list_evidence`, `get_evidence`, `run_check`, `get_check_run`, `attach_evidence`, `save_assessment_draft`, `create_gap`, and `prepare_publication`. Tool schemas SHALL describe required scope, revisions, and idempotency inputs; outputs SHALL include provenance and stable errors. Lists SHALL support bounded pagination.

#### Scenario: Agent discovers tools
- **WHEN** an authenticated compatible client requests the tool list
- **THEN** it receives the documented tool names and machine-readable input schemas

#### Scenario: Long-running check
- **WHEN** the agent invokes `run_check` for an authorized registered check
- **THEN** it receives a run ID and can retrieve progress and results using `get_check_run`

### Requirement: Agent contributions retain their identity
MCP mutations SHALL record the authenticated agent identity and SHALL preserve distinctions between supplied evidence references, collector observations, assessment drafts, and human reviews. Tool responses SHALL treat imported text as data, and tool arguments SHALL NOT grant execution authority to that text.

#### Scenario: Agent drafts an assessment
- **WHEN** an agent saves a cited draft
- **THEN** the system records an agent-authored revision and does not create a human review or collector result

#### Scenario: Agent requests reviewer action
- **WHEN** an agent credential attempts a human-review action through any application entry point
- **THEN** authorization denies the action

### Requirement: MCP and frontend share state
MCP tools SHALL use the same persisted entities, validation, optimistic concurrency, and audit behavior as frontend actions.

#### Scenario: End-to-end assessment
- **WHEN** an agent reads an applicable requirement, runs a check, retrieves evidence, and saves a draft
- **THEN** the frontend displays the same requirement version, run, evidence, draft, and gap state for human review
