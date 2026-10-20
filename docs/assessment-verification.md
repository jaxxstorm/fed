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
actual responses and reviewed artifact hashes. Then follow commit/push, hosted CI
and container publication, and the approved Pulumi rollout. Trust-centre delivery
starts disabled. Verify the live package, browser/MCP parity, and scoped external
access before an authorized human approves the actual service disclosure and
publishes a release. Preserve the publication receipt.

Guidance activation remains separately gated. No production rollout or live
publication is asserted by this verification document.
