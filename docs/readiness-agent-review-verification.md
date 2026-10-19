# Agent-review implementation verification

Verified locally on 2026-10-18. This record covers the agent-review workflow and
attribution, not completion of the full readiness-plan change or review of the
catalog's policy interpretations.

## Implemented

- Service-scoped agents with edit access can record source comparisons and
  publish guidance without human content approval.
- Comparisons require findings for applicability, obligations, prerequisites,
  evidence and timing, bound citations, limitations and explicit uncertainties.
- Reviewer identity/type come from authentication. JSON review claims cannot
  authorize publication. Publication requires every catalog variant to have a
  current recorded review.
- New review/release tables are append-only. Repeated identical submissions are
  idempotent; changed content or comparison conflicts preserve history.
- Browser, MCP instructions and exported snapshots carry the same guidance
  context, source revisions, limitations, uncertainties and release hash.
- The visible agent-review caveat does not request human certification of policy
  interpretations. Missing guidance never receives a reviewed label.
- `KSI_GUIDANCE_ENABLED` defaults to false. Human service-assessment review is
  unchanged.

## Checks passed

- PostgreSQL-backed `go test -race ./...`; affected readiness/MCP tests rerun
  after subsequent authorization/error and transport-parity changes.
- `go vet ./...`.
- Frontend contract check, lint, unit tests and production build.
- Six Playwright guidance tests across desktop, tablet and mobile, using
  `PLAYWRIGHT_CHANNEL=chrome bun run test:browser tests/guidance.spec.ts`.
- Strict OpenSpec validation and `git diff --check`.

The initial container attempt timed out on the OrbStack socket. A subsequent
`docker build -t fed:readiness-check .` succeeded, including the pinned Rules and
Markdown corpus. No image publication or deployment occurred.

The tests cover forged review metadata, cross-service isolation, revoked tokens,
immutable records, stale review hashes, missing comparison dimensions, unbound
citations, browser/MCP gap parity, read/export attribution parity, and visible
caveats/uncertainties without human content-approval controls.

## Still pending

Full-catalog substantive authoring and source-comparison review, complete
per-obligation environment/time/access sufficiency, operating-duty tracking,
broader end-to-end acceptance, and production rollout remain pending. No
production guidance review or publication was recorded by these synthetic tests.

## Subsequent readiness and evidence verification

- Immutable scoped work records now persist declarations, ownership, decisions,
  alternatives and candidate evidence references. Writes enforce current
  profile/scope/release bindings, eligible assignees and expected revisions.
- The dependency projection uses persisted decisions and separates service,
  account, region, cluster, namespace and repository identities. Browser and MCP
  expose the same plan and mutation service.
- Browser conflict handling preserves local edits until the operator compares
  the current declaration. Its journey passes on desktop, tablet and mobile.
- Guidance assertion changes invalidate current assessment reuse while keeping
  historical reviews and exports immutable.
- Evidence coverage returns candidate artifacts, assertion citations, assessment
  rationale and retained contradictory observations. PostgreSQL tests verify
  browser/MCP coverage parity and exact observation drill-down.
- Finding groups retain distinct environment/cause/access boundaries and exact
  affected-target counts. See `readiness-finding-explanations.md` for the collector
  inventory, interpretation boundaries and remaining legacy-check gaps.
- Requirement details render prerequisites, setup, operating, evidence and
  review steps, with authority labels and explicit event-trigger limitations.
- The expanded guidance suite passes all nine desktop/tablet/mobile tests.
- The full PostgreSQL-backed race suite passed after these changes using the
  isolated local database on port 55432.

Current checklist: 22 of 50 tasks complete. These implementation checks do not
substitute for full-catalog authored content or source-comparison reviews.

## Coverage and capability reads

The feature-gated browser/MCP APIs now include bounded published-guidance
coverage and exact scoped-capability reads. Coverage names the full catalog
variant denominator and does not count draft authoring files as published content.
Catalog inventories are cached by immutable payload identity and schema hash
(at most two retained inventories); source freshness, reviews, authorization and
service state are checked independently on each request.

PostgreSQL race tests cover missing releases, changed/unavailable source corpora,
cross-service denial, exact capability bindings, transport parity and extreme
pagination offsets. The expanded readiness browser suite passes six tests across
desktop, tablet and mobile, including coverage pagination and declaration conflicts.
