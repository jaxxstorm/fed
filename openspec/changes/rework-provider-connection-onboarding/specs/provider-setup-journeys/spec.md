## ADDED Requirements

### Requirement: Durable resumable provider setup
Fed SHALL provide service-owned setup sessions with expected revisions, idempotency, progress, expiry, cancellation and one-time finalization. Initial unverified provider identity SHALL remain in setup state rather than creating a collectable connection. Setup SHALL expire after 24 hours; provider authorization attempts SHALL retain their shorter expiry.

#### Scenario: Resume after navigation
- **WHEN** an authorized actor returns to an unexpired setup
- **THEN** completed steps and sanitized inputs are restored and a repeated completion returns the same resulting connection

#### Scenario: Expired setup
- **WHEN** a setup expires
- **THEN** staged grants are removed, completion is rejected, and the actor receives an explicit restart action without new sources or evidence

### Requirement: Browser and agent parity
Provider readiness, setup creation/update/status, generated AWS configuration, verification and completion SHALL use shared authorized services exposed through browser APIs and MCP. Browser mutations SHALL enforce CSRF. Setup mutations/material SHALL require connect permission, scope selection SHALL additionally require edit, and provider consent SHALL use a same-origin authorized browser handoff.

#### Scenario: Agent initiates consent
- **WHEN** a connect-authorized agent begins Workspace or GitHub setup
- **THEN** it receives a short-lived same-origin handoff and can subsequently inspect redacted status without receiving provider tokens

#### Scenario: Permission revoked during setup
- **WHEN** the initiating token or completing actor loses the required permission before completion
- **THEN** finalization is denied even if the provider redirect succeeds

### Requirement: Actionable provider-specific wizard
Each standard provider journey SHALL show prerequisites, numbered steps, back/resume controls, permission explanations, pending operation state, verified identity and actionable error recovery. AWS SHALL expose equivalent CLI/manual setup with copy/download controls; provider consent SHALL return to the correct service and setup. The interface SHALL preserve form inputs on conflicts and remain keyboard accessible at desktop, tablet and mobile sizes.

#### Scenario: Revision conflict or denied consent
- **WHEN** an operation fails due to concurrent configuration change or denied authorization
- **THEN** entered non-secret values remain available and the wizard explains what to review and how to retry without claiming success

### Requirement: Deployment and access boundaries are explicit
Operator documentation SHALL identify the task-role versus execution-role responsibilities, Google OAuth and GitHub App registrations, private-browser connectivity, server outbound dependencies and exact production callbacks for fedramp.ignistech.io. UI readiness SHALL expose configuration names and remedies without secrets.

#### Scenario: Private-origin consent
- **WHEN** a user follows a provider callback to the private Fed hostname
- **THEN** browser access through the approved private path is documented as required, and no public webhook or public ALB is implied by this onboarding flow

### Requirement: Preserve connection and evidence lifecycle
The onboarding rework SHALL preserve existing source IDs, immutable revisions/citations, encrypted credentials, tenant boundaries, retirement tombstones and generation fencing. Connect, test, renew and discover SHALL remain distinct from evidence collection and assessment approval.

#### Scenario: Disconnect races with completion
- **WHEN** a connection is disconnected, deleted or reconfigured while a setup callback is in flight
- **THEN** stale work cannot activate credentials or sources, and evidence and review history remain unchanged

#### Scenario: Existing connection upgrade
- **WHEN** the new schema and UI are deployed
- **THEN** legacy profile, manual, agent-managed and Kubernetes connections remain usable through their existing explicit modes and historical evidence/export payloads remain unchanged
