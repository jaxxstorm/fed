# Readiness acceptance record

## Local verified behavior — 2026-10-18

These are automated fixture results, not live acceptance or a claim that a novice
has evaluated the complete catalog.

| Journey | Verified result | Test |
| --- | --- | --- |
| Read repeated network-policy findings | One summary for equivalent targets; exact affected count; raw codes and per-target records remain expandable | `frontend/tests/guidance.spec.ts`, `internal/observation/finding_explanations_test.go` |
| Distinguish absent policy from failed read | Complete negative observation describes no selecting policy; access failures describe unknown control state | `internal/observation/checks_test.go`, `finding_explanations_test.go` |
| Read a passing configuration check | Passing check is visible with observation time and freshness; enforcement and whole-requirement support remain separate | `checks_test.go`, `frontend/tests/guidance.spec.ts` |
| Gather evidence before remediation | Actions ask owners for retained settings, records and tests; views perform no infrastructure remediation | `finding_explanations.go`, `frontend/tests/guidance.spec.ts` |
| Assess development records with missing production proof | Exact citations coexist with an explicit production scope/period/access gap; supported and evidence-complete submissions are rejected | `internal/assessment/assertion_gaps_test.go` |
| Record an event that has not occurred | Trigger remains not-triggered, evidence missing and review unassessed; no completed-duty claim | `internal/readiness/work_test.go`, `frontend/tests/readiness.spec.ts` |
| Move from evidence work to human review | Separate requirement groups and count units; human-only review boundary visible; filter resets pagination | `frontend/tests/readiness.spec.ts` |
| Read the same work in browser and MCP | Shared evidence matrix, requirement queue and guidance-quality values match; scope changes invalidate queue cursors | `internal/mcp/agent_evidence_test.go`, `internal/assessment/readiness_queue_test.go` |

The readiness browser suite passed 12 tests across desktop, tablet and mobile.
The guidance suite passed nine tests across the same sizes. PostgreSQL-backed
assessment, readiness, observation, web and MCP race checks passed for the
corresponding changes. These results cover synthetic records, not Ignis evidence
sufficiency.

Published-context parity was subsequently verified with a complete three-variant
synthetic catalog, authenticated review receipts and the real publication gate.
`internal/readiness/context_progress_test.go` verifies one work read and one
candidate-evidence read for both single and expanded card sets. The queue journey
also verifies 16 database queries at page limits 1, 25 and 100 over 291 resolved
requirements. Old-field consumer decoding verifies additive JSON compatibility;
this is not a claim to have executed every historical client binary. See
`readiness-journey-acceptance.md` for the exact limits of these acceptance fixtures.

## Required rollout gate

### Read-only live baseline — 2026-10-19

Renewed AWS SSO was verified as shared account `967721292901`. All AWS calls
were restricted to `us-east-1`. ECS remains on `fedramp-web-shared:10`, with one
running task and a completed deployment; no readiness deployment has occurred.

A read-only TLS-verified database snapshot retained row hashes for service
`ff3eb7dd-b556-49d1-8d5e-5c3fde044135`: **9,254 record revisions** and **3,160
observations**. This includes scope revision **6**, profile revision **1** and
**9,193 evidence records**; there were no service assessment, human review or
export records to compare. The private local baseline contains only identities
and row hashes, not artifact contents or credentials. After rollout, verify every
baseline identity/hash still exists unchanged; new collected records may append.

Before commit/push/publication/deployment acceptance is recorded, retain:

- Complete substantive coverage and authenticated source-comparison reviews for
  the unchanged 1,340-variant denominator. All variants now have authored drafts
  and source-comparison inputs. The full-catalog test recorded authenticated
  test-service reviews, published 1,338 playbooks and projected 291 requirements;
  production recording remains pending.
- Novice and agent comprehension acceptance across the completed technical,
  operational and documentary domain content, including exact proof and gaps.
- Final contract, lint, unit/race, browser and container-build results for the
  actual release commit.
- Successful image publication and the approved Pulumi deployment reference.
- Live browser/MCP verification and before/after immutable evidence, assessment,
  review and saved-boundary baselines, after old workers finish draining.

No live rollout or full-catalog comprehension acceptance is recorded here.

### Final local checks — 2026-10-19

The full browser suite passed **96 tests**, with **3 pre-existing conditional
skips**, across desktop, tablet and mobile. Contract freshness, frontend lint,
seven unit tests, `go vet ./...`, strict OpenSpec validation and
`git diff --check` passed. The final Docker build passed with pinned catalog and
certificate hashes verified. The full PostgreSQL-backed `go test -race ./...
-timeout=15m` suite passed; guidance-content tests completed in 612.627 seconds.
CI allows 30 minutes per test package to accommodate slower hosted runners.

The corrected browser journeys use an unchanged, hash-bound subset of the actual
authored RPL/INR/CED content, preserving two INR assertions and five CED assertions
and their original trigger/authority fields. All nine focused browser cases pass.
The full-catalog integration test also verifies authenticated browser/MCP parity
for these real domains and retains synthetic partial evidence/gaps without human
approval. Its extended non-race run passed in 42.6 seconds, but the targeted race
rerun failed on an authenticated CED requirement-page read returning HTTP 500.
That regression blocks deployment pending diagnosis and correction. The earlier
full race-suite pass predates this expanded transport case. Human novice
comprehension remains unverified and guidance
activation remains disabled pending acceptance.

### Release publication

Implementation commit `68fe797c8bbc62b516cd13a8828610b2cf869087` was pushed to
`main`. Container publication succeeded in
https://github.com/jaxxstorm/fed/actions/runs/39265010190 . Application CI and
Pulumi rollout verification are tracked separately; publication alone does not
establish a completed deployment or readiness acceptance.

Pulumi preview
https://app.pulumi.com/ignistech/fedramp/shared/previews/35a37930-407a-4e47-b9bf-5481c3c2028d
selected the published image and explicitly retained `KSI_GUIDANCE_ENABLED=false`.
No apply occurred. The temporary Ignis read/edit agent token was revoked and its
local file removed while the regression is investigated; no production guidance
review records or release were written.

Application CI run `39265010184` failed: the full-catalog integration exceeded
its context deadline, and a synthetic MCP parity test compared host-dependent
timestamp locations. The latter reproduced locally with `TZ=UTC`; normalizing
the expected receipt timestamp to UTC fixed that test, confirmed with a targeted
race run. No timestamp instant or persisted review content was changed. The
full-catalog performance/read failure was traced to repeated JSON/source work,
duplicate browser projections and duplicated inventory builds consuming the
shared four-minute context budget. Request-local source verification, direct
citation-reference traversal, shared snapshot projections and one bounded
two-identity inventory cache address this without increasing request deadlines.
The expanded full-catalog race test passed with `TZ=UTC` in 175.83 seconds;
catalog/readiness/assessment/web/MCP race suites also passed. A full UTC race-suite
rerun and updated container build are in progress before the corrected rollout.

The corrected `17291e388d30aa11210e7bc859cfe22007a0a486` full UTC PostgreSQL-backed
race suite passed, including guidance-content tests in 574.674 seconds. The
updated Docker build and container publication run `39269221486` also passed.
At the user's explicit deployment instruction, Pulumi apply was started with
guidance disabled while hosted CI was still running. Live verification remains
pending until apply completes and old workers have drained.

Pulumi update https://app.pulumi.com/ignistech/fedramp/shared/updates/15 succeeded:
web task revision **11**, migration revision **9**, commit `17291e3`, image
`sha256:50fcab70a29e1c94fa63dbe7370eac1a996283256a902b4a4a9155f10df767fe`.
The deployment hook verified private HTTPS readiness and the running digest.
Guidance remains disabled. ECS reported the new task running and the previous
deployment draining with zero running tasks; final stable/baseline verification
is pending. Hosted CI passed all backend checks but failed six readiness browser
cases (two scenarios across three viewports) in the Docker browser environment;
that separate UI issue is being investigated before guidance activation.

ECS subsequently reached service stability. The live task definition confirms
`KSI_GUIDANCE_ENABLED=false` and the expected image digest. Post-rollout read-only
verification matched **all 9,254 retained record revisions and 3,160 observations**
against their pre-rollout row hashes, with no changes or missing baseline rows.
Authenticated source-comparison recording has started under the approved
short-lived Ignis-only read/edit agent grant; no human evidence approval is implied.

The Docker browser failures reproduced as `crypto.randomUUID is not a function`
on the insecure HTTP development origin. Declaration keys now use 128 bits from
`crypto.getRandomValues`, retaining the existing idempotency-key rotation points.
All six targeted cases and the full Docker browser suite passed (**96 passed,
3 existing skips**), as did the production frontend build/typecheck and lint.
