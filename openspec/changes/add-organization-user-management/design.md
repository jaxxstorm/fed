## Context

Fed is a single-tenant Go/Echo/PostgreSQL application with a React browser client and shared browser/MCP assessment operations. Existing tables already include organizations, human/agent identities, permissions, sessions and API tokens. OIDC identities are unique by issuer/subject; `BootstrapAdmin` currently creates or grants the configured subject administrative permissions at startup. Browser bootstrap derives service capabilities from `auth.Authorize`.

This change adds a usable organization membership layer while preserving identity IDs, service ownership, agent scope and immutable evidence attribution. The proposal introduces `organization-admission` and `organization-membership-management`.

## Goals / Non-Goals

**Goals:**
- Verified-email initial admin setup and stable issuer/subject authentication thereafter.
- One configured organization, restricted admission and admin-managed membership.
- Consistent admin/user/viewer permissions across UI, shared services, MCP and background jobs.
- Immediate enforcement of membership changes, durable administrative history and a guest-ready model.
- Eligible users self-register through verified sign-in and remain jailed until administrative approval.

**Non-Goals:**
- Multiple organizations, organization switching, SCIM, arbitrary custom roles or multiple OIDC issuers.
- Guest invitation delivery, magic-link authentication, guest redemption or external evidence sharing in this release.
- Automatic active membership for everyone with an allowed domain; pre-added email membership workflows.

## Decisions

### 1. Extend the existing organization; separate membership from identity

Use the existing organization row and a singleton application-organization configuration referencing it. Add revisioned policy and membership records: organization ID, normalized email (nullable only for migrated subject-only identities), bound identity ID, role (`admin`, `user`, `viewer`, null while awaiting first approval), state (`pending`, `active`, `disabled`), revision and timestamps. New pending memberships are bound to the authenticated identity at first eligible sign-in. Enforce uniqueness of organization/email and organization/bound identity in PostgreSQL. Identity remains the durable attribution key; disabling a member never deletes it.

Keep membership category distinct from role, initially accepting only ordinary members. Future guest principals must use explicit resource grants and must not inherit member role permissions. No guest admission endpoint is exposed now. This avoids treating a future guest as an organization-wide viewer.

Alternative: add roles directly to identities. Rejected because authenticated identity, approval state and future guest grants have distinct lifecycles.

### 2. Sign in, then wait for administrative approval

The organization settings contain a display name and an exact, normalized email-domain allowlist. No wildcard/subdomain or email suffix matching; trim whitespace, normalize domain case/IDNA consistently, use a documented case-insensitive email key without provider-specific dot/plus rewriting. An empty domain list permits identities from the configured issuer to request approval, not to access product data.

Optional deployment settings `KSI_OIDC_ORG_CLAIM` and `KSI_OIDC_ORG_VALUE` restrict the configured issuer to a signed, top-level string tenant/organization claim. Both must be set together. This supports providers with claims such as `tid` or `hd`; it is not a universal cross-provider organization API. Compare exact values from the verified ID token, never query parameters or userinfo supplied by the browser. When configured, the claim restriction and email-domain policy both apply.

First sign-in requires verified email, policy compliance and the configured issuer. Atomically create an identity-bound pending membership with no role for an eligible new user, then issue a restricted session and redirect to a waiting-for-approval page. Repeat sign-ins reuse the same pending record. Only an administrator's explicit approval with an assigned role activates it. No pre-add or invitation is required.

The jailed session can access only its own minimal approval status, the waiting page and logout. It cannot enumerate services, members, organization policy, evidence or provider connections; invoke assessment/MCP operations; issue tokens; or queue work. Use separate authentication and active-membership guards so the waiting page works without weakening normal APIs. Return a stable `membership_pending` response for product API attempts; redirect browser navigation to the waiting page without fetching product bootstrap data. A minimal status endpoint lets the waiting page refresh after approval, reusing the authenticated session while checking live membership. Disabled accounts are denied and never silently become pending again.

A policy update checks the acting administrator remains eligible and at least one active administrator remains eligible. Pending approval also revalidates current policy. Claim/domain failures do not create pending memberships. Before bootstrap completion, only the matching bootstrap principal is admitted; other sign-ins cannot create competing bootstrap candidates.

Alternative: admin-pre-added emails or immediate domain-based access. Rejected in favor of the requested sign-in-first, jailed-until-approved workflow.

### 3. Bootstrap is a one-time claim, not a persistent permission override

Add `KSI_ADMIN_EMAIL`. New deployments accept exactly one of it or transitional `KSI_ADMIN_SUBJECT`; reject ambiguous configuration. Email mode waits for an OIDC callback with matching normalized email and `email_verified=true`. Validate issuer, audience, expiry and callback state/nonce before binding the identity and creating the first active admin. Initial domain policy has no domain restriction; after bootstrap, eligible issuer identities can request approval and the admin can configure narrower domain restrictions. The optional tenant claim restriction applies to bootstrap as well.

Serialize bootstrap with a database lock and persist completion in the same transaction as identity/membership creation. Concurrent callbacks cannot create competing first admins. Once completed, changing environment values or restarting cannot recreate, reactivate or promote administrators. Subsequent logins match issuer/subject, with current verified email/policy checks; email never silently rebinds a membership to another subject. Email and existing-identity collisions fail closed with an actionable admin-facing conflict.

The transitional subject path retains existing login behavior for known migrated subject-bound admins lacking email metadata. At a verified login attach email only if unique and compliant; email bootstrap and newly added memberships never accept unverified email. Subject-only compatibility members cannot satisfy a nonempty domain policy until their email is verified. Require an eligible admin before enabling such a policy.

Alternative: resolve email to subject with a provider admin API at startup. Rejected because it needs provider-specific credentials and does not establish proof of possession through sign-in.

### 4. Role permissions are resolved centrally and checked live

| Capability | Admin | User | Viewer |
| --- | --- | --- | --- |
| Read organization assessment/evidence | Yes | Yes | Yes |
| Edit scope, measures, drafts, evidence | Yes | Yes | No |
| Run collection; manage provider connections | Yes | Yes | No |
| Review assessments; prepare/export evidence | Yes | Yes | No |
| Create services; configure profiles/admin settings | Yes | No | No |
| Manage organization policy and members | Yes | No | No |
| Issue/revoke service-scoped agent tokens | Yes | No | No |

Map user to `read/edit/check/connect/review/export`; viewer to `read`; admin to all existing actions and separate human organization-management authorization. All member access is confined to services of the singleton organization. This adopts the existing combined assessor/reviewer workflow rather than introducing a new separation-of-duties policy.

Human membership state/role becomes authoritative: old permission rows must not retain privileges after demotion. Replace direct human permission-table assumptions in service enumeration, creation, provider operations, API endpoints and background authorization with shared resolution. Check organization membership on every authenticated browser request and authorization boundary; do not cache roles in session cookies. Domain policy changes invalidate eligibility for subsequent requests, requiring sign-in again when necessary. Tenant claim setting changes require existing browser sessions to reauthenticate, using a persisted/session policy fingerprint rather than trusting stale claims.

Agents retain explicit service/action scopes and never gain organization management via an `admin` wildcard. Record token issuer identity for new tokens. Disabling an issuer revokes their issued tokens; role demotion revokes their issued tokens so elevated automation cannot outlive administrative authority. Existing tokens with unknown issuers retain legacy scope/expiry until explicitly revoked, clearly documented in rollout. Background jobs continue to reauthorize at execution/commit boundaries. Administrative change and token revocation occur atomically.

Alternative: materialize per-service role grants. Rejected because it makes newly created services and role changes prone to inconsistent grants.

### 5. Transactional management, concurrency and history

Expose shared membership services and browser endpoints for listing, approving pending users with a role, changing roles, disabling/re-enabling members and editing organization policy. Require active human-admin authority and CSRF protection, with expected revision conflict responses. Recheck authority within the transaction and serialize admin-changing operations on the organization row. Reject last-active-admin demotion/disable, including concurrent operations. Admins can deny a pending request by disabling it. Re-enable preserves the bound identity and returns it to pending for explicit approval; it does not restore revoked tokens. Permit self-demotion/disable only when another eligible active admin remains, then immediately remove privileged UI access.

Write audit events in the same transaction as membership/policy changes with actor, target, before/after role/state and revision. Emit payload-free operational events after commit; use stable denial codes and IDs rather than emails, tokens, ID-token claims or provider errors. Use an organization audit table if the current service-oriented audit schema cannot represent organization-level events cleanly.

Existing in-flight read responses cannot be recalled. Subsequent requests must reject disabled members, and durable writes/jobs must recheck authorization at their existing transaction/fencing boundary.

### 6. Browser experience and contracts

Add an Organization settings entry for admins, with organization policy and a Users table (email/name, role, pending/active/disabled state), highlighting users awaiting approval. Forms support approving a pending user with an explicit role, denying requests, changing roles and disabling/re-enabling, and display revision conflicts and last-admin errors. No add-by-email form is included. The jailed user sees a standalone waiting-for-approval page with their own identity/status, refresh and sign-out, without assessment navigation or data requests. Admin UI visibility follows server-provided organization capabilities. Users/viewers see no management actions, and read-only assessment views hide mutation controls without relying on UI hiding for enforcement.

Extend generated browser contracts and bootstrap data with current membership role and organization-management capabilities. Reuse current styling, ActionForm behavior, accessible labels, keyboard navigation, responsive layout and diagnostic-reference error handling. Authentication denial pages explain that an administrator must grant access without revealing member lists or whether another email exists.

## Risks / Trade-offs

- Identity-provider email claims vary → require verified email for new membership and document provider prerequisites; preserve a narrow migration exception for known subject-bound identities.
- Existing per-service human grants may not map to organization-wide roles → preflight reports ambiguous identities and fails with IDs rather than widening access; require explicit operator resolution before activating the migration.
- Domain/tenant restrictions can lock out admins → validate settings, require an eligible remaining admin, document database-backed operator recovery through a dedicated audited CLI command rather than reusing startup bootstrap.
- Old binaries bypass membership restrictions → do not roll back to an auth-unaware image after membership changes; roll forward or use maintenance mode plus an explicitly reviewed data/config rollback.
- Future guests need different semantics → reserve the category boundary now; reject guest access until a separate invitation and scoped-grant capability is implemented.

## Migration Plan

1. Inventory existing organizations, human identities/grants, tokens and sessions; record immutable row counts/hashes. Fail on multiple occupied organizations rather than merging tenants. Empty installations initialize one organization.
2. Add membership/policy/bootstrap/audit/token-issuer schema without changing identity IDs or evidence records. Backfill known organization-wide human admins as active admins and mark bootstrap complete. Report other human grant patterns for explicit mapping; preserve exact legacy access until the deployment preflight is resolved, with no automatic widening.
3. Deploy centralized role enforcement and the revised callback/startup together. Expire legacy browser sessions to obtain current verified claims; retain the documented subject-only migration exception. Provision-runtime and development-identity paths must follow one-time bootstrap semantics.
4. For new installations set `KSI_ADMIN_EMAIL`; existing installations can replace the subject setting after verifying their active admin email, without granting anyone new access. Document optional tenant claims, domain configuration, issuer-token behavior and audited recovery command usage.
5. Verify admin sign-in, first-sign-in jail, approval/denial and subsequent access, user/viewer permissions, disable/demotion enforcement, token/job fencing and preserved history against real OIDC and deployed hostname. Rollout acceptance remains open until this is exercised.
6. Before policy activation a backup-based rollback is possible; afterwards treat rollback as an authorization migration, not a simple image downgrade. Never delete new membership history or silently restore old grants.

## Open Questions

No blocking product questions. Implementation must confirm the deployed provider's verified-email and optional organization claim shape and inventory legacy grants before rollout. Guest invitation delivery, expiry, acceptance and evidence-specific grants will be specified in a future change.
