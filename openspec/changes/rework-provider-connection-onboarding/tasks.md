## 1. Baseline and contracts

- [x] 1.1 Inventory existing connection/authorization/collector flows and overlapping unarchived specifications; capture connection, source, evidence, artifact and frozen-export compatibility fixtures.
- [x] 1.2 Define provider readiness, setup-step, expected-revision, access-strategy and generated-material DTOs; document standard, legacy and agent modes with transition tests.
- [x] 1.3 Inventory enabled AWS collector calls and create a versioned read-permission manifest with resource/region constraints and tests rejecting infrastructure write permissions.

## 2. Durable setup and authorization

- [x] 2.1 Add additive setup-session and access-strategy migrations; backfill legacy semantics without rewriting source/evidence payloads.
- [x] 2.2 Implement shared begin/get/update/cancel/expire setup services with 24-hour expiry, connect authorization, sanitized persistence, expected revisions and idempotent receipts.
- [x] 2.3 Add encrypted staged grants and cleanup, plus atomic verified-identity finalization that creates one connection and preserves reconnect tenant identity.
- [x] 2.4 Extend browser authorization attempts to setup sessions and independent redirect stages; test session/actor/service mismatch, expired state, replay, token revocation and disconnect/delete races.
- [x] 2.5 Add deployment-readiness reporting with redacted missing configuration and operator/customer recovery actions while preserving unrelated provider availability.

## 3. Delegated AWS setup

- [x] 3.1 Add KSI_AWS_FED_ROLE_ARN configuration and runtime STS principal verification, including role paths, assumed sessions, mismatch and task-role versus execution-role guidance.
- [x] 3.2 Generate encrypted per-setup external IDs and stable role suggestions; add authorized setup-material retrieval, explicit replacement and generation fencing.
- [x] 3.3 Render equivalent manual trust/policy instructions and safely encoded CLI scripts from the versioned manifest; test caller account mismatch, shell/JSON input handling, replay and existing-role conflicts.
- [x] 3.4 Require role ARN/external ID for delegated connections; implement scoped AssumeRole session caching/refresh and target-account verification, rejecting regions outside us-east-1 and all ambient fallback.
- [x] 3.5 Wire discovery and every AWS-backed collector to the explicit strategy; verify read-only SDK requests, trust propagation retry, permission denial, expiry and disconnect fencing.
- [x] 3.6 Add mocked configure/material → customer provision fixture → verify → discover → select → collect journeys, plus legacy profile and agent-managed compatibility cases.

## 4. Workspace consent

- [x] 4.1 Extend Google readiness and guides for OAuth web-client registration, Admin SDK project enablement, consent audience, tenant policies and exact production callback.
- [x] 4.2 Implement consent-first setup without a required customer ID; stage offline grants, resolve canonical customer identity and require confirmation before finalization.
- [x] 4.3 Add explicit denial, cancellation, missing identity scope, missing offline grant, policy restriction, wrong-customer reconnect and revoked-refresh recovery; retain omitted refresh-token replacements.
- [x] 4.4 Verify mocked consent → identity confirmation → partial Directory discovery → source selection → snapshot flow with unchanged scope/evidence until explicitly selected or collected.

## 5. GitHub App installation

- [x] 5.1 Add App readiness and registration guidance for read-only permissions, installation setup URL, OAuth callback and administrator approval prerequisites.
- [x] 5.2 Implement install/selection handoff and separate user authorization stages with durable setup state; remove manual installation-ID entry from the standard journey.
- [x] 5.3 Implement verified existing-installation selection and identity finalization using both App and user access checks; reject wrong App/account, replay and installation substitution.
- [x] 5.4 Support pending administrator approval, cancelled consent, suspended/removed installation and missing permission recovery without automatic activation or global-token fallback.
- [x] 5.5 Verify mocked installation → authorization → repository discovery → pinned source selection → read-only collection, including restricted repositories and preserved history.

## 6. Browser and MCP interfaces

- [x] 6.1 Expose shared setup/readiness/material/verification/finalization operations through session/CSRF browser endpoints and regenerate OpenAPI TypeScript contracts.
- [x] 6.2 Expose equivalent MCP tools and same-origin consent handoffs with connect/read redaction and edit requirements for scope selection; test revocation during completion and tenant isolation.
- [x] 6.3 Build original provider wizard layout with progress, prerequisites, back/resume, pending state, conflict retention and existing advanced/agent setup navigation.
- [x] 6.4 Implement AWS CLI/manual account → generated policy/trust → role ARN → verification → discovery UI with copy/download controls and explicit us-east-1 boundary.
- [x] 6.5 Implement Google readiness → consent → confirmed customer → discovery UI with actionable approval/permission failures and accurate Directory coverage.
- [x] 6.6 Implement GitHub readiness → installation/approval → user authorization → confirmed account → discovery UI with resumable callbacks and explicit repository/commit/path selection.
- [x] 6.7 Preserve source lifecycle controls, historical links and legacy/manual/Kubernetes flows; keep access verification, discovery, evidence freshness and assessment completion visibly separate.

## 7. Verification and rollout

- [x] 7.1 Add Docker Chromium journeys for all three standard provider setups, MCP handoff, consent cancellation/resume, partial access, configuration conflict and missing operator registration at 1440px, 768px and 390px.
- [x] 7.2 Verify keyboard/focus, screen-reader labels, copy/download, status announcements and secret-free browser storage; capture and inspect representative setup/recovery screenshots.
- [x] 7.3 Run PostgreSQL migration/idempotency/concurrency and collector regression tests, comparing immutable source/evidence/artifact/export baselines before and after upgrade.
- [x] 7.4 Run Go vet/race/integration, MCP regression, generated contracts, frontend type/lint/unit/browser, production/container build and strict OpenSpec validation; resolve failures and record results.
- [x] 7.5 Document ECS task-role permissions, generated target-role manifest, Google/GitHub registration, private callbacks at fedramp.ignistech.io, credential/external-ID rotation and connection-aware rollback.
- [ ] 7.6 Perform operator-owned live AWS delegated-role, Workspace OAuth and GitHub installation smoke journeys through the deployed private hostname; explicitly record unavailable registrations or consent as blockers, and do not infer live success from mocks.
