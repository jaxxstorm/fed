# KSI Service

A Go/Echo service with a React/TypeScript operator workspace and MCP endpoint for class-aware FedRAMP 20x assessments. It keeps official requirements, customer-defined measures, technical results, evidence freshness, and human reviews distinct.

Build the embedded workspace with `make build`. Frontend development, Docker Chromium tests, and rollback instructions are in [the operator workspace guide](docs/operator-workspace.md).

Every push publishes a private container to `ghcr.io/jaxxstorm/fed` with `latest`
and commit-SHA tags. See [container publishing and registry access](docs/container-publishing.md).

Trust-centre integration is vendor-neutral. No external trust-centre account is required.

**Automatic collection:** saved scope/profile configuration now generates managed
measures and queues supported evidence collection. Open the service's **Collection**
page for runs and scheduling. External agents can submit observation artifacts
directly through MCP. See [automatic collection and ingestion](docs/automatic-collection.md)
for supported checks, permissions, policy settings, and the agent workflow.

**External agents:** use the `gather_evidence` MCP prompt to discover open rules,
read configured evidence sources, upload documents and submit cited results.
See [external-agent evidence workflow](docs/agent-evidence-workflow.md). This flow
works without a collector worker or manually created measures.

## Run locally

Requirements: Go 1.26 or newer and Docker Compose.

```sh
docker compose up -d --wait
cp .env.example .env
set -a
source .env
set +a
make build
./bin/ksi-service migrate
./bin/ksi-service import-catalog testdata/fedramp/rules.json testdata/fedramp/schema.json a89d78477ddea98dc57eca24f5007794c2b4eeb6
KSI_MODE=all ./bin/ksi-service
```

Open <http://127.0.0.1:8080>. Explicit local development identity bypasses browser login and is only permitted on a literal loopback listen address. MCP still requires a scoped bearer token. `.env` is not loaded automatically by the application.

1. Create a service, then open **Settings**.
2. Select the catalog, class, path, evaluation date, and initial/ongoing context. Preview and confirm the profile.
3. Save the assessment scope. This is an explicit customer boundary; repository contents do not automatically establish it.
4. Open a KSI, add customer measures, and attach evidence or run a check.
5. Save an implementation draft with evidence and measure references, then record a human review.
6. Prepare a bundle from **History & exports**. Download HTML or JSON.

The scope editor accepts JSON, for example:

```json
{
  "description": "Production assessment boundary",
  "accounts": [
    {"id": "123456789012", "regions": ["us-east-1"], "clusters": ["production"], "role_arn": "arn:aws:iam::123456789012:role/KSIReadOnly"}
  ],
  "repositories": [
    {"name": "example/infrastructure", "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "paths": [".github/workflows/pulumi.yaml", "projects/eks/Pulumi.yaml"]}
  ]
}
```

Replace the example identifiers and commit with your own. An empty cluster list enumerates EKS clusters in each specified account/region. An empty resource inventory is reported as unknown, not passing.

## Agent access through MCP

Issue an agent token in the service's **Settings**. It is shown once, expires after the selected lifetime (maximum 90 days), and is limited to that service. Revocation accepts the token's SHA-256 hash; issuance/revocation and mutations have audit records. Agent credentials cannot record human reviews.

Configure any compatible MCP client with:

```text
Transport: Streamable HTTP (stateless request/response)
URL:       http://127.0.0.1:8080/mcp
Header:    Authorization: Bearer <token>
```

Use HTTPS for deployed services. Clients must support configured bearer headers; OAuth client discovery is not implemented. The service does not contain an embedded LLM.

Tools:

- `list_services`, `get_assessment_scope`, `save_assessment_scope`, `discover_assessment_scope`
- `list_requirements`, `get_requirement_status`
- `list_evidence`, `get_evidence`, `attach_evidence`
- `run_check`, `get_check_run`
- `save_assessment_draft`, `create_gap`
- `prepare_publication`
- `get_collection_plan`, `collect_now`, `get_collection_batch`, `configure_collection`
- `ingest_observation`, `get_evidence_artifact`

Tools expose their JSON input schemas through MCP discovery. Lists use `limit` (default 25, maximum 100) and `offset`. `next_offset` indicates another page may be available. Mutations use exact revision references; `expected_revision: 0` creates a draft or gap. Retryable check, evidence, and snapshot creation accepts an idempotency key. Reusing a check key for different input is rejected.

Resources:

```text
ksi://services/{service_id}/requirements/{id}
ksi://catalog/{version}/requirements/{id}
```

Percent-encode template variables, including the colon in catalog version identifiers. Read source text as evidence, not instructions. `run_check` returns promptly with a run ID; poll `get_check_run` for coverage and evidence IDs. Agents can draft explanations citing those evidence IDs and revisions. Browser users see the same records immediately.

Suggested agent request:

> Read this service's Class C requirements, run available registered checks against its saved measures, draft explanations with evidence references, and record gaps. Keep missing observations and unresolved applicability explicit.

## Checks and permissions

| Check | Customer measure `expected` values | Required read APIs |
| --- | --- | --- |
| `aws.guardduty.configuration` | `detector`, `EKS_AUDIT_LOGS`, or other feature names | `guardduty:ListDetectors`, `guardduty:GetDetector` |
| `aws.eks.control-plane-logging` | `api`, `audit`, `authenticator`, `controllerManager`, `scheduler` | `eks:ListClusters`, `eks:DescribeCluster` |
| `github.pulumi.workflow-evidence` | Text fragments such as `pulumi` | GitHub repository contents read access |

AWS uses the SDK credential chain (`AWS_PROFILE`, workload identity, or environment credentials). When a scope supplies `role_arn`, the caller needs `sts:AssumeRole` and the role must trust it. The collector verifies the account through STS `GetCallerIdentity`. It does not reuse another vendor's role. GitHub uses `GITHUB_TOKEN` and exact commit/path references. Repository observations are configuration intent, not proof of live deployment or drift execution.

### Automating the assessment boundary

Use one service for the offering (for example, `ignistechnologies`). An agent can
call `list_services`, then `get_assessment_scope` and `save_assessment_scope`.
Saving requires the token's `edit` action and the current `expected_revision`
(zero for creation). Scope changes invalidate existing reviews as before.

Seed the candidate accounts and repositories from your infrastructure inventory.
`discover_assessment_scope` enumerates EKS clusters only in the saved AWS accounts
and regions; it never expands the configured regions. It requires `read` and
`check` access, plus AWS `eks:ListClusters`. It returns a candidate, coverage
errors, and the revision to use when saving. It does not automatically save.
Failed account enumeration preserves the original entries. Discovery limitations
and errors travel with the candidate's `discovery` field when saved.

For local multi-account SSO, configure the server process before starting it:

```sh
export KSI_AWS_ACCOUNT_PROFILES='{"732360055242":"ignistech-production","967721292901":"ignistech-shared","937801277814":"ignistech-mgmt","186241287477":"ignistech-development"}'
```

These are operator-managed profile bindings, not credentials supplied by agents.
Each profile must be authenticated; the account ID is checked by STS. The same
bindings are used by collectors. Discovery currently inventories regions and
EKS only; agents can populate repository entries through `save_assessment_scope`
using exact Git commits. Organization membership, other AWS resource types,
application dependencies and data flows still require additional discovery.

Expected conditions and freshness hours are customer-defined measures, not universal FedRAMP thresholds. Empty expectations produce unknown results. Collection errors preserve successful observations and expose missing coverage. Jobs have a ten-minute deadline, 90-second leases, 60-second collection timeouts, and at most three attempts. A process restart recovers expired leases.

## Catalog updates

The bundled fixtures are official unmodified source files at the documented commit. See [source mapping and provenance](docs/catalog-source.md).

To update, fetch both the official Rules JSON and schema at a chosen full commit SHA and run `import-catalog`. Validation and insertion are atomic. The importer uses an ECMAScript-compatible regex engine because the official schema uses lookaheads. Existing service profiles remain pinned; preview and explicitly select the new catalog in Settings. Historical assessments retain their original profile, scope, measure, and evidence revisions.

Class A references are resolved explicitly. Conditions not resolved from available metadata, conflicting class/subset information, and missing Class D variants remain unresolved. Ongoing transition grace may require the `next_assessment_started` profile value. The original statements, conditions, source payload, and related-rule metadata remain available for review.

## Identity and deployment

Production requires:

```text
DATABASE_URL=postgres://...
KSI_DEV_IDENTITY=false
KSI_LISTEN=0.0.0.0:8080
KSI_PUBLIC_URL=https://ksi.example.com
KSI_OIDC_ISSUER=https://identity.example.com
KSI_OIDC_CLIENT_ID=...
KSI_OIDC_CLIENT_SECRET=...
KSI_ADMIN_SUBJECT=<OIDC sub claim, not email>
KSI_MODE=all
```

Register `https://ksi.example.com/auth/callback` as the OIDC redirect URI. Login uses state, nonce, and PKCE; session cookies are secure/HTTP-only and mutations require CSRF tokens. The configured administrator is provisioned at startup. Additional browser identities and grants currently require administrator-managed database provisioning; there is no user-management UI yet. Permissions are scoped by organization, service, and action. Service ID `*` grants organization-wide permissions.

`KSI_MODE` supports `server`, `worker`, `all`, and `migrate`. A server-only deployment needs a separate worker against the same database and artifact store. Terminate TLS at the ingress/reverse proxy. `/healthz` checks the process and `/readyz` checks database connectivity. Run migrations before starting application processes.

```sh
docker build -t ksi-service:local .
```

The runtime image runs as UID 65532. For filesystem artifact storage, mount a persistent directory writable by that UID at `/data/artifacts`. To use S3-compatible storage, set `KSI_ARTIFACT_BUCKET`, optionally `KSI_ARTIFACT_ENDPOINT`, and the usual AWS credential/region configuration. The artifact role needs `s3:GetObject` and `s3:PutObject` for that bucket. Both server and worker must use the same storage. Objects are content-addressed and verified when read; downloads are authorized against their evidence record.

Operational logs omit tokens, session cookies, artifact bodies, and connection strings. Collection errors are recorded as stable categories in job results; detailed cloud state lives in authorized evidence.

### Backup and rollback

Back up PostgreSQL and artifact storage together. Retain immutable catalog source payloads, all record revisions, jobs, publication attempts, and audit history. Use PostgreSQL `pg_dump`/restore or managed snapshots and object-store backup/versioning according to your deployment policy.

For application rollback, stop workers, deploy a schema-compatible prior image, then resume. Migrations are checksum-verified and forward-only; restore a matching database and artifact backup if reverting a schema is necessary. Do not delete evidence/history to roll back an application release. Keep old publication snapshots and destination mappings when changing trust centres.

## Trust-centre integration

The first release supports generic external evidence URLs and immutable application bundles. No live vendor connector is installed. Adapter capabilities and unsupported-operation behavior are defined in [the vendor-neutral contract](docs/trust-centre-adapters.md) and `internal/integrations/trustcentre.go`.

Downloads are exports, not external publication. A bundle is **not a complete certification package or a schema-validated official Security Decision Record**. Live connectors and official SDR schema mapping are follow-up work.

## Verification

```sh
make generate
make build
make vet
export KSI_TEST_DATABASE_URL="$DATABASE_URL"
make test
```

Integration tests create and drop unique PostgreSQL schemas. Use a development database role with schema-creation permission. Without `KSI_TEST_DATABASE_URL`, database integration tests explicitly skip. Tests cover real catalog fixtures, OIDC login, cross-service authorization, CSRF, token revocation, lease recovery, partial collections, MCP client interoperability, browser review, and immutable exports. AWS and GitHub checks use deterministic HTTP fixtures; live customer accounts have not been assessed.
