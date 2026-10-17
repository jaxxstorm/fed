## ADDED Requirements

### Requirement: Show evidence independently of drafts
The system SHALL derive current evidence availability and freshness from scoped observation links and collection-plan bindings independently of assessment citations. It SHALL expose technical evaluation, coverage, evidence freshness and human review as separate states, while preserving the exact evidence supporting historical drafts/reviews.

#### Scenario: Collection finishes before any draft exists
- **WHEN** a collector or agent ingestion publishes current mapped evidence for a requirement with no draft
- **THEN** dashboard and MCP show that evidence, its freshness, findings and coverage while human review remains pending

#### Scenario: One passing result alongside missing evidence
- **WHEN** a requirement has one passing plan entry and another missing, stale or failed entry
- **THEN** aggregate status exposes the incomplete coverage and does not present the passing entry as complete requirement satisfaction

### Requirement: Explain automation eligibility and blockers
The dashboard and MCP SHALL distinguish runnable collection, credential/binding blockers, policy-required evaluation, manual-evidence obligations, unsupported automation, unresolved applicability and not-applicable requirements. Administrative obligations SHALL NOT be represented as infrastructure checks by default.

#### Scenario: FedRAMP inbox obligation
- **WHEN** an applicable inbox-maintenance obligation has a manual-evidence mapping
- **THEN** the interface displays manual evidence required with guidance rather than implying an AWS scan can satisfy it

#### Scenario: Requirement is not applicable
- **WHEN** the pinned profile excludes a requirement
- **THEN** it is labeled not applicable without a misleading missing-collection alert

### Requirement: Provide collection overview and drill-down
The system SHALL expose service collection-plan and batch views, last/next collection, schedule state, collect-now and pause/resume, plus resource-level findings, provenance, coverage denominators, timestamps, artifact access and actionable failures. MCP SHALL provide get_collection_plan, collect_now, get_collection_batch and configure_collection through the same authorization and revision rules as the frontend.

#### Scenario: Credential gap blocks a target
- **WHEN** the worker cannot authenticate an Audit or Log Archive target
- **THEN** the overview identifies the target and required integration/access while preserving other targets' completed evidence

#### Scenario: UI and MCP observe the same batch
- **WHEN** an agent starts a batch and the browser loads collection status
- **THEN** both adapters return the same persisted batch, coverage, findings and evidence references

### Requirement: Preserve review semantics and export evidence state
Collection and ingestion MUST NOT create human reviews or change historical assessments. Fresh failures or stale supporting evidence SHALL mark relevant reviews as needing attention. Immutable exports SHALL include collection bindings, source provenance, findings, coverage, and freshness evaluated at snapshot time without requiring a draft for every observation.

#### Scenario: Evidence expires after review
- **WHEN** evidence supporting a human review exceeds its freshness policy
- **THEN** the current view marks review needed while the original review and cited revisions remain intact

#### Scenario: Export before drafting
- **WHEN** an authorized user exports a snapshot after collection but before drafting
- **THEN** matching HTML/JSON exports contain the collected evidence links, findings, limitations and pending review state
