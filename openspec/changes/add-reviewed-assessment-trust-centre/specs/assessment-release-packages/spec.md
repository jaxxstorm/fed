## ADDED Requirements

### Requirement: Complete traceable release manifest
Fed SHALL prepare a versioned manifest binding service description, contacts, boundary, responsibilities, pinned catalog/profile, applicability and exclusions, reporting period, implementation narratives, assertion conclusions, collection methods, evidence citations/hashes, reviews, issues, supporting deliverables, and prior-release comparison. Required content SHALL derive from explicit pinned applicability and unresolved mappings SHALL remain visible. A collection completion ratio MUST NOT be represented as overall evidence completeness or compliance.

#### Scenario: Collection is complete but operational evidence is missing
- **WHEN** all scheduled observations succeeded but operating assertions remain unknown
- **THEN** the package reports successful collection separately from incomplete evidence and assessment conclusions

### Requirement: Portable verifiable export
Fed SHALL generate deterministic JSON, readable offline HTML, and an authorized evidence-inclusive archive from the same frozen manifest. The archive SHALL include schema and artifact verification metadata with local references. Missing, corrupt, or oversized included artifacts MUST produce explicit preparation failure rather than silent omission. A content hash MUST NOT be labeled a cryptographic assessor signature.

#### Scenario: Offline recipient verifies evidence
- **WHEN** a recipient opens the exported archive without network access
- **THEN** the narrative, citations, included artifacts, and integrity manifest are usable without application credentials or live URLs

#### Scenario: Artifact integrity failure
- **WHEN** an included artifact is unavailable or does not match its recorded hash
- **THEN** preparation fails with the affected reference and no approved or published release is created

### Requirement: Immutable release lifecycle
Fed SHALL distinguish draft, prepared, approved, published, and withdrawn releases. Prepared bytes MUST remain immutable; content changes SHALL create a new release. Publication MUST require human adoption/review of included assessment content, verifiable artifacts, and approval of the exact manifest and audience projection. Explicitly incomplete reviewed reports SHALL remain labeled incomplete. Publication SHALL record a durable receipt and idempotent retries SHALL not duplicate releases.

#### Scenario: New collection follows publication
- **WHEN** evidence changes after a release was published
- **THEN** that release and its approval remain unchanged while a candidate successor can reference the new evidence

### Requirement: Defensible current finding counts
Fed SHALL use a shared browser/MCP/package projection with stable resource identity, condition deduplication, temporal coverage, and explicit denominator definitions. Absent resources SHALL be retired from current counts only following authoritative complete inventory with matching scope, profile, partition, and credential-generation fencing. Historical findings MUST remain inspectable.

#### Scenario: Incomplete inventory omits workloads
- **WHEN** a later inventory is incomplete and lacks previously observed workloads
- **THEN** their absence does not clear findings and the current-resource summary reports uncertainty

#### Scenario: Authoritative inventory confirms retirement
- **WHEN** a complete compatible inventory confirms a resource is absent
- **THEN** its findings leave current-resource counts with a lifecycle explanation while historical records remain available

### Requirement: Recurring reporting candidates
Fed SHALL support on-demand and scheduled candidate reporting periods with categorized changes since a selected previous release and in-product owner notifications. Candidate generation MUST NOT automatically approve, publish, or invoke an external agent. Scheduled retries SHALL be idempotent.

#### Scenario: Scheduled report detects expired support
- **WHEN** a new reporting cycle finds expired evidence underlying a previous conclusion
- **THEN** it proposes a candidate with freshness changes and evidence/re-review work while leaving the published release unchanged
