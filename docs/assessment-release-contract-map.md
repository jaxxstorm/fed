# Assessment release contract map

Implementation reference for `add-reviewed-assessment-trust-centre`, task 1.1.
This documents existing behavior and extension boundaries, not completed release
functionality.

## Reusable identities and records

| Existing contract | Identity/binding | Release-workflow integration |
| --- | --- | --- |
| `assessment.Record` / `records` | Service, kind, ID, revision; actor and creation time | Preserve immutable versions; extend allowed kinds with additive migrations. Never treat an ID without a revision as reviewed evidence. |
| `InstructionBinding` | Catalog, profile/scope revisions, instruction version, source revisions, optional guidance revision/hash/variant and assertion hash | Reuse for structured submissions. Validate the complete current binding inside the mutation transaction. Guidance fields remain optional when guidance is disabled. |
| `RuleResultSubmission` / `RuleReceipt` | Requirement ID, expected result and draft revisions, actor-scoped idempotency key | Keep the existing tool and input supported. Add a versioned structured contract that writes through the same result/draft chain. Preserve receipt references instead of creating a second assessment engine. |
| `Citation` | Exact evidence `Ref`, text line range or JSON pointer | Reuse syntax/range validation and artifact retrieval. Support does not follow merely from a valid selector; retain separate assessment rationale. |
| `Evidence` / `DocumentaryProvenance` | Evidence revision, artifact hash/key, source revision/version, observed/retrieved times | Keep event time separate from retrieval. A new retrieval is not proof of operation in a new period. |
| `AssertionResult` / `AssertionGap` | Assertion ID, supported/contradicted/unknown outcome, rationale, citations and explicit gaps | Extend with separate implementation and operating-support assessments; preserve existing outcome semantics for legacy clients. |
| `EvidenceWork` | State, assessment, missing assertions, blockers, reasons | Add review-readiness projection separately; do not reinterpret `evidence_complete` as human approval or publication readiness. |
| `readiness.WorkRecord` | Stable `WorkInstanceID`, revision, scope key, `WorkBinding` | Evidence requests link an existing work ID/revision where present. `WorkInstanceID` includes duty/capability and stable scope identity; avoid generating another work item from a label. |
| `Draft` / assessment record | Requirement ID as assessment ID; profile/scope and evidence/measure revision references | Attach structured implementation content to this revision chain; legacy free-text drafts require enrichment. |
| `Review` | UUID review ID, requirement ID, exact assessment reference, notes; actor/time on record | Retain legacy records. New adoption and review decisions need explicit capacity, procedures, sampling, dependencies and conclusion; do not infer these fields from old notes. |
| `Gap` | Service-local gap ID, revision, explanation and open/resolved status | Rich issues link gaps and findings. Existing resolved gaps are not silently migrated to evidence-backed issue closures. |
| `Bundle` / snapshot | Actor/service/idempotency-derived snapshot ID; repeatable-read preparation | Keep `ksi-assessment-bundle/v1` and its limitations. New release manifests are a separate format referencing exact existing records. |
| `Publication` / integration receipt | Snapshot, optional destination reference, state and confirmed receipt | First-party publishing uses the same distinction between preparing, exporting and actually publishing. An export/download is not a publication receipt. |

## Mutation and authorization boundaries

- `SubmitRuleResult` authorizes, locks the service, checks the operation's request
  digest, resolves current instructions, and compares exact result/draft revisions
  before writing. New submission paths must retain this atomicity.
- `Store.Review` requires the latest assessment revision and the human-only
  `review` action. New independent-assessor and adoption semantics must not be
  smuggled into the existing notes field.
- `auth.Authorize` currently uses active organization membership for humans;
  normal roles cover the organization's services. The generic `read` action is
  therefore unsuitable for external trust-centre readers.
- Browser OIDC callback currently calls `Admit`, creates an organization-member
  session, and directs pending members to `/approval`. External-reader admission
  needs an explicit separate flow; approving an external grant must not activate
  organization membership.
- Existing agent tokens allow read/edit/check/export/connect, not human review.
  New adoption, attestation, approval, and publication paths must enforce human
  identity explicitly instead of relying on the agent's general export action.
- `readiness.WorkBinding.ReleaseHash` refers to a **guidance release**, not an
  assessment publication. New release references must use distinct names/types.

## Projection and storage boundaries

- Browser and MCP currently consume shared assessment context; structured
  narratives and readiness must extend that shared projection.
- Observation status and presentation-cache aggregation both exist. Lifecycle
  reconciliation must converge on one shared current-resource projection rather
  than implementing a third package-only aggregation.
- `PreparePublication` includes historical records and observations and enforces
  a maximum snapshot size. The new evidence archive must stream artifacts rather
  than add binary contents to that JSON record.
- Legacy export code selects an explicit list of record kinds. New release
  metadata must not accidentally enter existing exports or public projections.
- Release delivery must validate grants against exact release and artifact
  membership. The general internal evidence download path is not an external
  reader authorization boundary.

## Compatibility verification targets

1. Existing MCP inputs, artifact receipts, citation validation, and conflict and
   idempotency behavior remain unchanged.
2. Existing assessments/reviews/exports load without requiring new fields, and
   migrations do not invent claims, approvals, or assessor authority.
3. Guidance-disabled instruction delivery and the separate guidance acceptance
   gates remain operational.
4. New external grants do not affect internal organization permissions.
5. Scope/profile/credential-generation fencing and immutable observation history
   survive current-resource retirement and release preparation.

## Confirmed readiness boundary

The user confirmed that Fed validates structural completeness, agents assess
what evidence demonstrates, and humans review substantive adequacy. Citation
validation establishes that an artifact and selector exist; it does not establish
what arbitrary prose proves. The readiness contract must label this distinction
explicitly. Missing structured implementation fields are server-detectable;
generic restatements and unsupported substantive claims require agent assessment
and human review. The design has no embedded model.

## Confirmed historical-period eligibility

The structured submission prototype reuses `SubmitRuleResult`'s atomic citation
and draft transaction. That transaction rejects documentary evidence whose
`ObservedAt + FreshnessHours` is before the current time, and applies an equivalent
current-time check to legacy measure evidence. `WorkStatus` also evaluates current
freshness. These existing rules are correct for the current-working assessment
contract but cannot by themselves represent historical-period eligibility.

For example, an otherwise verifiable September exercise record submitted in
October can be rejected even when the assessment period is September. The new
period contract needs an explicit choice: allow such historical citations for
their declared period while separately reporting current freshness, or retain
current-time freshness as a mandatory submission condition. The recommended
choice, now accepted by the user, is separate period eligibility and current
freshness, retaining artifact, scope/profile, provenance, and revision checks.
Structured submissions use that distinction; legacy current-working submissions
retain their freshness validation. The change has not been deployed.
