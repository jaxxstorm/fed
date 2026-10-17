## 1. Contracts and provider foundation

- [x] 1.1 Inventory current source reconciliation, scope validation, collector credential loading and MCP/browser source operations; record compatibility cases and baseline evidence/export hashes for migration tests.
- [x] 1.2 Define connection, source-binding, access-health and discovery DTOs with separate configuration/access/discovery/evidence states; add state-transition and revision-semantics tests.
- [x] 1.3 Add the provider registry and adapter interfaces for GitHub, Google Workspace, AWS, Kubernetes and manual/agent-managed access, including capability declarations and bounded HTTP/request policies.
- [x] 1.4 Define a shared versioned setup-guide/error catalog for all four integrations, identifying supported resource kinds, execution environment, required read permissions, registration prerequisites and actionable recovery codes.

## 2. Storage authorization and lifecycle

- [x] 2.1 Add migrations for connection revisions, source bindings/tombstones, credential references, authorization attempts and operation/discovery sessions, with service-scoped indexes and resource identity uniqueness.
- [x] 2.2 Implement encrypted credential storage with external versioned key configuration, contextual authenticated encryption, rotation and deletion; test missing keys, wrong-service reads and secret-free logs/API/export payloads.
- [x] 2.3 Add the connect capability to authorization and explicit agent-token issuance controls; verify old tokens retain their existing permissions and human review remains human-only.
- [x] 2.4 Implement shared create/configure/get/list/test connection operations with revision checks, idempotency receipts, audit attribution and execution-time permission checks.
- [x] 2.5 Implement renewal/disconnect/delete and source disable/retire/restore semantics, separating credential generations from evidence revisions and enforcing generation fences on late work.
- [x] 2.6 Implement source configure/create and explicit connection rebinding; route legacy source mutations/reconciliation through lifecycle checks so edit-only updates cannot bypass retirement or credential permissions.
- [x] 2.7 Add an idempotent legacy backfill for GitHub owner, AWS account/profile, Kubernetes cluster/context and manual-document connections; verify unchanged source IDs/revisions/citations and no tombstone resurrection.

## 3. Authorization attempts and browser handoff

- [x] 3.1 Implement single-use, expiring authorization attempts bound to provider/service/connection generation and initiating/completing actors; expose operation status without provider secrets.
- [x] 3.2 Add provider callback routing, configured-origin redirect construction, state/code replay defenses and applicable PKCE; test session expiry, wrong actor/service, callback cancellation and stale generation.
- [x] 3.3 Implement same-origin MCP authorization and credential-upload handoffs that bind an authorized browser session before proceeding; preserve setup state and return resumable operation outcomes.
- [x] 3.4 Implement coordinated renewal and local disconnect/remote-revocation results, preserving valid refresh tokens when responses omit replacements and preventing concurrent refresh from overwriting newer grants.

## 4. Shared discovery and selection

- [x] 4.1 Implement durable discovery sessions/pages, scoped cursors, search/selection filters, page limits and idempotent page/finalization receipts with content-mismatch conflicts.
- [x] 4.2 Implement agent-reported discovery ingestion with provider resource validation, observation/method/limitations attribution and explicit distinction from server-verified access.
- [x] 4.3 Implement stable-resource matching, duplicate detection, renames and explicit rebind behavior; unchanged scans must not revise selected evidence sources.
- [x] 4.4 Implement per-resource-kind coverage and partial failure recovery; only complete authoritative scans can mark unseen resources unavailable, and retired resources remain suppressed.
- [x] 4.5 Implement atomic source selection plus explicitly requested scope additions with connect/edit authorization and expected revisions; test races, out-of-scope selection and no partial activation.
- [x] 4.6 Add additive Workspace customer/resource and standalone Kubernetes cluster/namespace scope representations; update source locators, observation target checks and evidence validation while preserving existing scope decoding.

## 5. GitHub integration

- [x] 5.1 Add GitHub App deployment readiness/configuration and setup instructions for registration, read-only Metadata/Contents permissions, public callback/setup URLs and installation repository selection.
- [x] 5.2 Implement GitHub user authorization and verified installation/account binding; test substituted installation IDs, wrong organizations, denial and removed installations.
- [x] 5.3 Implement short-lived installation token mint/cache/renewal and paginated repository discovery with stable IDs, permission metadata and branch-to-full-commit resolution.
- [x] 5.4 Refactor GitHub collector credential resolution to use each active source connection at execution, preserving explicit legacy fallback only for unbound jobs; test that revoked bound access never falls through to a global token.
- [x] 5.5 Verify connect → discover → pinned repository/path selection → workflow artifact collection using provider mocks, including missing files, renamed repositories, rate limits and restricted installation repositories.

## 6. Google Workspace integration

- [x] 6.1 Add Google OAuth client readiness and setup instructions for Admin SDK enablement, consent audience/test users, callbacks and read-only customer/org-unit/group scopes.
- [x] 6.2 Implement offline OAuth exchange/refresh and customer identity verification through the authorized Directory customer API; test missing identity permission, partial grants, revoked refresh and omitted refresh-token replacements.
- [x] 6.3 Implement paginated Directory org-unit/group discovery using canonical customer/resource IDs, preserving partial permission outcomes and explicit initial coverage limits.
- [x] 6.4 Implement selected-resource JSON metadata snapshots through a trusted internal artifact path with immutable provenance, timestamps, ETag/hash and existing artifact-size limits; verify exact citations without auto-completing requirements.
- [x] 6.5 Verify Workspace connect → discover → source selection → evidence snapshot → renewal/disconnect using mock endpoints, including cross-customer rejection and administrator-policy failures.

## 7. AWS integration

- [x] 7.1 Add typed AWS profile/assume-role configuration and setup guides for server principal, role trust, external ID and enabled collectors' read permissions; preserve existing account/profile mappings as imported environment connections.
- [x] 7.2 Implement connection-scoped AWS credential resolution and STS account verification with SDK renewal; test wrong-account, expired SSO, assume-role denial and disconnect/revocation fencing.
- [x] 7.3 Implement account/region and paginated EKS discovery limited to us-east-1; support selecting multiple regional/cluster sources while preserving explicit per-account authorization.
- [x] 7.4 Integrate connection credentials with existing AWS collector execution and collection planning; verify all infrastructure requests remain read-only and other regions are rejected.
- [x] 7.5 Verify AWS configure → test → discover → select → collect → renew and agent-managed discovery parity with mocks, including partial account access and unaffected successful results.

## 8. Kubernetes integration

- [x] 8.1 Add typed server-context and write-only kubeconfig setup, encrypted credential storage and read-only RBAC guidance; validate TLS and configured endpoint allowlists and reject uploaded executable helpers/local file references.
- [x] 8.2 Implement cluster identity/access testing and credential renewal/replacement, supporting trusted operator-installed context authentication without executing helpers supplied by uploaded configuration.
- [x] 8.3 Implement cluster/namespace discovery with stable namespace UIDs and direct checks for known namespaces when listing is denied; test partial inventory and namespace recreation.
- [x] 8.4 Update Kubernetes collector configuration, planning and observation validation for standalone clusters and explicit namespace scope; retain legacy cluster-wide semantics and expose missing cluster-level coverage honestly.
- [x] 8.5 Verify Kubernetes connect → discover → namespace selection → read-only collection → renew/disconnect with mocks and agent discovery, including TLS errors, access denial and prevention of unrelated namespace reads.

## 9. MCP and browser APIs

- [x] 9.1 Expose provider catalog/guides and connection lifecycle/status MCP tools using shared services and explicit read/connect/edit/check permissions.
- [x] 9.2 Expose MCP authorization handoff, server/agent discovery, resource selection, source configure/delete/refresh and explicit restore/rebind operations with durable receipts and expected revisions.
- [x] 9.3 Preserve existing source/instructions/artifact/result MCP contracts with additive connection metadata and lifecycle-aware listings; test old-client submissions and exact historical references.
- [x] 9.4 Add session/CSRF-protected browser connection/source/operation endpoints and generated OpenAPI TypeScript contracts, including write-only kubeconfig handling and structured provider/field errors.
- [x] 9.5 Add MCP-to-browser integration tests proving equivalent mutations, agent attribution, tenant isolation, token revocation during operations and no credential leakage.

## 10. Connection-first web experience

- [x] 10.1 Build connection catalog/list/detail and all-sources navigation with provider/account/access-mode labels, timestamped health and actionable discovered/selected resource counts; preserve existing source links.
- [x] 10.2 Build resumable GitHub and Google setup steps from application readiness through browser consent, access test and discovery, using the shared guide catalog.
- [x] 10.3 Build AWS profile/role and Kubernetes context/kubeconfig setup steps with named fields, execution-environment guidance, identity results and actionable read-permission errors.
- [x] 10.4 Build paginated searchable source selection preserving cross-page selections, explicit scope changes, GitHub commit/path controls and Workspace/namespace identity boundaries.
- [x] 10.5 Wire configure/test/renew/disconnect/delete/restore and source refresh/retirement controls with real pending/results, affected-source context and edit retention on conflicts.
- [x] 10.6 Link existing missing-access collector errors to the appropriate connection/setup path; distinguish renewal, inventory discovery and evidence refresh, retaining successful sibling results.

## 11. Acceptance and rollout

- [x] 11.1 Add Docker Chromium journeys for all four provider setups, MCP handoff completion, partial discovery, multi-page selection, renewal, source/connection deletion and citation/history preservation using controlled provider fixtures.
- [x] 11.2 Capture and inspect connection catalog/setup/detail/source-picker/recovery views at 1440px, 768px and 390px; verify keyboard/focus, labels, contrast, status announcements and secret-free browser storage.
- [x] 11.3 Run migration and concurrency tests covering callback/delete races, refresh races, discovery replay, scope conflicts, canonical source identity and lifecycle-aware legacy reconciliation against representative existing data.
- [x] 11.4 Run generated-contract, frontend type/lint/unit/browser, Go vet/race/integration, MCP regression, production/container build and strict OpenSpec checks; record results and resolve failures.
- [x] 11.5 Document provider app registration, AWS trust/profile setup, Kubernetes RBAC/credential setup, callback origins, encryption-key rotation, agent permissions and connection-aware rollback procedures.
- [ ] 11.6 Deploy locally and verify read-only GitHub, Google Workspace, AWS and Kubernetes smoke journeys with available operator-owned registrations/credentials; leave unavailable provider verification explicitly blocked, and compare evidence/export baselines before closing the change.
