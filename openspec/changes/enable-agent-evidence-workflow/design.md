## Context

The Go service already resolves profile-specific requirements, authenticates service-scoped MCP clients, stores immutable evidence and drafts, and exposes a versioned scope. `list_requirements` filters catalog properties rather than outstanding evidence work. `get_assessment_scope` exposes AWS and repository boundaries but no general source directory. `ingest_observation` accepts specific configuration schemas and exact measure links; `attach_evidence` accepts external references. Neither is a general documentary evidence workflow. The newly added `gather_evidence` prompt describes an investigation loop but cannot supply the missing contracts.

The user's workflow is: choose profile, connect an external agent, discover open rules and instructions, discover configured sources, investigate using the agent's tools, and submit evidence for a clean completion receipt. No agent lifecycle is managed by this service. The active automatic-collection change is incomplete and overlapping code must be verified rather than assumed finished.

## Goals / Non-Goals

**Goals:**
- Complete the workflow over MCP using external agents with their own credentials and tools.
- Make every applicable rule discoverable, including documentary obligations, unsupported collector mappings, and unresolved applicability.
- Provide exact evidence expectations, source context, direct artifact submission, citation links, and truthful completion states.
- Reuse existing storage, authorization, catalog, UI and collector evidence while eliminating manual measure setup from the external-agent path.

**Non-Goals:**
- Embedded models, model selection, autonomous dispatch, agent processes, mandatory task claims/leases, or scheduled LLM execution.
- Infrastructure remediation, automatic human approval, or certification claims.
- Server-side fetching of arbitrary source URLs, storage of agent credentials, or universal document parsing.

## Decisions

### 1. A pull-oriented MCP workflow over shared application services

Add `list_open_rules`, `get_rule_instructions`, `list_evidence_sources`, `submit_evidence_artifact`, and `submit_rule_result`. Update `gather_evidence` to use these tools. Existing tools remain available. Discovery and instructions work in server-only mode even if collector plans have never reconciled. Use scoped `read` grants for discovery/artifact reads, `edit` for source changes and submissions, and retain `check` for invoking collectors. Source editing initially uses the configuration UI and shared application service; it is not a prerequisite MCP permission for investigation.

`list_open_rules` defaults to all applicable KSI/FRR rules whose evidence-work state is open, partial, blocked, or needs_refresh; unresolved applicability is included explicitly. Not-applicable and evidence-complete rules are omitted by default but remain inspectable. Use bounded page sizes (default 25, maximum 100), deterministic rule-ID order, and an opaque continuation cursor bound to service, filters and a work-view revision. If relevant state changes between pages, return `view_changed` with restart instructions instead of silently skipping rules. Details are fetched separately to keep responses bounded.

Alternative: require agents to join catalog lists, status, plans, evidence and gaps client-side. Rejected because it reproduces the current incomplete workflow. Mandatory work claims are unnecessary; optimistic revisions and idempotency handle concurrent agents without a queue.

### 2. Versioned instructions describe evidence obligations

An instruction bundle identifies catalog snapshot, profile/scope revisions, rule ID, instruction version, and relevant source revisions. Include official text and applicability rationale, stable assertion IDs, required versus optional assertions, acceptable evidence kinds, suggested investigation steps, relevant source IDs, existing evidence/citations, missing assertions, limitations, and the submission schema. Guidance is application-authored and visibly distinct from official text.

Create a reviewed instruction registry for the pinned catalog. Assertions cover the statement's evidence obligations; completing only a subset cannot earn a complete checkbox. Where no reviewed decomposition exists, provide a fallback assertion requiring a cited analysis against the entire official statement, explicitly label guidance as generic, and retain agent-asserted assessment provenance. Unknown applicability must be recorded as unresolved rather than guessed. The agent can gather evidence and record its assessment but cannot change service applicability through a completion submission.

Instructions never require a collector measure for documentary submission. Persist direct requirement/assertion links, reusing exact existing evidence revisions where relevant. Existing configuration observations retain their original measure bindings and deterministic evaluations.

Alternative: treat collector mappings as the complete evidence specification. Rejected because those mappings provide only partial technical support and omit organizational obligations.

### 3. Sources are shared configuration, separate from credentials

Add immutable service-owned source revisions with stable IDs, type, name, locator, relevant scope references, optional rule/category associations, allowed paths/targets, non-secret access hints, enabled state, and freshness policy. Supported types: repository, AWS, Kubernetes, document collection, and operational-record system. Locators are references for the external agent; the server does not fetch them. Initial direct artifacts support UTF-8 text/Markdown and JSON; agents can submit selected text excerpts from other formats with original source/version and extraction limitations.

The UI and `list_evidence_sources` read the same data. Seed source entries idempotently from existing saved scope, preserving user-edited source metadata on reconciliation. AWS/repository/cluster scope remains authoritative: a source entry cannot add regions or targets outside it. Document/record source boundaries are explicitly configured, rather than forced into AWS scope. Disabling or narrowing a source changes current evidence eligibility but retains historic artifacts. Access hints include profile/context names or client-side integration names, never tokens, cookies or credential-bearing URLs. A configured source does not imply that the current agent can access it.

Alternative: reuse only `Scope` fields. Rejected because a repository commit is not a document-system locator, access hint, or source freshness policy. Reuse scope identities rather than duplicating boundary authority.

### 4. Separate artifact ingestion from rule-result submission

`submit_evidence_artifact` stores one bounded artifact with an idempotency key, source ID/revision, source locator/version, scope revision, media type, observed/event time, retrieved-at time, method/tool declaration, description and limitations. Enforce a maximum of 1 MiB of UTF-8 artifact content and the existing 2 MiB MCP request limit, with stable `artifact_too_large` errors and guidance to submit focused excerpts. Do not silently truncate. Store content-addressed bytes before atomically recording provenance and evidence metadata. Compute hashes, actor, and receipt time server-side; agent evidence remains agent-authored. Reuse `get_evidence_artifact` with media-aware bounded responses, preserving existing JSON consumers.

`submit_rule_result` references already stored evidence IDs/revisions with line ranges for text or JSON Pointers for JSON. Verify citations resolve against exact immutable content. Include the instruction bindings, expected prior result revision (0 initially), per-assertion cited rationale and outcome, missing items/blockers, and requested work state. Assertion outcomes are `supported`, `contradicted`, or `unknown`. Supported means the agent asserts the claim is evidenced, not that the server independently verified meaning. A contradiction is useful evidence and can finish evidence work while leaving the assessment failing.

The result transaction saves immutable result/assertion links, audit, and a cited draft using existing review semantics, then returns a receipt with result revision, evidence references, draft revision, computed work state, assessment outcome, missing assertions and review state. Zero-evidence blocked submissions are allowed; complete submissions require citations for every required assertion, no unknown assertions, current bindings and no unresolved applicability or blockers. Reject a requested complete result that fails these conditions with actionable validation details. Semantic sufficiency remains an agent assessment for human review; structural validation is not a compliance judgment.

Idempotency is scoped by service/actor/operation/key and canonical request digest. Identical retries return the original receipt; changed payloads conflict. Use expected revisions to prevent one agent overwriting another result or a concurrent draft edit; return conflicts with current revisions. Recheck bindings at commit. Artifact upload can preserve older evidence, but a stale rule-result submission cannot establish current completion. Unlinked uploads remain discoverable evidence and do not mark rules complete.

Alternative: one unbounded submission containing every artifact and rule. Rejected because it exceeds transport limits, makes retry recovery expensive, and prevents artifact reuse across rules.

### Compatibility amendment: reusable evidence and legacy drafts

New documentary evidence is service-owned. Immutable rule-result citations link exact evidence revisions to multiple requirements without copying artifacts. Shared draft validation and requirement evidence lists accept explicit rule links alongside legacy requirement-owned evidence. Result transactions stage those links before validating the cited draft. General uploads alone do not create links or completion.

New evidence freezes its source freshness policy at ingestion; legacy evidence retains measure-based freshness. Mixed drafts evaluate every cited artifact under its applicable policy. Source policy changes reopen affected evidence work, while old drafts and receipts retain their original bindings. Default source freshness is 48 hours (an application default), overridable per source; original event dates and retrieval dates remain distinct.

Status and export projections accept a shared query/transaction context and reference time. New snapshot work status includes all profile requirements, including those with no records. Existing evidence IDs, draft revisions and exported bundles remain unchanged. Regression tests cover legacy and mixed-evidence drafts.

### 5. Completion is a projection with independent assessment and review

Expose `evidence_work_state` as `open`, `partial`, `blocked`, `evidence_complete`, or `needs_refresh`; applicability is a separate dimension. Label the checkbox “Evidence complete” and show “Agent assessment: supported/contradicted/unknown” and human review separately. A fail finding can coexist with completed evidence work. “Manual evidence required” becomes “Agent investigation needed”; known missing access and required human decisions receive specific blocker messages.

The authoritative projection combines the latest valid result, required assertions, exact evidence revisions, freshness, current profile/scope/instruction/source bindings and review context. Relevant version changes or expired support reopen work as needs_refresh. A new contradictory observation affecting a cited assertion also reopens the result for reassessment; unrelated observations must not reopen every rule. The external agent sees this on its next query; no background LLM is launched. Preserve prior receipts and exports as snapshot-time history. Do not refresh an event's age merely because it was retrieved again. Source freshness and observation windows are explicit application policies, not FedRAMP-prescribed thresholds.

Use one shared projection for MCP, requirement lists/details and exports. A versioned service work-view counter or equivalent binding digest invalidates discovery cursors on relevant writes; crossing evidence freshness boundaries must also invalidate a cursor (e.g. cursor expiry at earliest freshness transition).

Alternative: directly toggle a boolean on artifact upload. Rejected because it cannot represent partial coverage, expired support, or concurrent rule changes.

## Risks / Trade-offs

- [Agent-provided rationale appears authoritative] → Distinguish official statements, application guidance, agent assertions, deterministic findings, and human reviews in contracts and UI.
- [Reviewed guidance coverage takes time] → Provide a whole-statement fallback with explicit generic-guidance labeling; never hide unsupported rules or silently omit clauses.
- [Source declarations cannot prove actual access or authenticity] → Preserve source/version declarations and content hashes as provenance, with agent verification status and concrete access blockers.
- [Source document instructions influence the agent] → Prompt identifies artifacts as evidence, not executable instructions; artifacts render escaped, with no active HTML execution.
- [Concurrent work or scope edits falsely close a rule] → Exact bindings, transaction-time validation, optimistic revisions and idempotency.
- [Overlapping unfinished changes complicate delivery] → Verify shared prerequisites first, document ownership, and leave unrelated automatic-collection tasks unchecked.
- [Large/binary evidence exceeds the first ingestion contract] → Explicit bounded text/JSON excerpt workflow with original references and extraction limitations; binary/chunked upload remains future work.

## Migration Plan

1. Audit shared assessment/status/artifact paths and the incomplete automatic-collection work; establish integration tests for reusable contracts.
2. Add source revisions, instruction bindings, rule results, assertion citations and work-view versioning with additive migrations. Seed scope-derived sources idempotently; never rewrite old evidence provenance or imply old rules are complete.
3. Deploy shared discovery and submission services plus MCP tools, then update the prompt and UI. Keep existing MCP tools compatible.
4. Verify the complete workflow against a fixture service in server-only mode using an external SDK client, including documentary evidence and failing infrastructure evidence.
5. Exercise a read-only Ignis investigation using saved sources and us-east-1 scope, recording unavailable sources as blockers. Release updated client instructions and clarify evidence-complete versus review labels.
6. Roll back to a schema-compatible application binary while retaining source/evidence/result history. Older clients continue to use existing tools; do not remove migrations or mutate historical receipts to roll back UI behavior.

## Open Questions

- Which document and operational-record systems should Ignis configure first? Generic source records unblock implementation; actual locators and access are deployment inputs.
- Which rule-specific evidence windows should replace the explicit application freshness default for policies versus operational samples? Implement per-source overrides now and retain original event dates.
- Other catalog snapshots will need reviewed instruction sets; the whole-statement fallback remains available without claiming reviewed guidance.
