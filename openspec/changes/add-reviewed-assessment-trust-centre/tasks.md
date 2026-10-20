## 1. Contracts and persistence

- [x] 1.1 Map the existing agent-result, readiness-work, review, publication, and organization contracts to this change; document reusable record IDs and compatibility boundaries.
- [x] 1.2 Define versioned Go/JSON/frontend contracts for implementation narratives, operational coverage, readiness blockers, and adoption/review decisions.
- [x] 1.3 Add additive persistence for evidence requests, issue milestones, review dependencies, and service description/contact/responsibility metadata without backfilling approvals.
- [x] 1.4 Add release manifests, lifecycle/receipt records, reporting-cycle records, scoped external grants, and indexed lookup migrations.
- [x] 1.5 Test migration from existing service data and ensure legacy MCP submissions, review history, and HTML/JSON exports remain readable.

## 2. Agent investigation and readiness

- [x] 2.1 Enrich MCP instructions and the gather-evidence prompt with the structured narrative, assertion, operational-period, sampling, and explicit-unknown contract; retain guidance-disabled behavior.
- [x] 2.2 Implement versioned implementation-assessment submission reusing artifact/citation validation, atomic revision fencing, and idempotency receipts.
- [x] 2.3 Implement separate implementation and operating-support conclusions, event/retrieval timestamps, reporting periods, sample/population declarations, and limitations.
- [x] 2.4 Add deterministic ready-for-review checks and actionable blocker projections without equating partial reviewability to compliance.
- [x] 2.5 Add browser narrative/detail views using the same projection as MCP, including legacy-draft enrichment requirements.
- [x] 2.6 Test invalid citations, unsupported operating conclusions, scope/profile conflicts, retry conflicts, explicit contradictions, and limited-sample coverage.

## 3. Evidence requests and issues

- [x] 3.1 Implement request create/update/list/get operations with binding revisions, owners, criteria, periods, source hints, filtering, pagination, and readiness-work links.
- [x] 3.2 Implement citation-based fulfillment and human accept/reject/cancel transitions with obsolete-binding protection and audit events.
- [x] 3.3 Implement issue ownership, milestones, target dates, dispositions, and evidence-bound human closure linked to existing gaps/findings.
- [x] 3.4 Expose requests and issues in browser and MCP; make remaining requests discoverable in the agent's normal work queue.
- [x] 3.5 Test stale fulfillment acceptance, agent self-acceptance denial, duplicate work prevention, issue closure provenance, and service isolation.

## 4. Human adoption and review cycles

- [x] 4.1 Implement human organizational adoption and internal/independent review records with procedures, sample, conclusion, organization, capacity, and exact evidence bindings.
- [x] 4.2 Implement separately administered service-scoped assessor designation and reject agent adoption, human-review, attestation, and publication-approval calls.
- [x] 4.3 Implement dependency fingerprints, freshness evaluation, changed-input comparisons, and selective re-review requirements while preserving historical decisions.
- [x] 4.4 Add browser review workflow for supported, partial, and negative assessments; expose review status and comparisons to agents read-only.
- [x] 4.5 Test relevant versus unrelated evidence changes, period/profile/policy changes, assessor authority, and separation from guidance-source reviews.

## 5. Current-resource reporting integrity

- [x] 5.1 Define authoritative inventory completion and resource-lifecycle records with partition, scope/profile, and credential-generation fencing.
- [x] 5.2 Implement stable-identity/condition deduplication and retirement only after complete compatible inventory, retaining historical findings and lifecycle explanations.
- [x] 5.3 Share current-resource counts, uncertainty, denominator definitions, and observation windows across browser, MCP, and package projections.
- [x] 5.4 Test ephemeral resource retirement, recreated names with different identities, partial inventory, obsolete credentials, stale evidence, and projection parity.

## 6. Assessment packages

- [x] 6.1 Implement service-context editing and explicit reporting-period selection with required deliverable mappings derived from pinned applicability and visible unresolved mappings.
- [x] 6.2 Build consistent-snapshot release manifests containing applicability, narratives, assertions, evidence, methods, reviews, issues, deliverables, and prior-release references.
- [x] 6.3 Implement deterministic JSON serialization, manifest hashes, schema validation, and readable offline HTML from the frozen manifest.
- [x] 6.4 Implement bounded streaming evidence archives with local citation paths, schema, verification metadata, and explicit missing/corrupt/oversized artifact failures.
- [x] 6.5 Implement prepared/approved release transitions and approval bound to manifest plus audience projection; preserve incomplete-report labels and authority requirements.
- [x] 6.6 Add the assessment-package workspace with content readiness, blockers, preview, exports, and exact-version approval.
- [x] 6.7 Test immutable release bytes after later collection, deterministic regeneration, concurrent edits, idempotency conflicts, offline verification, and large-package failure recovery.

## 7. Trust-centre access and publication

- [x] 7.1 Implement public service/release routes using explicit approved field allowlists and separate public narrative previews.
- [x] 7.2 Implement verified-identity access requests and scoped reader/assessor grants without granting organization-wide membership; expose unsupported-issuer limitations.
- [x] 7.3 Implement authorization on every restricted page, search, manifest, artifact, and export path with grant expiry/revocation and appropriate cache headers.
- [x] 7.4 Implement atomic publish receipts, release index/latest pointer, withdrawal tombstones, and idempotent recovery using the first-party destination.
- [x] 7.5 Build reader-facing requirement views with plain-language implementation, support, limitations, review capacity, issues, and expandable diagnostics.
- [x] 7.6 Test public-data leakage, cross-service/release access, revoked sessions/grants, artifact URL authorization, unpublished content isolation, and withdrawal delivery behavior.

## 8. Recurring reporting

- [x] 8.1 Implement categorized comparisons between releases for implementation, applicability/boundary, evidence/freshness, reviews, and issues.
- [x] 8.2 Add on-demand and scheduled candidate reporting jobs with idempotent period keys and durable retry handling.
- [x] 8.3 Add in-product owner notifications and MCP discovery of resulting evidence/re-review work without automatically invoking agents or publishing.
- [x] 8.4 Test repeated schedules, overlapping reporting periods, expired supporting evidence, notifications, and immutability of the previous published release.

## 9. Verification and rollout

- [x] 9.1 Run a browser/MCP end-to-end security-inbox journey from agent investigation through operating records, human adoption/review, offline package, and public/restricted publication.
- [x] 9.2 Run a technical requirement journey with conflicting evidence, access gaps, incomplete inventory, an open issue, and an explicitly incomplete published assessment.
- [x] 9.3 Run contract checks, frontend lint/unit/build checks, PostgreSQL-backed race tests, browser access-control journeys, and package integrity checks appropriate to the change.
- [ ] 9.4 Record genuine human comprehension and disclosure-preview acceptance; retain separate unresolved guidance acceptance gates.
- [x] 9.5 Document MCP workflows, reviewer/assessor responsibilities, external-reader setup, package schema/limitations, reporting schedules, and withdrawal/rollback procedures.
- [x] 9.6 Commit/push the approved implementation, verify successful CI and container publication, and perform the approved Pulumi deployment with publication initially disabled.
- [ ] 9.7 Verify live browser/MCP parity, package integrity, scoped external access, and readiness; explicitly enable the service trust centre after the approved release/disclosure checks and record the publication receipt.
