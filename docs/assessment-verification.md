# Reviewed-assessment verification

Change: `add-reviewed-assessment-trust-centre`.

## Automated journeys

`internal/mcp/reviewed_release_test.go` exercises real MCP transport and browser
HTTP handlers against PostgreSQL for:

- `AFC-CSO-INB`: instructions, dated operating evidence, structured narrative,
  browser adoption/internal review, frozen package, public disclosure, scoped
  external access, and MCP discovery of the same release;
- `KSI-CNA-RNT`: contradictory implementation evidence, an access-blocked partial
  observation, an open issue, and publication of an explicitly incomplete report.

These are synthetic fixtures. Reviewer actions are simulated in tests and do not
constitute genuine human acceptance or production release approval.

Additional suites cover inventory-retirement fencing and cached/uncached
projection parity; evidence-request deduplication and service isolation;
dependency invalidation and guidance-review separation; immutable and concurrent
package preparation; offline checksums and oversized/corrupt artifacts;
grant/session expiry and revocation; public projection and withdrawal; recurring
report retries and expired support without changing a published release.

## Checks

Run backend checks with `TZ=UTC` and `KSI_TEST_DATABASE_URL` set:

```sh
go test -race ./... -timeout=10m
```

Frontend checks, from `frontend/`:

```sh
bun run contracts:check
bun run lint
bun run test
bun run build
```

Also run `git diff --check` and
`openspec validate add-reviewed-assessment-trust-centre --strict`.

## Outstanding human and live gates

The full PostgreSQL-backed `go test -race ./... -timeout=10m` suite passed.
The final affected-package race run also passed for assessment, MCP, web, and
storage after the last implementation edits. Frontend contracts, lint, unit
tests, and build passed, as did strict OpenSpec validation and diff whitespace
checks. Both final walkthrough archives passed their offline checksum checks.

Follow [the human walkthrough](assessment-human-acceptance.md) and record the
actual responses and reviewed artifact hashes. The implementation rollout below
keeps trust-centre delivery disabled. Verify the live package, browser/MCP parity, and scoped external
access before an authorized human approves the actual service disclosure and
publishes a release. Preserve the publication receipt.

## Approved implementation deployment

The user explicitly requested commit, push, and deployment. Application commit
`59c83b015746ca616e3640b840247bd7fae38142` was pushed to `main` and deployed after:

- [Application checks](https://github.com/jaxxstorm/fed/actions/runs/39369498871)
  passed, including race tests, Docker browser tests, and container build;
- [Container publication](https://github.com/jaxxstorm/fed/actions/runs/39369499042)
  passed;
- [Pulumi shared update 22](https://app.pulumi.com/ignistech/fedramp/shared/updates/22)
  completed migration, runtime provisioning, catalog import, and rollout.

Deployed image:
`sha256:3d3c50cd9e704e19505bef4dff1e647c85bfef28a24bbe83751dcb0433f9bcf0`.
Web task revision: **17**. Migration task revision: **15**.
Pulumi verified the running digest and private HTTPS readiness; an independent
post-update request to `/readyz` returned **200**. The unconfigured
`/trust/ignis` route returned **404**, and the deployed task retains
`KSI_GUIDANCE_ENABLED=false`.
The ECS stability waiter completed, and a final service read showed one running
task, desired count one, and a single `PRIMARY` deployment with rollout state
`COMPLETED`. The post-rollout web-log error query returned no error events.

Deployment approval does not constitute comprehension or disclosure acceptance.
No trust-centre release was published. Authenticated live MCP verification remains
pending because the temporary client token expired. Guidance activation remains
separately gated.
