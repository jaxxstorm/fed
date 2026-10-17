# Implementation verification

## Current result

35 of 36 tasks are complete. Task 7.6 remains blocked on live provider
registration, authorized administrator consent and deployed private connectivity.
The sections below record automated and local verification, not live acceptance.

## Implemented foundation

- Approved design amendment: setup-owned authorization attempts, separate from existing connection-bound attempts.
- Setup contracts and guarded state transitions; standard provider strategies and explicit legacy backfill.
- Additive migration `010_connection_setups.sql`; migration replay and preservation of existing connection JSON tested in an isolated PostgreSQL schema.
- Shared setup creation, status, declaration edits, cancellation, revision conflicts and idempotent retries. A bounded startup/periodic cleanup loop removes expired staged secrets.
- Encrypted grant staging with setup/revision-bound authenticated data and atomic connection finalization. Completion rechecks the initiating actor/token; reconnects pin identity and generation. Finalization deletes staged secrets and preserves source/evidence history.
- Setup-owned browser authorization with separate installation/OAuth stages, PKCE, session binding, single-use state and post-network authorization rechecks, wired through browser routes and provider exchanges. Tests cover expiry, replay, actor/session and revision fences.
- Versioned AWS read-policy manifest for existing collectors, scoped to target account resources and us-east-1; explicit action allowlist tests.
- Generated browser contracts include the additive connection access-strategy field.
- Guided AWS CLI/manual delegation, Workspace consent and GitHub installation flows share browser/MCP setup operations and require verified identity confirmation before connection activation.
- AWS runtime principal/path verification, encrypted external-ID material and explicit rotation, generation-scoped session caching and read-only collector journeys are covered by tests.
- Provider readiness and durable recovery codes distinguish missing registration, consent restrictions, missing offline grants and suspended/removed installations.
- Operator registration, private callback, task-role, rotation and rollback instructions are in `docs/provider-applications.md` and `docs/provider-onboarding-rollout.md`.

## Checks passed

With `KSI_TEST_DATABASE_URL` set to the local PostgreSQL test database (isolated test schemas):

```sh
go test -race ./...
go vet ./...
```

Frontend:

```sh
bun run contracts:check
bun run build
bun run lint
bun run test
bun run test:docker
```

OpenSpec:

```sh
openspec validate rework-provider-connection-onboarding --strict
```

Frontend unit tests passed (2), and Docker Chromium passed 51 tests across
1440px, 768px and 390px viewports, including guided provider journeys and
accessibility checks. Representative AWS mobile manual setup and Workspace
desktop confirmation screenshots were inspected.

`docker build -t fed:onboarding-check .` passed, producing local image
`sha256:9c07888d61ca576f99daf0ee785a4899e0d5d98cad2dfc793ebef389b62cb64d`.

## Local migration preservation

Migrations 010–013 were applied to the local database after a fresh backup.
Before/after comparisons preserved all immutable record kinds, existing
connection metadata (excluding the additive strategy field) and all eight
artifact hashes. Backup and comparison records are under:

`/private/var/folders/_9/zz5h76fj6sg42r_w3n0408mw0000gn/T/opencode/provider-onboarding-rollout.1r5lrsyf`

This verifies the schema upgrade; it does not establish that the running local
application has been restarted with this build.

## Live acceptance blockers

- `curl --head https://fedramp.ignistech.io` failed with hostname resolution error from this environment.
- The current shell has no configured `KSI_AWS_FED_ROLE_ARN`, Google client ID/secret, GitHub App/client/key-file settings or `KSI_PUBLIC_URL`.
- Actual AWS customer trust provisioning, Workspace administrator consent and GitHub organization installation must be exercised through the deployed private hostname. Mock provider journeys do not satisfy task 7.6.
- The local image build does not establish remote publication or infrastructure deployment.
