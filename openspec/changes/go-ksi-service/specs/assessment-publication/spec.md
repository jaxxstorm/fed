## ADDED Requirements

### Requirement: Immutable assessment bundles
The system SHALL prepare immutable versioned assessment snapshots and render human-readable HTML and application JSON from the same snapshot. Bundles SHALL include scope/profile/catalog versions, measures, evidence provenance and references, freshness, limitations, gaps, and review attribution. They SHALL be labeled as application assessment bundles rather than complete certification packages or schema-validated official SDRs.

#### Scenario: Records change after export
- **WHEN** an assessment changes after a snapshot is prepared
- **THEN** both formats of the original snapshot retain their original content and later changes require a new snapshot

### Requirement: Authorized artifact access
Export downloads and referenced artifact retrieval SHALL enforce the same service permissions as their source records. Published bundles SHALL exclude integration credentials and preserve relevant evidence limitations.

#### Scenario: Unauthorized download
- **WHEN** a caller outside the service scope requests a snapshot or artifact
- **THEN** access is denied even if the caller knows its identifier

### Requirement: Vendor-neutral trust-centre integration
The initial service SHALL accept manually registered external evidence references with supplied metadata and verification state. It SHALL expose a vendor-neutral adapter interface and capability information and return `integration_unavailable` for unsupported live operations. Core assessment, evidence storage, and export functionality SHALL operate without a trust-centre account or vendor API. Vendor-specific configuration and external IDs SHALL remain isolated from core assessment identifiers.

#### Scenario: No trust-centre adapter is configured
- **WHEN** a user registers an external document URL
- **THEN** the service retains the reference as unverified external evidence and remains usable without API access

#### Scenario: Unsupported publication
- **WHEN** a caller attempts live trust-centre publication without a supported configured adapter
- **THEN** the service reports `integration_unavailable` and does not mark the bundle published

### Requirement: Publication state and history
The system SHALL distinguish prepared, exported, published, and failed states and retain publication attempts, destinations, timestamps, errors, and external receipts when available. Downloading a bundle SHALL NOT imply successful external publication. Changing the destination SHALL preserve internal assessment, evidence, snapshot, and publication history.

#### Scenario: Manual export
- **WHEN** a user downloads a prepared assessment bundle
- **THEN** the system records an export event while leaving external publication unconfirmed

#### Scenario: Destination changes
- **WHEN** an administrator selects a different trust-centre destination
- **THEN** existing assessment and evidence identifiers remain unchanged and prior publication attempts retain their original destination
