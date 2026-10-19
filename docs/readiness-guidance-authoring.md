# Readiness guidance authoring

The implementation includes guidance validation, agent source-comparison reviews,
immutable service-scoped publication and shared browser/MCP/export attribution.
All 1,340 catalog variants now have authored, validated guidance and distinct
source-comparison inputs. Production authenticated recording and publication
remain pending; local inputs and test-service receipts are not production reviews.

## Inventory and domain batches

Generate the complete profile inventory with `go run ./cmd/guidance-inventory`.
Use `go run ./cmd/guidance-inventory --manifest` for the compact authoring input:
every domain lists exact requirement IDs, variant IDs and applicability counts.
Both commands use the same pinned rules and production resolver. See
`readiness-coverage-baseline.md` for the saved denominator and provenance.

Author each domain as a separate batch. Record progress per exact variant ID;
one class's playbook does not automatically cover every class or path. The
domain batches are below. Each now has a completed source-comparison input;
all await authenticated production recording.

| Batch | Variants | Reviewer |
| --- | ---: | --- |
| AFC | 49 | Comparison prepared |
| AGU | 20 | Comparison prepared |
| CCM | 107 | Comparison prepared |
| CDS | 137 | Comparison prepared |
| CMU | 27 | Comparison prepared |
| CPO | 42 | Comparison prepared |
| FRC | 150 | Comparison prepared |
| IEC | 76 | Comparison prepared |
| IVV | 80 | Comparison prepared |
| CED | 3 | Agent comparison prepared; authenticated recording pending |
| CMT | 12 | Comparison prepared |
| CNA | 26 | Comparison prepared |
| IAM | 18 | Comparison prepared |
| INR | 9 | Agent comparison prepared; authenticated recording pending |
| MLA | 17 | Comparison prepared |
| PIY | 15 | Comparison prepared |
| RPL | 12 | Agent comparison prepared; authenticated recording pending |
| SCR | 6 | Comparison prepared |
| SVC | 30 | Comparison prepared |
| MAS | 29 | Comparison prepared |
| MKT | 14 | Comparison prepared |
| REC | 16 | Comparison prepared |
| SCG | 36 | Comparison prepared |
| SCN | 81 | Comparison prepared |
| SDR | 34 | Comparison prepared |
| VDR | 147 | Comparison prepared |
| VER | 147 | Comparison prepared |

Large batches can be split by requirement ID without changing their denominator.
All domains have typed authored drafts in `internal/guidancecontent/`, covering
all 1,340 exact variant IDs. Emit a domain package with
`go run ./cmd/guidance-package --batch CED` (substitute the domain ID). Each builder verifies pinned
source excerpts and structural validity; tests also validate exact inventory
coverage. Separate source-comparison inputs in `guidance/reviews/` retain exact
content hashes, findings and uncertainties. Tests reject stale comparison hashes.
These files are not authenticated service review receipts, so the batches still
contribute zero production published-reviewed variants. Authenticated production
recording remains pending. Run `go run ./cmd/guidance-package --coverage` to obtain the full original
denominator and exact encoded/remaining IDs per domain. Duplicate batch variants
are rejected rather than inflating coverage. This local report is not the
authenticated service guidance-quality report.
Emit the assembled release with `--batch all`. Assembly canonicalizes citation IDs
without merging conflicting excerpts, rejects conflicting shared capabilities,
and clears untrusted review metadata. `--batch all --comparison-inputs guidance/reviews`
emits comparison inputs rebound to assembled hashes only after checking that the
normalized playbook/dependency/source closure has not changed. This does not
authenticate the comparisons or grant publication authority.
Excluded and unresolved variants remain visible. Excluded plans explain why the
obligation is outside the selected profile; needs-decision plans state the exact
question and responsible role. Neither creates a made-up implementation checklist.

## Draft structure

`internal/readiness/playbook.go` defines `requirement-playbooks/v1`. Actionable
plans include purpose, applicability, prerequisite capabilities, setup/operation/
collection/review steps, clause-linked assertions and evidence acceptance routes.
Each route is a combination of required artifacts; routes are alternatives.
Every route explains what it proves and its limitations. Statements label their
authority as official, recommended implementation, or service choice.

`ValidateDraft` checks references, required sections, clause-text coverage and
dependency graph integrity. It rejects known generic fallback instructions.
These checks cannot certify the semantic quality of prose. Reviewers must verify
that the clauses are correctly interpreted, prerequisites are justified and
evidence criteria are genuinely sufficient rather than just populated fields.

`VerifySources` binds rules to the catalog payload hash and source commits in the
Markdown corpus's `_sources.json`. Citation excerpts and file hashes must match.
`ValidateInventory` binds plan kinds and exact selected statements to the real
profile variants. Do not edit mirrored FedRAMP policy to make guidance validate.

## Content review and publication

### Approved workflow update

Content review is performed by an agent against official public FedRAMP sources;
individual human approval and human domain-reviewer assignments are not required.
The source comparisons are not requests for the user to certify policy
interpretations. For each plan, the agent must compare
applicability, obligations, prerequisites, evidence expectations and timing with
the cited rules, definitions and narrative guidance, recording findings and
specific uncertainties separately from drafting.

Browser guidance shows **Agent-reviewed guidance** with this caveat:
"Checked against cited official FedRAMP sources. Interpretations may be incomplete
or incorrect; this is not an official FedRAMP determination."
MCP and exports must carry equivalent attribution and limitations.

Each comparison dimension requires findings and bound citations. Every citation
in the playbook/dependency closure must be included in the comparison. Review
limitations are mandatory; an explicit empty uncertainty list means the reviewer
identified none. These are traceability checks, not proof of semantic accuracy.

Content review is distinct from assessment review for an individual service.
`Store.RecordContentReview` requires authenticated service edit authorization
and stores a review of the exact playbook plus its dependency/citation closure.
Reviewer identity and human/agent type come from the authenticated identity,
not from supplied review metadata. Agents can review without human content approval.
`Store.Publish` ignores supplied JSON review claims and loads authenticated
database review records. It requires all catalog variants to have current review
coverage. Both operations audit successful new records and preserve idempotency.

Migration `016_agent_guidance_review.sql` adds service-scoped append-only source
reviews and guidance releases. It preserves the earlier tenant-level tables as
history. Updating or deleting review/release records is rejected. Review records
cannot be reused across services. Repeating an identical comparison is idempotent;
changing a recorded comparison for the same content returns a conflict.

`KSI_GUIDANCE_ENABLED=true` enables guidance context and the MCP tools
`review_requirement_guidance` and `publish_requirement_guidance`. The default is
false. Source roots are operator-controlled `KSI_GUIDANCE_RULES_DIRECTORY`
(default `/app/catalog`) and `KSI_GUIDANCE_MARKDOWN_DIRECTORY` (default
`/app/2026-markdown`). The container includes the pinned Markdown corpus.
Published packages must cover the full catalog before they can be read as
reviewed guidance. Missing packages and unavailable/changed sources produce
explicit gaps or revalidation states, never a reviewed label.

The browser requirement page and MCP `get_rule_instructions` use the same context;
exports pin its package, citations, review findings and release hash in the
historical snapshot. Disabling the flag rolls back presentation and tools while
retaining additive tables and historical exports. Full readiness activation and
catalog authoring remain separate outstanding tasks.

`Coverage` reports missing/draft/reviewed/needs-revalidation by variant and profile.
Published coverage is available through MCP `get_guidance_coverage` and
`GET /api/ui/v1/services/{service}/readiness/coverage`. Use `limit` (1–100) and
`offset`; compare catalog identity and release hash when combining pages.
The service readiness page includes an expandable coverage view. Its counts are
catalog variants, not service compliance. `get_readiness_capability` and
`GET /api/ui/v1/services/{service}/readiness/capabilities/{id}` read one exact
scoped instance using an ID returned by the service plan.
It verifies each book's referenced source subset, so a changed definition can
invalidate dependent books without flagging unrelated books. Its review metadata
is descriptive; only the authenticated publication service accepts reviews as
authority. A generated package cannot approve itself by filling in reviewer IDs.

## Updating and rolling back guidance

1. Inventory the new pinned Rules payload and matching `_sources.json` before
   editing product guidance. Compare exact variant IDs, applicability and cited
   source hashes; a definition or narrative change also requires review.
2. Revise affected playbooks and shared capabilities in product-owned files.
   Increment their revisions, preserve official normative strength and retain
   explicit uncertainties. Revalidate the complete dependency/citation closure.
3. Regenerate local draft coverage and compare the original denominator. Missing
   variants remain missing; neither a new catalog nor a failed review permits
   silently dropping variants. Prepare distinct five-dimension comparisons for
   changed content hashes. A prior comparison must not be relabeled current.
4. Record comparisons with the authenticated service-scoped agent tools. Retain
   their immutable receipts; local JSON inputs and author identities do not count
   as authenticated reviews. A corrected comparison needs a new content revision
   rather than an overwrite of its immutable predecessor.
5. Assemble and validate the intended full release, resolving conflicting shared
   identifiers explicitly. Verify all 1,340 current inventory variants have
   substantive, authenticated-reviewed coverage before publishing. Partial batch
   packages are authoring inputs, not deployable catalog releases.
6. Follow implementation, commit/push, successful image publication, then the
   approved Pulumi deployment. Check exact live scope/evidence/review baselines
   and representative browser/MCP views before recording release acceptance.

To roll back presentation, set `KSI_GUIDANCE_ENABLED=false` through the approved
deployment configuration and deploy it. Verify guidance tools/routes and browser
entry points are disabled, while existing evidence, assessments and frozen
exports remain readable. Retain migrations and append-only guidance/review/work
records. Re-enabling the feature does not waive source freshness or full-coverage
checks, and reverting UI code must not rewrite historical assertion bindings.

Publication gate regression coverage is in `internal/readiness/publication_test.go`
and `internal/readiness/playbook_test.go`: incomplete/unreviewed packages,
untrusted review claims, stale source bindings and cross-service review reuse
must be rejected. Those tests verify the gate, not completion of actual content.

## Baseline verification

`TestReadinessMigrationHistoricalBaseline` builds an existing documentary
evidence/assessment/human-review/export fixture, then changes live scope and
verifies that frozen export bytes and exact historical record revisions remain
unchanged. The fixture is synthetic and contains no production credentials or
customer records.
