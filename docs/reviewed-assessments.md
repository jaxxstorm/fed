# Reviewed assessments and trust-centre delivery

## Evidence and decisions

Agents use `get_rule_instructions`, collect read-only evidence, and submit exact
artifact revisions through the existing evidence tools. Use
`submit_implementation_assessment` to describe the actual service: responsible
roles, mechanisms, procedures, frequency, boundary, and assertion-specific
implementation and operating conclusions. Declare the reporting period, event
dates, sample method, population uncertainty, and limitations explicitly.

Fed checks structural completeness and citation integrity. These checks do not
establish substantive support or compliance. Historical records can support their
declared period while being stale today. A collection completion ratio is not a
requirement compliance score.

Use `save_evidence_request` and `list_evidence_requests` for missing evidence.
Fulfillment cites exact evidence revisions; humans accept or reject it against
the current binding. Use `save_assessment_issue` and `list_assessment_issues` for
ownership, milestones, and evidence-bound dispositions. Closing an issue does not
erase its underlying historical finding or gap.

Human organizational adoption, internal evidence review, and independent-assessor
review are separate decisions. An administrator must designate an independent
assessor for the service. A trust-centre assessor *reader grant* does not confer
authority to record an independent assessment. Agents can inspect review targets
but cannot record human decisions. Guidance-source review is a separate workflow.

## Packages

The service's **History & exports** workspace contains service context,
supporting-deliverable mappings, reporting-period selection, and release controls.
Mappings reference the pinned requirement and exact evidence revision. Unresolved
supporting-material mappings and unassessed requirements remain explicit.

Preparation freezes a repeatable database snapshot. The release contains the
profile/catalog, scope, applicability, implementation assessments, reviews,
collection status, issues, evidence index, and an explicit public disclosure.
Approval binds both the manifest hash and the public-disclosure hash. Editing
content requires a new release; subsequent collection does not change existing
release bytes. Incomplete reports are labeled incomplete.

The `fed-assessment-release/v1` archive contains:

- `manifest.json`: the frozen assessment and exact references;
- `schema.json`: Fed's package schema;
- `index.html`: offline report;
- `evidence/<sha256>`: included artifact bytes;
- `SHA256SUMS`: file-integrity metadata.

Verify with `shasum -a 256 -c SHA256SUMS` from the extracted directory. Artifact
paths are content-addressed, not remote download links. The archive is limited to
256 MiB uncompressed, and individual artifacts retain the storage limit of 16 MiB.
Missing or corrupt artifacts fail preparation. Hashes are integrity checks, not
assessor signatures. This is a Fed package, not an official schema-validated
authorization submission.

## Public and restricted delivery

Configure a service slug in the workspace. Delivery is disabled by default.
Prepared and approved releases are not publicly delivered. A human must approve
the exact disclosure and explicitly publish the approved release.

`/trust/<slug>` exposes only the explicit public projection. It does not serialize
the internal manifest, evidence metadata, or reviewer identities. Public material
may be copied by recipients and cannot be recalled.

External readers sign in through the configured OIDC provider and admission
policy. Arbitrary external issuers are not supported. A verified identity may
remain pending for organization membership while requesting a service-wide or
release-specific trust-centre grant. Approving that grant does not approve
workspace membership. Administrators select reader/assessor capacity and expiry.
Restricted reports, JSON, archives, and artifacts check the grant and published
release state on every request; responses use `private, no-store`.

Revoking a grant stops subsequent delivery. Withdrawing a release stops restricted
delivery, leaves a public withdrawal tombstone, and updates the latest pointer.
It does not erase immutable history or copies already downloaded. Disabling the
trust centre stops its public and restricted routes. Keep delivery disabled during
rollout until the disclosure and access-control acceptance checks are complete.

## Recurring reporting

The workspace can configure an owner, interval, first period, and release
configuration. The worker checks due schedules once per minute and processes a
bounded candidate job. Period/schedule keys prevent duplicate scheduled runs.
Preparation failures leave a durable notification and retry up to three times,
with five minutes between retries. An expired worker lease can be reclaimed;
release-preparation idempotency prevents duplicate candidate releases.

Candidates never approve, publish, or invoke agents. Owners see notifications in
the workspace. MCP clients use `list_reporting_candidates`,
`list_assessment_releases`, `list_evidence_requests`, and
`get_assessment_review_target` to investigate freshness and re-review work.
The selected previous release remains immutable. Review the new period's
evidence and disclosure before any approval.

## Deployment and acceptance

Apply additive migrations before starting the new web and worker processes.
Keep trust-centre delivery and `KSI_GUIDANCE_ENABLED` disabled until their separate
human acceptance gates pass. Follow repository CI/container publication and the
approved Pulumi rollout process. For rollback, disable trust-centre delivery
before rolling back application processes; preserve migrations and evidence
history. Do not restore an older database over newer review or publication data.

The implementation checklist in
`openspec/changes/add-reviewed-assessment-trust-centre/tasks.md` tracks remaining
verification and live acceptance. Documentation is not a record of human approval.
