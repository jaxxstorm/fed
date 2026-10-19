## 1. Inventory and compatibility baseline

- [x] 1.1 Enumerate supported catalog/profile requirement variants, including KSIs and recommended/optional/conditional rules, and save the coverage denominator.
- [x] 1.2 Map existing instruction overrides, assessment bindings, UI contracts and unarchived specs to the new capabilities; record compatibility constraints.
- [x] 1.3 Capture representative evidence, assessment, review, scope and export baselines for migration regression tests.

## 2. Guidance schema and source validation

- [x] 2.0 Add and test actionable, excluded and needs-decision plan kinds against inventory applicability, retaining source-backed explanations without fabricated obligations.

- [x] 2.1 Define versioned playbook, assertion, source citation, prerequisite capability, step, condition and alternative schemas with official/recommended/service-choice labels.
- [x] 2.2 Implement pinned Rules/Markdown source resolution, excerpt hashing and validation against `_sources.json` provenance without editing mirrored policy files.
- [x] 2.3 Implement structural and clause-coverage validation, explicit-none rationales and detection/reporting of generic-only guidance.
- [x] 2.4 Implement graph validation for dangling IDs, cycles, duplicate definitions and invalid conditions.
- [x] 2.5 Implement immutable guidance publication metadata and authorized agent source-comparison review recording, including reviewer type, citations and limitations, distinct from service assessment review.
- [x] 2.6 Implement coverage reports and changed-source detection, including definition-only and narrative-only invalidation.

## 3. Catalog-wide guidance authoring

- [x] 3.1 Generate a per-domain authoring manifest covering every inventoried variant; expand this section into bounded domain batches before authoring and define the agent official-source review checklist.
- [x] 3.2 Define shared capability IDs, scope-instance rules and cross-requirement prerequisites from authoritative context, with inference rationale where appropriate.
- [x] 3.3 Convert existing specialized KSI guidance into playbooks with prerequisites, setup, operation, evidence examples and explicit review criteria.
- [x] 3.4 Complete the manifest's procedural and communications batches, including event-triggered obligations and referenced definitions; record remaining variant IDs after each batch.
- [x] 3.5 Complete the manifest's technical and operational batches, separating configured intent from demonstrated operation; record remaining variant IDs after each batch.
- [x] 3.6 Complete the manifest's governance, documentation and all remaining domain batches, including recommended, optional and conditional variants.
- [x] 3.7 Perform and record agent comparison against official public sources for each batch, preserve cited uncertainties without asking lay users to certify interpretations, and validate full reviewed coverage against the original denominator.

Authoring batches (authenticated review/publication pending): AFC (49), AGU (20), CCM (107), CDS (137),
CMU (27), CPO (42), FRC (150), IEC (76), IVV (80), CED (3), CMT (12), CNA (26),
IAM (18), INR (9), MLA (17), PIY (15), RPL (12), SCR (6), SVC (30), MAS (29),
MKT (14), REC (16), SCG (36), SCN (81), SDR (34), VDR (147), VER (147).
Counts are variant units. Generate exact IDs with `go run ./cmd/guidance-inventory
--manifest`; use the five-dimension official-source review checklist in
`docs/readiness-guidance-authoring.md` for each batch.
All 27 batches now cover 1,340 encoded/validated variants; zero remain unencoded.
The assembled 1,338 playbooks retain 149 shared capabilities and 618 citations.
Distinct source-comparison inputs cover every playbook; live authenticated recording
remains tracked separately under 3.7.
Run `go run ./cmd/guidance-package --coverage` for exact remaining IDs by batch.

## 4. Readiness records and dependency resolution

- [x] 4.1 Add additive versioned storage for guidance bindings, scoped capability instances, declarations, ownership, decisions and evidence references.
- [x] 4.2 Implement authorized, audited, idempotent readiness mutations with expected-revision conflicts and eligible-assignee validation.
- [x] 4.3 Implement three-valued condition resolution, implementation alternatives and dependency ordering with explicit unknown-condition questions.
- [x] 4.4 Implement scope-aware shared-capability deduplication and downstream-impact queries without cross-service or cross-environment leakage.
- [x] 4.5 Implement separate implementation, evidence, trigger and review projections; preserve unknown implementation when proof is absent.
- [x] 4.6 Implement recurring/event-triggered work using cited cadence or labeled service decisions and distinguish not-triggered from completed duties.
- [x] 4.7 Bind current guidance assertions to assessments and generate reassessment needs for changed meaning/applicability while preserving historical results.

## 5. Shared browser and agent contracts

- [x] 5.1 Implement the batched readiness-context projection with citations, relevant evidence, exact bindings, limitations, owner roles and next-action rationale.
- [x] 5.2 Add authorized paginated browser endpoints for requirement context, service plans, capability details and readiness mutations; update API schemas and generated frontend types.
- [x] 5.3 Extend MCP instructions additively and expose equivalent readiness reads/mutations with stable IDs and explicit human-only/access boundaries.
- [x] 5.4 Add guidance-quality coverage responses, immutable readiness export bindings and feature-flagged activation with a full-coverage gate.

## 6. Human-facing readiness experience

- [x] 6.1 Build requirement detail sections for meaning, applicability, prerequisites, ordered actions, current support, missing proof and review criteria with expandable official sources.
- [x] 6.2 Replace generic requirement-card calls to action with specific next actions and reasons while retaining current compliance statuses.
- [x] 6.3 Build the service readiness view grouping shared prerequisites, operations, evidence and review, with downstream links and distinct count units.
- [x] 6.4 Add ownership/declaration/decision forms with conflict handling and explicit separation between declared completion and verified support.
- [x] 6.5 Render missing, draft and stale guidance states, unresolved conditions, and inaccessible evidence sources consistently in both list and detail views.
- [x] 6.6 Display the agent-reviewed label and concise interpretation caveat alongside guidance; include equivalent attribution, limitations and source revisions in MCP responses and exports.

## 7. Verification and rollout

- [x] 7.1 Test full-catalog coverage and source traceability; reject filler-only packages, invented mandatory deadlines, unreviewed publication and malformed graphs.
- [x] 7.2 Test shared prerequisite reuse, environment isolation, alternative selection, unknown conditions, event-not-triggered duties and configuration-only evidence gaps.
- [x] 7.3 Test authorization, revision conflicts, idempotency, stale guidance, assertion-binding changes and immutable historical exports using PostgreSQL-backed integration tests.
- [x] 7.4 Verify equivalent browser/MCP context at identical revisions, old client compatibility, bounded pagination and batched query behavior.
- [ ] 7.5 Exercise novice and agent journeys across multiple requirement domains: identify prerequisites, responsible role, first action, acceptable proof and remaining review without generic inference.
- [x] 7.6 Run repository-required contract, lint, unit, race, browser and build checks; confirm declarations and check completion cannot bypass contradictory findings or human review.
- [x] 7.7 Document guidance authoring/update procedures, reviewer responsibilities, coverage reports and feature-flag rollback; verify the supported-set publication gate.
- [ ] 7.8 Deploy through the approved release workflow, verify representative live readiness views and compare preserved scope/evidence/assessment/review baselines; record acceptance results without marking unfinished content review complete.

## 8. Evidence sufficiency and finding clarity (complete before rollout)

- [x] 8.1 Extend playbooks with assertion-level evidence acceptance criteria, alternatives/combinations, scope/time/provenance expectations and insufficiency examples; include these in every domain's coverage validation and content review.
- [x] 8.2 Implement the shared obligation-to-evidence matrix with exact citations and rationale, separating candidate, supported, partial, missing, contradicted, stale and access-blocked evidence.
- [x] 8.3 Inventory current collector condition codes and implement reviewed explanation templates for expected/observed facts, impact limits, owner-directed actions and evidence needed to verify resolution; provide an honest unknown-code fallback.
- [x] 8.4 Implement finding grouping with exact target counts and expandable per-resource provenance without merging distinct causes, environments or access errors.
- [x] 8.5 Add concise evidence-coverage sections and check rows to the requirement UI, including last-observed time, evidence links, inline definitions and secondary raw diagnostics; expose identical semantics through MCP.
- [x] 8.6 Verify negative failure wording, missing-policy versus failed-enumeration distinctions, configuration-versus-enforcement limits, partial environment coverage, candidate-versus-supported evidence and passing-check-versus-passing-requirement distinctions.
- [ ] 8.7 Run novice comprehension acceptance across technical, operational and documentary requirements: users must identify what is required, what proof is acceptable, what we have, why it is insufficient or failing, and the next action without interpreting raw codes.
- [x] 8.8 Verify repeated findings render concise grouped summaries and evidence-first actions remain separate from remediation; include these acceptance results in the rollout gate.
