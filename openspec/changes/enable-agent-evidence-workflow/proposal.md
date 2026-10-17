## Why

The intended workflow is for an external agent to read open FedRAMP requirements through MCP, discover configured sources, gather evidence with its own tools, and submit results that visibly complete evidence work. Today the service exposes fragmented assessment tools and collector-centric plans, leaving documentary requirements labeled “manual evidence required” without an actionable end-to-end agent contract.

## What Changes

- Expose paginated open-rule discovery and revision-bound rule instructions containing official text, evidence expectations, existing support, missing assertions, relevant sources, and submission instructions.
- Make evidence sources configurable in the UI and discoverable through MCP, including repository, AWS, Kubernetes, document, and operational-record locations with non-secret access hints.
- Support direct general evidence artifacts and claim-level citations alongside existing typed configuration observations, without external hosting or manually creating collector measures.
- Provide an idempotent rule-result submission and receipt with evidence-work states: evidence complete, partial, blocked, or needs refresh. Preserve agent assessment and human review as separate fields.
- Update the requirement UI and MCP prompt to guide the complete external-agent loop and show consistent completion checkboxes and explicit blockers.
- Reuse existing collectors as optional tools. No embedded LLM, agent dispatcher, mandatory claim queue, or unattended agent scheduling is introduced.

## Capabilities

### New Capabilities
- `agent-rule-discovery`: Open-rule discovery, versioned assertion-level instructions, and the MCP investigation prompt.
- `evidence-source-directory`: Shared UI/MCP configuration and discovery of scoped evidence sources.
- `agent-evidence-submission`: Direct documentary artifacts, citations, idempotent rule results, and completion receipts.
- `evidence-work-status`: Consistent evidence-work completion, reopening, provenance, and independent review states across UI/MCP/exports.

### Modified Capabilities

None in the main spec tree, which is currently empty. This change builds on existing implementation and the active `automate-evidence-collection` change; its external-agent contracts replace that change's terminal manual-evidence labeling without removing collector behavior.

## Impact

- Shared assessment services, PostgreSQL additive migrations, artifact storage, MCP tools/prompts, and Echo/templ configuration and requirement views.
- Existing profile selection, scope, catalog, authorization, typed observation ingestion, collector jobs, and human review remain foundations.
- External clients gain a complete workflow using their own tool access and credentials; no model provider dependency is added to the Go service.
- Documentation and SDK/browser integration tests must cover an external-agent flow without a running collector worker, direct document submission, partial/blocked results, and refresh behavior.
- Coordinate overlapping status/provenance/export work with the incomplete automatic-collection change; do not treat its unchecked tasks as verified prerequisites.
