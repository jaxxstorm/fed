## ADDED Requirements

### Requirement: Actionable evidence requests
Fed SHALL maintain revisioned requests linked to service, requirement/assertions, scope/profile, source hints, requested period, acceptance criteria, and owner or explicit unassigned state. Requests SHALL be discoverable with pagination and filtering in browser and MCP and SHALL link existing readiness work rather than silently duplicating it.

#### Scenario: Missing inbox operating records
- **WHEN** an assessment lacks evidence that the security inbox is monitored
- **THEN** a request identifies the required dated handling records, period, owner status, source hints, and acceptance criteria
- **AND** an agent can discover that request in its work queue

### Requirement: Citation-based fulfillment with human acceptance
Request states SHALL include open, submitted, accepted, and cancelled. Agents SHALL submit fulfillment with immutable evidence citations but MUST NOT accept their own work as human review. Rejection SHALL return the request to open with a reason. Acceptance MUST check current bindings and exact fulfillment revision.

#### Scenario: Agent submits incomplete fulfillment
- **WHEN** an agent submits an inbox configuration to a request for operating records
- **THEN** the request remains submitted until human evaluation and can be rejected with the missing operating evidence explained

#### Scenario: Request context changed
- **WHEN** scope or requested period changes after fulfillment
- **THEN** acceptance against the obsolete binding fails and the request displays revalidation-needed without deleting the old submission

### Requirement: Outstanding issue lifecycle
Fed SHALL link issues to gaps, findings, requirements, and evidence requests and record ownership, milestones, target dates, disposition, and closure evidence. Closing an issue MUST require a human decision bound to its revision and supporting evidence or an explicit reasoned disposition. Tracking MUST NOT execute workload remediation.

#### Scenario: Issue is reported in a release
- **WHEN** a release includes an unresolved network issue
- **THEN** it includes the issue's exact revision, owner status, milestones, dates, and disposition without presenting planned work as completed

#### Scenario: Closure does not erase findings
- **WHEN** a reviewer closes an issue using new supporting evidence
- **THEN** its historical findings and prior milestones remain available and the closure identifies the evidence and reviewer
