## Why

The application tracks assessments but does not yet deliver the automatic collection-to-evidence workflow: agents' infrastructure observations remain in conversations, checks require manually created measures, and collected evidence can appear missing until cited in a draft. Customers need scoped collection that durably records evidence, maps findings to requirements, refreshes it, and exposes coverage without manual transcription.

## What Changes

- Generate versioned collection plans and customer-measure defaults from the selected catalog/profile, explicit scope, registered collectors, and configured integrations; collect supported evidence without hand-entering measures.
- Enqueue initial collection when a service has a valid scope and profile, support collect-now, and refresh on durable schedules. Expose missing credentials, unsupported mappings, and policy-dependent evaluations rather than silently skipping them.
- Add MCP artifact ingestion for agent-observed AWS, Kubernetes, and repository evidence with hashes, scope validation, provenance, idempotency, and exact requirement/measure references. Agent observations remain distinct from server-collected observations.
- Add RDS encryption and backup retention, Redis transit encryption, EKS endpoint access, and Kubernetes NetworkPolicy/enforcement collectors alongside existing checks.
- Respect saved accounts, regions, clusters, repositories, and Kubernetes bindings. The current Ignis boundary is us-east-1 only; automatic discovery and refresh MUST NOT widen it.
- Display collection state, coverage, findings, artifact links, and freshness independently of draft/review state. Distinguish manual-evidence obligations from unsupported automation and not-applicable requirements.
- Preserve human review, immutable evidence and assessment history, vendor-neutral exports, and read-only infrastructure access.

## Capabilities

### New Capabilities
- `automatic-collection-planning`: Versioned collector mappings, automatic measure/plan generation, scope-safe initial and scheduled collection, and collect-now operations.
- `agent-observation-ingestion`: Authorized, durable MCP ingestion and retrieval of attributed observation artifacts and findings without external hosting.
- `infrastructure-evidence-collectors`: Scoped AWS and Kubernetes collectors for encryption, backup configuration, endpoint exposure, and network-policy evidence with explicit coverage limitations.
- `collection-evidence-visibility`: Draft-independent collection/evidence status, actionable findings, manual-evidence labels, and consistent dashboard/MCP/export views.

### Modified Capabilities

None in the main spec tree, which is currently empty. This change builds on the implemented, unarchived `go-ksi-service` change; its existing authorization, revision, human-review, and publication contracts remain prerequisites. These new specs define the added automation behavior rather than duplicating that change's capability files.

## Impact

- Go assessment, check-runner, storage, authorization, MCP, and Echo/templ modules; additive PostgreSQL migrations for plans, schedules, findings, evidence links, and ingestion metadata.
- Additional AWS RDS/ElastiCache and Kubernetes clients, operator-managed account/profile and cluster/context bindings, and documented read permissions.
- New MCP plan/collection and ingestion tools; existing evidence/status/export behavior gains provenance and draft-independent state.
- Existing services need plan reconciliation after upgrade; no deletion of saved scopes, custom measures, evidence, reviews, or artifacts. No infrastructure remediation or automatic expansion of the assessment boundary.
