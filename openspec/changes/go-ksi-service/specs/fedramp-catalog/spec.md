## ADDED Requirements

### Requirement: Versioned authoritative catalog
The system SHALL import official FedRAMP Rules JSON using a pinned upstream commit and matching schema, preserve its source payload and hash, and retain IDs, variants, normative strength, conditions, dates, dependencies, and narrative references. Invalid imports SHALL leave the active catalog unchanged.

#### Scenario: Valid import
- **WHEN** an administrator imports schema-valid data from a pinned source
- **THEN** the system creates an immutable catalog version with traceable source metadata

#### Scenario: Invalid import
- **WHEN** an import fails schema validation
- **THEN** the system reports validation errors without replacing existing catalog data

### Requirement: Profile-aware applicability
The system SHALL resolve requirements for type, path, class, evaluation date, and initial or ongoing context, including referenced obligations. It SHALL distinguish required, recommended, optional, not applicable, and unresolved expectations and preserve conditional variants.

#### Scenario: Class A references another ruleset
- **WHEN** a Class A profile includes a mandatory reference to a requirement in another ruleset
- **THEN** the resolved requirement list includes that obligation with its source relationship

#### Scenario: Applicability cannot be resolved
- **WHEN** a source condition or pilot-dependent expectation cannot be resolved from the profile
- **THEN** the system presents it as unresolved with the source condition instead of excluding or satisfying it

### Requirement: Profile revisions preserve history
The system SHALL pin assessments to catalog and profile revisions and SHALL preview changed obligations before changing a service profile.

#### Scenario: Class changes
- **WHEN** a user changes a service from Class B to Class C
- **THEN** the system creates a profile revision, shows changed expectations, retains reusable evidence, and preserves historical assessments against their original profile
