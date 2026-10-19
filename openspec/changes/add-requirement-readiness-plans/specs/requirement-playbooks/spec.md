## ADDED Requirements

### Requirement: Applicability-only plans
The system SHALL distinguish actionable playbooks from exclusion explanations and applicability decision requests. Not-applicable variants SHALL explain the exclusion and cite its policy basis. Unresolved variants without a selected obligation SHALL state the specific decision needed, responsible role and relevant source context. Such plans SHALL remain in coverage reporting but SHALL NOT fabricate assertions, evidence acceptance criteria or setup duties. An applicability-only plan SHALL NOT satisfy guidance coverage for a selected required, recommended or optional obligation.

#### Scenario: Excluded variant has no selected statement
- **WHEN** the resolver excludes a variant before selecting class-specific text
- **THEN** its plan explains the exclusion using source citations without requiring an invented statement or action checklist

#### Scenario: Unresolved class applicability
- **WHEN** the resolver reports a class/subset conflict with no selected statement
- **THEN** the plan identifies the conflict, asks a specific applicability question and retains the raw source context for review
- **AND** the plan does not mark the requirement fulfilled or excluded

### Requirement: Explicit evidence acceptance criteria
Every playbook SHALL define evidence acceptance criteria for each assessable obligation, including the fact to demonstrate, relevant scope and targets, time period or freshness basis, required context and provenance, and acceptable evidence alternatives or combinations. Examples SHALL explain what they prove and their limitations, and SHALL NOT imply that uploading an example artifact automatically satisfies the requirement. The system SHALL distinguish policy-mandated evidence characteristics from product guidance and service-specific choices.

#### Scenario: Monitoring evidence examples
- **WHEN** a playbook describes evidence for an infrastructure-monitoring obligation
- **THEN** it explains which configuration, coverage, alerting or operational records establish each applicable assertion and what context each record must contain
- **AND** a dashboard image alone is not represented as proving all resources are monitored or alerts are acted upon unless that conclusion is supported

#### Scenario: Multiple acceptable evidence routes
- **WHEN** equivalent facts can be demonstrated by provider records or dated documentary records
- **THEN** the playbook describes both routes and their sufficiency criteria without requiring a particular vendor integration

### Requirement: Catalog-wide actionable playbooks
The system SHALL provide a versioned playbook for every rule and KSI variant exposed by supported catalogs and profiles, including required, recommended, optional and conditional requirements. Each playbook SHALL explain purpose, applicability, prerequisites, setup steps, operating duties, responsible roles, acceptable evidence, insufficient evidence and review criteria. A legitimately inapplicable section SHALL contain an explicit rationale rather than generic filler.

#### Scenario: Complete supported catalog coverage
- **WHEN** a guidance release is evaluated for activation
- **THEN** every inventoried supported requirement variant has substantive reviewed guidance and a machine-readable coverage report identifies the denominator and coverage by profile
- **AND** missing, generic-only, draft or invalid guidance prevents a claim of complete coverage

#### Scenario: Requirement without recurring work
- **WHEN** a requirement has no recurring operational duty
- **THEN** its playbook explicitly explains that fact rather than inventing a cadence

### Requirement: Traceable policy interpretation
The system SHALL retain authoritative statements and cite the pinned Rules and matching Markdown sources, including relevant definitions, related rules and narrative context. Each actionable interpretation SHALL distinguish official obligation, implementation recommendation and service-specific choice, preserving actor, normative strength, conditions and timing. Inferred prerequisites SHALL carry supporting citations and reviewer rationale.

#### Scenario: Recommendation is not a mandate
- **WHEN** a rule uses SHOULD and a playbook suggests one implementation approach
- **THEN** the UI and agent context identify the recommendation and suggested approach separately without presenting the approach as an official MUST

#### Scenario: Definition supplies timing context
- **WHEN** a duty refers to a defined timing term
- **THEN** the playbook includes its cited definition or exposes unresolved interpretation and does not invent a numeric deadline

### Requirement: Reviewed immutable guidance publication
The system SHALL validate playbook structure, assertion coverage, references and dependency consistency before publication. An authorized agent SHALL review each plan against the official public FedRAMP Rules, definitions and relevant narrative guidance, recording the comparison, citations, limitations, reviewer identity/type, review date and exact content/source revisions. A distinct source-comparison review record SHALL be required; generating prose or passing structural validation alone SHALL NOT count as review. Human content approval SHALL NOT be required for individual plans or domain batches. Published releases SHALL be immutable and independent of service assessment reviews.

#### Scenario: Unreviewed generated content
- **WHEN** an agent drafts a complete-looking playbook without recording a source-comparison review
- **THEN** its state remains draft and it does not satisfy reviewed coverage gates

#### Scenario: Agent completes source-backed review
- **WHEN** an authorized agent compares the plan's obligations, applicability, prerequisites, evidence expectations and timing with the official public sources and records its findings
- **THEN** the plan can satisfy the content-review gate without individual human approval
- **AND** its review attribution remains agent-reviewed, never human-reviewed or FedRAMP-approved

#### Scenario: Public sources do not settle an interpretation
- **WHEN** source comparison finds ambiguity or conflicting context
- **THEN** the plan identifies the specific uncertainty and citations, keeps uncertain advice qualified and does not invent a definitive obligation
- **AND** the product does not require the lay user to certify the policy interpretation as correct

### Requirement: Source change and missing guidance handling
The system SHALL expose missing, draft, reviewed and needs-revalidation guidance states independently of requirement assessment status. A changed referenced statement, definition or narrative SHALL mark affected guidance for revalidation. Unsupported catalogs SHALL retain official text with an explicit guidance gap.

#### Scenario: Narrative-only source change
- **WHEN** a referenced definition changes but the requirement statement does not
- **THEN** the affected playbook requires revalidation and its previous reviewed release remains available historically

#### Scenario: New catalog without guidance
- **WHEN** an imported catalog has no compatible guidance package
- **THEN** its requirements remain accessible with a visible guidance gap and generic fallback is not labeled reviewed guidance
