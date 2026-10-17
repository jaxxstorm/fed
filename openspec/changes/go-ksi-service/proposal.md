## Why

Ignis needs a practical way to track class-specific FedRAMP 20x expectations against its AWS, Kubernetes, Pulumi, and GitHub infrastructure. A shared service can turn manual KSI reviews and automated observations into traceable evidence that both people and agents can use, while remaining independent of any trust-centre vendor.

## What Changes

- Introduce a standalone Go service in this repository with a web frontend and MCP interface backed by the same application services.
- Import a pinned official FedRAMP rules catalog with class/profile applicability, source provenance, effective dates, and explicit unresolved applicability.
- Maintain customer service scopes, KSI measures, evidence, assessment drafts, human reviews, gaps, and revision history.
- Execute registered read-only checks asynchronously; initially inspect AWS GuardDuty/EKS configuration and GitHub/Pulumi repository evidence.
- Allow external agents to read requirements, request checks, attach evidence, and save cited assessment drafts through MCP.
- Provide versioned human-readable and JSON assessment exports, generic external evidence references, and a vendor-neutral trust-centre adapter interface. Core functionality requires no trust-centre account or vendor API.
- Show KSI coverage separately from broader certification obligations and official certification status.

## Capabilities

### New Capabilities
- `fedramp-catalog`: Versioned official requirements and profile-specific applicability.
- `ksi-assessments`: Service scopes, measures, evidence, reviews, gaps, and dashboard.
- `evidence-checks`: Asynchronous registered collectors, reproducible evaluations, and collection coverage.
- `agent-mcp`: Authenticated MCP resources and tools sharing application authorization and state.
- `assessment-publication`: Versioned exports, publication tracking, and vendor-neutral trust-centre integration boundary.

### Modified Capabilities

None; there are no existing application specs.

## Impact

- Adds a Go module, Echo/templ frontend, PostgreSQL migrations, artifact storage abstraction, background worker, and MCP server.
- Adds authenticated integrations with AWS and GitHub; customer infrastructure repositories are read-only inputs and will not be modified by this change.
- Uses `2026-markdown/` and its pinned upstream source metadata as authoritative context; generated corpus content stays upstream-owned.
- Introduces configuration for identity, customer scopes, credentials, and storage, with optional destination-specific configuration for future trust-centre adapters.
- No embedded LLM, autonomous infrastructure modification, or assertion of FedRAMP certification is included in this initial service.
