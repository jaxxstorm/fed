# Readiness comprehension review — agent exercise

Date: 2026-10-19. Reviewer: independent AI agent, not a novice participant.

## Follow-up: authored acceptance fixture corrections

The review below records the **original** simplified fixtures. Its exact fixture
answer tables and fixture-specific findings are historical; the follow-up here
supersedes claims that the current browser tests use rewritten guidance, collapse
assertions or label every duty One Time.

`frontend/tests/readiness-journeys.spec.ts` now loads
`frontend/tests/fixtures/readiness-authored-subset.json`, extracted without text
rewrites from the assembled package. The subset includes the three actionable
playbooks, referenced capabilities and referenced source citations, retaining
source identities, authority, timing, alternatives and human-review criteria.
`readiness-authored-manifest.json` binds both input and generated output:

- Assembled SHA-256: `c335557fc88800d076f6d01320fd9953dc0af4e15e53796fe04d114ca7aeaa42`
- Subset SHA-256: `aba91ecfaff9d13c40fdf4e431bf9a7ea5cc403e60038165f9fa6eb3f3bf50f4`

The test checks the subset file hash before running. Independently compared every
selected playbook, capability and citation object with the original assembled
JSON: all are exactly equal. Reproduction from the repository root:

```sh
python3 frontend/tests/fixtures/generate-readiness-subset.py /path/to/readiness-assembled-draft.json
```

This is a draft test subset, not a complete publishable catalog or an
authenticated source-comparison receipt. Hashes establish identity, not approval.

New browser assertions verify:

- RPL has one evidence assertion, INR two, and CED five, each with its own visible
  obligation, synthetic evidence rationale, distinct gap, owner-directed next
  action and revision-3/line-1 reference.
- Actual prerequisite actions and owners display unchanged. CED's initial owner
  is Training owner, with role leads on subsequent setup steps.
- INR's operating triggers are event and recurring; RPL and CED are recurring.
  Each duty displays its actual authority and trigger, and recurring/event duty
  items cannot contain One Time. INR's event prerequisite explanation is visible.
- Each assertion's own acceptable fact, provenance, insufficiency examples and
  evidence-route proof/limitations display inside its own disclosure.
- Actual authored review actions and human-review completion criteria display.
- Guidance remains visibly Draft guidance and never Agent-reviewed guidance when
  the supplied test context has no authenticated receipt. Official-obligation
  wording remains official in the authored content; draft review status is a
  separate attribution dimension.
- Evidence, artifacts and service context are explicitly synthetic. Reading the
  journey makes no non-GET intercepted API requests.

Focused command, from `frontend/`:

```sh
PLAYWRIGHT_CHANNEL=chrome bunx playwright test --config readiness-journeys.config.ts
```

Result after corrections: **9 passed** (three domains × three viewports).
The first attempted run required correcting JSON import attributes; the completed
run passed all tests. No production code, guidance or source edits were needed.

**Remaining gaps:** authenticated real-domain publication/projection and service
variant resolution are not exercised by this intercepted-browser fixture. It
does not test real review receipts, same saved-profile/scope bindings through UI
and MCP, server-side rejection of fabricated attribution, or acceptance of actual
service evidence. Those require the real publication/authenticated transport
path, not an assigned `state: draft` response. Excluded/needs-decision variants
are outside these selected actionable journeys. Real novice participants still
must supply unprompted answers, navigation observations and scored comprehension
across all three domains, including vocabulary and the human-review boundary.
Tasks 7.5 and 8.7 remain incomplete; no checkbox was changed.

## Result and method

The agent can extract prerequisite, owner, first action, acceptable proof and
remaining review from the assembled authored guidance for all three sampled
requirements. The current browser journeys also display explicit evidence gaps
and next actions. **This is partial evidence for task 7.5, not completion of
novice acceptance in 7.5 or 8.7.** No human comprehension scores are claimed.

Inputs:

- `/private/var/folders/_9/zz5h76fj6sg42r_w3n0408mw0000gn/T/opencode/readiness-assembled-draft.json`
  (revision 1; rules commit `a89d78477ddea98dc57eca24f5007794c2b4eeb6`;
  site commit `8404168e6c150b40e4d0e45152d18965ab27e001`).
- `frontend/tests/readiness-journeys.spec.ts`.
- `openspec/changes/add-requirement-readiness-plans/tasks.md`, tasks 7.5 and 8.7.
- `docs/readiness-journey-acceptance.md`.

I read the actual actionable playbooks and their referenced capability setup,
then compared their answers with the exact strings checked in the browser.
The assembled package supplies authored content, not service evidence. The
browser intercepts API responses with synthetic draft guidance; it does not load
this assembled package. Accordingly, authored-content quotations below are
source-field extraction, **not a claim that those exact authored fields were
observed in an authenticated browser session**. “What we have” and the specific
gap/next-action answers come exclusively from the synthetic evidence fixture.

Re-ran from `frontend/`:

```sh
PLAYWRIGHT_CHANNEL=chrome bunx playwright test --config readiness-journeys.config.ts
```

Result: **9 passed**, across desktop, tablet and mobile. These tests check visible
text, the acceptable-proof disclosure, revision-3/line-1 evidence references,
remaining review, and no non-GET intercepted API calls. They establish visibility
of scripted answers, not an unprompted participant's understanding. No new tests
were necessary to establish this distinction.

## Exact answers from displayed fixture fields

Every quotation in this section is the literal fixture text verified by the
browser test (lines 7–40, assertions at lines 81–100).

### Technical — KSI-RPL-TRC

| Prompt | Answer |
| --- | --- |
| What is required? | “Repeatedly test recovery capability against the applicable defined objectives.” |
| Prerequisite | “Defined service recovery objectives” |
| Owner | “Recovery owner and business owner” |
| First action | “Locate the desired recovery-time and recovery-point objectives and the resources they cover.” |
| Acceptable proof | “Executed test scenarios, resource scope, dates, actual results and comparisons with objective versions.” |
| What we have | “Development restore execution report”; “Development restore completed; production was not tested.” |
| Why insufficient | “Production recovery capability remains untested in the supplied records.” Also: “Backup settings or an unexecuted test script cannot demonstrate an executed technical restore.” |
| Next action | “Ask the recovery owner for authorized production test records and objective comparisons.” |
| Human review | “An authorized human must review the cited execution results, objective comparisons and untested dependencies.” |

Evidence link: `KSI-RPL-TRC-proof`, revision 3, lines 1–1. The production gap
belongs to this fixture's declared development/production boundary; it is not a
new blanket requirement to execute a live production recovery test.

### Operational — KSI-INR-AAR

| Prompt | Answer |
| --- | --- |
| What is required? | “Generate incident after-action reports and persistently incorporate lessons learned.” |
| Prerequisite | “Retained incident history” |
| Owner | “Incident response owner” |
| First action | “Locate the incident register and retained investigations, including the period and service boundary covered.” |
| Acceptable proof | “Incident-linked after-action reports plus dated lesson dispositions and resulting procedure or operating-practice records.” |
| What we have | “Incident 42 after-action report”; “The incident report lists lessons, but their actions remain open.” |
| Why insufficient | “Lesson incorporation is not demonstrated by an open action list.” Also: “An empty report template or a list of lessons alone does not demonstrate incorporation.” |
| Next action | “Ask the incident response owner for dated lesson dispositions and records of resulting practices.” |
| Human review | “An authorized human must review report coverage and lesson incorporation separately, preserving open lessons.” |

Evidence link: `KSI-INR-AAR-proof`, revision 3, lines 1–1. A report can support
report generation while leaving incorporation unsupported; it must not be
interpreted as wholly useless or sufficient for both duties.

### Documentary — KSI-CED-RAT

| Prompt | Answer |
| --- | --- |
| What is required? | “Persistently review relevant training effectiveness for all four named populations.” |
| Prerequisite | “Workforce training effectiveness review” |
| Owner | “Training owner and role leads” |
| First action | “Identify general employees, high-risk roles, engineers and response/recovery staff in the training population map.” |
| Acceptable proof | “Population-to-training mappings and dated effectiveness reviews with methods, findings and conclusions for each population.” |
| What we have | “General employee attendance export”; “The export records attendance for general employees only.” |
| Why insufficient | “Effectiveness conclusions and engineer, high-risk and response/recovery population coverage are missing.” Also: “An attendance percentage alone does not establish effectiveness review or coverage of all populations.” |
| Next action | “Ask the training owner for the population map and dated effectiveness conclusions for each population.” |
| Human review | “An authorized human must review exact revisions, population coverage and repeated review activity; attendance is not approval.” |

Evidence link: `KSI-CED-RAT-proof`, revision 3, lines 1–1. Even for the population
represented, attendance alone does not answer whether effectiveness was reviewed.

## Actual assembled guidance: extracted answers and differences

These use field selectors rather than unstable line numbers in the large JSON.
Playbooks are selected by `requirement_id` and `kind: actionable`; prerequisite
names and first actions come from the capability identified by
`prerequisites[].capability_id`. These selections do not resolve a real service's
profile. RPL and INR also have excluded and needs-decision variants; CED has a
needs-decision variant. A real journey must resolve applicability first.

### `trc-required`

- **Required (`purpose.text`):** “Test recovery capability against the defined objectives repeatedly. A backup setting, recovery plan or objective review alone does not demonstrate an executed recovery test.”
- **Prerequisite / first owner:** “Defined service recovery objectives” / “Recovery owner and business owner”.
- **First action (capability setup):** “Locate the desired Recovery Time Objectives and Recovery Point Objectives, their service/target coverage, and the business/capability rationale. If unavailable, record the gap rather than inventing targets.”
- **Acceptable fact (`assertions[].acceptance.fact`):** “Recovery capability is actually tested repeatedly against the relevant defined objectives.”
- **Proof collection (`collect.items[].action.text`), owner “Recovery owner”:** “Collect dated recovery-test execution records, the scenarios and resources tested, actual observed recovery results, and comparisons with the applicable objective versions.”
- **Insufficient (`acceptance.insufficient`):** “Backup configuration, a plan, an objective review, an unexecuted test script, or a tabletop discussion claimed as proof of an untested technical restore.”
- **Review (`review_steps`), owner “Service assessment owner”:** “Distinguish planned exercises from executed tests. Assess the incident/contingency scenarios and service scope covered, actual outcomes and objective comparisons; preserve failed results and untested dependencies.” Completion: “Exact retained citations and scope/period limitations are recorded for each assertion; human service evidence review remains separate.”
- **What we have / concrete next action:** Not present as service-specific facts in the package. Use the displayed fixture answers above only for that synthetic case.

The authored first action adds business rationale and an explicit missing-target
path that the fixture omits. The agent understands that missing targets remain a
gap, not permission to invent numbers or infer successful recovery from backups.

### `aar-required`

- **Required (`purpose.text`):** “After an incident, retain an after-action report and show that lessons are repeatedly incorporated into how the service operates. Writing a report is not the same as incorporating its lessons.”
- **Prerequisite / first owner:** “Retained incident history” / “Incident response owner”.
- **First action:** “Locate the service's incident register and retained investigation records. Record the period and service boundary covered, including any claimed no-incident period.”
- **Acceptable facts, separately:** “Actual incidents have attributable after-action reports.” / “Lessons from incidents are incorporated repeatedly, with current activity status known.”
- **Proof collection, owner “Incident response owner”:** “Collect the incident inventory, after-action reports and records showing how identified lessons were incorporated over time.”
- **Insufficient:** “An empty report template, or a claim of no incidents based only on missing records.” / “A list of lessons or open action items without evidence of incorporation.”
- **Review, owner “Service assessment owner”:** “Separate report generation from lesson incorporation. Link each claimed lesson to its disposition and resulting practice; preserve open lessons rather than treating a report as completion.” Completion: “The assessment cites exact retained revisions, covers the relevant period and service boundary, and preserves unsupported assertions and limitations for separate human evidence review.”
- **What we have / concrete next action:** Absent from authored content; only the synthetic Incident 42 evidence supplies these answers here.

The authored prerequisite's completion criterion explicitly rejects treating
missing access or records as no incidents. The fixture does not exercise this
important ambiguity or the two separate assertion outcomes.

### `ksi-ced-rat-required`

- **Required (`purpose.text`):** “Check whether security training helps the relevant people perform their duties, and retain the repeated reviews. Attendance by itself does not show effectiveness was reviewed.”
- **Prerequisite reason:** “A relevant training/population map and review process are prerequisites to demonstrating repeated effectiveness review across the four named populations.” Capability name: “Workforce training effectiveness review”.
- **First owner / action:** “Training owner” / “Identify all employees, high-risk roles, development/engineering staff, and incident-response/disaster-recovery staff covered by the service's training.”
- **Acceptable facts, five assertions:** “Review activity repeats over time and its current status is known.” / “General training effectiveness review covers the employee population.” / “Identified high-risk roles have relevant training and effectiveness review.” / “Secure-delivery training effectiveness is reviewed for development and engineering staff.” / “Relevant response and recovery training effectiveness is reviewed.”
- **Proof collection, owner “Training owner”:** “Collect the current population-to-training map and dated effectiveness reviews for general employees, high-risk roles, engineers, and response/recovery staff.” Completion: “Exact artifact revisions cover the training objectives, review period, methods, reviewer provenance, findings and limitations for each named population.”
- **Insufficient examples:** “A completion percentage without its employee denominator or effectiveness conclusions.” / “Incident or restore records that never evaluate training effectiveness.”
- **Review, owner “Service assessment owner”:** “Assess each population and persistence assertion separately; distinguish attendance, effectiveness-review activity and real-world security outcomes.” Completion: “The assessment cites exact revisions and explains missing populations or periods. A completed checklist cannot replace human evidence review.”
- **What we have / concrete next action:** Absent from the package; the attendance export and request for missing conclusions are synthetic fixture facts.

The actual initial owner is Training owner; role leads join subsequent setup and
operation. The fixture uses the combined owner everywhere. Additional authored
setup links populations to content/objectives and selects review methods; its
dated population map explicitly identifies role overlap and uncovered groups.
The agent can distinguish five obligations without treating the four populations
as necessarily disjoint or requiring a particular learning-system product.

## Findings and actionable acceptance blockers

1. **Completed-content browser coverage remains unestablished.** Passing fixtures
   use rewritten strings and one combined assertion per domain. They do not prove
   display of actual prerequisite setup, INR's two assertions, CED's five
   assertions, applicability variants or authored alternatives. Exercise resolved
   authored content through the authenticated UI and MCP, with evidence bound to
   the same saved profile/scope and exact revisions. Preserve per-assertion gaps.
   The synthetic published parity test documented in the acceptance report does
   not close this real-domain gap.
2. **Fixture timing can mislead.** The shared `step` helper assigns `one_time` to
   operating duties as well as setup and review. Thus repeated/persistent work is
   paired with “One Time”; it also marks every statement recommended. Use actual
   authored triggers and obligation attribution in completed-content acceptance,
   and check that recurring work remains distinguishable from a one-time request
   to collect its records. Do not introduce an unsupported numeric cadence.
3. **Technical vocabulary needs novice observation.** Recovery Time/Point
   Objectives are expanded but not explained in the first action; “disposition”,
   “provenance”, “population”, “persistence assertion” and “employee denominator”
   require domain knowledge. Ask novices to explain these in their own words.
   Candidate improvements are short contextual definitions (e.g. disposition as
   the recorded decision/action for a lesson) and plain-language labels alongside
   exact technical terms. Agent comprehension is not evidence these are clear to
   beginners.
4. **CED prerequisite heading can sound circular.** “Workforce training
   effectiveness review” names the eventual activity, while its reason and setup
   provide the useful prerequisites: population map, training objectives and review
   method. Confirm users discover those setup details rather than believing an
   already-completed review is needed before starting. The actual purpose is
   notably clearer than the compact fixture's “persistently” wording.
5. **Evidence specificity is synthetic.** “Current review period” and “Service
   boundary” in fixture gap fields are placeholders, not actual dates or scope.
   A live acceptance participant needs real bounded records and an assigned person
   behind each responsible role. The package cannot answer which records a real
   service has or who personally owns the next request.
6. **Human boundary is explicit but untested with people.** Actual review steps
   distinguish assessment preparation from human evidence review. Verify that a
   participant understands collection, an agent-reviewed content release, and a
   completed checklist do not grant service evidence approval.

## Remaining real-user acceptance

Task **7.5 remains partial**: this provides agent extraction from three authored
domains plus repeatable synthetic browser visibility. Its novice journeys still
require representative real participants using the resolved completed guidance.
Task **8.7 remains open**: recruit novices for technical, operational and
documentary journeys; record their unprompted answers to required work,
acceptable proof, supplied proof, insufficiency and next action, plus prerequisite,
owner and human-review prompts. Capture exact answers, assistance needed,
misinterpretations and navigation to proof without interpreting raw codes.
Record participant context, selected variants, evidence revisions and the agreed
acceptance rubric; do not backfill scores from this agent's extracted answers.

No production edits, task-checkbox changes, publication, commit or deployment
were performed for this review. This report is not full-catalog content review,
live evidence acceptance or a human usability study.
