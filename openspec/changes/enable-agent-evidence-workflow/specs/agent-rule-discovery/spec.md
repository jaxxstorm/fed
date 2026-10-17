## ADDED Requirements

### Requirement: Open-rule discovery through MCP
The system SHALL expose authorized, bounded, paginated `list_open_rules` results for the selected service profile, including open, partial, blocked, needs-refresh, and unresolved-applicability rules across KSI and broader obligations. Completed and not-applicable rules SHALL be excluded by default and remain explicitly inspectable. Discovery SHALL work without a collector worker or reconciled collection plan.

#### Scenario: External agent starts with a selected profile
- **WHEN** an authorized agent lists open rules for a configured service in server-only mode
- **THEN** it receives outstanding rule IDs, official titles, applicability, work states, gap summaries and instruction references without creating measures or jobs

#### Scenario: Work changes during pagination
- **WHEN** relevant work state or freshness changes between continuation requests
- **THEN** the service rejects the stale continuation with `view_changed` and restart guidance rather than silently skipping rules

#### Scenario: Different service requested
- **WHEN** an agent requests rules outside its token scope
- **THEN** the service denies access without returning that service's requirements or evidence

### Requirement: Actionable versioned rule instructions
The system SHALL expose `get_rule_instructions` with official text, applicability rationale, revision bindings, stable required/optional assertion IDs, evidence expectations, investigation guidance, relevant configured sources, existing support, missing assertions and a submission contract. Application guidance SHALL be distinguished from official requirements. Absence of collector support SHALL NOT prevent instructions or evidence submission.

#### Scenario: Documentary requirement
- **WHEN** an agent requests instructions for a rule previously labeled manual evidence required
- **THEN** it receives evidence obligations and source-oriented investigation guidance without a prerequisite collector measure

#### Scenario: Reviewed guidance unavailable
- **WHEN** the catalog statement has no reviewed assertion decomposition
- **THEN** instructions expose its full official text and a whole-statement cited-analysis fallback labeled generic guidance rather than dropping the rule or inventing official thresholds

### Requirement: Prompt executes the external-agent workflow
The `gather_evidence` MCP prompt SHALL direct external agents to discover open rules and sources, investigate with their own tools, submit artifacts and rule results, inspect receipts, and continue until work is complete or specifically blocked. The service SHALL NOT require an embedded model, agent dispatcher, or claim queue.

#### Scenario: Agent follows the prompt
- **WHEN** a client retrieves the prompt for an authorized service
- **THEN** instructions describe the available discovery/submission tools, evidence provenance, source boundaries and blocked-result path rather than asking a human to gather accessible evidence
