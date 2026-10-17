## Context

This repository contains OpenSpec and the mirrored FedRAMP 2026 Markdown corpus, with no existing application. Ignis infrastructure lives separately under `~/src/github/ignistechnologies`: Python Pulumi provisions AWS and EKS and GitHub workflows deploy it. `ignitor-ops` demonstrates existing Echo/templ conventions. Source inspection establishes intended configuration, not deployment state. The service must remain independent of the customer's current trust-centre provider.

The authoritative context is `2026-markdown/providers/20x/`, especially the KSI guidance, certification rules, Security Decision Record, and Certification Data Sharing. `_sources.json` pins Rules commit `a89d78477ddea98dc57eca24f5007794c2b4eeb6`. Structured Rules JSON and its schema must be inspected during implementation before finalizing the importer; the Markdown layout is not the JSON schema.

## Goals / Non-Goals

**Goals:**
- Give people and external agents one durable, auditable model of class-specific KSI coverage.
- Deliver the complete path: select profile, inspect requirement, run check through MCP, draft a cited assessment, review it in the UI, export a snapshot.
- Reuse external evidence through generic references and support interchangeable trust-centre adapters later.

**Non-Goals:**
- Embedded LLM orchestration, infrastructure remediation, or automatic certification decisions.
- A complete FedRAMP trust centre, full certification package generator, or vendor-specific live connector in the initial release.
- Exhaustive automated checks for every KSI. Unsupported and human-dependent measures remain visible.

## Decisions

### 1. One Go application with shared domain services

Use Echo and templ for server-rendered pages, the official Go MCP SDK for MCP, and PostgreSQL for durable state. Organize `cmd/ksi-service` and `internal/{catalog,assessment,evidence,checks,integrations,web,mcp,auth,storage}`. Pin compatible stable dependency versions during bootstrap. Support server and worker modes from the same binary; local development can run both together.

Both adapters invoke the same domain services for authorization, validation, revision checks, and transactions. This avoids duplicated behavior in a frontend API and agent API. A SPA and separate microservices add coordination without improving the first end-to-end workflow.

### 2. Immutable catalog snapshots with explicit applicability

Import the official Rules JSON and matching schema from a pinned upstream commit; retain payload hashes and source URLs. Import is an explicit command, not an automatic replacement on startup. Use the Markdown corpus for narrative references. Normalize rule/KSI IDs, text variants, normative strength, applicability conditions, dates, and dependencies without dropping original data.

Resolve requirements using type, path, class, evaluation date, and initial/ongoing context. Class A references to other rules must be included; optional, recommended, required, not applicable, and unresolved remain distinct. Higher class selection is not a blanket superset assumption. Unrecognized conditions produce unresolved status, not silent exclusion. Class D pilot-dependent requirements stay unresolved where the source is insufficient. Changing a service's pinned profile creates a new revision and never rewrites old assessments.

### 3. Evidence and reviews have independent lifecycles

Core records: organization, service, scope revision, profile revision, catalog version, requirement, measure/version, evidence, check run, result, assessment revision, review, gap, audit event, export snapshot, and publication attempt.

Evidence includes source type, account/region/resource or repository/commit, collection and observation times, collector version, content hash or external reference, limitations, and a customer-defined freshness policy. External references distinguish verified content from unverified pointers. Artifact bytes use a storage interface with filesystem storage for development and S3-compatible storage for deployment; the database stores metadata and object keys.

Measures are customer-defined interpretations mapped to official requirements. They carry rationale, scope, expected condition, cadence, and evidence needs. Evaluation statuses are pass/fail/unknown/error; evidence freshness is current/stale/missing; human review is pending/reviewed/review-needed. A technical pass does not automatically satisfy a KSI.

Assessments reference exact evidence and measure versions. Reviews record the authenticated human and reviewed revision. Changed measures, new failing checks, or expired supporting evidence mark current review as needing attention while preserving past reviews. Use optimistic concurrency on user/agent edits, and append audit events in the same transaction as changes.

### 4. Registered asynchronous checks

Persist jobs in PostgreSQL with leases, bounded attempts, deadlines, and idempotency keys; workers claim jobs transactionally and recover expired leases. A retry cannot create duplicate logical runs or erase prior observations. Scope authorization is checked before enqueue and execution.

Initial checks:
- `aws.guardduty.configuration`: enumerate detectors and feature status per configured account/region; capture collection coverage.
- `aws.eks.control-plane-logging`: enumerate configured clusters and retrieve actual enabled log types, evaluating an explicitly configured customer measure.
- `github.pulumi.workflow-evidence`: inspect configured repository files at an exact commit and collect Pulumi project/workflow evidence. Mark findings as configuration intent; do not assert live drift monitoring or deployment success.

AWS uses SDK default credentials or configured assumed roles. GitHub uses configured scoped credentials. Scope lists accounts, regions, clusters, and repositories explicitly; failures and omitted coverage remain visible. Checks retrieve observations without modifying customer infrastructure. The check registry, not arbitrary shell commands supplied through MCP, defines executable behavior.

### 5. MCP serves external agents

Expose Streamable HTTP at `/mcp` using the MCP SDK. Read-only resources include `ksi://catalog/{version}/requirements/{id}` and `ksi://services/{service_id}/requirements/{id}`. Pagination and filters bound lists; evidence artifacts are retrieved through authorized application routes rather than unbounded resource payloads.

Tools: `list_requirements`, `get_requirement_status`, `list_evidence`, `get_evidence`, `run_check`, `get_check_run`, `attach_evidence`, `save_assessment_draft`, `create_gap`, and `prepare_publication`. All service-scoped calls include a service ID; edits carry expected revisions and retryable creates accept idempotency keys. Tool results use structured schemas, stable error codes, and explicit provenance. `run_check` returns a run ID promptly.

Use application-issued opaque bearer tokens for the initial MCP client configuration, stored hashed and scoped by organization, service, and actions. Browser users authenticate with OIDC and secure sessions; browser mutations use CSRF protection. Roles distinguish viewer, editor/agent, reviewer, and administrator. Agent tokens cannot invoke human review operations. Production requires configured authentication; a deliberately enabled local development identity binds only to loopback. OAuth discovery/client authorization can be added later if the selected clients require it; initial compatibility requires clients supporting configured bearer headers.

Evidence text is untrusted content returned as data, never executable instructions. Agents are responsible for reasoning; the service verifies references, scope, schema, and permissions and never promotes an agent assertion into a collector observation.

### 6. Dashboard reflects coverage, not certification

Provide service/profile selection, KSI and broader-rule lists, applicability filters, gap summaries, requirement details, measures, evidence, check runs, draft editing, and review controls. Show freshness and unknown coverage alongside pass/fail outcomes. Human checkboxes record review of a specific revision. Show the official requirement separately from customer-defined measures and agent interpretation. Class changes preview the changed obligations and retained evidence before committing a profile revision.

### 7. Versioned exports and a vendor-neutral trust-centre boundary

Generate human-readable HTML and application JSON from the same immutable assessment snapshot. The application export is explicitly an assessment bundle, not a schema-valid FedRAMP Security Decision Record or a complete certification package. Include source versions, scope, measures, evidence references, gaps, actor/review provenance, and observation/publication timestamps. Artifact access and exports obey the same authorization as the source records.

Initially support manually registering document/result URLs from any source with owner-supplied metadata, marked unverified until content is imported or otherwise verified. Define vendor-neutral adapter capabilities for evidence listing/retrieval and publication without claiming they exist. Unsupported operations return `integration_unavailable`. Store publication states as prepared, exported, published, or failed; a successful download is exported, not published to an external trust centre.

The core model owns requirements, evidence, assessments, and snapshots using internal IDs. Optional destinations have adapter type, configuration reference, and external receipt metadata; vendor-specific fields stay within adapter configuration and mappings. No vendor account is required to run, assess, or export. Switching destinations preserves all internal history. Live connectors and official SDR schema mapping require follow-up changes once their contracts are verified; provider selection is not a prerequisite for this release.

## Risks / Trade-offs

- [FedRAMP rules evolve or have conditional applicability] → Pin source versions, preserve raw input, expose unresolved cases, and test against real source fixtures.
- [A narrow technical check is mistaken for KSI coverage] → Explicit measure rationale, scope limitations, separate review status, and no automatic certification badge.
- [Incomplete permissions yield false confidence] → Store collection errors and denominator coverage; unknown is never pass.
- [A destination's APIs or publishing are unavailable] → Manual evidence references and versioned exports remain usable; adapters report capabilities honestly.
- [Concurrent human and agent edits overwrite work] → Revision checks, idempotency keys, and transactional audit events.
- [Sensitive evidence leaks through agent or export access] → Shared scope authorization, metadata-only list responses, controlled artifact routes, and secret-free logs.
- [Initial MCP bearer authentication limits clients] → Document compatible client configuration and keep transport auth isolated for later OAuth support.

## Migration Plan

1. Bootstrap the standalone application, PostgreSQL, development artifact storage, and explicit local identity configuration.
2. Apply forward migrations and import the pinned rules snapshot after schema validation.
3. Configure an Ignis service scope with customer-confirmed accounts, regions, repositories, and target profile; do not infer assessment boundaries from directory names.
4. Run fixture-based checks, then opt into live checks with configured credentials.
5. Enable OIDC and scoped MCP tokens for deployment; configure persistent artifact storage and backups.
6. Roll back by stopping workers and reverting the application to a compatible release. Retain catalog, evidence, and audit history; restore database/artifact backups if a schema rollback is necessary rather than destructive automatic down-migrations.

## Open Questions

- Which optional trust-centre adapter should be added first after the vendor-neutral workflow is available?
- What are Ignis's precise assessment boundary, certification path/class, and initial versus ongoing context? These are onboarding inputs.
- Which OIDC provider and deployment environment will host this standalone service? Both remain configurable.
- Which external agent clients need OAuth-based MCP discovery rather than configured bearer tokens?
