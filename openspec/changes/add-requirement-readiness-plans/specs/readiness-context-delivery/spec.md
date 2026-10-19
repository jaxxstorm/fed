## ADDED Requirements

### Requirement: Visible agent-review attribution
The browser SHALL visibly label agent-reviewed explanations "Agent-reviewed guidance" and explain that they were checked against cited official public sources but can contain interpretation errors and are not an official FedRAMP determination. The concise caveat SHALL accompany the guidance rather than being hidden only in technical details. MCP responses and exports SHALL include equivalent reviewer-type, review-date, source-revision, limitation and uncertainty fields. Content-review attribution SHALL remain distinct from human review of service evidence.

#### Scenario: A lay user reads a requirement plan
- **WHEN** agent-reviewed guidance is displayed
- **THEN** the user sees "Agent-reviewed guidance" and "Checked against cited official FedRAMP sources. Interpretations may be incomplete or incorrect; this is not an official FedRAMP determination."
- **AND** can inspect source citations and any specific uncertainties without being asked to approve each interpretation

### Requirement: Assertion-level evidence coverage explanation
The shared context SHALL map each assessable obligation to acceptance criteria, relevant collected artifacts and check results, exact evidence revisions and citations, and a plain-language sufficiency rationale. It SHALL distinguish candidate evidence, assessed support, partial support, missing evidence, contradictory evidence, stale evidence and access-blocked evidence. The view SHALL identify precisely which fact, target, environment or time period remains unsupported, and SHALL preserve simultaneous support and contradiction rather than flattening them into one reassuring label.

#### Scenario: Evidence exists but does not establish the whole obligation
- **WHEN** a configuration snapshot supports configured intent but operational execution remains unproven
- **THEN** the view explains what the snapshot demonstrates, what it cannot demonstrate and which operational record would close the remaining gap
- **AND** it links to the exact artifact revision and relevant excerpt

#### Scenario: Partial environment coverage
- **WHEN** current supporting evidence covers development but the obligation also applies to production
- **THEN** the view identifies development as covered and production as missing without labeling the whole obligation supported

#### Scenario: Evidence has not been assessed
- **WHEN** a relevant artifact has been collected but has no sufficiency assessment
- **THEN** it is labeled available for assessment, not evidence that already meets the requirement

### Requirement: Plain-language findings with expected and observed facts
Every displayed check or finding SHALL explain the checked condition, why it matters to the mapped obligation, expected behavior, observed result, affected scope, observation time, evidence provenance and a concrete next action. Failure wording SHALL describe the failed condition rather than repeat the positive test predicate. Raw identifiers SHALL remain accessible as secondary technical detail. Explanations SHALL preserve collector limitations and SHALL NOT infer observed network exposure, enforcement or operational impact from configuration-only evidence.

#### Scenario: Missing inbound isolation policy
- **WHEN** a complete workload/policy observation reports a failed ingress-isolation condition because no selecting policy was found
- **THEN** the primary explanation states that no collected NetworkPolicy selects the workload to restrict incoming traffic, identifies its environment and namespace, and explains the expected condition
- **AND** it does not claim that the workload is publicly exposed or that traffic enforcement was tested
- **AND** it gives an owner-directed next action and the evidence needed to verify resolution rather than only "Investigate ingress_isolation"

#### Scenario: Collection failed rather than control failed
- **WHEN** the collector cannot read the policies needed to evaluate isolation
- **THEN** the explanation says the condition could not be determined because access failed and does not say no policy exists

#### Scenario: Unknown diagnostic code
- **WHEN** no reviewed explanation exists for a collector condition
- **THEN** the view clearly identifies the explanation gap, retains the observed facts and raw code in technical details, and avoids inventing a cause or fix

### Requirement: Concise grouped checks and actionable gaps
The requirement page SHALL expose clearly labeled sections for requirement meaning, required actions, acceptable evidence examples, collected evidence and its coverage, failing or undetermined checks, and remaining work. Equivalent findings SHALL be grouped by condition, cause where known, scope and next action, with exact affected counts and expandable resource details. Check rows SHALL show human-readable names, result, last observation time and evidence/detail links. Grouping SHALL preserve genuine differences and SHALL NOT obscure individual failures, freshness or incomplete coverage.

#### Scenario: Many workloads fail the same check
- **WHEN** many workloads in the same scope fail the same condition for the same known reason
- **THEN** the page presents one concise finding group with the affected count, shared explanation and next action, and expandable individual resources and evidence
- **AND** it does not render a long repeated instruction paragraph for every workload

#### Scenario: Passing check with remaining requirement gaps
- **WHEN** a configuration check passes but other assertions or human review remain outstanding
- **THEN** the check is shown as passing while the requirement clearly lists the remaining work and does not imply overall compliance

#### Scenario: Evidence-first operator
- **WHEN** a finding needs a system change but the operator is collecting evidence only
- **THEN** the view separates evidence collection or verification actions from suggested remediation and makes no infrastructure change as a side effect

### Requirement: Shared human and agent context
Browser and MCP transports SHALL use one authoritative readiness projection containing stable requirement/capability/step identifiers, guidance and source revisions, applicability rationale, prerequisites, actions, owner roles, evidence references, limitations and unresolved decisions. Existing instruction contracts SHALL remain compatible through additive changes, and new submissions SHALL identify the relevant guidance/assertion bindings.

#### Scenario: Same requirement viewed through two transports
- **WHEN** a human and an authorized agent request the same service requirement at the same revisions
- **THEN** both receive equivalent obligations, dependencies, evidence gaps and completion criteria
- **AND** agents additionally receive machine-readable identifiers and collection/human-action boundaries without stronger claims than the browser

### Requirement: Actionable novice-facing presentation
Requirement details SHALL prioritize plain-English meaning, why the rule applies, what must exist first, what to do, existing support, missing support and ready-for-review criteria. Requirement cards SHALL pair status with a specific reason and next action. Official source text and technical payloads SHALL remain accessible through progressive disclosure.

#### Scenario: Novice opens an untouched requirement
- **WHEN** a user opens a requirement without an assessment
- **THEN** the page explains prerequisites and concrete first actions without requiring the user to infer them from generic investigation instructions

#### Scenario: Evidence is missing
- **WHEN** a requirement needs a specific operational record
- **THEN** its card identifies that record and responsible role rather than only saying "Ask an agent to gather evidence"

### Requirement: Service-wide ordered readiness view
The system SHALL expose a bounded, filterable service plan grouping shared prerequisite work, operating duties, evidence gaps and human review. Blocking prerequisites SHALL appear before dependent actions, with visible ordering rationale and downstream impact. Coverage and readiness counts SHALL name their units and SHALL NOT replace compliance-status denominators.

#### Scenario: Shared prerequisite is unfinished
- **WHEN** an unresolved capability blocks multiple requirements
- **THEN** the service plan identifies the capability once, explains affected requirements and presents the next prerequisite action ahead of dependent evidence tasks

### Requirement: Explicit guidance quality and access boundaries
Both transports SHALL expose guidance gaps, revalidation needs and source citations. Agent context SHALL distinguish suggested evidence from assessed support, available collection access from needed access, and agent-permitted actions from human-only decisions. New endpoints SHALL enforce existing service authorization, bounded pagination and secret redaction.

#### Scenario: Unsupported source access
- **WHEN** the plan calls for records from a system without an authorized connection
- **THEN** it identifies the missing access or human-provided record and does not imply the agent can inspect that system

#### Scenario: Unauthorized service request
- **WHEN** an actor requests another service's readiness plan without permission
- **THEN** access is denied without disclosing its owners, evidence or operating details

### Requirement: Preservation of assessment and historical truth
Readiness presentation SHALL preserve existing assessment status, contradictory findings and human-review requirements. Evidence artifacts, prior assessments and reviews SHALL NOT be rewritten by plan creation or guidance updates. Exported readiness context SHALL pin its guidance and service bindings.

#### Scenario: Checklist finished without review
- **WHEN** all setup steps are declared complete but evidence assessment or human review remains outstanding
- **THEN** the requirement does not become passing and the view explains the remaining work

#### Scenario: Historical export after guidance update
- **WHEN** a historical readiness export is inspected after a new package is published
- **THEN** it retains the guidance, citations and evidence bindings used at export time
