# CED authoring batch: Reviewing All Training

Status: authoring notes for the typed, validated CED draft in
`internal/guidancecontent/ced.go`. A separate content-hash-bound source comparison
is in `guidance/reviews/CED.json`; authenticated service-scoped review recording
and full-catalog publication are still pending.

## Exact inventory boundary

Requirement: `KSI-CED-RAT`. All three inventoried variants retain the same
statement. Keep their applicability distinctions:

| Variant | Plan kind | Resolver outcome |
| --- | --- | --- |
| `2e959240c5101e26990ad59de38559b7df6f4e39c4d4d5ee88f1a7a4058b129b` | actionable | required |
| `712c24e80c7d2f030e4a78c9a59113c301bd34e8159cc226b368e12f05f57271` | actionable | required |
| `0d0ce7e029d4c66a490ff6192725e74b04c48c32885e3f6c0995a82de8a3c231` | needs_decision | KSI applicability for this path requires review |

The unresolved variant must not inherit the actionable checklist. Ask the service
assessment owner: “Which certification path and its applicable KSI rule set are
being assessed? The saved path does not resolve this KSI's applicability.” Retain
the source context; do not ask the user to certify this interpretation.

## Official source set

Rules commit: `a89d78477ddea98dc57eca24f5007794c2b4eeb6`.
Site commit: `8404168e6c150b40e4d0e45152d18965ab27e001`.

- `2026-markdown/providers/20x/key-security-indicators/cybersecurity-education.md#reviewing-all-training`
- `2026-markdown/definitions.md#persistently` (`FRD-PER`)
- `2026-markdown/definitions.md#incident` (`FRD-INT`)
- `2026-markdown/definitions.md#vulnerability-response` (`FRD-VLR`)
- `2026-markdown/providers/20x/rules/fedramp-certification.md`: Class A required-rule
  list explicitly includes `KSI-CED-RAT`, including the clarification about future
  Rev5 program intentions. This does not resolve every path automatically.
- Pinned `testdata/fedramp/rules.json` and the exact generated variant inventory
  supply structured applicability and selected statements.

Official statement:

> The effectiveness of relevant cybersecurity education and training is
> persistently reviewed, including at least general training for all employees,
> role-specific training for employees in high risk roles, training for
> development and engineering staff on secure software delivery, and training
> for staff involved with incident response or disaster recovery.

## Meaning and first action

Determine whether the training helps people perform their security duties, and
keep doing that review over time. Course attendance alone does not demonstrate
that anyone reviewed its effectiveness. Start by asking the training owner for
the current training-to-role map and the most recent effectiveness reviews for
the four named populations. Missing records mean the capability is unknown, not
that training does not exist.

## Shared prerequisite proposal

Stable capability ID: `workforce-training-review`, scoped to the service.
Reuse only where the same service boundary and employee/role population are
demonstrated. A parent-company training program is candidate evidence, not
automatic coverage of this service's engineers or responders.

Recommended setup (inferred from needing to review relevant training):

1. Training owner identifies the employee population, high-risk roles,
   development/engineering roles, and incident-response/disaster-recovery roles.
   Completion: a dated scope map identifies overlap and uncovered roles.
2. Training owner links each population to its learning objectives and actual
   training material. Completion: the map distinguishes generic awareness from
   role-specific content and secure-delivery, response and recovery content.
3. Training owner and role leads select how effectiveness will be evaluated and
   retain the results. Completion: each method names the population and the
   security skill or behavior being evaluated.

The role map and record format are implementation recommendations, not named
FedRAMP deliverables. No particular learning-management system, quiz, phishing
exercise, score threshold or training vendor is mandated by this statement.
Equivalent documented review methods are acceptable candidates.

## Operating duty and timing

Official: persistently review effectiveness across all four named populations.
`FRD-PER` allows intentional irregular cycles, interruptions and waiting periods;
their attributes should be understood and documented and activity status always
known. It does not establish an annual, quarterly or monthly deadline.

Service choice: the training owner records the actual review cycle, responsible
reviewers, current status and reasons for interruptions. A new program with one
initial review can demonstrate initial activity but cannot by itself establish
repetition over a long period. Preserve that limitation rather than inventing a
minimum number of reviews or elapsed days.

Recommended operating record: each review explains the objective, population,
method, findings and disposition. An ineffective training result is not, by
itself, proof that effectiveness was never reviewed. Preserve the finding and
ask whether the review obligation was performed; assess any required corrective
action under its own applicable obligation.

## Assertion-level acceptance

All routes need scope, training versions, review dates or period, reviewer
provenance, method and conclusions. These are recommended sufficiency criteria
to make the official facts assessable, not additional quoted policy mandates.

| Assertion | Fact to demonstrate | Candidate proof | Insufficient alone |
| --- | --- | --- | --- |
| `persistent-effectiveness-review` | Relevant education/training effectiveness is repeatedly reviewed with current activity status known | Dated review history, chosen cycle/status record and documented interruptions | A policy promising future reviews; a single undated dashboard |
| `general-employee-training` | Effectiveness review includes general training for all employees | Employee/population reconciliation, general-training objectives, evaluation results and review conclusions | Completion percentage without its employee denominator or conclusions |
| `high-risk-role-training` | Review includes role-specific training for employees in high-risk roles | High-risk-role identification rationale, training mapping and role-specific effectiveness review | Generic awareness course assigned to everyone without role-specific review |
| `secure-delivery-training` | Review includes development/engineering training on secure software delivery | Engineering population/content mapping and review of secure-delivery learning or applied outcomes | A secure pipeline configuration, repository protection or course catalog |
| `response-recovery-training` | Review includes staff involved in incident response or disaster recovery | Responder/recovery role roster, relevant training and dated effectiveness conclusions | Incident or restore records that never evaluate training effectiveness |

Evidence route A: learning-system exports plus population/role mapping and
review reports. The export proves recorded assignments, participation or recorded
evaluation results; the review reports must demonstrate interpretation of those
results. Neither automatically proves real-world behavior or complete population
coverage.

Evidence route B: retained training materials plus dated, attributable exercise,
interview or practical-evaluation records and the review conclusions, reconciled
to the role/population map. This can demonstrate the same review facts without
a learning-system integration. A single exercise may cover several roles, but
its scope must say which; do not silently count it for all employees.

For both routes, cite exact immutable artifact revisions and relevant passages.
Keep development, production and shared-service staffing coverage visible where
responsibilities differ. Do not replace the actual employee denominator with
the number of cloud accounts or repositories collected.

## Evidence-first next actions

- No role map: ask the training owner for the service's employee and role coverage;
  do not infer absent training or prescribe a new tool.
- Attendance records only: ask for effectiveness methods, findings and review
  conclusions for each named population.
- General awareness only: ask the engineering and response/recovery leads for
  their role-specific evidence; leave those assertions missing.
- Old reviews: identify the period and population change, obtain the current
  activity-status record, and assess the chosen cycle against `FRD-PER`.
- Access unavailable: request an authorized export or owner-supplied records;
  report an access gap rather than no training.
- Poor evaluation results: retain them as evidence of the review and its outcome;
  separate follow-up training changes from collecting the existing evidence.

## Ready-for-assessment criteria and review notes

Check all five assertions, named populations, exact citations, actual review
period, provenance and limitations. Record partial coverage explicitly. Automated
configuration checks cannot establish this KSI's operational training-review
facts. A completed evidence checklist still needs assertion assessment and
separate service evidence review.

Source-comparison issues to preserve in the later authenticated review:

- “Effectiveness” has no numeric pass score in the selected statement. The chosen
  methods need a stated rationale; do not claim FedRAMP mandates a specific test.
- The statement names employees and specified staff; this draft does not invent
  a universal contractor exclusion or inclusion rule. Establish the relevant
  workforce and retain any unresolved boundary interpretation.
- The linked vulnerability-response definition describes a broader process, but
  the selected statement does not add a separate fifth training population.
- Training-review evidence does not prove successful incident handling, recovery
  capability or software security. Those outcomes have separate obligations.

Batch accounting: three variants encoded and validated, with separate
source-comparison inputs prepared; zero authenticated-reviewed or published
variants. File-based comparisons cannot satisfy the authenticated publication gate.
