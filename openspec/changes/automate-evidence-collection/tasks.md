## 1. Contracts and persistence

- [x] 1.1 Review the pinned catalog statements and define versioned mappings for existing/new collectors and manual obligations, including rationale, partial coverage, policy inputs and unsupported-catalog behavior.
- [x] 1.2 Define typed observation, target, coverage, evaluation, finding and evidence-link schemas; define stable MCP validation/size/scope errors and payload limits.
- [x] 1.3 Add additive PostgreSQL migrations for collection plans/entries, managed-measure metadata, outbox events, schedules, batches/job links and service automation identities; verify repeatable migration and existing-record compatibility.
- [x] 1.4 Add observation/link, ingestion-idempotency and finding-history persistence with indexes and uniqueness constraints; verify transaction rollback and cross-service constraints.

## 2. Shared observation ingestion

- [x] 2.1 Implement authorized ingestion validation for scope revisions, AWS/cluster/repository identities, exact requirement/measure references, timestamps and typed artifacts; reject out-of-bound targets and malformed/oversize payloads.
- [x] 2.2 Implement content-addressed artifact storage and transactional evidence/link/finding/audit finalization with canonical request digests and concurrent retry deduplication; verify storage and commit failure behavior.
- [x] 2.3 Separate agent observations, server collector observations and unverified external references; enforce server-owned identity/hash/receipt fields and label agent assertions separately from registered evaluations.
- [ ] 2.4 Adapt the existing three collectors to shared ingestion and immutable run bindings; preserve legacy provenance and backfill links only where stored bindings establish them.
- [x] 2.5 Implement stable finding identity and append-only observation/resolution history; verify partial or failed collection cannot resolve a previous finding.

## 3. Automatic plans and measures

- [x] 3.1 Implement collection template registration, generated measure IDs, explicit application defaults and requirement eligibility classification against real catalog fixtures.
- [x] 3.2 Implement idempotent plan reconciliation on profile/scope/template changes, preserving custom measures and overrides; verify missing policy permits observation collection and unsupported mappings remain explicit.
- [x] 3.3 Publish reconciliation events transactionally from scope/profile changes and reconcile existing services at startup; verify ready services automatically receive their initial collection batch.
- [x] 3.4 Implement immutable configuration revisions in plan identity and run bindings, versioned cadence/freshness/policy overrides and pause/resume with shared authorization, revision conflict handling and audit records.

## 4. Batch execution and scheduling

- [ ] 4.1 Implement asynchronous collect-now and batch status services, grouping shared target collection across requirement mappings and deduplicating caller retries.
- [ ] 4.2 Implement durable scheduled refresh with service automation identity, default 24-hour cadence/48-hour freshness, unique due-slot claims, bounded concurrency/backoff and coalesced catch-up.
- [ ] 4.3 Integrate schedule/outbox processing with worker/all modes and graceful shutdown; verify multi-worker claiming, pause behavior, expired leases and restart recovery.
- [ ] 4.4 Revalidate current boundaries and bindings at execution/finalization; retain obsolete results only as history and verify no us-east-1 scope run targets another region.

## 5. Infrastructure collectors

- [ ] 5.1 Add pinned RDS/ElastiCache/Kubernetes client dependencies and operator-managed cluster/context bindings; verify AWS account and cluster identity checks and actionable missing-binding errors.
- [ ] 5.2 Implement paginated RDS instance/cluster encryption and backup observations/evaluations with primary/replica/Aurora handling; test zero retention, missing fields and policy-required retention objectives.
- [ ] 5.3 Implement supported ElastiCache Redis/Valkey transit-encryption observations/evaluations, including replication groups, serverless resources, optional TLS modes and unsupported-field behavior.
- [ ] 5.4 Implement EKS endpoint configuration collection/evaluation for IPv4/IPv6 unrestricted CIDRs, private-only endpoints and customer-approved ranges.
- [ ] 5.5 Implement read-only Kubernetes namespace/workload/NetworkPolicy collection, selector isolation analysis, explicit namespace exclusions and supported CNI enforcement detection; preserve unknown enforcement where unsupported.
- [ ] 5.6 Add deterministic collector fixtures covering pagination, complete-empty inventories, denied targets, partial collections, identity mismatch and no Secret-value retrieval or mutation APIs.

## 6. MCP workflow

- [x] 6.1 Expose ingest_observation and get_evidence_artifact with bounded schemas, provenance, exact links and shared service authorization; add SDK-client tests for idempotency, spoofing, scope denial and revoked tokens.
- [x] 6.2 Expose get_collection_plan, collect_now, get_collection_batch and configure_collection with pagination where needed, prompt batch responses and stable errors; verify read/edit/check separation and revision conflicts.
- [ ] 6.3 Document and exercise an agent workflow that reads the plan, inspects in-scope resources, submits a direct JSON observation and retrieves its artifact without an external URL or browser session.

## 7. Evidence-first views and exports

- [ ] 7.1 Implement indexed current observation/coverage/freshness projections independent of drafts; keep historical draft support separate and verify mixed pass/fail/missing/stale entries aggregate truthfully.
- [ ] 7.2 Update requirement lists and MCP status with evidence counts, last collection, coverage and automation/manual/not-applicable labels; test that collecting evidence does not create human review.
- [ ] 7.3 Build service collection overview and plan/batch pages with next refresh, collect-now, pause/resume and credential/binding/policy blockers; wire browser actions through shared services with CSRF protection.
- [x] 7.4 Add resource finding, provenance, limitation and artifact drill-downs; verify an MCP-ingested observation appears in the browser before any assessment draft exists.
- [ ] 7.5 Extend immutable HTML/JSON snapshots with observation links, collection/evaluator versions, findings, coverage and snapshot-time freshness; preserve existing review-needed transitions and test matching exports without drafts.

## 8. Verification and rollout

- [ ] 8.1 Run a fixture-backed full flow: configure scope/profile, automatically generate plan/measures, collect mixed successful/failing resources, display evidence without drafts, ingest agent evidence, schedule refresh and export a snapshot.
- [ ] 8.2 Verify migration from existing services retains scopes/custom measures/history, scope changes cannot credit obsolete results, failed scans do not close findings, and expired evidence updates current review status.
- [x] 8.3 Document template coverage, read-only AWS/Kubernetes permissions, profile/context bindings, policy defaults, scheduler operations, ingestion limits and rollout/rollback procedures.
- [ ] 8.4 Run formatting, templ generation, build/vet, PostgreSQL integration and race tests, SDK/browser end-to-end checks, and strict OpenSpec validation; fix failures before marking implementation complete.
- [ ] 8.5 Run scoped live collection for ignistechnologies using its configured us-east-1 accounts and explicitly bound clusters; verify dashboard artifacts/findings against fresh observations and record unavailable credentials/dependencies as gaps rather than claiming complete coverage.
