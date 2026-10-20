## Context

Fed uses Go/Echo, PostgreSQL, embedded React/TypeScript, content-addressed artifacts, and external agents over MCP. `SubmitRuleResult` already validates assertion citations and revision bindings and creates drafts. `Review` currently identifies an assessment revision and notes; `Gap` contains explanation/status. `PreparePublication` takes a repeatable-read snapshot of historical records and observations, producing an application bundle rather than a submission-ready package. Organization roles currently grant access across services; these roles are unsuitable for restricted external readers.

The active agent-evidence, readiness, and organization-management changes are compatibility dependencies. Guidance activation remains subject to its existing acceptance gates. The new workflow must function using official requirement context even when reviewed guidance is disabled or unavailable. Paramify's Phase One submission demonstrates useful presentation, not authoritative current Class A semantics.

## Goals / Non-Goals

**Goals:**
- Make the ordinary MCP workflow produce cited implementation narratives and explicit operational-period conclusions.
- Turn missing evidence into discoverable work and preserve internal versus independent conclusions.
- Assemble reviewed, immutable releases and deliver them through a first-party trust centre.
- Keep current working state, published state, and historical evidence independently inspectable.

**Non-Goals:**
- Operate workloads, create policies on behalf of the organization, or invent missing historical records.
- Run an embedded LLM or autonomous agent scheduler; Fed schedules collection and reporting preparation, external agents perform investigation.
- Claim FedRAMP authorization, independent assurance, or official schema conformance merely from an export.
- Implement vendor-specific trust-centre connectors, complete incident-management software, or every OSCAL model in this change.

## Decisions

### 1. Extend existing assessment records with a versioned submission contract

Introduce `implementation-assessment/v1` as a structured payload associated with existing rule results/drafts rather than a parallel assessment engine. It contains implementation description, responsible roles, mechanisms/procedures, operating frequency or justified non-applicability, boundary/profile bindings, claim-to-assertion mappings, citations, operational period, sampling method/population when known, limitations, and agent conclusions. Unknown fields carry explicit unknown reasons instead of fabricated values. Each assertion distinguishes implementation/design evidence from operating evidence and records supported, contradicted, or unknown conclusions.

Expose a versioned MCP submission tool plus enriched existing instructions and browser projections. Preserve legacy `submit_rule_result` input; legacy results remain usable but do not become release-ready through migration. Agents can draft organizational claims; a human must adopt those claims before they are represented as the organization's approved account. This is more precise than making a long explanation string the sole release contract.

### 2. Compute readiness deterministically, not by model confidence

The server checks citation existence/ranges, artifact availability, profile/scope and assertion revisions, required fields, temporal coverage declarations, and unresolved discrepancies. It reports machine-readable blockers and human-readable next actions. Structurally complete partial or negative assessments can be ready for review; ready for review never means compliant. Fresh retrieval does not refresh an old operating event. Sampling declarations never imply full population coverage.

Structural readiness is explicitly not semantic verification. Agents judge what evidence demonstrates and flag generic restatements or configuration-only operating claims; human reviewers evaluate adequacy. The server enforces declared evidence categories, coverage consistency, and required rationale, but does not infer the meaning of arbitrary artifact prose or certify agent conclusions. This distinction was confirmed by the user before implementation.

Historical-period eligibility is distinct from current freshness, as confirmed by the user. Structured assessments can cite verifiable historical records despite current expiry, while legacy submissions retain their existing current-time freshness checks. Shared projections report the current freshness of those citations separately from the immutable reporting period. Event dates must fall within that period; scope/profile, source bindings, artifact integrity, and citation validation still apply. Later source or scope changes remain revalidation reasons.

Organizational choices and periods are explicitly recorded; no universal retention period or response deadline is inferred from a pilot example. Catalog changes require explicit profile adoption. Guidance source review remains distinct from service-evidence review.

### 3. Add evidence requests and bounded issue records

Use revisioned service-scoped records with stable IDs, requirement/assertion links, source hints, owner or explicit unassigned state, requested period, acceptance criteria, and optional due date. States are open, submitted, accepted, cancelled; rejected submissions return to open with a reason. Agent fulfillment cites immutable evidence and cannot self-accept. Changed bindings produce a revalidation-needed flag and prevent acceptance against obsolete context.

Related issues link existing gaps/findings and hold owner, milestones, target dates, disposition, and closure evidence. Issue tracking records work without executing remediation. This supports package issue schedules without claiming every export is an official POA&M format. Reuse readiness work IDs where available rather than duplicating queues.

### 4. Add review cycles with explicit authority and dependency fingerprints

Review records identify human actor, organization, capacity (internal reviewer or independently designated assessor), procedures, sample, conclusion, notes, time, and exact assessment/evidence bindings. Independent designation is service-scoped and administered separately from normal edit rights. Agent tokens never record human review, independent attestation, or publication approval.

Compute a dependency fingerprint from claim revisions, assertion mapping, scope/profile, cited artifact versions, operational period, relevant issue dispositions, and evaluation settings. New unrelated evidence does not invalidate a review. Changes to bound inputs produce a comparison and current re-review requirement; historical decisions remain unchanged. Freshness expiry also changes current readiness. Package-level human approval binds the complete manifest and audience projection, not a mutable service pointer.

### 5. Introduce immutable release manifests beside legacy exports

Retain existing exports unchanged. New release preparation selects an explicit reporting period and exact revisions in a consistent snapshot. A versioned manifest includes service description, contacts, boundary, responsibilities, applicability with exclusions/unknowns, narratives, assertion conclusions, collection methods, evidence hashes and citation selectors, reviews, unresolved issues, supporting deliverables, and comparison to the previous release. Required deliverables derive from pinned applicability; unsupported or unresolved mappings remain visible blockers rather than guessed requirements.

Build deterministic JSON and a readable offline HTML view plus an evidence-inclusive archive for authorized recipients. Stable ordering and explicit serialization support manifest hashing. Hashes prove content integrity, not signer identity. Artifacts are streamed into a bounded archive rather than embedded into a database JSON row; missing/corrupt objects fail preparation with retryable diagnostics. Include schema and an offline hash-verification manifest. Evidence links in the offline package are local paths; restricted artifacts are excluded from public projections.

Release states: draft, prepared, approved, published, withdrawn. Preparation freezes bytes; modifying content creates a new release. Approval binds the manifest hash and selected audience projection. Publication records a durable first-party receipt only after routes/content are available. Retrying with the same idempotency key returns the same result; conflicting input fails. Partial assessments can be published as explicitly incomplete reports after human approval; missing review/adoption or unverifiable included artifacts block publication. Independent-assessment labels require the corresponding designated assessor conclusion. Authorization claims require separately recorded authority evidence and are never inferred.

### 6. Separate current-resource projections from evidence history

Published counts include the profile, scope, observation window, denominator definition, and coverage status. Supersession must use authoritative inventory with matching partition and credential-generation fencing. Only complete inventory can retire absent ephemeral resources; incomplete inventory retains uncertainty. Historical findings remain accessible and excluded from current counts only with an explicit lifecycle explanation. Deduplicate by stable resource identity and condition, never merely by display name. Browser, MCP, and package generation consume the same projection.

### 7. Publish audience-specific projections through Fed

Add `/trust/{service-slug}` with a public overview and published-release list, and authenticated restricted release/artifact routes. Public content is an explicit allowlist of approved fields, not a filtered copy of raw internal records. Public narratives require separate preview because prose can contain sensitive details. Raw evidence is restricted by default.

Introduce service/release-scoped external-reader and assessor grants tied to verified OIDC identities. External access does not enroll a visitor in organization-wide roles; pending visitors see only their request status. Existing issuer restrictions apply; multi-issuer federation is deferred. Administrators can approve access requests, expire/revoke grants, and audit downloads. Authorization is enforced for every manifest, search, artifact, and export request; avoid long-lived presigned links and use private/no-store caching for restricted responses. Published public downloads cannot be recalled, which is disclosed during publication.

Publication and withdrawal atomically update the public index and receipt. Withdrawal preserves internal immutable bytes and audit history while removing external delivery; a tombstone explains withdrawal without exposing restricted content. An atomic latest-published pointer never points to a draft. Existing publications remain fixed as new collection arrives.

### 8. Recurring reports create proposals, not automatic approvals

Reuse durable job/scheduling patterns to prepare candidate reporting cycles and notify internal owners through an in-product queue. Changes are categorized as implementation, evidence/freshness, applicability/boundary, review, or issue changes. Delivery retries are idempotent. Initial notifications are in-product; external email/webhooks are deferred. An agent can discover work through MCP but is not automatically invoked by Fed.

## Risks / Trade-offs

- Agent prose can overclaim → structured assertion support, explicit unknowns, human adoption, and source-linked review.
- Sensitive details can leak through summaries → separately approved public projection and access-control tests across all delivery paths.
- Large evidence collections can exceed current snapshot limits → streaming archives with explicit size limits and all-or-nothing preparation.
- Historical findings can inflate reports → authoritative lifecycle reconciliation and parity tests before publishing counts.
- Cross-change overlap can fork contracts → extend existing result, work, review, and artifact structures; retain backward-compatibility tests.
- Invalidation can overwhelm reviewers → dependency fingerprints and explicit changed-input explanations rather than global reset on every collection.
- New external identities may not authenticate with the deployed issuer → expose the constraint during access requests; do not bypass identity policy.

## Migration Plan

1. Add schema versions, record kinds, tables/indexes, grants, and optional fields using additive migrations. Backfill no approvals or fabricated narratives.
2. Implement contracts and shared projections; keep existing MCP tools and exports operational.
3. Ship internal narrative/request/review/package workflows with external publishing disabled by default.
4. Exercise the security-inbox journey and a technical requirement with partial/contradictory evidence; verify offline export and restricted-reader isolation.
5. Obtain actual human acceptance of comprehension and public projection, without treating that as acceptance of the separate readiness-guidance change.
6. Enable publication explicitly for the service; deploy via commit/push, successful container publication and checks, then approved Pulumi rollout. Recheck live browser/MCP and external-reader behavior.
7. Roll back by disabling trust-centre delivery and reporting jobs and reverting application code only when schema-compatible. Retain additive records/artifacts; never delete evidence to roll back. Revoke grants and withdraw affected releases when necessary.

## Open Questions

- Which current official submission format/profile should receive a later validated adapter? Initial outputs remain explicitly Fed assessment packages with a versioned schema.
- Which external assessor identities can use the deployed OIDC issuer? Product behavior for unsupported identities is an explicit access limitation, not organization-wide admission.
- Service owners must supply actual contacts, operational periods, owners, and disclosure choices during onboarding; these are service data, not hard-coded implementation assumptions.
