## ADDED Requirements

### Requirement: Service-scoped measures and evidence
The system SHALL maintain explicit service scope revisions and versioned customer measures linked to official requirements. Evidence SHALL carry source, scope, timestamps, collector or submitting actor, content hash or external reference, verification state, and limitations. Customer-defined measures SHALL be labeled separately from official wording.

#### Scenario: Evidence supports a measure
- **WHEN** an editor attaches evidence to a measure
- **THEN** the system records provenance and scope and exposes the relationship without changing the official requirement text

### Requirement: Independent status dimensions
The system SHALL distinguish technical pass/fail/unknown/error, evidence current/stale/missing, and human pending/reviewed/review-needed states. Freshness SHALL use explicit measure policies, not invented FedRAMP-wide thresholds.

#### Scenario: Evidence expires
- **WHEN** evidence supporting a reviewed assessment exceeds its configured freshness policy
- **THEN** the current view marks the evidence stale and the review as needing attention while preserving the historical review

#### Scenario: Automated check passes
- **WHEN** an evaluator produces a passing result
- **THEN** the system updates that check result without automatically marking the KSI reviewed or the service certified

### Requirement: Cited drafts and revision-specific human reviews
The system SHALL accept assessment drafts citing existing evidence versions in the same authorized service, require an expected revision for updates, and record human reviews against an exact assessment revision. It SHALL reject unsupported evidence references and agent attempts to create human reviews.

#### Scenario: Concurrent edit
- **WHEN** an agent saves a draft using an outdated expected revision
- **THEN** the system returns a revision conflict without overwriting the newer draft

#### Scenario: Human review
- **WHEN** an authorized human checks the review control
- **THEN** the system stores that person's identity, timestamp, notes, and assessment revision

#### Scenario: Invalid citation
- **WHEN** a draft cites nonexistent evidence or evidence belonging to another service
- **THEN** the system rejects the draft without saving those references

### Requirement: Shared identity and authorization
The system SHALL enforce organization, service, and action permissions in domain services for browser and MCP callers, audit mutations transactionally, authenticate browser users with OIDC, and protect session-based mutations against CSRF. Development identity bypass SHALL require explicit configuration and loopback binding.

#### Scenario: Cross-service access
- **WHEN** a caller requests a record outside its permitted service scope
- **THEN** both frontend and MCP entry points deny access without exposing the record

#### Scenario: Unauthenticated production startup
- **WHEN** deployment configuration lacks required authentication settings and local development identity is not explicitly enabled
- **THEN** the application fails startup with a configuration error

### Requirement: Class-aware dashboard
The frontend SHALL provide service/profile selection, KSI and broader-obligation views, requirement details, measures, evidence, run status, editable drafts, human review controls, and gap filters. It SHALL expose source versions, freshness, unresolved applicability, and collection coverage without presenting checklist completion as official certification.

#### Scenario: Agent saves a draft
- **WHEN** an authorized agent saves an assessment draft through MCP
- **THEN** a human viewing the same requirement sees that draft, its citations, actor attribution, and pending review state
