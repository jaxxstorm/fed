# Shared-contract audit

Task 1.1 completed against the current implementation. PostgreSQL-backed tests
passed for `internal/assessment`, `internal/mcp`, and `internal/storage` using
`go test ./internal/assessment ./internal/mcp ./internal/storage -timeout 60s`.
This is a baseline check, not verification of the new workflow.

## Reusable foundations

- `internal/assessment/store.go`: immutable record revisions, service write lock,
  typed mutation validation, audit, shared service authorization and exact revision
  references. New record kinds require an additive database constraint migration.
- `internal/storage/artifacts.go`: content-addressed filesystem/S3 artifacts,
  integrity verification on read, and storage-before-record persistence. Domain
  validation must impose the new 1 MiB limit below storage's 16 MiB ceiling.
- `internal/storage/migrations/001_initial.sql`: existing operation-scoped
  idempotency table can support the new submission operations; do not create a
  second general idempotency mechanism unnecessarily.
- `internal/assessment/domain.go`: saved scope/profile, official requirement
  resolution, exact draft revisions and explicit human review.
- `internal/mcp/server.go`: authenticated adapters and shared permission checks;
  new workflow tools can be additive. Artifact retrieval currently hard-codes
  JSON media type and needs compatibility-preserving extension.

## Compatibility issue requiring a design amendment

The design requires one uploaded artifact to support multiple requirements and
produce cited drafts without measures. Existing contracts contradict this:

1. `SaveDraft` in `internal/assessment/domain.go` rejects evidence whose
   `Record.RequirementID` differs from the draft's rule.
2. `Status` in the same file marks a draft with evidence but no measures as
   missing evidence because freshness is measure-derived.
3. Requirement evidence lists in `internal/web/app.go` and MCP use `Store.List`,
   which filters the single `records.requirement_id` field. Service-level
   reusable evidence would not appear on all linked rule pages.
4. Snapshot status enumeration in `internal/assessment/publication.go` visits
   rule IDs present on records rather than all resolved profile requirements.
   The new projection must include open rules without records as well.
5. Snapshot preparation reads records in a transaction but calls status through
   the pool. New work-state export reads must share the snapshot transaction to
   avoid mixing source/result revisions under concurrent writes.

The design mentions direct assertion links but does not specify how those links
change existing draft validation, evidence-list membership and legacy freshness.
Implementing only new MCP endpoints would leave successful receipts disagreeing
with the browser and existing review path.

Amendment approved by the user and captured in design/specs:

- New documentary evidence is service-owned and reusable. Immutable rule-result
  citations bind exact evidence revisions to rules; no per-rule artifact copies.
- Both draft validation and evidence listing consult explicit authorized rule
  links as well as legacy requirement-owned evidence. A new result transaction
  stages its links before validating its cited draft.
- New evidence uses its frozen source freshness policy, while existing evidence
  retains legacy measure freshness. A draft's supporting evidence must all be
  current under the applicable policy; source policy changes reopen new work.
- Shared status/export reads accept the same query/transaction context and use
  one reference time. Include all profile rules in new snapshot work status.
- Preserve old evidence IDs, provenance, draft revisions and exported bundles;
  add regression tests for legacy and mixed-evidence drafts.

## Change ownership

- This change owns the source directory, general artifacts, direct rule/assertion
  links, rule-result receipts, evidence-work projection and external-agent prompt.
- Its integration tasks must update shared draft validation, evidence lists,
  freshness and export paths for the new evidence type.
- `automate-evidence-collection` continues to own collector scheduling, typed
  observations, collector compatibility/backfill and live collector verification.
  Its unchecked tasks remain unchecked; the existing passing tests do not prove
  every outstanding task complete.
- Shared changes must retain collector provenance and existing technical/review
  states. Evidence work completion is an additional projection, not a replacement
  for technical findings or human review.
