## ADDED Requirements

### Requirement: Direct documentary artifact ingestion
The system SHALL expose `submit_evidence_artifact` for UTF-8 text, Markdown and JSON artifacts up to 1 MiB of content within the 2 MiB MCP request limit. Submissions SHALL include source/revision, locator/version, scope binding, observation/event and retrieval times, method, description and limitations. Server-owned actor, hash and receipt time SHALL preserve agent-authored provenance. An external URL and collector measure SHALL NOT be required.

#### Scenario: Local repository evidence
- **WHEN** an authorized agent submits a scoped repository excerpt with commit, path and source revision
- **THEN** the service stores the artifact and provenance and returns a retrievable immutable evidence ID/revision and hash

#### Scenario: Oversized or unsupported content
- **WHEN** an agent submits content beyond the limit or an unsupported media type
- **THEN** the service returns an actionable validation error without silently truncating or creating a completed rule result

#### Scenario: Failed persistence
- **WHEN** artifact storage or evidence transaction persistence fails
- **THEN** no evidence record references an unavailable artifact and a retry cannot create duplicate evidence

### Requirement: Citation-backed rule results
The system SHALL expose `submit_rule_result` with exact instruction/profile/scope/source bindings, expected result and draft revisions, per-assertion outcomes and rationale, evidence revision citations, blockers and requested work state. Text citations SHALL resolve line ranges and JSON citations SHALL resolve JSON Pointers against stored content. Direct requirement/assertion links SHALL support submissions without generated collector measures.

#### Scenario: Complete cited evidence work
- **WHEN** an agent submits current evidence and valid citations for all required assertions with no unknown assertion, unresolved applicability or blocker
- **THEN** the service saves an immutable result and cited draft and returns a completion receipt with evidence references, result/draft revisions, computed work state, assessment outcome and separate review status

#### Scenario: Missing required assertion
- **WHEN** an agent requests complete status while a required assertion lacks resolvable citations
- **THEN** the service rejects completion with the missing assertion details and does not check the rule off

#### Scenario: Blocked investigation
- **WHEN** an agent submits concrete source-access blockers and no evidence
- **THEN** the service accepts a blocked result, records its rationale and leaves the rule in open-work discovery

### Requirement: Authorized idempotent revision-safe submissions
Submissions SHALL require service-scoped edit permission, enforce source and target boundaries, deduplicate canonical identical requests, and conflict on changed payloads sharing a key. The service SHALL revalidate result bindings and optimistic revisions transactionally before crediting current completion. Stale artifacts SHALL retain their original provenance without creating current completion.

#### Scenario: Retry after lost receipt
- **WHEN** an agent retries the identical submission with the same idempotency key
- **THEN** it receives the original receipt without duplicated artifacts, results or drafts

#### Scenario: Parallel agents or changed scope
- **WHEN** another agent updates the result/draft or the service scope changes before submission commits
- **THEN** the stale result receives an actionable conflict and cannot overwrite the newer state or close current work

#### Scenario: Read-only or cross-service submission
- **WHEN** an agent submits with only read permission or references another service's evidence
- **THEN** the service denies the operation without leaking or linking unauthorized artifacts

### Requirement: Existing collector provenance remains intact
The general evidence workflow SHALL coexist with typed observation ingestion, deterministic collector findings and external references, preserving their distinct provenance and exact revisions. General agent submissions SHALL NOT claim collector verification or human approval.

#### Scenario: Reuse technical and documentary evidence
- **WHEN** a rule result cites a collector observation and an agent-submitted policy excerpt
- **THEN** both are linked with their original provenance and neither is relabeled as human reviewed

#### Scenario: Reuse documentary evidence across requirements
- **WHEN** two rule results cite the same service-owned documentary evidence revision
- **THEN** both drafts and requirement evidence lists resolve the explicit rule links without duplicating the artifact or changing its provenance
