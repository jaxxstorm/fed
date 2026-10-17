## Context

The existing Go application has shared assessment services, immutable record revisions, PostgreSQL jobs, filesystem/S3 artifacts, Echo/templ UI, and authenticated Streamable HTTP MCP. It has three collectors, versioned scope, operator-owned AWS profile bindings, and scope discovery limited to saved regions. `SaveScope` permits scoped agent edits. The initial `go-ksi-service` change is implemented but unarchived; the main spec directory is empty.

The missing connection is orchestration and evidence ingestion. Measures must be created manually, external evidence registration requires HTTPS URLs, and status derives freshness mostly from assessment citations. Consequently, an agent can identify infrastructure issues without making them visible as collected evidence. The customer's request is to automate this workflow, not merely describe how to configure it.

The current service is `ignistechnologies`. Its explicit AWS boundary is us-east-1, across saved development, production, shared, and management accounts. External agents have AWS profiles, Kubernetes contexts, and local repositories under `~/src/github/ignistechnologies`; those credentials and files are not necessarily available to a deployed server. Both server collection and agent ingestion must therefore be first-class paths.

## Goals / Non-Goals

**Goals:**
- Start and refresh supported collection automatically after scope/profile configuration, with no per-requirement measure entry.
- Persist immutable observations, artifacts, coverage, evaluations, and requirement links before any draft exists.
- Ingest direct agent observations without requiring external URLs or pretending they were collected by the server.
- Cover the reported RDS, Redis, EKS endpoint, and NetworkPolicy findings with reproducible read-only collectors.
- Show actionable collection failures and manual-evidence needs consistently in dashboard, MCP, and snapshots.
- Preserve existing custom measures, human review, attribution, historical revisions, and explicit regional boundaries.

**Non-Goals:**
- Infrastructure remediation, executing arbitrary MCP-supplied commands, embedded agent runtime, or automatic certification.
- Automatically widening scope to organization accounts, enabled AWS regions, kubeconfig contexts, or every local repository.
- Proving all NetworkPolicy behavior through configuration inspection, or automatically satisfying administrative obligations from infrastructure scans.
- Vendor-specific trust-centre connectors or guaranteed historical reconstruction from agent chat summaries.

## Decisions

### 1. Versioned collection templates and reconciled plans

Add a registry of collection templates separate from executable collectors. Each template identifies collector/evaluator versions, supported resource kinds, required integrations, observation schema, default cadence/freshness, customer-measure rationale, policy inputs, and explicit requirement mappings compatible with a pinned catalog. Mappings must be reviewed against the actual official statements and describe partial support, never infer coverage from similarly named rules. Catalog versions without verified mappings produce `unsupported_automation`, not guessed mappings.

Reconcile a plan keyed by service, profile revision, scope revision, and template-set version. Stable generated measure IDs carry `managed_by=collection-template` and a template version. Preserve user-owned measures and edited overrides; do not overwrite them during reconciliation. Default configuration observations run without customer policy input. Baseline evaluations (for example encryption disabled) are labeled application defaults; policy-dependent checks such as minimum retention days or approved endpoint CIDRs remain `policy_required` until configured. Collection itself continues.

Plan entries have explicit states: runnable, blocked_credentials, blocked_binding, policy_required, manual_evidence, unsupported_automation, not_applicable, or unresolved_applicability. `policy_required` is runnable for observation collection. Missing profile/scope is a visible onboarding blocker. Reconciliation runs after profile/scope mutations and at startup for existing services, using a durable outbox so commit success cannot lose the trigger. This replaces manual measure creation as the default path while retaining manual customization.

Alternative: hard-code one check per KSI in the UI. Rejected because mappings are many-to-many, catalog-dependent, and needed by MCP and workers too.

### Configuration revision amendment

Plan identity additionally includes the collection-configuration revision. Every policy, freshness, cadence, pause/resume, or managed-measure override change records an immutable configuration revision and transactionally requests reconciliation. Each plan and job freezes that revision and its policy inputs. Observation links retain the exact measure revision. A new configuration never mutates queued inputs or historical evaluations, and results from superseded bindings do not establish current coverage. Customer-edited managed measures retain their explicit overrides when defaults change.

### 2. Durable batches and schedules using the existing runner

Add collection-plan revisions, plan entries, schedules, batches, and batch-job links. A batch freezes profile/scope/measure/template bindings. Group resource collection by collector, account/region or cluster/repository target, and policy parameters; share an observation across multiple requirement links rather than rescanning for each requirement.

When both scope and profile exist, reconcile and enqueue the initial batch automatically. `collect_now` returns a batch ID immediately, reports blocked entries, and supports caller idempotency. Scheduled refresh defaults to every 24 hours with 48-hour freshness, clearly labeled configurable application defaults. Per-template/service overrides and pause/resume are versioned and audited. Scheduling is processed in worker/all mode with PostgreSQL row locks and a unique schedule-slot key; multiple workers cannot enqueue duplicate logical runs. Missed intervals coalesce into one catch-up run, not an unbounded backlog.

Scheduled work uses a durable organization/service automation identity with only collection privileges, rather than a human browser session or an expiring MCP token. API authorization still controls plan changes and collect-now. Pause, removed integration bindings, and current scope are rechecked at claim and before publishing current results. An in-flight result from an obsolete scope revision is retained as historical evidence but never credited to the current boundary. Credential failures back off on a bounded schedule and remain visible; no hot retry loop.

Continue using existing leases, deadlines, attempt history, and artifact storage. Finalizing observations, evidence links, findings, job state, and audit is transactional. Storage writes precede database commit; failed commits can leave unreferenced objects eligible for bounded orphan cleanup, never evidence rows pointing to missing blobs. Worker restarts and lease retries must deduplicate evidence by logical run/observation key.

Alternative: browser-triggered synchronous scans or in-memory timers. Rejected because navigation, restarts, and multiple workers would lose or duplicate collection.

### 3. Shared evidence ingestion with explicit provenance

Introduce a shared observation ingestion service used by collectors and MCP. An immutable observation contains service and scope revision, structured target identity, observed-at time, server receipt time, source kind, submitting actor, collector/tool name and version, collection method, artifact media type and SHA-256, limitations, coverage, and optional evaluations. Requirement/measure links include exact revisions and mapping rationale. Schema validation checks target membership and references before publishing any evidence.

Add `ingest_observation` for JSON artifacts within the existing 2 MiB MCP request ceiling (1 MiB serialized artifact limit), with an idempotency key and canonical request digest. Return evidence ID/revision, artifact hash, findings, and links. No arbitrary URL fetch or remote command execution is part of ingestion. Reusing a key with different content yields `revision_conflict`; identical concurrent requests return the same result. Large inputs return a stable size error rather than truncation. The server computes hashes and collected/received timestamps; callers cannot choose verification state or actor identity.

Server collectors produce `collector_observation`; external agents produce `agent_observation`, with claimed method/version stored as declarations. Agent-provided findings remain agent assertions unless a registered evaluator can independently evaluate the submitted structured observation; even then source provenance remains agent-authored. Existing external references remain `unverified_reference`. Scope metadata/discovery notes are configuration, not promoted to evidence.

Targets support AWS account/region/ARN, Kubernetes cluster/namespace/kind/name/UID, and repository remote/commit/path. Agent submissions outside the saved scope are rejected with actionable scope errors. Historical scope ingestion can be retained when explicitly referenced, but cannot populate current coverage. Future timestamps beyond a small documented tolerance are rejected; old observations are accepted and shown stale. Repository observations declare committed versus dirty-worktree content and configuration intent. Schemas allow only relevant configuration fields; collectors do not retrieve Secret values, database contents, or credentials. Input artifacts are never executed or rendered as trusted HTML.

Add `get_evidence_artifact` for authorized bounded JSON/text retrieval so an external MCP client can inspect submitted evidence without browser cookies. Lists remain metadata-only. Permission checks are repeated on metadata and artifact access; hashes are not authorization tokens.

Alternative: generate fake HTTPS evidence URLs or label uploaded JSON server-verified. Rejected because both misrepresent provenance and make collection depend on external hosting.

### 4. Resource collectors and evaluation boundaries

All collectors operate only on saved account/region combinations and explicitly bound clusters/repositories. Continue STS account verification and operator-managed profile/role bindings. Cluster bindings specify expected account, region, cluster identity and kubeconfig context; workers validate the binding against the target, use explicit client configuration, and never enumerate every context. A deployed worker without a binding reports a blocker; an agent can still ingest observations from its own configured context.

New collectors:
- RDS: paginated DB instance and cluster descriptions, resource ARNs, engine, replica/cluster relationships, storage encryption/KMS references, and backup retention. Evaluate instance/cluster ownership correctly so Aurora members and replicas are not counted as independent disabled-backup primaries. Zero retention is flagged where the resource controls its own backups. Positive retention is reported but meeting a customer retention objective requires that objective. No claim of restore success from retention configuration.
- ElastiCache Redis/Valkey: supported replication-group, cache-cluster, and serverless descriptions, engine/version, transit-encryption enabled/mode, and relevant resource relationships. `preferred`/mixed-mode is not equivalent to TLS enforcement. Unsupported types or missing fields remain unknown.
- EKS endpoint access: endpoint public/private access configuration and public CIDRs. Detect `0.0.0.0/0` and `::/0` when public access is enabled; evaluate approved ranges only when provided. A private-only endpoint makes public CIDRs irrelevant. This does not establish reachability through all network layers.
- Kubernetes: namespace/workload metadata, NetworkPolicies, selectors and ingress/egress isolation, and supported CNI enforcement configuration. Report namespaces/workloads lacking isolation, unknown selector semantics, and enforcement state separately. Policy existence alone never proves enforcement; unknown CNI enforcement stays unknown. System namespace exclusions must be explicit, not hidden. No active traffic probes in this change.

Every run records expected target partitions, observed targets, explicit empty inventories, denied/unreachable partitions, and pagination completion. Zero discovered resources in an authorized complete listing is `no_resources`, not technical pass; failed enumeration is unknown/error. Resource-level findings are keyed by service, check, target and condition. Failed/partial scans cannot resolve prior findings; resolution requires a fresh, complete relevant observation. Collectors retain successful partitions when others fail.

Alternative: have an LLM interpret raw API output and publish pass/fail directly. Rejected for built-in evaluations because deterministic, versioned rules are reproducible; agents remain useful for supplemental evidence and assessment drafting.

### 5. Draft-independent status and collection-first UI

Read current evidence through observation links to current scope/plan/measure versions, not through draft citations. Preserve a separate projection of evidence supporting a reviewed draft. Current collection state, observed/expected coverage, evidence freshness, technical outcome, open findings, and human-review status are independent fields.

Requirement rows show last collection, coverage, evidence count, and automation eligibility. Manual administrative obligations display `manual evidence required`; unsupported mappings display `automation not available`; not-applicable requirements do not display misleading missing-evidence alerts. Unknown applicability stays explicit. A single passing observation cannot mask other failed, missing, or stale plan entries.

Add a service collection overview with initial/running/completed/partial/blocked states, next refresh, collect-now, pause/resume, and actionable permission/binding/policy gaps. Detail pages expose resource findings, source type, artifact access, collection failures, timestamps, and mapping rationale. Infrastructure findings appear as soon as evidence is finalized, without agent-authored prose. Successful collection never creates a human review. New failures or expired supporting evidence preserve existing review-needed behavior.

Extend MCP with `get_collection_plan`, `collect_now`, `get_collection_batch`, `configure_collection` (expected revision, cadence/pause/policy overrides), `ingest_observation`, and `get_evidence_artifact`. Use existing service-scoped read/edit/check/export grants: read for reads, check for collect-now, edit for configuration/ingestion. Browser configuration uses CSRF protection. Snapshots freeze observation links, collection/evaluation versions, coverage, findings, freshness-at-export and provenance alongside assessment history.

Alternative: auto-create drafts just to activate existing status code. Rejected because observation existence and assessment interpretation are independent states.

## Risks / Trade-offs

- [Mapping technical checks to official obligations overstates coverage] → Versioned, tested mappings with explicit rationale and limitations; no automatic compliance badge.
- [Automatic runs produce unexpected API load] → Target-level deduplication, concurrency limits, daily defaults, pause/resume, backoff, and visible schedules.
- [Agent payloads are inaccurate or sensitive] → Distinct provenance, bounded typed inputs, allowlisted collector fields, no secret retrieval, authorized artifacts, and no automatic human review.
- [Kubernetes policy inspection cannot establish runtime enforcement universally] → Versioned supported CNI detectors and an explicit unknown state; no pass from policy existence alone.
- [Profile/scope changes race collection] → Freeze input revisions and recheck current bindings; retain obsolete results only as history.
- [Aggregation is expensive] → Query indexed current observation/link/finding projections in batches rather than a query per requirement; test pagination and representative service sizes.
- [Existing APIs or deployments assume three collectors and ten tools] → Additive schemas/tool discovery, compatibility tests, refreshed documentation, and explicit integration blockers.

## Migration Plan

1. Add schema and typed models for managed measures, plans/schedules/batches, observations/links, finding history, and automation identity. Keep existing records and jobs readable.
2. Introduce shared ingestion and projection services; adapt existing collectors to use them. Preserve original evidence verification states; infer legacy links only from stored run/measure bindings and mark unmappable evidence as legacy/unmapped.
3. Register reviewed mappings and new collectors. Reconcile existing services without modifying scope or custom measures. Queue initial collection when prerequisites exist; surface blockers otherwise.
4. Deploy UI/MCP additions and scheduler-enabled workers. For Ignis, verify all requests remain in us-east-1 and explicitly bound clusters. Authenticate required profiles and inspect credential gaps; do not manufacture findings from the earlier chat summary.
5. Verify deterministic end-to-end behavior, then run live scoped collection and confirm evidence is visible before creating any draft.
6. Roll back by pausing schedules and deploying a schema-compatible binary. Retain evidence, revisions, audit and artifact objects. New schema is forward-only; database/artifact restore is a coordinated operational rollback, not destructive down-migration.

## Open Questions

- Customer-specific minimum backup retention, approved endpoint CIDRs, and namespace exclusions need configuration; collection proceeds with observed facts and `policy_required` evaluations meanwhile.
- Audit/Log Archive credentials and exact cluster-context bindings must be supplied where absent; implementation and tests do not depend on live access.
- Archived dependency retirement and external-service boundaries need customer evidence. They remain explicit gaps rather than automatic exclusions.
