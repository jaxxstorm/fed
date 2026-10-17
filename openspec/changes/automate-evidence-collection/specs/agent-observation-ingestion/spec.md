## ADDED Requirements

### Requirement: Ingest attributed observation artifacts through MCP
The system SHALL expose ingest_observation through shared domain services, accepting bounded JSON artifacts, service/scope references, target identity, observation timestamp, declared method/version, limitations, requirement/measure links and an idempotency key. It SHALL persist artifacts and evidence without an external URL or assessment draft and return immutable evidence references and server-computed hashes. Serialized artifact size SHALL be limited to 1 MiB within the transport request limit.

#### Scenario: Agent submits an RDS observation
- **WHEN** an authorized agent submits a valid in-scope RDS configuration artifact and links
- **THEN** the system stores the artifact, records agent attribution and receipt time, and returns evidence references immediately usable by status queries and drafts

#### Scenario: Payload is too large or malformed
- **WHEN** an artifact exceeds the size limit or fails its declared schema
- **THEN** ingestion returns a stable validation/size error without partial evidence publication or silent truncation

### Requirement: Validate provenance and scope without promoting trust
The system SHALL assign source kind, actor identity, receipt timestamp and content hash server-side. Agent observations MUST remain distinguishable from collector observations and unverified external references. It SHALL validate target membership, exact links and timestamps; historical references SHALL NOT establish current coverage. Agent-reported evaluations SHALL be labeled assertions unless a registered evaluator evaluates the structured artifact, while retaining agent source provenance.

#### Scenario: Caller attempts to spoof collector provenance
- **WHEN** an agent supplies a collector verification state or another actor identity
- **THEN** the request is rejected or those caller-controlled provenance fields are excluded by schema, and no server-collector observation is created

#### Scenario: Target is outside the boundary
- **WHEN** an agent submits evidence for another service, an unsaved AWS region, cluster or repository
- **THEN** ingestion is denied with a scope/authorization error and no evidence is published

#### Scenario: Old or future observation
- **WHEN** evidence has an old observation time or a timestamp beyond the documented future tolerance
- **THEN** old evidence is retained with truthful freshness and future-invalid evidence is rejected rather than treated as newly observed

### Requirement: Idempotent ingestion and authorized artifact retrieval
The system SHALL deduplicate ingestion using service, actor, operation, idempotency key and canonical request digest, including concurrent submissions. It SHALL expose bounded artifact retrieval through MCP and browser routes with the same service authorization as metadata. Artifact content SHALL be treated as data, never executable instructions or trusted markup.

#### Scenario: Concurrent retry
- **WHEN** identical submissions with the same key arrive concurrently
- **THEN** they return the same evidence references and produce one logical ingestion/audit result

#### Scenario: Key reused with different data
- **WHEN** a caller reuses an ingestion key with a different artifact or binding
- **THEN** the system returns revision_conflict and preserves the original evidence

#### Scenario: Cross-service or revoked-token artifact access
- **WHEN** a caller lacks access to the evidence service or uses a revoked token
- **THEN** metadata and artifact retrieval are denied even if the caller knows the artifact hash
