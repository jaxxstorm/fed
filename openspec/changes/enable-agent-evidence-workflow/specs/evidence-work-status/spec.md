## ADDED Requirements

### Requirement: Independent evidence-work completion
The system SHALL expose open, partial, blocked, evidence_complete and needs_refresh work states independently of applicability, technical findings, agent assessment and human review. A checked requirement indicator SHALL be labeled evidence complete and SHALL require validated coverage of every required assertion. Uploading an artifact alone SHALL NOT complete evidence work.

#### Scenario: Evidence demonstrates a failure
- **WHEN** an agent fully documents every required assertion but evidence contradicts a requirement
- **THEN** the UI and MCP show completed evidence work alongside the failing agent assessment or technical finding and pending human review

#### Scenario: Incomplete investigation
- **WHEN** a submitted result covers only some required assertions
- **THEN** the rule remains unchecked with partial status and explicit missing assertions

### Requirement: Reopen evidence work when support changes
The service SHALL project needs_refresh when supporting evidence expires, relevant scope/profile/instruction/source bindings change, or new evidence contradicts a cited assertion. It SHALL retain immutable historical results and receipts. Retrieval time SHALL NOT replace original event time, and unrelated source/evidence changes SHALL NOT reopen unaffected rules.

#### Scenario: Evidence expires
- **WHEN** an otherwise complete result loses current support under its configured freshness policy
- **THEN** the checkbox becomes needs refresh, the rule reappears in open discovery, and its prior receipt remains historical

#### Scenario: New failure against cited support
- **WHEN** a fresh observation contradicts an assertion used by a completed result
- **THEN** that result requires reassessment without automatically modifying an earlier human review or running an agent

### Requirement: Consistent UI MCP and export projections
Requirement lists/details, MCP receipts/status and immutable exports SHALL use the same evidence-work semantics and include citations, provenance, missing assertions, blockers and separate review state. Documentary obligations SHALL display agent investigation needed rather than a terminal manual-evidence instruction. Genuine unavailable inputs SHALL remain specific blockers.

#### Scenario: External agent completes evidence collection
- **WHEN** an SDK client submits a valid rule result without using a browser
- **THEN** the requirement page shows the same completion state and evidence links as the receipt and a subsequent export freezes that state

#### Scenario: Historical export after refresh
- **WHEN** current evidence becomes stale after an export was prepared
- **THEN** the immutable export preserves its snapshot-time status while current UI and MCP show needs refresh

### Requirement: Compatibility of documentary and legacy freshness
Documentary evidence SHALL use its frozen source freshness policy without requiring collector measures. Legacy evidence SHALL retain measure-based freshness. Shared status and snapshot projections SHALL evaluate mixed evidence consistently at one reference time and snapshots SHALL include all profile rules.

#### Scenario: Documentary draft without collector measures
- **WHEN** a draft cites current documentary evidence linked to its rule and has no collector measures
- **THEN** its evidence freshness is current under the source policy rather than missing due to absent measures

#### Scenario: Mixed legacy and documentary support
- **WHEN** one cited artifact expires under its applicable source or measure policy
- **THEN** the draft no longer presents all supporting evidence as current and previous exported status remains immutable
