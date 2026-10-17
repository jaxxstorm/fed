# Provider onboarding rollout

## Production registration

Set `KSI_PUBLIC_URL=https://fedramp.ignistech.io`. Register these exact URLs:

- Google OAuth redirect: `https://fedramp.ignistech.io/connections/callback/google_workspace`
- GitHub OAuth callback: `https://fedramp.ignistech.io/connections/callback/github`
- GitHub installation setup: `https://fedramp.ignistech.io/connections/setup/github`

These redirects are followed by the administrator's browser. The browser needs
approved private connectivity to the Fed hostname; Cloudflare DNS alone does not
provide it. No public webhook or public ALB is required. The server needs outbound
HTTPS to Google authorization/token/Admin SDK endpoints, GitHub API/token endpoints,
and AWS STS/IAM plus enabled regional APIs. See `provider-applications.md` for
registration settings and the task-role/execution-role distinction.

## Migrations and rollout

1. Back up PostgreSQL and immutable artifacts, recording evidence/source and frozen
   export baselines. Stop workers while upgrading schema and runtime together.
2. Run `ksi-service migrate`. Migrations 010–013 add setup state, encrypted staged
   grants/material, short-lived browser handoffs and sanitized recovery codes.
   Existing connection metadata JSON and source/evidence revisions are untouched;
   existing connections receive explicit `legacy` credential semantics.
3. Deploy connection-aware web and worker binaries with the same encryption keyring.
   Confirm registration readiness and AWS task-role identity before customer setup.
4. Exercise consent/role setup, identity confirmation, discovery, explicit source
   selection and collection independently. Compare retained immutable baselines.
5. Verify expired setups are swept: cleanup processes up to 100 expired setups per
   minute per process, with cross-replica service locks. Expired setups cannot be used
   even before the cleanup sweep removes their encrypted payloads.

## Keys and rollback

Keep previous encryption keys while rotating connection credentials and until all
setup grants/material and browser verifiers encrypted under them are completed,
cancelled or expired and cleaned up. Setups last 24 hours; browser attempts and
handoffs last at most 10 minutes. The existing credential rotation command does
not re-encrypt staged setup records, so do not remove an old key merely because
connection credential rotation completed. Never print keyrings or grants in logs.

Roll back the UI only alongside a backend that understands delegated strategies.
Keep additive tables and encryption keys. Never run an older worker that could
interpret a delegated connection as ambient/profile access. Stop workers if a
connection-aware rollback build is unavailable. Restore historical evidence only
from a deliberate recovery procedure, not as part of ordinary UI rollback.

## Live acceptance record

Use an operator-owned test account/organization and the private production hostname.
Record verified provider identity, exact selected sources, collection outcome and
retained historical hashes. AWS trust changes are executed by the customer
administrator; Workspace and GitHub approval require authorized humans. Mocked
journeys do not substitute for this deployment acceptance.
