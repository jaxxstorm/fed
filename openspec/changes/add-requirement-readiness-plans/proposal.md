## Why

Fed collects evidence but leaves users and agents to infer what must exist, who must act, and what constitutes adequate proof. Generic instructions obscure prerequisites and shared operational responsibilities across the catalog, so users cannot turn requirements into a practical readiness plan.

## What Changes

- Introduce versioned, source-backed playbooks for every requirement in each supported catalog/profile, including rules and KSIs, rather than a handful of example requirements.
- Explain purpose, applicability, prerequisite capabilities, setup steps, recurring/event-triggered duties, responsible roles, evidence expectations, and review criteria.
- Resolve definitions and related narrative guidance alongside official rule statements; distinguish official obligations, implementation suggestions, and service-specific decisions.
- Model shared prerequisites and conditional dependencies so services establish common capabilities once and see all affected requirements.
- Project service-specific next actions from playbooks, scope, evidence, assessments, and review state, distinguishing missing implementation from missing proof and unknown implementation.
- Present concise readiness plans in the browser and expose the same structured context through MCP. Preserve existing assessment semantics and immutable evidence/history.
- Add assertion-level evidence coverage: what is needed, what each collected artifact proves, why it is sufficient or insufficient, what is missing, and what contradicts the requirement. Explain check failures with expected versus observed behavior and grouped affected resources in plain language.
- Add catalog-wide completeness and provenance validation, reviewed guidance publication, and visible handling of unsupported or changed guidance. Generated text alone cannot satisfy publication gates.
- Agents review guidance against official public FedRAMP sources without requiring human approval of every explanation. Show an agent-review caveat, citations and unresolved interpretations in UI, MCP and exports; keep service evidence review separate.

## Capabilities

### New Capabilities
- `requirement-playbooks`: Catalog-wide, reviewed, versioned requirement explanations and actionable, source-backed plans.
- `readiness-dependency-plans`: Service-scoped shared prerequisites, conditional dependency resolution, and evidence-backed work tracking.
- `readiness-context-delivery`: Consistent human/agent readiness context, actionable UI, MCP contracts, and coverage reporting.

### Modified Capabilities

None in the archived main spec set, which is currently empty. These additions integrate with implemented, unarchived assessment-experience and agent-evidence-workflow changes without replacing their authorization or compliance-status contracts.

## Impact

- Extend Go catalog import/validation, assessment instructions and presentation, PostgreSQL versioned records, and evidence/result bindings.
- Extend shared web/MCP services, browser API contracts, generated TypeScript types, and React requirement/overview views.
- Add maintained guidance artifacts derived from pinned FedRAMP Rules data and the matching 2026 Markdown narrative corpus, with a review and update workflow.
- Preserve current source access boundaries, credential-generation fencing, human review, evidence retention, and existing clients through additive contracts.
- No automatic infrastructure remediation, provider permission expansion, embedded agent runtime, or claim that product readiness constitutes FedRAMP certification.
