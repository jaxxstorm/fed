# External-agent workflow verification

The new build and additive migration were applied to the local service on
2026-10-17. Database backup: the approved temporary directory contains
`pre-agent-evidence.dump`.

Migration retained all 784 pre-existing evidence records, 11 measures, five scope
revisions, one profile, 38 gaps and three historical assessments. Source discovery
materialized 49 stable sources (41 repositories, six AWS accounts, two clusters).
Repeated discovery left 49 source records/revisions and did not complete any rule.

## Real external-agent investigation

Using an ephemeral service-scoped token over HTTP MCP:

1. Listed open KSI rules and fetched instructions for `KSI-SVC-ACM`.
2. Discovered the scoped `IgnisTechnologies/infrastructure` repository source.
3. Read `.github/workflows/pulumi.yaml` with `git show` at saved commit
   `188f51f38ebf13869d1821c10a33e764aaa7fdbc`. No working-tree content or
   infrastructure changes were involved; scope revision 5 remained unchanged.
4. Submitted the file directly through `submit_evidence_artifact`, then retrieved
   and compared its bytes and SHA-256 through `get_evidence_artifact`.
5. Submitted a cited partial result and verified its evidence ID and work state
   in the requirement browser page. The temporary token was revoked afterward.

Evidence ID: `1a312869-0dea-4d0c-9110-7933747d207f`, revision 1.

Artifact SHA-256:
`e925ef9be665d3c5d686782d2fe66d06dde939357477f1683180705d3fdf9c65`.

Rule result and draft: `KSI-SVC-ACM`, revision 1. Receipt: `partial`, agent
assessment `unknown`, human review `pending`.

The workflow shows pull-request previews, push deployments and dependency-layer
execution. It establishes configuration intent, not persistent drift review or
complete deployed-resource coverage. Execution/drift-review records were not
gathered in this investigation and remain explicit missing inputs. Accordingly,
the live rule was not falsely checked complete. No AWS request expanded beyond
the saved us-east-1 boundary; this investigation only read a scoped repository.

## Automated verification

The SDK/browser fixture flow independently verifies successful completion,
contradicting evidence with completed investigation, partial and blocked results,
direct artifact retrieval and matching browser status with no collector worker,
collection plan or measures. Assessment tests verify shared evidence across rules,
source-based and mixed legacy freshness, new mapped findings reopening only the
affected rule, immutable snapshot history and retry/failure behavior.
