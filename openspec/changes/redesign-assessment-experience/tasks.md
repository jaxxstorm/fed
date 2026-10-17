## 1. Experience and domain contract inventory

- [x] 1.1 Inventory all existing browser routes, forms, downloads and operator actions; create a parity checklist covering onboarding, overview, requirements, evidence, activity, sources, review, settings and exports.
- [x] 1.2 Define and test the primary-status truth table, secondary labels and progress denominators against existing evidence-work, technical, applicability and review states, including empty/unknown/stale cases.
- [x] 1.3 Create representative fixture data for mixed requirement states, partial collection, access failures, agent accomplishments, hundreds of artifacts, long names and incomplete onboarding.
- [x] 1.4 Produce original wireframes and design tokens for the overview, requirement detail, evidence preview and source/error workflows; document page questions, primary actions and concise copy conventions.

## 2. Frontend foundation and delivery

- [x] 2.1 Create the Bun-managed React/TypeScript/Vite workspace with pinned dependencies, strict type checking, linting, formatting, Vitest and Playwright scripts.
- [x] 2.2 Implement the responsive application shell, service switcher, sidebar, breadcrumbs, route/query-state handling and reusable visual components using accessible primitives.
- [x] 2.3 Build a fixture gallery for status badges, progress summaries, attention items, tables, forms, evidence previews and loading/empty/error states; inspect desktop and mobile screenshots before route integration.
- [x] 2.4 Add Go static-bundle delivery and development proxy support; update Makefile/Docker builds to package real frontend assets, reject missing release assets and retain a Go-only production runtime.

## 3. Shared browser APIs and progress projections

- [x] 3.1 Define OpenAPI contracts for session bootstrap, service overview, requirement/evidence lists and details, activity, sources, settings and exports; generate and verify TypeScript client types.
- [x] 3.2 Add browser-session API authentication, capability discovery, CSRF handling and structured 401/403 responses; verify existing OIDC/development auth and MCP behavior remain compatible.
- [x] 3.3 Implement batched requirement/category/service projections with a common reference time, explicit denominator units, primary/secondary statuses and drill-down filters; assert bounded database query counts on representative data.
- [x] 3.4 Implement paginated/filterable evidence, activity and source read models with meaningful labels, exact revision references, lazy artifact retrieval and known source-access outcomes; preserve unknown health explicitly.
- [x] 3.5 Expose browser mutation adapters for existing scope/profile/source/draft/review/collection/token/export services with shared authorization, expected revisions, idempotency and canonical post-save state.

## 4. Actionable errors and recovery

- [x] 4.1 Define a structured error/attention catalog with titles, affected targets, execution environment, field paths, retryability, next actions and safe diagnostic references; inventory legacy error fallbacks.
- [x] 4.2 Enrich known collector access/error results additively where target or cause metadata is missing; verify expired access, denied cluster reads and partial collection produce precise messages without guessing unknown causes.
- [x] 4.3 Implement shared frontend error, empty, loading, stale-data and session-expiry states; preserve successful data and timestamps when refreshes fail.
- [x] 4.4 Implement inline field validation and revision-conflict reload/compare behavior that retains user edits; verify duplicate pending submits are prevented and retries use the existing operation contracts.
- [x] 4.5 Wire attention actions to real settings, source, requirement, retry or copyable agent-instruction destinations; verify no decorative fix buttons or error acknowledgements falsely resolve underlying work.

## 5. Overview requirements and agent achievements

- [x] 5.1 Build service selection and overview with clear readiness/evidence/review summaries, category breakdown, scope decisions and prioritized attention items; make every count drill into matching work.
- [x] 5.2 Build searchable/filterable requirements views with category grouping, readable primary/secondary status and shareable query-state URLs; verify counts match overview totals.
- [x] 5.3 Build requirement details with status reason, assertion checklist, assessment summary, missing evidence, findings and next steps; move raw records to optional diagnostics.
- [x] 5.4 Build agent accomplishment and collector activity views from persisted receipts/audit/jobs, including meaningful timestamps, outcomes, partial results and run details without invented agent-session progress.

## 6. Evidence sources and operator actions

- [x] 6.1 Build the evidence library with search, source/type/freshness/requirement filters, bounded pagination and meaningful source/date/provenance labels instead of UUID headings.
- [x] 6.2 Build safe text/Markdown and JSON field previews with exact citation navigation, linked requirements, improvement guidance, missing-artifact recovery and optional raw details.
- [x] 6.3 Build the revision-aware review journey with evidence inspection, notes, explicit submission feedback and preserved input on concurrent agent edits.
- [x] 6.4 Build source directory and typed source/scope/profile configuration forms, including repeatable account/region/repository/path rows, source health, policy/freshness settings and profile-change preview.
- [x] 6.5 Build onboarding and agent handoff with setup checklist, scoped copyable prompt/connection instructions and explicit token issuance/revocation; verify ordinary setup requires no JSON editing.

## 7. Collection settings exports and route parity

- [x] 7.1 Build normal controls for collect-now, pause/resume, cadence and policy inputs with per-target run outcomes, pending/success/error feedback and relevant recovery links.
- [x] 7.2 Build export/history pages showing snapshot date, profile, frozen progress, outstanding items, revision history and working download actions.
- [x] 7.3 Complete route parity for settings, custom measure/draft/gap actions and all existing deep links; verify the SPA fallback never captures MCP, API, auth, health or artifact/export requests.
- [x] 7.4 Review copy across every route: replace implementation jargon, generic caveats and JSON-dependent instructions with clear achievements, missing items, improvements and next actions.

## 8. Visual verification and rollout

- [x] 8.1 Add Playwright operator journeys for progress drill-down, agent accomplishments, missing evidence, error recovery, source setup, cited review and export; include real API integration and permission/session failure cases.
- [x] 8.2 Capture and inspect every route at 1440px, 768px and 390px with representative datasets; correct visual hierarchy, overflow, density and long-identifier issues instead of accepting HTTP-only checks.
- [x] 8.3 Verify keyboard/focus behavior, semantic labels, contrast, screen-reader status announcements and reduced motion; resolve accessibility issues against the WCAG 2.2 AA target.
- [x] 8.4 Run frontend type/lint/unit/browser checks, contract generation checks, production/container builds, Go vet/race/integration tests, MCP regressions and strict OpenSpec validation; document results and resolve failures.
- [x] 8.5 Deploy the new frontend locally, verify the operator tasks against actual service data, retire superseded operator templ pages after parity, and document build/development/rollback procedures without changing stored evidence or historical exports.
