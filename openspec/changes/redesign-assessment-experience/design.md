## Context

The current Echo/templ browser UI uses a shared page structure with navigation links, generic sections and JSON form payloads. Recent fixes moved artifact links out of navigation and paginated evidence, but the interface still lacks a coherent operator model. Sources, requirements, findings, agent receipts, collection outcomes and human reviews exist in the backend; users must mentally assemble them to understand progress.

The existing Go application owns OIDC/development sessions, CSRF, scoped permissions, PostgreSQL records, artifacts, collector jobs and MCP. The external-agent workflow is implemented and unarchived. Automatic-collection verification remains incomplete. This redesign should expose the actual available domain state, not invent missing collector or agent telemetry.

The user supplied Oneleet screenshots as references for immediate pass/fail comprehension, progress and grouping. The implementation must have original composition, styling and assets. Plain-language actionability and every-page consistency matter more than reproducing any particular chart.

## Goals / Non-Goals

**Goals:**
- A first-time operator can identify overall progress, the most important blocker, what an agent accomplished, and the next action without opening diagnostics.
- Every route explains its relevant completed work, missing work, evidence improvements and contribution to service progress.
- A cohesive, responsive TypeScript UI replaces generic JSON screens across all existing operator functions.
- Errors and partial successes remain visible and recoverable; source settings, review and collection actions are usable without editing JSON.
- Preserve existing authorization, records, evidence links, external-agent MCP behavior and immutable exports.

**Non-Goals:**
- Embedded agents, model orchestration, new compliance frameworks, new trust-centre dependencies or infrastructure remediation.
- Copying Oneleet/Vanta layout, branding, logos, text or proprietary assets.
- Rewriting the assessment domain in TypeScript, or making Bun a production server dependency.

## Decisions

### 1. React/TypeScript frontend, Go application backend

Create a `frontend/` workspace using React, TypeScript strict mode, Vite and Bun for package management/scripts. Use React Router for navigation, TanStack Query for remote state and invalidation, Tailwind CSS with application-owned tokens, accessible Radix primitives where interactive behavior warrants them, and a consistent open-source icon set. Pin dependencies and commit the Bun lockfile. Use Vitest for presentation logic/components and Playwright for browser, accessibility and screenshot verification.

Go remains the only production service. Build frontend assets into an embedded/static bundle served by Go; provide a Go-embed placeholder so backend-only tests work before assets are built, but release builds must fail if the real frontend bundle is absent. A Bun/Vite development server proxies browser API/auth requests to the configured Go backend. Production browser calls are same-origin. The Docker build gains a frontend builder stage and retains its minimal Go runtime image.

Alternative: keep adding CSS to templ. Rejected because generic page records and server-rendered JSON forms constrain the required interactive filtering, previews, inline validation and cohesive navigation. A separate production JavaScript backend would duplicate auth and domain logic unnecessarily.

### 2. Stable workspace navigation and complete route coverage

Use a persistent, compact sidebar with service switcher and grouped navigation: Overview, Requirements, Evidence, Activity, Sources, and Settings. Reviews are a saved Requirements filter plus a review action on details. Collection runs appear in Activity with a clear automatic-collection tab. Exports/history appear in a dedicated page linked from Overview and Settings. The top bar provides breadcrumbs, current profile and a compact service progress link. Mobile uses a drawer without losing page context.

Each page has a short title, scope/context, a relevant progress or health summary, a prioritized work area, and a primary action. Keep counts actionable and route state in query parameters so filtered URLs can be shared. Artifact IDs and run IDs belong in copyable details, not headings or navigation. Preserve existing bookmarked requirement, artifact, run and export URLs via matching routes or explicit redirects. Do not let SPA fallback intercept `/mcp`, authentication, APIs, downloads or health endpoints.

| Page | Main question | Main content/action |
| --- | --- | --- |
| Service selector/onboarding | Where do I start? | Service cards, setup checklist, choose profile |
| Overview | How close are we, and what needs attention? | Progress, status breakdown, priority actions, recent agent accomplishments |
| Requirements | What is passing/failing/missing? | Searchable category groups/table, status filters, evidence/review counts |
| Requirement detail | What is needed to finish this? | Status reason, remaining assertions, findings, evidence, review action |
| Evidence | What was gathered and what needs improvement? | Search/filter library, freshness/source badges, previews, linked requirements |
| Activity | What happened and what succeeded? | Agent submissions and collector runs with outcomes, timelines and retryable failures |
| Sources | Can evidence be gathered from the right places? | Source cards/table, configured access hints, last observed access result, edit/retry guidance |
| Settings | Is this service correctly configured? | Field-based profile, scope, policy, source, schedule and token controls |
| Exports/history | What is ready to share, and what changed? | Snapshot summary, timestamp, download actions and revision history |

Alternative: a dashboard-only facelift. Rejected because operators would immediately fall back into raw forms and ambiguous detail pages.

### 3. Original visual language with progressive disclosure

Default to a light neutral canvas, white surfaces, strong dark text, restrained indigo/teal accents and semantic status colors. Use consistent spacing/radius/type tokens, readable content widths, stable table density, clear section hierarchy and ample breathing room. Charts supplement labeled counts; they do not carry information alone. Use text and icons alongside green/red/amber/blue/gray states. Add dark appearance only if it receives the same verification; it is not a prerequisite for this change.

Build shared components for app shell, page heading, progress breakdown, metric card, status badge, attention item, filter toolbar, paginated table, evidence preview, source form, activity timeline, empty state, error state and confirmation feedback. A visual fixture gallery defines loading, empty, partial, failed and successful examples before full route integration.

Keep raw payloads in optional technical-detail drawers with copy actions. Render JSON evidence itself as labeled fields and selectable paths, with raw JSON available for citation/debugging. Text/Markdown evidence uses a safe reader with line references. Long identifiers wrap or truncate with accessible expansion/copy, never stretch navigation.

### 4. One authoritative progress and status projection

Add batched presentation projections to shared Go assessment services and typed browser DTOs. Do not calculate competing readiness rules in React or infer whole-rule compliance from one passing collector. Return `as_of`, current bindings, status reason, counts, denominator definition, contributing requirement IDs/filter links and next actions. Existing MCP status remains compatible; shared semantics must agree where fields overlap.

Default service progress is **Passing required requirements / applicable required requirements**. Recommended and optional rules appear separately, with an explicit alternate filter rather than silently changing the denominator. Not-applicable rules are excluded. Unresolved applicability is shown as a separate “Scope decisions needed” count and prevents an “All done” service state. An empty denominator shows “Choose a profile” or “No required requirements” as appropriate, never 100%. Category and global numbers must reconcile at the same `as_of` time.

Also show **Evidence gathered** (required rules with current evidence-work completion) and **Reviews remaining**. Each number names its unit; a resource finding count is never mixed with a requirement count. Evidence collection can finish while a rule is failing. “Passing” requires current complete supported evidence, no outstanding relevant contradictory finding, and current human review. An explicit new result cannot turn failures green just by supplying citations.

Use a mutually exclusive primary requirement status with ordered precedence:

1. Not applicable (outside the applicable set).
2. Needs scoping (unresolved applicability).
3. Failing (current mapped failure or contradictory agent assessment remains unresolved).
4. Blocked (an explicit access/dependency blocker prevents remaining work).
5. Needs refresh (support has expired or relevant bindings changed).
6. Needs evidence (an investigation has started and identified unsupported required assertions; untouched rules remain Not started).
7. In review (current supported evidence work complete, current review outstanding).
8. Passing (current evidence supported and current human review complete).
9. In progress (evidence exists or a supported collection operation is active, but no complete assessment exists).
10. Not started (no evidence or activity).

Secondary badges retain simultaneous facts such as “Failing · access blocked” or “Evidence gathered · review pending.” Supporting explanations link to the responsible evidence, failed targets or missing assertions. Documentary requirements do not require nonexistent collector checks. Stale findings remain visible with their age; do not quietly convert them into a fresh pass. Freeze the exact truth table in tests before building aggregate charts.

Agent accomplishments come from persisted rule receipts, artifact submissions and audit events, grouped by rule/result and timestamp with meaningful summaries. Collector runs are explicitly labeled. Do not display “agent running,” percentage of an agent session, or an ETA without real telemetry. Show last update and refresh activity after writes or bounded polling of active collector jobs.

### 5. Browser APIs reuse authorization and domain mutations

Add versioned same-origin browser APIs under `/api/ui/v1/`, returning purpose-built service overview, requirement list/detail, evidence list/detail, activity, sources, settings and export models. Describe the browser contract in OpenAPI and generate TypeScript types/client bindings; verify contract generation in CI. Use bounded server-side pagination/filtering and batched database reads for progress, rather than invoking `Status` once per row over HTTP or shipping every artifact to the browser.

Reuse authenticated browser sessions. A bootstrap response provides identity, allowed actions, service context and CSRF token; mutations carry the CSRF header and expected revision/idempotency fields where applicable. API session expiry returns a structured 401 with a login destination, not HTML masquerading as JSON. Controls reflect capabilities and server checks remain authoritative. Agent tokens are issued only by explicit operator actions and never stored in browser local storage. Retain existing MCP authentication and tool schemas.

DTOs separate labels/presentation reasons from canonical domain states. Browser actions call the same source/profile/scope/draft/review/collection/export services as existing transports. Return canonical updated data or invalidate affected queries after success. Preserve user input on conflicts and offer reload/compare instead of silently overwriting. Operational errors need structured target/code fields from collectors where currently only strings exist; extend those payloads additively with a fallback for old records.

### 6. Errors become repairable work

Define structured error/attention DTOs: stable code, title, short explanation, affected targets, operation/source identity, retryability, field errors, suggested actions and diagnostic reference. The backend maps known codes and supplied metadata; the frontend renders them consistently. Never infer a credential problem solely from every generic AWS failure.

Examples:
- “AWS sign-in expired” → name the profile/account and whether the failure came from server collection or an external agent; explain where to sign in; offer the relevant retry action.
- “Cluster access denied” → show cluster and failed read operation, the required read access if known, and source settings/retry links.
- “Evidence needs updating” → show which artifact is old, when it was observed, and a copyable scoped agent instruction to gather it again.
- “Changes were saved elsewhere” → preserve entered values and offer reload/compare for the current revision.
- “Some sources could not be checked” → retain successful results and list each failed source with its next action.
- Unknown service error → concise failure description, retry only if safe, and a copyable diagnostic reference. No raw SQL, stack traces or guessed fixes.

Recovery controls must perform a real authorized action or provide a specific usable instruction. Do not add decorative “Fix” buttons that silently edit infrastructure. Distinguish editing a source from fixing access in the external agent's environment. Retry collection with deduplicated/idempotent requests; show progress and final feedback. Acknowledging an error does not resolve it.

Copy rules: lead with outcome, state the specific missing evidence and next step, and reserve detailed rationale for expansion. Replace repeated “not evidence of…” boilerplate with direct requests such as “Add dated drift-review results.” Preserve relevant limitations as labeled metadata without letting caveats dominate the screen.

### 7. Evidence and review are readable workflows

The evidence library supports search, source/type/freshness/requirement filters, pagination, title, observation date, provenance and linked-rule count. Detail previews show source context, safe content, line/JSON-pointer citations, freshness and limitations. Empty/missing artifacts get a clear recovery state. Do not fetch full artifact bodies for list rows.

Requirement detail opens with status/reason and “Next steps,” followed by a short assessment summary and assertion checklist. Tabs or anchored sections expose evidence, findings and history. Show missing assertion coverage and improvements separately from failed infrastructure findings. Reviewers see the exact draft/evidence revisions they are reviewing, can inspect citations and submit review with existing domain semantics. Existing review means reviewed, not automatically approved; passing still requires supported current evidence. Keep agent results and human actions visibly attributed.

Configuration forms have named fields, add/remove rows, help text, inline errors, pending states and explicit saved feedback. Profile changes keep their existing preview of affected requirements. Account/region, repository commit/path, source freshness and namespace/policy configuration receive typed controls. No ordinary workflow requires hand-editing JSON. Advanced diagnostics can remain available.

### 8. Acceptance is visual and task-based

Implement a fixture gallery and Playwright journeys at desktop (1440px), tablet (768px) and mobile (390px). Cover long names, hundreds of artifacts, many findings, incomplete setup, empty results, partial collection, forbidden/expired sessions and concurrent edits. Verify keyboard navigation, focus management, labels, contrast, reduced motion and screen-reader announcements for status changes, targeting WCAG 2.2 AA.

Acceptance tasks: identify the overall passing count and denominator; find the highest-priority blocker; locate what an agent last accomplished; identify a missing evidence item; follow an error to its correction screen; review cited evidence; configure a source without JSON. Each task must be possible from the relevant page without opening technical diagnostics. Capture screenshots for every route and inspect them, not just HTTP response codes. Evidence lists render at most 25 records per page by default; use server-side pagination and show responsive loading rather than blocking on artifact content. Assert bounded query counts on aggregate endpoints using a representative multi-hundred-rule fixture.

## Risks / Trade-offs

- [A beautiful dashboard implies more certainty than the stored data] → Shared tested status precedence, labeled denominators, unresolved-scope counts, explicit stale/unknown states and separate evidence/review stages.
- [Frontend migration breaks authentication or existing links] → Same-origin sessions/CSRF, API-specific auth responses, route inventory and contract/browser regression tests before cutover.
- [Visual polish hides operator actions behind diagnostics] → Test the concrete operator tasks without opening technical detail drawers.
- [Large datasets create slow pages or giant responses] → Batched summaries, server-side filters/pagination, lazy artifact loading and bounded query tests.
- [Legacy errors lack context for a precise remedy] → Show known context and a useful diagnostic reference; enrich future results rather than guessing historical causes.
- [Two UIs diverge during migration] → Short-lived explicit preview path, route parity checklist, then one default frontend. Keep domain services shared throughout.
- [User input is lost on failed settings mutations] → Preserve form state, field-level errors and revision-aware reload/compare.

## Migration Plan

1. Inventory current routes, actions, domain states and error payloads. Create the status truth table, representative fixture set and visual direction.
2. Add the frontend workspace, shared design system and browser API contracts without changing MCP or records.
3. Implement authenticated batched read models and structured action/error adapters. Build and review all routes behind an explicit preview path or development flag.
4. Complete visual, accessibility and task-based browser checks alongside existing Go/MCP tests. Verify source edits, reviews, tokens, collection controls and downloads against the local service.
5. Update Makefile/Docker/release scripts; switch existing browser URLs to the new frontend once parity is verified. Preserve old deep links and protect API/download routing.
6. Retire superseded templ operator pages only after parity; retain any unrelated template use such as immutable export rendering. Document rollback to a previous compatible binary without database reversal or evidence loss.

## Open Questions

- Source error payloads vary across existing collectors; implementation should inventory supported recovery codes and explicitly identify legacy fallbacks.
- User-supplied screenshots establish the desired clarity. Use the original light visual system described here as the first implementation, then assess actual screenshots before finalizing spacing/density.
- No blocker requires selecting a model provider or external compliance vendor; this is an operator experience change over the existing service.
