## 1. Bootstrap and contract discovery

- [x] 1.1 Inspect the pinned official Rules JSON and schema; document actual field mappings, class references, conditional applicability, and source fixtures before implementing the importer.
- [x] 1.2 Define the vendor-neutral trust-centre adapter contract and capability model, including operation without any configured provider and isolated destination-specific metadata.
- [x] 1.3 Initialize the Go module, command/module structure, compatible pinned Echo/templ/MCP SDK dependencies, build targets, and frontend generation.
- [x] 1.4 Add validated runtime configuration, server/worker modes, local PostgreSQL setup, and explicitly enabled loopback-only development identity.

## 2. Persistence, identity, and authorization

- [x] 2.1 Add migrations for organizations, services, scope/profile revisions, identities, permissions, and hashed revocable MCP tokens.
- [x] 2.2 Add migrations for catalog snapshots, requirements, measures, evidence, assessments, reviews, gaps, jobs/results, audit events, and publication snapshots/attempts.
- [x] 2.3 Implement shared domain authorization and transactional audit recording; verify cross-service denial and rollback on failed mutations.
- [x] 2.4 Implement OIDC browser login, secure sessions, CSRF protection, and initial administrator provisioning.
- [x] 2.5 Implement scoped MCP token issuance/revocation and actor-type restrictions; verify that agent credentials cannot record human reviews.
- [x] 2.6 Implement artifact storage interfaces with filesystem and S3-compatible backends, content hashes, and authorized retrieval.

## 3. Requirements catalog

- [x] 3.1 Implement explicit schema-validating catalog import with immutable source payloads, commit metadata, hashes, and atomic activation.
- [x] 3.2 Normalize rule/KSI variants, dates, normative strength, relationships, and Markdown narrative references while retaining original source data.
- [x] 3.3 Implement profile applicability resolution, referenced obligations, and explicit unresolved conditions.
- [x] 3.4 Implement profile-change previews and revision creation while preserving old assessment bindings.
- [x] 3.5 Verify real-source fixtures for Class A references, B/C differences, conditional dates, unresolved Class D cases, and rejected invalid imports.

## 4. Assessment and evidence domain

- [x] 4.1 Implement service onboarding and explicit account/region/cluster/repository scope configuration.
- [x] 4.2 Implement versioned measures, evidence registration, external-reference verification state, and scoped evidence queries.
- [x] 4.3 Implement assessment drafts with evidence-reference validation, expected-revision checks, and idempotent creation where applicable.
- [x] 4.4 Implement human review records, gap management, configurable freshness evaluation, and review-needed transitions without rewriting history.
- [x] 4.5 Verify stale evidence, new failures, concurrent edits, invalid citations, and technical-pass versus human-review separation.

## 5. Check execution and initial integrations

- [x] 5.1 Implement the registered-check schema, enqueue/run-result APIs, idempotency behavior, and input/scope validation.
- [x] 5.2 Implement PostgreSQL job claiming, leases, retries, deadlines, graceful worker shutdown, and recovery of interrupted attempts.
- [x] 5.3 Implement AWS credential/assumed-role configuration and the GuardDuty detector/features collector with per-account/region coverage.
- [x] 5.4 Implement the EKS control-plane logging collector and evaluator against explicitly configured customer measures.
- [x] 5.5 Implement authenticated GitHub repository inspection for Pulumi project/workflow evidence at exact commits, labeled as configuration intent.
- [x] 5.6 Verify partial collection, permission denial, missing resources, pagination, idempotent enqueue, expired leases, and unknown-check rejection using deterministic fixtures.

## 6. MCP interface

- [x] 6.1 Mount authenticated Streamable HTTP MCP and expose requirement resource templates with scoped, paginated reads.
- [x] 6.2 Implement `list_requirements`, `get_requirement_status`, `list_evidence`, and `get_evidence` with structured results and provenance.
- [x] 6.3 Implement `run_check` and `get_check_run` with prompt run-ID responses and stable error codes.
- [x] 6.4 Implement `attach_evidence`, `save_assessment_draft`, and `create_gap` through shared domain services, preserving agent attribution and revision semantics.
- [x] 6.5 Verify MCP discovery and tool calls using an SDK client, including revoked tokens, scope denial, pagination, retry behavior, and revision conflicts.

## 7. Web dashboard

- [x] 7.1 Build service/profile selection and class-change previews with official source/version details and applicability labels.
- [x] 7.2 Build KSI and broader-obligation lists with gap filters and separate technical, freshness, review, and coverage indicators.
- [x] 7.3 Build requirement detail pages with measure editing, evidence registration/viewing, draft editing, citations, and human-review checkboxes.
- [x] 7.4 Add check launch/status views and show agent-originated changes and audit history in the same records.
- [x] 7.5 Verify browser login, CSRF handling, profile switching, concurrent-edit feedback, and human review of an MCP-created draft.

## 8. Assessment exports and trust-centre boundary

- [x] 8.1 Implement immutable assessment snapshots and matching HTML/application-JSON exports with provenance, limitations, and explicit assessment-bundle labeling.
- [x] 8.2 Implement generic external evidence-reference registration and vendor-neutral adapter capability reporting with `integration_unavailable` for unsupported live operations.
- [x] 8.3 Implement optional destination configuration, publication attempt/history records, and prepared/exported/published/failed state handling without treating downloads as external publication.
- [x] 8.4 Add frontend export/history controls and the MCP `prepare_publication` tool.
- [x] 8.5 Verify snapshot immutability, cross-format consistency, authorized downloads, operation without any trust-centre provider, destination changes preserving history, and truthful publication states.

## 9. End-to-end readiness

- [x] 9.1 Exercise the full fixture-backed flow: import catalog, onboard service, select class, read requirement via MCP, run check, cite evidence in draft, review in UI, and export snapshot.
- [x] 9.2 Document local startup, catalog updates, OIDC, token management, generic MCP bearer-header configuration, collector permissions, evidence storage, and the vendor-neutral trust-centre adapter contract.
- [x] 9.3 Add deployment/container configuration, health/readiness endpoints, secret-free operational logging, and database/artifact backup and rollback instructions.
- [x] 9.4 Run formatting, templ generation, Go build/vet, focused domain/integration tests, and end-to-end checks; resolve failures and record any live-integration prerequisites.
