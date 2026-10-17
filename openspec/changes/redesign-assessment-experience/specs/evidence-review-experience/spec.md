## ADDED Requirements

### Requirement: Human-readable requirement detail
Requirement pages SHALL lead with status, its reason, remaining assertions, prioritized next steps and an assessment summary. Findings, evidence and history SHALL have dedicated readable presentations. Raw record JSON and UUIDs SHALL NOT be the primary explanation of requirement state.

#### Scenario: Missing drift-review evidence
- **WHEN** configuration automation is documented but drift-review records remain missing
- **THEN** the detail page requests dated drift-review results, identifies the remaining assertion and relevant source/action without repetitive generic caveats

### Requirement: Evidence library and safe previews
The UI SHALL provide a paginated evidence library with search and source/type/freshness/requirement filters. Records SHALL display meaningful titles, observation dates, provenance and linked requirements. Text/Markdown SHALL have safe previews with line references; JSON evidence SHALL offer readable field/path inspection and optional raw content. Full artifact bodies SHALL be loaded on demand rather than for every list row.

#### Scenario: Operator inspects a citation
- **WHEN** a reviewer opens a cited line range or JSON Pointer
- **THEN** the preview displays the exact immutable evidence revision and highlights or locates the referenced content

#### Scenario: Evidence needs improvement
- **WHEN** support is stale, incomplete or lacks an operational example
- **THEN** the evidence/requirement view names the affected item, the needed improvement and an action to gather or refresh it

### Requirement: Revision-aware human review
The UI SHALL let authorized humans inspect the draft and cited evidence, record a review and see the resulting state without editing JSON. Agent analysis and human review SHALL remain attributed separately. Revision conflicts SHALL preserve input and offer a current-version comparison/reload path.

#### Scenario: Draft changes during review
- **WHEN** a reviewer submits against a draft that an agent has since updated
- **THEN** the UI preserves review notes, explains the newer revision and asks the reviewer to inspect it before retrying

### Requirement: Concise outcome-oriented copy
Primary UI text SHALL explain achievements, concrete missing items and next actions in plain language. Relevant evidence limitations SHALL remain accessible as labeled details without repeated generic negative disclaimers dominating pages.

#### Scenario: Operator reads an assessment summary
- **WHEN** a non-specialist opens a partially completed requirement
- **THEN** the primary text identifies what is done and what to add next without requiring interpretation of raw JSON or internal state names
