## ADDED Requirements

### Requirement: Shared prerequisite graph
The system SHALL represent prerequisite capabilities and ordered dependencies using stable identifiers and service-scoped instances. Shared work SHALL be reused only for matching capability and scope identities, and SHALL display affected downstream requirements. Publication SHALL reject cycles, dangling references and conflicting definitions.

#### Scenario: Multiple requirements share a capability
- **WHEN** several applicable requirements depend on the same service-scoped capability
- **THEN** the readiness plan presents one shared instance with prerequisite actions and links to every dependent requirement

#### Scenario: Environments differ
- **WHEN** two capability instances concern distinct environments
- **THEN** completion or evidence for one does not silently satisfy the other

#### Scenario: Invalid dependency graph
- **WHEN** a guidance package contains a dependency cycle or missing capability reference
- **THEN** publication fails with the affected identifiers

### Requirement: Conditional dependencies and alternatives
The system SHALL evaluate applicability and dependency conditions as true, false or unknown using profile, scope and recorded decisions. Unknown conditions SHALL produce specific scoping questions. The plan SHALL represent valid alternative implementations without making one recommendation mandatory merely because it supports a required outcome.

#### Scenario: Undecided condition
- **WHEN** prerequisite applicability depends on an unanswered service decision
- **THEN** the plan requests that decision and does not silently exclude or mandate the prerequisite

#### Scenario: Alternative selected
- **WHEN** an authorized user selects a supported implementation alternative
- **THEN** the plan records the choice and rationale and projects the relevant steps without marking unchosen suggestions as failed obligations

### Requirement: Independent implementation and evidence states
The system SHALL distinguish unknown, declared-missing and declared-present implementation from evidence coverage, operating trigger and assessment/review state. Evidence absence SHALL NOT establish implementation absence. Declared completion SHALL NOT establish requirement compliance or override contradictory findings.

#### Scenario: No evidence of an inbox
- **WHEN** the service has neither evidence nor a declaration about a required communications capability
- **THEN** its implementation is unknown and the next action asks whether it exists and requests proof rather than asserting it must be created

#### Scenario: Capability declared present
- **WHEN** an operator declares a capability present without supporting operational records
- **THEN** the plan preserves the declaration while showing missing proof and retaining existing assessment and review requirements

### Requirement: Operating triggers and evidence sufficiency
The system SHALL distinguish one-time setup, recurring duties and event-triggered actions. Cadence and deadlines SHALL cite policy or a labeled service decision. Evidence suggestions SHALL identify what each item proves, scope/freshness limitations and remaining assertions; collection success alone SHALL NOT establish sufficiency.

#### Scenario: Event has not occurred
- **WHEN** an event-triggered duty has no established triggering event
- **THEN** the plan distinguishes not-yet-triggered or unknown trigger state from failure and still exposes applicable readiness/setup work

#### Scenario: Configuration without operating proof
- **WHEN** configuration evidence exists for a requirement that also requires demonstrated operation
- **THEN** the plan identifies configuration support and the missing operating records separately

### Requirement: Audited revisioned readiness records
Readiness declarations, ownership, decisions and evidence bindings SHALL be service-scoped, authorized, revisioned and audited with idempotent mutations. Stale revisions SHALL be rejected. Historical records SHALL retain guidance, profile, scope and evidence bindings. Guidance changes that alter assertion meaning or applicability SHALL create explicit reassessment needs rather than silently reusing prior approval.

#### Scenario: Conflicting edits
- **WHEN** an actor submits an outdated readiness-record revision
- **THEN** the mutation returns a conflict without overwriting the newer record

#### Scenario: Updated assertion coverage
- **WHEN** new guidance introduces an obligation not assessed in an older whole-statement result
- **THEN** the old result and review remain historical and the new plan identifies the assertion requiring assessment
