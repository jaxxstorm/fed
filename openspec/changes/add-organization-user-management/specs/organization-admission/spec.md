## ADDED Requirements

### Requirement: Single organization configuration
The system SHALL use one application organization with a display name and revisioned admission policy, preserving existing organization and service ownership. It MUST reject ambiguous multiple occupied organizations during migration rather than merge them.

#### Scenario: Existing deployment
- **WHEN** migration finds one occupied organization
- **THEN** settings reference that organization without changing identity IDs or service ownership

#### Scenario: Multiple occupied organizations
- **WHEN** migration preflight finds multiple occupied organizations
- **THEN** activation stops with an actionable report and no automatic merge

### Requirement: Verified-email first-admin bootstrap
The system SHALL support `KSI_ADMIN_EMAIL` for initial admin bootstrap through verified OIDC authentication. It MUST validate issuer, audience, expiry, callback state/nonce, matching normalized email, `email_verified=true` and any configured tenant claim. It SHALL persist bootstrap completion atomically and SHALL NOT re-grant privileges on restart or configuration changes. Simultaneous email and subject bootstrap configuration MUST be rejected.

#### Scenario: First admin signs in
- **WHEN** an unbootstrapped deployment receives a valid callback matching the bootstrap email
- **THEN** it binds the issuer/subject identity, creates one active admin and marks bootstrap complete in one transaction

#### Scenario: Unverified or incorrect identity
- **WHEN** the bootstrap callback has missing/false email verification, a different email or invalid token/tenant claims
- **THEN** no admin or product session is created

#### Scenario: Concurrent bootstrap and restart
- **WHEN** matching callbacks race or the process restarts after bootstrap
- **THEN** one bootstrap succeeds and stored membership remains authoritative without duplicate or restored admin grants

### Requirement: Organization eligibility restrictions
The system SHALL check configured exact email domains and optional signed string organization/tenant claims before creating pending membership or granting access. Domain normalization SHALL be consistent and SHALL NOT match suffix lookalikes, implicit subdomains or provider-specific email aliases. An empty domain list SHALL permit eligible issuer identities to request approval without granting active membership.

#### Scenario: Allowed domain requests access
- **WHEN** a verified first-time identity matches the configured domain and tenant restrictions after bootstrap
- **THEN** the identity is eligible for pending approval but receives no organization role

#### Scenario: Domain or tenant rejection
- **WHEN** a verified identity has a disallowed domain, suffix lookalike, missing required tenant claim or mismatched claim
- **THEN** the callback denies access and does not create a pending request

#### Scenario: Restriction changed
- **WHEN** an admin updates domain policy or deployment tenant-claim settings change
- **THEN** subsequent authenticated access rechecks eligibility and stale tenant-claim sessions require reauthentication

### Requirement: Sign-in-first jailed membership
After bootstrap, the system SHALL create an identity-bound pending membership with no role on an eligible user's first sign-in and issue only a restricted session. Pending users MUST remain jailed until an active admin explicitly approves them. Repeated sign-ins MUST reuse the same pending membership; disabled users MUST NOT be automatically recreated or reactivated.

#### Scenario: New user signs in without pre-addition
- **WHEN** an eligible unknown user successfully authenticates
- **THEN** they see a waiting-for-approval page and have no assessment, evidence, provider, member-list, token or job access

#### Scenario: Pending user retries
- **WHEN** a pending user signs in again or refreshes their status
- **THEN** the existing pending request is reused without changing role or approval state

#### Scenario: Disabled user retries
- **WHEN** a disabled identity signs in again
- **THEN** it remains disabled and cannot bypass denial by creating a new pending record

### Requirement: Stable identity binding
The system SHALL bind membership to issuer/subject and SHALL use normalized verified email for eligibility and collision detection, not automatic identity reassignment. It MUST reject conflicting email/identity bindings without transferring permissions.

#### Scenario: Same email with a different subject
- **WHEN** a new subject presents an email already bound to another identity
- **THEN** authentication fails with a safe conflict outcome and the existing member's access is unchanged

#### Scenario: Existing identity email changes
- **WHEN** a bound subject presents a different verified email
- **THEN** email metadata updates only if unique and policy-compliant, preserving the identity ID and current approval state

### Requirement: Compatible bootstrap migration
The system SHALL preserve existing subject-bound administrators and attribution during migration, support transitional subject bootstrap and stop startup from restoring administrator grants. Known migrated subject-only identities SHALL retain their legacy sign-in path until verified email is attached; new email-based membership MUST require verified email. Ambiguous legacy human grants MUST require explicit resolution without broadening access.

#### Scenario: Existing subject-based administrator
- **WHEN** an existing administrator is migrated
- **THEN** its identity ID is preserved, bootstrap is marked complete and a later verified login attaches unique eligible email metadata

#### Scenario: Subject-only admin and domain policy
- **WHEN** a domain restriction is enabled while an admin lacks verified email
- **THEN** that admin cannot satisfy the policy and the change is rejected if it would leave no eligible active administrator

#### Scenario: Legacy service-limited human
- **WHEN** preflight finds human grants not equivalent to the organization-wide roles
- **THEN** it reports the identity for explicit operator mapping and does not silently widen access
