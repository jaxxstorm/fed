## Context

Fed already persists catalog/profile/scope revisions, evidence artifacts, assessments, collection outcomes and human reviews. `internal/assessment/agent_rules.go` currently supplies a whole-statement assertion and generic investigation steps, with limited reviewed overrides. `presentation.go` explains statuses, but a status is not a plan for establishing and operating a capability. The React UI and MCP need a shared interpretation layer rather than independently generated prose.

The main spec directory is empty; implemented behavior is documented in unarchived changes, especially `redesign-assessment-experience` and `enable-agent-evidence-workflow`. Preserve their permission, revision, assessment and review contracts. The Markdown corpus is mirrored upstream content and must not be edited to add product advice.

## Goals / Non-Goals

**Goals:**
- Cover every rule and KSI exposed by supported catalogs/profiles, with understandable prerequisite-to-review plans.
- Make shared work, applicability conditions, owners, operational triggers and evidence gaps explicit.
- Give browsers and agents the same versioned, cited interpretation and current service context.
- Explain uncertainty without inventing obligations or inferring absent implementation from absent evidence.

**Non-Goals:**
- Automatically remediate systems, expand access, establish official certification, or replace human compliance review.
- Require an LLM at runtime, build a general project-management suite, or replace existing requirement statuses.
- Treat a connection, a checklist tick, or a passing keyword check as sufficient requirement support.

## Decisions

### 1. Reviewed guidance packages separate from upstream policy

Store declarative playbooks and shared capability definitions in a product-owned versioned guidance package. Each immutable release binds to catalog identity, upstream Rules commit, Markdown source commits from `_sources.json`, and hashes of the specific statement, definitions and narrative passages used. References include stable source identifiers, paths/anchors, excerpts and content hashes; line numbers alone are insufficient.

Each playbook contains purpose, applicability rationale, statement-to-assertion mapping, prerequisite capability references, ordered setup/operating/evidence/review steps, owner roles, trigger or cadence, evidence examples, insufficiency examples and completion criteria. Explicit `none` with rationale is valid where a section genuinely does not apply; generic repeated text is not completeness.

Each step labels its basis: official obligation, recommended implementation, or service decision. Preserve MUST/SHOULD/MAY, actor, class, track, event conditions and definitions. Numeric deadlines come from cited policy or an explicitly labeled service decision, never an agent's inference. Reasoned prerequisite inferences carry supporting references and reviewer rationale without masquerading as verbatim policy.

Alternative: runtime-generated explanations without a source-comparison review. Rejected because outputs would vary by session and could invent mandatory practices. Agents draft packages and perform a distinct review against official public FedRAMP sources, recording comparisons, citations and limitations. Machine validation complements that review. Individual human content approval is not required. Content review remains distinct from human assessment review of a service's evidence.

The UI labels this content **Agent-reviewed guidance**, with the concise caveat:
"Checked against cited official FedRAMP sources. Interpretations may be incomplete
or incorrect; this is not an official FedRAMP determination." MCP and exports
retain equivalent attribution. The agent compares applicability, obligations,
prerequisites, evidence criteria and timing; drafting alone does not count as
review. Ambiguity produces a cited, explicit uncertainty, not a demand that a
lay user certify policy correctness. Service-specific factual questions (for
example, whether a triggering event occurred) remain legitimate user decisions.

Content-review and release operations use existing edit authorization within the
selected service. Agent grants never become organization-wide publication
authority; persisted review/release records retain service and tenant scope.

### 2. Full-catalog coverage is a release deliverable

Use three explicit plan kinds: `actionable`, `excluded`, and `needs_decision`.
The inventory contains 46 variants without selected statements (31 unresolved,
15 excluded). Applicability-only plans retain policy citations and the resolver
reason; `needs_decision` also requires a concrete question and responsible role.
They have no invented assertion, setup or evidence checklist. Validate each
plan kind against its bound inventory variants so an exclusion explanation
cannot satisfy coverage for a required obligation. A later applicability decision
resolves to a compatible actionable plan rather than mutating the historical
explanation into proof of compliance.

Generate an inventory from every supported catalog/profile, deduplicated by rule revision and relevant applicability variant. Include recommended, optional, conditional and unresolved items, not only the required denominator. Map both generic rules and existing specialized KSI guidance into the same package format.

Track `missing`, `draft`, `reviewed`, `needs_revalidation` coverage independently from requirement status. Validate identifiers, required sections, assertion coverage, references, conditions, dependency edges and review metadata. Emit per-profile totals and explicit gaps. Runtime handles future/imported unsupported catalogs honestly, but initial delivery is not complete with mostly generic or unreviewed plans. Publish full coverage only when every inventoried variant has reviewed substantive guidance; unresolved interpretation is surfaced as an explicit decision with authoritative context, not guessed content.

Alternative: implement only inbox guidance or a few popular controls. Rejected because it reproduces the user's central problem across the rest of the catalog.

### 3. Conditional capability graph with service-scoped instances

Represent common capabilities (for example, security communications) using stable IDs and scoped instances. Requirements reference capability steps rather than copying independent tasks. Dependency edges carry predicates over known profile/scope attributes and recorded decisions. Evaluate predicates as true, false or unknown; an unknown creates a scoping question, not an implementation demand or silent exclusion.

Resolve edges into a directed acyclic graph. Reject cycles, dangling IDs and conflicting duplicate definitions at publication. Reuse work only when the capability, service boundary and instance identity match; distinct environments must not be collapsed accidentally. Show downstream requirements and the authority of each edge. A recommended action feeding a required outcome does not automatically become the only mandatory implementation method; model alternative approaches explicitly.

Alternative: a flat checklist per rule. Rejected because it duplicates shared work and hides ordering and conditional applicability.

### 4. Revisioned work records, evidence-backed projections

Add service-owned versioned readiness records with stable step/instance IDs, guidance revision, owner role/optional eligible assignee, notes, chosen alternative, implementation declaration, evidence references and applicability decisions. Mutations use existing edit authorization, expected revisions, idempotency and audit conventions. Resolve current state in a shared Go projection using pinned guidance, current scope/profile, evidence freshness and existing assessments/reviews.

Keep separate dimensions: implementation (`unknown`, `declared_missing`, `declared_present`), evidence coverage (missing/partial/current/stale), operating trigger (not yet triggered/due/unknown) and assessment/review. No evidence means unknown implementation unless an authorized declaration or supported assessment says otherwise. A capability being declared present does not complete its evidence duties or pass dependent requirements. A missing connection is an access blocker, not proof the capability is absent.

Event-triggered duties retain readiness/setup expectations while distinguishing no event observed from a completed operational duty. Recurring work uses cited cadence or labeled service policy; it never fabricates timing. Existing evidence is suggested based on source/target relevance; it becomes support only through explicit cited assertion assessment. Preserve contradictory findings and mandatory human review.

Alternative: reuse the requirement `passing` status for tasks. Rejected because establishment, operation, proof and approval are distinct facts.

### 5. One context model for UI, MCP and next-action ordering

Implement a shared `ReadinessContext` projection returning requirement and capability IDs, guidance/source revisions, profile/scope bindings, applicability and conditions, actionable steps, owner roles, current evidence references, unresolved questions, blockers and downstream impact. Extend existing instruction responses additively and add bounded service-plan/readiness endpoints and MCP tools. Writes reuse the same domain service and authorization. Preserve old fields during client migration and bind new assessment submissions to guidance/assertion revisions.

Default UI order: plain-English meaning; why it applies; what must exist; ordered actions; what we have/what is missing; ready-for-review criteria. Official text and technical payloads remain expandable. Requirement cards show a concrete next action and reason rather than generic agent instructions. Service overview groups shared prerequisite work and separates setup, ongoing operations, evidence and review. Order blocking prerequisites before dependent actions, then known due obligations; show rationale rather than an opaque priority score.

An agent receives the same steps plus stable IDs, source excerpts, exact evidence citations, available collection routes, human-only actions and unresolved decisions. It must not receive irrelevant sources as implied proof. Paginate broad inventories and allow fetching one requirement or capability without downloading the entire corpus. Use batched reads to avoid per-card database fan-out.

### 6. Evidence coverage matrix and reviewed finding explanations

Make the central requirement-detail model an assertion-to-evidence matrix: obligation, acceptance criteria, expected scope/time coverage, candidate artifacts, assessed support with exact citations, contradictory observations, sufficiency rationale and remaining gaps. Candidate relevance is not sufficiency. Explain why an artifact supports an assertion or falls short, including missing environments, stale observations, operational proof and unavailable access. Evidence alternatives can be combined; no single integration or example screenshot is inherently sufficient. Preserve policy authority labels on evidence expectations, including timestamp requirements.

Define versioned explanation templates for collector conditions keyed by collector/condition semantics, with reviewed meanings, expected and observed descriptions, limitations and next-action categories. Populate them from structured facts, not free-form runtime guesses. A failed positive predicate must be rendered as the actual negative observation. For example, a complete observation with no selecting NetworkPolicy can explain that no policy was found to restrict incoming traffic for the named workload; it cannot assert public exposure or tested enforcement. Failed enumeration must instead say the condition could not be evaluated. Unknown codes have an explicit explanation-gap fallback.

Return these explanations and the matrix from the shared projection to both UI and MCP. Human-readable check rows show result, last observation, affected scope, evidence links and the assertions they address. Group repeated findings by condition, known cause, scope and action; retain exact target counts and expandable per-target provenance, ages and exceptions. Keep different causes and access failures separate. Translate terms inline, with raw codes such as `ingress_isolation` in technical details.

Use a concise page hierarchy: what this means; action items; evidence that can meet it; evidence we have and what it proves; checks and findings; what remains. List evidence/verification actions separately from potential remediation and never execute changes through a read or collection action. Oneleet screenshots illustrate desired clarity, not policy authority or a design to copy.

Alternative: replace identifiers with nicer labels only. Rejected because labels alone do not establish expected-versus-observed facts, evidence sufficiency or missing coverage. Alternative: a generated summary detached from evidence. Rejected because it cannot reliably preserve provenance, contradiction and uncertainty.

### 7. Immutable guidance and conservative change invalidation

Published guidance is immutable. A changed cited policy passage invalidates affected playbooks for revalidation, including definition-only or narrative-only changes. A catalog import without a matching package displays a guidance gap and preserves the official rule. Service declarations and assessments retain their original bindings. New assertion coverage cannot be satisfied by replaying a prior whole-statement result; show reassessment work. Guidance availability alone neither grants nor revokes human approval; changes affecting assertion meaning or applicability produce explicit reassessment requirements through the existing freshness/binding model.

Pin guidance in exported readiness snapshots. Rollback disables new presentation/mutations and retains additive tables and historical records; never rewrite old evidence or pretend newer assertions were historically reviewed.

## Risks / Trade-offs

- Large authoring/review effort → inventory by domain, reuse common capabilities, automate traceability checks, and require catalog-wide acceptance rather than hiding unfinished coverage.
- Over-prescriptive guidance → separate policy obligations from example implementations, support alternatives, and require citations/reviewer rationale.
- Misleading completeness → keep implementation, evidence, triggers and review separate; use novice usability and agent-context tests.
- Policy changes outside rule text → hash the full referenced source set and invalidate affected guidance.
- Incorrect shared-capability reuse → scope instances explicitly and test cross-environment and cross-service isolation.
- Existing assessment compatibility → additive APIs and records, stable historical bindings, explicit reassessment for new assertions.
- Imported text or repository content influencing agents → treat retrieved material as untrusted evidence, not agent instructions; preserve bounded payloads and existing authorization.

## Migration Plan

1. Inventory supported catalogs/profiles, existing specialized guidance, schemas, and unarchived contracts; freeze baseline status/evidence/export fixtures.
2. Add guidance schemas, import validation, coverage reporting and the full source-pinned content package. Run agent source-comparison review per domain with recorded reviewer identity/type/date/revision and limitations.
3. Add additive readiness storage and projections behind a feature flag. Populate views lazily; do not manufacture declarations or evidence for existing services.
4. Add compatible browser/MCP APIs, generated contracts and UI; exercise paired human/agent scenarios against the same context.
5. Gate activation on complete reviewed guidance coverage for supported catalog variants and regression/usability checks. Unknown future variants remain explicitly unsupported for guidance.
6. Deploy using the existing approved release process. Compare historical evidence, scope, assessments and reviews before/after; verify service-specific next actions and stale-guidance behavior.
7. Roll back presentation via the flag if needed, preserving schema, guidance packages and immutable records.

## Open Questions

- Agent source-comparison review is the approved content workflow; no human domain-reviewer assignment is needed. Preserve explicit uncertainty where official public context does not resolve an interpretation.
- Which organization-specific operating choices should be collected during onboarding versus deferred to individual readiness steps? Default to explicit unresolved questions rather than assumed values.
- Confirm the supported catalog/profile inventory during implementation; the acceptance denominator is the actual supported set, not a handpicked subset of Ignis requirements.
