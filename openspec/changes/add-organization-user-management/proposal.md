## Why

Fed currently requires an OIDC subject to provision its administrator and has no practical organization or human-user management flow. Operators need email-based bootstrap, organization admission restrictions and an admin interface to manage access without changing deployment configuration for every person.

## What Changes

- Bootstrap the first administrator through a verified OIDC email, binding subsequent authentication to issuer and subject.
- Introduce a single-organization settings and membership model with explicit email-domain restrictions and optional configured identity-provider organization/tenant claims.
- Let eligible users sign in and enter a jailed, pending-approval state with no assessment/evidence access; let administrators approve them as admins/users/viewers, change roles and disable access.
- Enforce current membership and role permissions on existing sessions and shared service operations, retaining service-scoped agent authorization.
- Preserve existing identities, record attribution and compatible subject-based deployments through a documented migration path; stop re-granting administrator access on every startup.
- Design separate membership and resource-grant concepts for future invited email guests, without implementing external invitations or granting guests organization-wide access in this change.
- Add accessible organization/user-management screens, audit events, structured operational logs and deployment guidance.

## Capabilities

### New Capabilities
- `organization-admission`: Single-organization configuration, verified-email bootstrap, organization login restrictions and stable OIDC identity binding.
- `organization-membership-management`: Administrative membership lifecycle, role enforcement, session revocation and guest-ready boundaries.

### Modified Capabilities

None. `openspec/specs/` currently contains no main specifications; existing auth and evidence behavior is considered explicitly in migration and compatibility requirements.

## Impact

- Auth/configuration/startup, OIDC callback/session handling, permission resolution and scoped agent token lifecycle.
- Additive PostgreSQL migrations for organization policy, membership metadata and bootstrap completion, preserving existing identity IDs and immutable history.
- Browser management endpoints, generated API contracts, navigation/capabilities and React organization settings UI.
- ECS configuration gains an email-based bootstrap option and optional provider tenant restriction; subject-based bootstrap remains a transitional compatibility option.
- Operational/audit events for admission and membership changes. Future guest invitations and evidence-specific grants remain separate from ordinary organization roles.
