# Implementation verification

The redesign is implemented and deployed locally at http://127.0.0.1:8080.
The Go process retains the existing development identity, database, artifact
directory, AWS profile bindings and Kubernetes contexts. No schema migration or
evidence/export rewrite was required.

## Delivered

- React/strict TypeScript, Bun/Vite, Router, TanStack Query, Radix dialogs,
  application-owned design tokens and Lucide icons; embedded Go production assets.
- Shared session/CSRF-protected browser adapters, generated OpenAPI contracts,
  capability-aware controls and structured recovery responses.
- Repeatable-read progress projection with eight batched input queries and no
  additional per-rule status queries. Cached/uncached parity covers documentary
  support, current review, stale evidence and technical findings.
- Searchable requirements, evidence and source pages; exact-revision lazy
  previews; highlighted line/JSON citations; labeled draft evidence selection;
  review notes retained across conflicts.
- Profile/scope/source/policy forms, agent prompts, token issuance/revocation,
  collection run summaries, immutable snapshot downloads and history.
- Superseded operator templ rendering retired; existing deep links, downloads,
  MCP, auth and health routing retained. Build/development/rollback procedures:
  `docs/operator-workspace.md`.

## Verification results

- `bun run build`: TypeScript and Vite production build passed.
- `bun run lint`, `bun run format`: passed without outstanding warnings.
- `bun run contracts:check`: generated schema/types match Go response contracts.
- `bun run test`: 2 API error-handling tests passed.
- `bun run test:docker`: 27 browser tests passed across 1440px, 768px and 390px.
  Covers fixtures, 284-record pagination, onboarding, source validation,
  concurrent source edits, session expiry, stale successful results after failed
  refresh, exact cited review with concurrent draft changes, permission failure,
  immutable export navigation and keyboard focus restoration.
- `bunx playwright test --config playwright.live.config.ts`: 6 tests passed
  against the deployed service, covering all workspace routes, direct requirement
  and run links, evidence artifact retrieval and count/drill-down reconciliation.
- Axe WCAG 2 A/AA, 2.1 AA and 2.2 AA checks pass on the gallery, every live
  navigation page, requirement detail and evidence dialog. Keyboard/escape/focus
  tests pass; reduced-motion CSS disables animation; pending/saved/error states
  use semantic output/alert elements.
- `KSI_TEST_DATABASE_URL=… go test -race ./...`: passed, including isolated-schema
  PostgreSQL integration and MCP-to-browser source/draft/review/export tests.
- `go vet ./...`: passed.
- `make build`: passed with real embedded frontend assets.
- `docker build -t fed:workspace .`: passed with the Bun builder and Go-only
  distroless runtime.
- `openspec validate redesign-assessment-experience --strict`: passed.

## Visual review

Initial gallery overview screenshots were inspected before live route integration.
The extended form/preview/recovery gallery and all live routes were subsequently
captured and inspected at the three required widths. Screenshots are generated in
ignored `frontend/test-results/` by the browser suites.

Reviewed service selection, overview, requirements, evidence, activity, sources,
settings, exports, requirement details, run details and evidence previews, plus
fixture onboarding, cited review and frozen snapshots. Corrections included:

- collapsed technical collection plans and audit payloads;
- concise agent investigation summaries with full explanations on demand;
- labeled source names, paginated sources and searchable evidence;
- wrapped identifiers, mobile drawer navigation and larger disclosure targets;
- a separate gathered-record count alongside requirement completion;
- explicit empty export/onboarding states;
- collector completion distinguished from collection success, with repeated
  failures grouped by cause and affected targets.

Live data remains honest: the service currently has 785 evidence records but no
passing required requirements; recorded access gaps and missing operational
evidence remain visible. The browser verification performs read-only inspection
of that service. Mutation/regression verification uses fixtures and isolated test
schemas rather than changing its assessments or historical exports.

The completed change remains unarchived and uncommitted. The separate automatic
collection change retains its own outstanding checklist.
