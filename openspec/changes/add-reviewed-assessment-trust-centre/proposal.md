## Why

Fed collects evidence and preserves assessment drafts, but users cannot yet turn that work into a complete, reviewed account of their service's implementation and ongoing operations with a shareable trust centre. Agents should investigate and explain through MCP; Fed should enforce traceability, support human assessment, and publish immutable assessment releases.

## What Changes

- Extend agent instructions and submissions with structured service-specific implementation narratives, operational-period support, sampling limitations, and explicit missing or contradictory evidence.
- Add revision-bound evidence requests that agents can discover, fulfill with citations, and hand to humans for review.
- Separate organizational implementation claims, agent evidence conclusions, internal human reviews, and independent assessor conclusions.
- Add review readiness checks and meaningful-change comparisons without rewriting historical reviews or automatically approving new evidence.
- Assemble versioned assessment packages with service context, boundary, responsibilities, requirements, evidence manifests, review results, and outstanding issues.
- Publish those packages through Fed's own trust centre, with deliberately selected public summaries and service/release-scoped restricted access.
- Propose recurring reporting releases with changes since the previous release; publication remains an explicit human action.
- Reconcile current-resource finding summaries with historical evidence before using counts in published reports.

## Capabilities

### New Capabilities
- `agent-implementation-assessment`: MCP-driven implementation narratives and operating-evidence assessments with a verifiable ready-for-review contract.
- `assessment-evidence-requests`: Owned, revision-bound requests for missing evidence and related issue tracking.
- `assessment-review-cycles`: Distinct reviewer/assessor conclusions, revision-bound readiness, and change-triggered re-review.
- `assessment-release-packages`: Reproducible, validated release packages and reporting-period comparisons.
- `service-trust-centre`: First-party public and restricted publication of reviewed assessment releases.

### Modified Capabilities

None against archived main specifications: `openspec/specs/` currently contains no spec files. These additive capabilities extend the implemented contracts in the active agent-evidence, readiness, service, and organization-management changes; implementation must preserve those contracts.

## Impact

- Go assessment/domain, observation projections, publication, auth, storage migrations, MCP tools and prompts, Echo routes, and the embedded React workspace.
- Additive versioned JSON contracts, immutable records, release manifests, scoped external-reader grants, and a first-party publication receipt.
- Existing agent submissions and HTML/JSON exports remain supported; legacy drafts require enrichment before entering the new release workflow.
- Retain official catalog/profile applicability, credential/scope fencing, immutable history, and separate access, freshness, technical, assessment, and review states.
- No workload remediation, automatic organizational policy adoption, embedded agent runtime, or invented operating records. Independent assessment must be performed by an actual authorized assessor. Pilot examples inform presentation, not current normative requirements.
