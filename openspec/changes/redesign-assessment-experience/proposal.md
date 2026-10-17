## Why

The current interface exposes implementation details instead of explaining progress: operators encounter UUIDs, JSON, ambiguous states and errors without useful recovery steps. A non-specialist should immediately understand what the agent achieved, what is passing or failing, what evidence needs improvement, and what to do next.

## What Changes

- Replace the templ-based operator interface with a cohesive React/TypeScript frontend built with Bun and Vite, served by the existing Go application.
- Introduce an original visual system and information architecture inspired by the clarity of Oneleet and Vanta: readable typography, restrained color, accessible status indicators, progress summaries and focused action lists.
- Redesign every operator journey: service selection/onboarding, overview, requirements, evidence, agent activity, sources, collection runs, review, settings and exports/history.
- Define consistent progress calculations and status explanations, with drill-downs from every count to the affected work. Show requirements, evidence gathering and human review as understandable stages.
- Replace JSON-heavy screens with purpose-built summaries, evidence previews, field-based configuration and prioritized next actions. Keep technical diagnostics optional.
- Translate collection, access, validation and service errors into actionable messages explaining the issue, affected resources and the next recovery step.
- Use concise, affirmative copy. Explain concrete missing items and improvements rather than repeating generic “this is not evidence of X” caveats.
- Preserve the external-agent MCP workflow, evidence provenance, existing records and authorization while adding typed browser APIs and modern frontend build/test tooling.

## Capabilities

### New Capabilities
- `operator-workspace`: Modern application shell, navigation, visual system, route coverage, accessible layouts and frontend delivery.
- `assessment-progress`: Consistent readiness/status projections, service/category progress, work filtering and agent accomplishment summaries.
- `evidence-review-experience`: Human-readable requirement details, evidence library/previews, citations, improvement guidance and review workflows.
- `actionable-operator-feedback`: Structured errors, field validation, contextual recovery actions and honest loading/empty/stale states.
- `guided-service-configuration`: Plain-language onboarding, source/scope/profile configuration, collection controls, agent connection and export/history journeys.

### Modified Capabilities

None in the main spec tree, which is currently empty. This change replaces the browser presentation of the existing assessment and agent-evidence capabilities while retaining their domain contracts. Coordinate with the unarchived `enable-agent-evidence-workflow` and incomplete `automate-evidence-collection` changes rather than duplicating or declaring their outstanding work complete.

## Impact

- New frontend workspace, Bun lockfile, React/TypeScript/Vite tooling, reusable UI components and browser/visual tests.
- Go browser API adapters and batched presentation projections over existing assessment services; session/CSRF integration and static frontend delivery.
- Build, Makefile, Docker and development workflow changes; no Bun/Node runtime required in the production Go container.
- Migration of existing browser routes, forms and artifact/export links, with regression checks for external MCP agents and authenticated human review.
- Visual and task-based acceptance on representative real-sized data, including hundreds of artifacts and mixed success/failure states. Screenshot inspiration guides clarity, not copied layouts, branding or proprietary assets.
