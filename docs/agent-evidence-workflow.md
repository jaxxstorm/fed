# External-agent evidence workflow

Choose a service profile and save its assessment scope in Settings. Configure
**Evidence sources** in the service navigation, then connect an external agent
to `/mcp` with a service-scoped bearer token. The agent uses its own tools and
credentials. A collector worker, collection plan, model provider or agent queue
is not required for this workflow.

Invoke the MCP `gather_evidence` prompt with `service_id`, or ask the agent to
follow this loop:

1. `list_open_rules`: discover outstanding KSI and broader obligations. Follow
   `next_cursor`; collect IDs before submitting mutations. On `view_changed`,
   restart without the cursor. Page size defaults to 25 and is limited to 100.
2. `get_rule_instructions`: read official text, applicability, assertion IDs,
   investigation steps, existing evidence and exact revision bindings.
3. `list_evidence_sources`: page the shared source directory using limit/offset.
   Read relevant sources using your own repository, AWS, Kubernetes or document
   tools. Source configuration declares where to look, not proof of access.
4. `submit_evidence_artifact`: upload bounded evidence directly.
5. `submit_rule_result`: cite the evidence, assess each assertion and report
   evidence_complete, partial or blocked. The receipt includes a saved draft.
6. Inspect the receipt and continue through other outstanding rules. Reuse an
   artifact across requirements instead of uploading copies.

## Source configuration

The UI source forms accept versioned JSON. For example:

```json
{
  "type": "document",
  "name": "Security policies",
  "locator": "https://docs.example/security",
  "scope_revision": 5,
  "paths": [],
  "access_hint": "company-docs",
  "requirement_ids": [],
  "enabled": true,
  "freshness_hours": 720
}
```

Use the actual scope revision and organization source. An empty requirement list
makes the source discoverable for all rules. Supported types are `repository`,
`aws`, `kubernetes`, `document`, and `operational_records`. Existing scoped accounts,
clusters and repositories receive stable derived source IDs on discovery. Edits
preserve access hints and source policy when scope-derived bindings reconcile.

AWS sources cannot expand saved regions; Ignis remains in us-east-1. Repository
paths are relative allowlisted paths/subtrees at the saved commit. Document and
record locators are HTTPS source boundaries, not URLs the server will fetch.
Hints are symbolic profile/context/integration names, never credentials. Source
freshness defaults to 48 hours; customize it for the evidence type. This is an
application policy, not a FedRAMP threshold.

## Direct artifact submission

Example tool arguments (replace all identifiers, dates and content with actual
discovery results and observed evidence):

```json
{
  "service_id": "SERVICE_ID",
  "idempotency_key": "policy-observation-unique-key",
  "source": {"id": "SOURCE_ID", "revision": 1},
  "scope_revision": 5,
  "locator": "https://docs.example/security/change-policy",
  "source_version": "approved-v3",
  "media_type": "text/markdown",
  "content": "# Change policy\nProduction changes require recorded review.\n",
  "observed_at": "2026-10-17T12:00:00Z",
  "retrieved_at": "2026-10-17T12:05:00Z",
  "method": "Read approved document version through configured document client",
  "description": "Change policy excerpt",
  "limitations": ["Documented intent; operational review records not included"]
}
```

For a repository source, `locator` is the relative path and `source_version` is
the pinned commit. Supported media types are `text/plain`, `text/markdown` and
`application/json`. Content is limited to 1 MiB within the 2 MiB MCP request cap.
Binary documents need a focused text excerpt with original source/version and
extraction limitations. Unsupported/oversized input is rejected, never silently
truncated. Original observation/event time is distinct from retrieval time.

The response contains evidence `id`, `revision` and server-computed artifact hash
in `data.artifact_key`. `get_evidence_artifact` returns content, media type and
SHA-256. Evidence remains agent-authored, even if it describes an AWS response.

## Submit a rule result

Copy `binding`, assertion IDs, `expected_result_revision` and
`expected_draft_revision` from `get_rule_instructions`. Submit:

```json
{
  "service_id": "SERVICE_ID",
  "requirement_id": "KSI-SVC-ACM",
  "idempotency_key": "acm-investigation-unique-key",
  "binding": {
    "catalog_id": "CATALOG_ID_FROM_INSTRUCTIONS",
    "profile_revision": 1,
    "scope_revision": 5,
    "version": "agent-evidence/v1",
    "sources": [{"id": "SOURCE_ID", "revision": 1}]
  },
  "expected_result_revision": 0,
  "expected_draft_revision": 0,
  "requested_state": "partial",
  "explanation": "Policy intent was found; operational automation and drift-review evidence still need investigation.",
  "assertions": [{
    "assertion_id": "whole-statement",
    "outcome": "unknown",
    "rationale": "The cited policy alone does not demonstrate automation and persistent drift review.",
    "citations": [{"evidence": {"id": "EVIDENCE_ID", "revision": 1}, "start_line": 1, "end_line": 2}]
  }],
  "blockers": ["No dated drift-review records were found in the inspected source"]
}
```

Text citations use 1-based inclusive line ranges. JSON citations use
`json_pointer` instead of line ranges; an empty pointer refers to the whole JSON
document. A pointer must resolve against the stored artifact. Keep the exact full
source binding returned by instructions rather than shortening it as in examples.

Assertion outcomes are `supported`, `contradicted` or `unknown`. Only request
`evidence_complete` when every required assertion has valid current citations,
no unknown outcome or blocker remains, and applicability is resolved. Generic
guidance requires evidence addressing the whole statement, not merely one clause.
The service validates structure and bindings; semantic interpretation remains an
agent assertion for review. Completion never creates human approval.

Receipts include `result`, `draft`, `evidence`, `work` and `review`. A typical
partial receipt has `work.state: partial`, `work.assessment: unknown`, explicit
`missing_assertions` and `blockers`, and `review: pending`. Complete evidence may
show `assessment: contradicted`: the investigation finished but found a failure.

## Retries, access and freshness

- Identical idempotency keys/payloads return the original receipt. Different
  payloads with the same key conflict. Use expected revisions for concurrent edits.
- On `revision_conflict`, reread instructions and reconcile current work before
  submitting a changed result. Historical artifacts never close a newer scope.
- Discovery requires `read`; submissions require `edit`; invoking collectors
  still requires `check`. Token revocation is enforced on every request.
- One artifact can support multiple rules through explicit citation links. Those
  links make it visible in requirement lists and valid in subsequent draft edits.
- Documentary evidence uses its frozen source freshness policy; legacy evidence
  retains measure-based policy. Expired evidence, changed bindings or new mapped
  contradictory observations reopen affected work as `needs_refresh`.
- Current status can change after a receipt. Historical receipts, drafts and
  snapshot exports remain immutable.

## Operations

Back up the database and artifact store, stop the old worker/server, run the new
binary's `migrate` command, then restart the service. Existing evidence and review
history are preserved. New source reconciliation does not mark existing rules
complete. Roll back only to a schema-compatible binary and retain new records.
No external agent is launched or scheduled by the Go service.
