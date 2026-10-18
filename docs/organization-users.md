# Organization and user management

Fed has one organization. Users **sign in first**, then wait on an approval page
with no assessment/evidence access. An administrator opens **Organization & users**
to approve each request and choose a role, or deny it. There is no pre-add email
workflow and no invitation mail is sent.

## Roles

| Action | Admin | User | Viewer |
| --- | --- | --- | --- |
| Read assessments and evidence | Yes | Yes | Yes |
| Edit evidence/scope/drafts, collect, manage connections | Yes | Yes | No |
| Review assessments and export | Yes | Yes | No |
| Create services, configure profiles, issue agent tokens | Yes | No | No |
| Manage organization policy and users | Yes | No | No |

Roles cover the organization's services. Pending users have no role or product
access; they can only view their own approval status and sign out. Refreshing the
waiting page after approval opens the workspace without another sign-in.

Disabling a user immediately invalidates their sessions. Demotions take effect on
subsequent requests, and an admin's recorded issued tokens are revoked on demotion
or disable. Re-enabling returns a person to pending approval and does not restore
old tokens. At least one active, eligible administrator must remain.

## New installation

Configure the normal OIDC issuer/client/secret/public URL and set:

```text
KSI_ADMIN_EMAIL=administrator@example.com
```

Do not also set `KSI_ADMIN_SUBJECT`. The first admin must sign in through the
configured provider with that email and a boolean `email_verified: true` claim.
The callback still checks issuer, audience, expiry, one-use state, nonce and PKCE.
After the first successful bootstrap, deployment configuration never promotes or
reactivates an account. Identity binding uses issuer and subject, not email alone.
The bootstrap setting can be removed after completion. Maintenance commands
that act as an administrator still require an explicit configured subject/email.

Email comparison trims surrounding whitespace and normalizes case and domain
IDNA. It does not remove plus suffixes or dots. A conflicting identity/email
binding is rejected instead of transferring permissions.

## Organization restrictions

In Organization settings, enter comma-separated exact allowed email domains.
Subdomains are separate entries. An empty list lets users from the configured
issuer **request approval**; it never grants product access automatically.
Rejected domains do not create pending requests. Policy edits recheck current
access and cannot exclude the acting administrator.

For identity providers with a signed string tenant/organization claim, optionally
set both:

```text
KSI_OIDC_ORG_CLAIM=tid
KSI_OIDC_ORG_VALUE=<expected-tenant-id>
```

Use your provider's actual signed claim (for example `tid` or `hd`), not an email
suffix inferred from a request. This is an exact top-level string comparison.
It applies together with domain restrictions and also applies to bootstrap.
Changing these settings forces sessions to reauthenticate. Confirm your provider
emits verified email before enabling new-user registration.

## Existing installation and migration

Migration 014 extends the existing organization and preserves identity IDs,
services, immutable records and evidence attribution. It expires browser sessions.
Refresh runtime database table grants using the existing `provision-runtime`
deployment step after migration.

Preflight rejects multiple occupied organizations and humans without an
organization-wide `*:admin` grant. Such grants need an explicit operator-approved
mapping before activation; do not silently turn service-limited people into
organization-wide users. Review the reported identity IDs and existing grants in
a backup/staging copy first. The migration runs transactionally, so a rejected
mapping leaves the existing database unchanged.

Existing global human admins become active admins with bootstrap complete.
Their subject-based sign-in remains compatible until a verified email is attached;
they cannot satisfy a nonempty domain allowlist without that verified email.
`KSI_ADMIN_SUBJECT` remains a transitional setting. Once the existing admin has a
verified email, replace it with `KSI_ADMIN_EMAIL`; this does not create new grants.
Connection maintenance commands resolve the configured active admin by subject
or verified email rather than invoking bootstrap.

Existing agent and internal automation grants remain service-scoped. Legacy
tokens have no issuer attribution and retain their existing scope/expiry until
explicitly revoked. Newly issued tokens record their issuer so revocation can
follow admin demotion/disable. Inventory/revoke legacy tokens separately if needed.

## Operator recovery

If all valid sign-in paths are blocked, correct issuer/tenant configuration first.
Using the migration database login (the identity-table owner), explicitly recover
an existing human identity:

```text
ksi-service recover-admin <identity-id>
ksi-service recover-admin <identity-id> --clear-domains
```

The second command also clears the email-domain allowlist. It does not bypass the
OIDC issuer or deployment tenant restriction. Runtime DML credentials cannot call
this recovery operation. Recovery preserves identity/history, records an operator
audit event and does not restore revoked tokens. Changing bootstrap environment
values is not a recovery mechanism.

## Audit, rollback and verification

`organization_audit_events` records role/state changes and organization policy
edits transactionally, including database actor attribution for recovery. Structured
`membership.*` and `organization.updated` events contain IDs and revisions, not
email addresses or raw identity claims.

Before rollout, back up the database and compare immutable record counts/hashes
after migration. Verify admin access, a second user's jailed sign-in, approval,
viewer restrictions, demotion/disable and audit visibility on the deployed host.
An old binary does not understand membership gates: after activation, prefer a
roll-forward repair. An image-only downgrade is not an authorization rollback;
any backup-based rollback must explicitly account for grants and newer history.

## Future guests

Invited external evidence guests are a separate future capability. Ordinary
viewers can see the organization's assessments, so they are not a guest substitute.
Guest invitation redemption and evidence-specific grants must be implemented
before a guest category can gain access; unsupported categories currently fail
closed.
