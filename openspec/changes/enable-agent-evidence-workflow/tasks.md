## 1. Shared contracts and persistence

- [x] 1.1 Audit shared artifacts, scope, drafts, status and exports against the unfinished automatic-collection change; document reusable contracts and overlapping task ownership without marking unverified work complete.
- [x] 1.2 Define typed source, instruction/assertion, evidence artifact, citation, rule result and receipt contracts with bounded payloads, revision bindings, permissions and stable errors.
- [x] 1.3 Add additive migrations for source revisions, immutable rule results/assertion citations, operation idempotency and work-view versioning; verify migration repeatability and preservation of existing records/provenance.

## 2. Evidence source configuration

- [x] 2.1 Implement shared authorized source reads/writes, expected-revision conflicts, source-type boundary validation, non-secret access hints and explicit freshness policies.
- [x] 2.2 Reconcile stable sources from saved AWS/Kubernetes/repository scope in server-only mode; verify repeatability, scope narrowing, disable behavior and preservation of edited source metadata.
- [x] 2.3 Add configuration UI forms for source types, locators, associations and access hints using existing CSRF and authorization patterns; verify saved records are shared with MCP reads.

## 3. Instructions and outstanding work

- [x] 3.1 Implement versioned rule instruction resolution with official text, applicability, stable required/optional assertions, source associations, existing support and missing assertions independent of collector plans/measures.
- [x] 3.2 Review instruction templates for representative technical and documentary rules from the pinned catalog; implement and test whole-statement generic guidance fallback for all remaining statements/catalogs.
- [x] 3.3 Implement the shared evidence-work projection and relevant-change dependency mapping, including how registered observation conditions map to cited assertions; verify unrelated evidence does not invalidate completed work.
- [x] 3.4 Implement deterministic open-rule filtering and bounded continuation cursors, including service/filter binding, write invalidation and expiry-driven view changes; test KSI/FRR, unresolved and not-applicable cases.

## 4. General artifact ingestion

- [x] 4.1 Implement direct text/Markdown/JSON artifact validation, scope/source membership, original event/retrieval timestamps and the 1 MiB content limit within existing MCP limits.
- [x] 4.2 Persist content-addressed artifacts and agent provenance transactionally with canonical idempotency; test identical concurrent retries, changed-payload conflicts, storage failures and commit rollback.
- [x] 4.3 Extend authorized artifact retrieval for media-aware text and JSON without breaking existing JSON observation clients; verify escaped rendering, source/version metadata and exact byte hashes.
- [x] 4.4 Validate text line-range and JSON Pointer citations against exact stored revisions; reject missing, out-of-range and cross-service citations while allowing reuse across rules.

## 5. Rule results and completion receipts

- [x] 5.1 Implement assertion-result validation for supported/contradicted/unknown outcomes, complete/partial/blocked submissions and actionable missing-assertion errors without collector measure prerequisites.
- [x] 5.2 Atomically save result history, assertion/evidence links, cited draft and audit with expected result/draft revisions; return an immutable receipt and deduplicate retries.
- [x] 5.3 Revalidate profile/scope/instruction/source bindings at commit and test concurrent agents, draft edits, stale artifacts and scope races without crediting obsolete work as current.
- [x] 5.4 Implement freshness and relevant-source/instruction/evidence invalidation; verify current checkboxes reopen while original event times, results, reviews and receipts remain intact.

## 6. MCP workflow

- [x] 6.1 Expose list_open_rules, get_rule_instructions and list_evidence_sources with bounded schemas, source references, revision bindings, stable errors and shared service authorization.
- [x] 6.2 Expose submit_evidence_artifact and submit_rule_result with complete receipts and actionable validation errors; test read/edit/check separation, revoked tokens and cross-service references.
- [x] 6.3 Update gather_evidence to execute discovery, source investigation, artifact submission, rule-result submission and receipt inspection through the new tools; verify prompt discovery and service authorization.
- [x] 6.4 Add an SDK-client end-to-end test in server-only mode with no collector plan or measures: read open rules and sources, submit documentary evidence, complete one rule, partially support another and record an access blocker.

## 7. Completion UI and exports

- [x] 7.1 Update requirement lists/details with Evidence complete checkboxes, Agent investigation needed labels, missing assertions and explicit blockers; show technical outcome, agent assessment and human review independently.
- [x] 7.2 Add source/artifact/citation drill-downs and verify an MCP-submitted documentary result appears with the same completion state before any browser editing or human review.
- [x] 7.3 Extend immutable JSON/HTML exports with source/instruction bindings, assertion citations, receipts and snapshot-time work states; verify later freshness changes do not mutate exports.
- [x] 7.4 Test completed evidence with a failing assessment, incomplete coverage, expired evidence and new relevant failures across MCP/UI/export projections while preserving review-needed behavior.

## 8. Verification and rollout

- [x] 8.1 Document profile-to-agent onboarding, source configuration, exact MCP request/receipt examples, provenance, size limits, completion semantics and the external agent's responsibility for source access.
- [x] 8.2 Run formatting, templ generation, build/vet, PostgreSQL-backed race/integration tests, SDK/browser workflow checks and strict OpenSpec validation; resolve failures before marking implementation complete.
- [x] 8.3 Apply additive migrations and verify existing services retain scope, evidence, custom measures and history while new sources reconcile without duplicates or false completion.
- [x] 8.4 Exercise one read-only external-agent evidence investigation for ignistechnologies using configured sources and saved us-east-1 scope; verify uploaded artifacts, citations and completion/blocker receipts in the UI and document real access gaps.
