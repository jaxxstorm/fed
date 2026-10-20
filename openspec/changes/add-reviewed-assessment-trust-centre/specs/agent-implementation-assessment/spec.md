## ADDED Requirements

### Requirement: Structured implementation investigation
Fed SHALL expose equivalent browser and MCP instructions for a service-specific implementation assessment, including official duties, applicability, assertion IDs, evidence sources, current bindings, required narrative fields, and unresolved questions. Submissions MUST distinguish organizational claims, observed configuration, documented intent, and agent conclusions, and MUST cite exact evidence revisions for supported claims.

#### Scenario: Agent drafts an implementation
- **WHEN** an agent investigates a requirement through MCP
- **THEN** it can submit mechanisms, responsible roles, procedures, frequency, boundary, assertion mappings, citations, and explicit unknowns as a revisioned draft visible in the browser
- **AND** missing structured implementation fields are identified as incomplete by the server, while agents identify generic restatements and humans review narrative adequacy

#### Scenario: Guidance is disabled
- **WHEN** reviewed guidance is unavailable or disabled
- **THEN** instructions retain official duties and the structured submission contract without presenting unavailable guidance as approved

### Requirement: Operational evidence is period-specific
Fed SHALL record the reporting period, observed event times, sampling basis, population when known, coverage limitations, and separate implementation and operating-support conclusions. Retrieval time MUST NOT substitute for event time, and configuration evidence alone MUST NOT establish operating effectiveness.

Verifiable historical records SHALL be eligible for their declared assessment period even when they are stale today. Current freshness SHALL be reported separately and MUST NOT refresh or extend the historical conclusion. Scope/profile, source provenance, artifact integrity, and citation checks SHALL remain enforced.

#### Scenario: Historical exercise is stale today
- **WHEN** an agent cites a verifiable September exercise for a September assessment after its current freshness window expires
- **THEN** the period-bound submission is accepted with current freshness reported as stale
- **AND** neither the record nor its period conclusion establishes October operations

#### Scenario: Restore configuration without exercise records
- **WHEN** an agent submits backup configuration without completed restore-exercise records for the assessed period
- **THEN** the configuration can support implementation assertions while operating support remains unknown with an evidence request or explicit gap

#### Scenario: Limited sample
- **WHEN** an agent cites two access reviews from a larger or unknown population
- **THEN** the result records the sample and population limitations and does not report full-period or full-population coverage without support

### Requirement: Server-validated review readiness
Fed SHALL compute structural review readiness from structured fields, citation integrity and availability, revision bindings, and explicit treatment of every selected assertion. It MUST distinguish structurally reviewable partial or contradicted assessments from supported assessments. Invalid citations and stale binding submissions MUST be rejected atomically. Agents SHALL assess the substantive meaning and sufficiency of cited evidence; humans SHALL review that assessment. The server MUST NOT present structural readiness as independent semantic verification or infer operating effectiveness from valid citations alone.

#### Scenario: Structurally valid but substantively inadequate evidence
- **WHEN** a submission contains valid citations and complete fields but its evidence does not substantiate the narrative
- **THEN** structural readiness does not approve the narrative or establish operating effectiveness
- **AND** the human reviewer can reject the agent's conclusion with the substantive gap recorded

#### Scenario: Partial but reviewable assessment
- **WHEN** all assertions have explicit supported, contradicted, or unknown conclusions with reasons and valid citations where support is claimed
- **THEN** the assessment can be ready for human review while its evidence or assessment state remains partial or negative

#### Scenario: Concurrent scope change
- **WHEN** scope changes after instructions were retrieved and before submission
- **THEN** the submission returns a conflict and writes neither a replacement draft nor a partial result

### Requirement: Backward-compatible agent workflow
Fed SHALL retain existing artifact submission and rule-result tools and add an explicitly versioned structured assessment contract. Legacy results MUST NOT be silently converted into adopted or reviewed implementations. Idempotent retries MUST return the same receipt and conflicting reuse MUST fail.

#### Scenario: Legacy client submits a draft
- **WHEN** a supported legacy MCP client submits a rule result
- **THEN** its existing workflow succeeds and any missing release-readiness fields are reported as enrichment work
