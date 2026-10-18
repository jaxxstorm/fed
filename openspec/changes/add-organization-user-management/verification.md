# Organization membership verification

## Implementation result

30/31 tasks are complete. Task 6.5 remains blocked on deployed acceptance.

Implementation and local verification cover the sign-in-first workflow: verified
OIDC identity → pending/jailed session → explicit admin approval with a role.
There is no pre-added email workflow. Organization settings, member management,
role enforcement, token revocation, operator recovery, browser contracts and
responsive UI are implemented.

The original fixture regressions are resolved. Session fixtures now carry the
current tenant-policy fingerprint; second-admin fixtures use admission/approval
instead of repeatedly granting privileges through bootstrap. Read-only snapshots
remain compatible while writable authorization transactions retain row locks to
fence membership changes and token revocation through commit.

## Checks passed

- Full PostgreSQL-backed `go test -race ./...` passed using isolated schemas.
- Subsequent targeted race checks for auth/web/config/entrypoint passed after the
  final bootstrap-setting-removal and sign-out refinements.
- `go vet ./...` passed.
- Frontend generated contracts check, TypeScript/production build, Biome lint and
  Vitest checks passed.
- 63 Chromium browser checks passed across desktop/tablet/mobile with the actual
  Go waiting page served from an isolated fixture; no skipped jailed-page tests.
- Three additional denial/re-enable/self-demotion browser checks passed, for 66
  passing browser scenarios across the two runs. Existing token disclosure and
  assessment/provider workflows remain covered.
- Axe checks passed for organization management and the real jailed page;
  keyboard focus, viewport fit and absence of jailed product API fetching were
  verified.
- Runtime DML permissions and rejection of owner-only recovery passed in a
  disposable PostgreSQL container using `TestProvisionRuntimePermissions`.
- Strict OpenSpec validation and `git diff --check` passed.

## Auth and data coverage

- Real signed test-OIDC tokens exercise verified-email bootstrap, wrong tenant,
  unverified email, nonce failure, pending sign-in and existing-session approval.
- Current tenant-policy fingerprint mismatch forces reauthentication; disabled
  identities cannot re-enter through login.
- Concurrent bootstrap produces one admin; changing bootstrap settings does not
  promote another user. Completed deployments can omit bootstrap configuration.
- Repeat pending login, changed email, email collision, exact-domain matching,
  stale revisions, policy change before approval, concurrent admin removal,
  last-admin protection and explicit recovery are tested.
- HTTP CSRF and pending direct-API denial, in-process MCP product denial, all
  role actions, organization isolation, old grant non-escalation and revocation
  of newly attributed tokens are tested. Existing worker fencing tests pass.
- Migration tests cover empty/repeated upgrades, existing administrator identity
  preservation, session expiry, unchanged immutable record digest and atomic
  rejection of ambiguous organization/grant mappings.

## Container and isolated application checks

Production container builds passed, including embedded React assets. Container
smoke tests used both fresh schemas and schemas created by `fed:logging-check`
(migrations through 013) with a legacy admin and immutable record. Migration 014
and repeat migration, unchanged record digest, Organization UI, approval API,
pending HTTP denial, waiting page and explicit recovery all passed.
Final local image: `fed:organization-check`,
`sha256:751e3dcfe633f3555cbc3589943d78fec7181d7c34fd6d034478aae15fcc7a6b`.

The signed test-OIDC journey runs in Go integration tests; container smoke tests
use loopback-only development identity. These are complementary local checks,
not evidence of deployed OIDC acceptance.

The real waiting-page browser fixture and recovery smoke diagnostics are at:
`/private/var/folders/_9/zz5h76fj6sg42r_w3n0408mw0000gn/T/opencode/fed-membership-mg19trnx`.
Temporary schemas/containers/processes were removed. No migration was applied to
the existing local application schema, and no production resources were changed.

## Remaining deployed acceptance: task 6.5

The new implementation is local and uncommitted; it has not been published or
deployed. Live acceptance requires deployment of this image/migration, refreshed
runtime table grants and two real OIDC users on the private application hostname.
Verify migrated/initial admin access, second-user jail, approval/denial, role
enforcement, and durable audit/CloudWatch visibility there. Keep 6.5 open until
those observations are recorded; local identity-provider tests do not replace it.

Deployment and recovery instructions: `docs/organization-users.md`.
