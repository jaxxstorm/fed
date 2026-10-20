## ADDED Requirements

### Requirement: First-party public trust centre
Fed SHALL serve a service overview and approved published-release index through its own trust-centre routes. Public fields and narratives MUST be explicitly selected, previewed, and approved. Drafts, internal notes, raw evidence, and restricted identifiers MUST NOT be exposed through public serialization, search, links, or downloads by default.

#### Scenario: Public visitor opens a service
- **WHEN** an unauthenticated visitor opens the trust centre
- **THEN** only the approved public projection and publicly published releases are visible
- **AND** internal workspace records and raw artifacts are inaccessible

### Requirement: Scoped restricted-reader access
Fed SHALL support verified-identity access requests and administrator-managed service/release-scoped reader and assessor grants with expiry and revocation. External-reader approval MUST NOT grant organization-wide membership. Every restricted route and artifact/export request SHALL enforce the current grant; restricted responses MUST avoid public caching and long-lived bearer download URLs.

#### Scenario: Reader attempts another service
- **WHEN** a reader granted access to one service/release requests another service or ungranted release
- **THEN** access is denied without revealing restricted content

#### Scenario: Access is revoked
- **WHEN** an administrator revokes or expires a reader grant
- **THEN** subsequent restricted page, artifact, and export requests fail even if the identity still has an active login session

#### Scenario: External identity cannot authenticate
- **WHEN** a requester is unsupported by the configured issuer or identity policy
- **THEN** the product explains the access limitation without bypassing admission restrictions or granting organization-wide access

### Requirement: Atomic publication and withdrawal
Fed SHALL update the published index and first-party publication receipt atomically after approved content is available. The latest pointer MUST select a published release, never a draft. Withdrawal SHALL remove external delivery and preserve an appropriately redacted tombstone, internal bytes, decisions, and audit history. The publication preview SHALL disclose that previously downloaded public bytes cannot be recalled.

#### Scenario: Publication retries after interruption
- **WHEN** a publisher retries a publication with the same idempotency key after interruption
- **THEN** one consistent publication receipt and index entry exist, or a clear retryable failure is returned

#### Scenario: Release is withdrawn
- **WHEN** an authorized publisher withdraws a release
- **THEN** public and restricted delivery of that release stops, its index entry becomes a redacted tombstone, and internal history remains available

### Requirement: Readable shared assessment context
Fed SHALL show implementation claims, supporting evidence context, operational period, limitations, review capacity, and open issues using the same release manifest in trust-centre views and exports. Access-blocked, missing, stale, partial, and contradictory evidence SHALL remain distinct from technical findings. Detailed diagnostics SHALL be expandable rather than replacing plain-language explanations.

#### Scenario: Reader inspects a requirement
- **WHEN** a reader opens a requirement in a published release
- **THEN** the reader can distinguish what the service does, what evidence supports it, what remains unproven, and who reviewed it without interpreting raw collector output
