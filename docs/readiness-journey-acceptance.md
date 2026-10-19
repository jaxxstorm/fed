# Readiness journey acceptance evidence

## Local automated results — 2026-10-18

These tests establish rendering and protocol behavior using synthetic evidence.
They are **not a human usability study**, a full-catalog content review, or live
service evidence acceptance. No production code, authoring content, existing tests
or task checkboxes were changed for this work.

### Requirement journeys

`frontend/tests/readiness-journeys.spec.ts` exercises three distinct journeys on
desktop (1440×1000), tablet (768×1024) and mobile (390×844), using installed Chrome.
All **9 tests passed**. The fixture wording is informed by the existing RPL, INR
and CED drafts, but is deliberately marked draft and is not a published package.

| Journey | Prerequisite / responsible role / first action | Acceptable proof | Supplied proof and explicit gap | Remaining action and review |
| --- | --- | --- | --- | --- |
| Technical: KSI-RPL-TRC | Defined recovery objectives; recovery and business owners; locate desired recovery-time/recovery-point objectives and resource coverage | Dated executed scenarios, scope, observed results and comparisons with objective versions | Development restore report, revision 3, line 1; production execution is not demonstrated | Request authorized production execution records and objective comparisons; human reviews results and untested dependencies |
| Operational: KSI-INR-AAR | Retained incident history; incident response owner; locate incident register and retained investigations with period/boundary | Incident-linked after-action reports plus dated lesson dispositions and resulting practices | Incident 42 report, revision 3, line 1; lessons remain open | Request dated dispositions and resulting-practice records; human separately reviews report coverage and lesson incorporation |
| Documentary: KSI-CED-RAT | Training population map and effectiveness-review process; training owner and role leads; identify all four populations | Population mappings and dated effectiveness reviews with methods, findings and conclusions | General-employee attendance export, revision 3, line 1; effectiveness and other population coverage are missing | Request population map and effectiveness conclusions; human reviews population coverage and repeated review activity |

Each browser test checks visible answers for required work, prerequisites, owner,
first action, supplied evidence, insufficiency, next action and remaining review.
It opens the acceptable-proof disclosure and checks provenance, insufficiency and
exact revision links/line references. Answers do not depend on raw condition codes.
All intercepted API requests remain GET requests: reading the journey performs no
record mutations or remediation. That assertion covers the exercised browser flow,
not every possible infrastructure integration.

### Real transport and database checks

`internal/mcp/readiness_journey_test.go` runs the three domains through an actual
MCP SDK client, authenticated MCP HTTP handler, browser HTTP handler and isolated
PostgreSQL schema. It submits retained artifact content and partial assertion
results, then verifies:

- Browser and MCP guidance/evidence contexts are deeply equal, including exact
  artifact revision, line citation, gap reason, period and owner-directed action.
- Saved catalog/profile/scope/source bindings are unchanged during each journey;
  profile and scope revisions are explicitly 1.
- Partial evidence remains partial with human review pending.
- The MCP artifact reader returns the original retained proof text.
- A consumer decoding only the original instruction fields can still obtain
  service, requirement, binding, steps and assertions despite additive fields.

The three-domain evidence fixture has **no published guidance release**. Its guidance parity assertion
therefore establishes honest missing-guidance parity, not equivalence of published
prerequisites, acceptance criteria or source-comparison reviews. The old-consumer
test is an additive JSON compatibility check, not execution of an older client
binary. The browser fixtures supply draft guidance directly and do not establish
that server publication or projection delivers those drafts.

### Published-guidance parity through the real gate

`TestReadinessJourneyPublishedSyntheticGuidanceParity`, in the same MCP test file,
now closes the previously untested **published-guidance transport parity** path.
It constructs a clearly labeled synthetic catalog with **one requirement and
three inventoried variants**, covering its complete denominator. This is not the
real 1,340-variant catalog. Its statement requires retained monitoring inventory
and dated alert review records; its playbook includes an inferred prerequisite,
owner-directed collection actions, assertion acceptance criteria, insufficiency
examples, and an explicit human-only service-evidence review boundary.

The test:

- Imports synthetic catalog/source bytes with pinned commit identities, excerpt
  and file hashes, and `_sources.json` provenance.
- Confirms `Store.Publish` rejects the unreviewed package.
- Obtains a service-scoped agent identity from an issued read/edit token, records
  each variant's five-dimension comparison through `Store.RecordContentReview`,
  and checks returned reviewer identity, agent type and review time. Comparisons
  expressly describe only synthetic source content, not real FedRAMP review.
- Publishes through `Store.Publish` and verifies all **3/3 synthetic variants**
  are reviewed and coverage is complete. No release rows or review receipts are
  inserted directly and no publication bypass is used.
- Reads instructions through the actual authenticated MCP SDK/HTTP path and
  requirement details through the browser HTTP handler. The reviewed guidance and
  evidence contexts must deeply match. The returned playbook and capabilities
  must also match the original reviewed content, rather than merely agreeing with
  one another. This includes obligations, prerequisites, owner roles, first
  actions, proof acceptance and limits, and human-review criteria.
- Checks exact review-receipt identity/time/comparison/content hash, source
  revisions/citation hashes, release revision/hash, variant, assertion hash,
  attribution label and caveat. Requirement progress and service plan use the
  same release and profile/scope revision **1/1**; browser/MCP plans deeply match.
- Confirms missing proof remains missing and service review remains unassessed.
  The agent's attempted service-evidence review is forbidden; publication creates
  no assessment/review records. Cross-service review, publication and MCP reads
  are rejected.
- Decodes the published instruction response with an explicitly old-field-only
  consumer (including the pre-guidance binding fields) and verifies those fields
  remain available.

The follow-up command below passed with the real PostgreSQL database and race
detector, including the new published test, all three evidence journeys, and the
query-budget/traversal test:

```sh
KSI_TEST_DATABASE_URL='postgres://ksi:ksi-local-development@127.0.0.1:55432/ksi?sslmode=disable' go test -race ./internal/mcp ./internal/assessment -run TestReadinessJourney -count=1 -v
```

This closes the synthetic publication/transport acceptance gap. It does not
publish or review any actual domain draft, demonstrate novice comprehension,
execute a historical client binary, or measure query scaling with a large
published guidance corpus.

`internal/assessment/readiness_journey_test.go` traces actual PostgreSQL calls
through a pgx tracer, including direct database calls outside the record-reader
wrapper. At page sizes **1, 25 and 100**, the queue used **16 queries each** for a
**291-requirement** saved profile. The test enforces an absolute 40-query budget
and equal counts across those page sizes. A full traversal verifies no duplicate
or omitted requirement, stable view hash and saved profile/scope revisions, and
cursor termination. This establishes bounded returned pages and batched behavior
for this fixture; it is not a latency, database row-scan, or catalog-growth bound.

The targeted race run also passed the existing tests for browser/MCP queue and
coverage parity, cursor invalidation after evidence changes, scope/filter cursor
fences, invalid page-size rejection, and presentation cache parity/query bounds.

## Reproduction

From the repository root:

```sh
KSI_TEST_DATABASE_URL='postgres://ksi:ksi-local-development@127.0.0.1:55432/ksi?sslmode=disable' go test -race ./internal/assessment ./internal/mcp -run 'TestReadinessJourney|TestExternalDocumentWorkflowWithoutWorker|TestReadinessQueuePaginationAndScopeFence|TestPresentationBatchParityAndQueryBound' -count=1 -v
```

From `frontend/`:

```sh
PLAYWRIGHT_CHANNEL=chrome bunx playwright test --config readiness-journeys.config.ts
bunx biome check tests/readiness-journeys.spec.ts readiness-journeys.config.ts
```

The separate config inherits the standard browser setup and all three viewport
projects and permits a focused run. The parent integration also registered these
journeys in the default browser config, so the standard browser command includes
them. Repository Biome
configuration excludes browser tests, so its successful check covers the config,
not formatting/linting of the journey test file.

## Task disposition

| Task | Evidence satisfied here | Remaining limitation / disposition |
| --- | --- | --- |
| 7.4 | Same saved-revision evidence context; **reviewed published-guidance parity through the real gate**, including obligations, prerequisites, exact review/source/release bindings and human-only boundaries; old-field decoding before and after publication; bounded exhaustive pagination; actual database query counts | **Published-parity gap closed for the complete synthetic catalog.** Automated fixture coverage now addresses each requested behavior. Large published-corpus batching and historical client-binary compatibility remain outside these tests; the measured 291-requirement queue uses missing guidance. No real-catalog publication or live acceptance is implied. |
| 7.5 | Automated multi-domain browser prompts and real MCP evidence/gap journeys, plus actual retrieval of published synthetic prerequisites/owner/actions/acceptance criteria | **Partial.** The published synthetic rule does not substitute for completed authored domain guidance. Representative novices have not completed these journeys. |
| 8.7 | Automated visibility checks for required work, acceptable proof, supplied proof, insufficiency and next action without raw codes | **Not completed.** No real novice participants, observed answers, comprehension scores or authenticated completed-domain acceptance are recorded. |

No production failure was exposed by these exercised paths. Only an isolated test
release was published. No production release, commit, deployment, live acceptance
or real full-catalog coverage claim follows from these
results. The rollout gates in `docs/readiness-acceptance.md` still apply.
