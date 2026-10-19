# Readiness guidance coverage baseline

## Reproduction

Run `go run ./cmd/guidance-inventory > inventory.json`. The command reads the
pinned local JSON/schema, uses the production catalog parser and resolver, and
does not connect to a database or activate guidance. The full manifest includes
every variant and its matching profile IDs; it is generated rather than checked
in because the current expanded manifest is approximately 40 MB.

## Saved denominator

- Rules commit: `a89d78477ddea98dc57eca24f5007794c2b4eeb6`
- Payload hash: `fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4`
- Markdown site commit: `8404168e6c150b40e4d0e45152d18965ab27e001`
- Imported requirements: **291** (FRR and KSI).
- Supported categorical profiles: **48** (20x; Program/Agency; classes A–D;
  initial/ongoing; next-assessment unknown/false/true).
- Source-derived date representatives: **41**, including the lower bound and
  both sides of source date transitions; **1,968** total profile samples.
- Distinct resolved variants: **1,340**. These include **182 required**,
  **72 recommended**, **336 optional**, **265 unresolved**, and
  **485 not-applicable** variants. These are variants, not unique requirements
  or an individual service's compliance denominator. Non-applicable variants
  preserve the exclusion reason; they do not create implementation obligations.
- Domain variant counts: AFC 49, AGU 20, CCM 107, CDS 137, CMU 27, CPO 42,
  FRC 150, IEC 76, IVV 80, CED 3, CMT 12, CNA 26, IAM 18, INR 9, MLA 17,
  PIY 15, RPL 12, SCR 6, SVC 30, MAS 29, MKT 14, REC 16, SCG 36,
  SCN 81, SDR 34, VDR 147, VER 147.

The manifest deliberately retains all imported rules and exclusion states, so
coverage cannot improve by dropping inconvenient variants. Dates found anywhere
in raw policy metadata are included conservatively; extra unchanged date samples
do not create duplicate semantic variants. New catalogs require regeneration.

## Compatibility map

| Existing boundary | Integration constraint |
| --- | --- |
| `internal/catalog/catalog.go` | Use `Profile.Validate` and `Resolve`; current support is 20x only, not all FedRAMP certification tracks. Preserve unresolved/grace-period conditions and official force. |
| `internal/assessment/agent_rules.go` | Replace generic-only experience through additive guidance fields; existing `whole-statement` results remain historical rather than becoming proof of new assertions. |
| `internal/assessment/agent_contracts.go` | Preserve catalog/profile/scope/source/version bindings and exact citation revisions; add guidance bindings without silently interpreting old receipts as new submissions. |
| `internal/assessment/presentation.go` | Keep status precedence, contradictions and human review; readiness is a separate projection. |
| `internal/assessment/rule_results.go` | Preserve authorization, optimistic concurrency, idempotency and evidence/citation validation. |
| `internal/assessment/observation_status.go` | Preserve target scope, freshness and incomplete collection coverage when explaining findings. |
| Web contracts and generated frontend client | Extend contracts and regenerate together; browser and MCP use one domain projection. |
| Existing exports and versioned records | No rewrite of historical evidence, scope, reviews or frozen exports; pin new guidance in new exports only. |

Implemented but unarchived contracts remain relevant in
`openspec/changes/redesign-assessment-experience/` and
`openspec/changes/enable-agent-evidence-workflow/`. The main spec directory is
empty; this change must not discard those implemented contracts.

## Content publication boundary

This inventory is an authoring input, not reviewed guidance. There are currently
no published readiness playbooks. Agents perform a distinct comparison against
the official public sources and record findings, citations, limitations and
uncertainties before the full-coverage publication gate can be satisfied. Human
approval of each explanation is not required. See `readiness-guidance-authoring.md`.

## Approved design adjustment: variants without selected statements

The generated inventory contains **46** variants with an empty selected
statement: **31 unresolved**, **15 not applicable**. For example,
`CCM-QTR-MTG` and `CDS-CSO-AVR` expose class/subset conflicts, while excluded
track variants can return before selecting a class-specific statement.

These are resolver outcomes, not evidence that official source text is missing.
They must remain in the denominator and expose the raw source and applicability
reason. The current actionable-playbook schema requires a nonempty selected
statement and assertions, so those outcomes cannot satisfy it honestly.
Approved follow-up design decision: distinguish an actionable playbook from a
source-backed applicability explanation/decision request, with no fabricated
setup duties or evidence acceptance criteria for an unselected obligation.
