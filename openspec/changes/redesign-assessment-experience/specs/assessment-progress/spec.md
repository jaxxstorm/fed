## ADDED Requirements

### Requirement: Consistent progress with explicit denominators
The system SHALL expose one authoritative batched presentation projection for overview, category and requirement progress. Default readiness SHALL count passing applicable required requirements against all applicable required requirements; recommended/optional rules SHALL be separate, not-applicable rules excluded, and unresolved applicability shown as outstanding scope decisions. Empty denominators SHALL NOT display 100% readiness. Every count SHALL identify its unit and link to the contributing work.

#### Scenario: Mixed applicability
- **WHEN** a service contains required, optional, not-applicable and unresolved rules
- **THEN** the dashboard displays the required-rule denominator, separate optional work and scope decisions, and category totals reconcile with the same projection timestamp

#### Scenario: No profile selected
- **WHEN** the service has no selected profile
- **THEN** the overview presents profile setup as the next step rather than a completed progress bar

### Requirement: Explain passing failing and unfinished work
Requirements SHALL have a tested primary status and actionable reason, with secondary labels for simultaneous conditions. The primary statuses SHALL distinguish passing, failing, blocked, needs refresh, needs evidence, in review, in progress, not started, needs scoping and not applicable. Passing SHALL require current supported complete evidence and current human review without unresolved relevant contradictory findings. Documentary rules SHALL NOT require unrelated collector checks.

#### Scenario: Investigation complete but failure found
- **WHEN** the agent completes evidence work and reports a supported failure finding
- **THEN** evidence-gathering progress advances while the requirement is shown as failing with the affected evidence and next steps

#### Scenario: Evidence expires after review
- **WHEN** previously passing support expires or its relevant binding changes
- **THEN** current readiness updates, the rule is shown as needing attention, and historical exports remain unchanged

### Requirement: Show agent accomplishments from real activity
The overview and activity views SHALL summarize persisted agent submissions, evidence additions and rule results with timestamps, outcomes and links. Collector jobs SHALL be distinguished from agent work. Running states, progress percentages and ETAs SHALL be shown only when backed by actual telemetry.

#### Scenario: Agent submits partial work
- **WHEN** an agent adds evidence and a partial rule result
- **THEN** the operator sees what was added, what remains missing and where to continue, without implying an agent is still running

### Requirement: Prioritized actionable attention list
The overview SHALL prioritize failing requirements, access blockers, expired support, missing evidence and reviews using explicit status/impact ordering. Operators SHALL be able to search and filter requirements by status and category with shareable URLs.

#### Scenario: Operator selects failing count
- **WHEN** the operator clicks the failing requirements count
- **THEN** a filtered list shows exactly those requirements with their reasons and next actions
