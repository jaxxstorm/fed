## ADDED Requirements

### Requirement: Administrative approval and membership lifecycle
Active human admins SHALL list pending/active/disabled members, approve pending users with an explicit admin/user/viewer role, deny requests by disabling them, change roles and disable/re-enable members. There SHALL be no requirement to pre-add emails. Re-enabling SHALL return a disabled member to pending for explicit approval, preserve identity and leave revoked tokens revoked. Mutations MUST use expected revisions and transactional authority checks.

#### Scenario: Approve pending viewer
- **WHEN** an admin approves a pending membership as viewer at its current revision
- **THEN** it becomes active with viewer permissions and its existing restricted session gains only those permissions on the next live membership check

#### Scenario: Deny and reconsider
- **WHEN** an admin denies a request and later re-enables it
- **THEN** it moves from pending to disabled to pending and receives no product access until approved again

#### Scenario: Concurrent edits
- **WHEN** two admins submit different changes using the same membership revision
- **THEN** only one succeeds and the other receives a revision conflict without overwriting it

### Requirement: Central organization role authorization
Authorization SHALL apply live active membership and restrict all human roles to services in the application organization. Admins SHALL have all current service actions and human organization management. Users SHALL have read/edit/check/connect/review/export but not admin, service creation, organization management or token issuance. Viewers SHALL have read only. Legacy human grants MUST NOT override membership state or assigned role. Direct browser and MCP/shared-service calls MUST enforce the same rules.

#### Scenario: User works on assessments
- **WHEN** an active user edits evidence, runs collection, manages a connection, reviews an assessment or exports
- **THEN** the corresponding authorized service operation is available, while profile/admin and membership operations are forbidden

#### Scenario: Viewer attempts mutation
- **WHEN** an active viewer directly calls a write, review, collection, export or token endpoint
- **THEN** the server denies it regardless of client UI or old permission rows

#### Scenario: Organization boundary
- **WHEN** a human attempts to access a service belonging to another organization
- **THEN** authorization denies access even if the human is an admin

### Requirement: Pending session isolation
Pending sessions SHALL allow only their own minimal approval status, the waiting page and logout. Product APIs SHALL return a stable `membership_pending` denial and browser product navigation SHALL redirect to the waiting page. Pending sessions MUST NOT expose service/evidence/organization-policy/member data, create tokens, invoke product tools or enqueue jobs.

#### Scenario: Direct API bypass attempt
- **WHEN** a jailed user requests normal bootstrap, a service API or an MCP product operation
- **THEN** no product data or mutation is returned and approval remains required

#### Scenario: Waiting page
- **WHEN** a pending user opens the application
- **THEN** they see their own waiting status with refresh and sign-out controls without assessment navigation or product data fetching

### Requirement: Last administrator protection
Admin-changing operations SHALL serialize on organization state and MUST preserve at least one eligible active admin. Pending admins SHALL NOT count. Policy edits and approvals SHALL validate current eligibility.

#### Scenario: Concurrent last-admin removal
- **WHEN** two active admins concurrently attempt to disable or demote each other
- **THEN** at least one remains eligible and active, with the unsafe operation rejected

#### Scenario: Stale approval eligibility
- **WHEN** a pending user's email no longer meets current policy when approval is submitted
- **THEN** approval is rejected and the user gains no product access

### Requirement: Session and agent authority changes
The system SHALL enforce membership state/role changes on subsequent requests and recheck durable operations at authorization/commit boundaries. Disabling or demoting an admin SHALL atomically revoke tokens recorded as issued by that admin. New tokens SHALL record issuer identity and remain limited to explicit service/action scopes. Agents MUST NOT gain human organization administration. Legacy issuer-unknown tokens SHALL retain existing scope/expiry until explicitly revoked and SHALL be identified in migration guidance.

#### Scenario: Admin demoted with existing session and tokens
- **WHEN** another admin demotes an administrator
- **THEN** its next request uses the new role, its attributable agent tokens are revoked and queued work cannot use revoked authorization

#### Scenario: Disabled member with running work
- **WHEN** a member is disabled before an operation's next durable authorization boundary
- **THEN** the operation fails authorization without committing unauthorized evidence changes

#### Scenario: Agent attempts member approval
- **WHEN** a service-scoped agent attempts an organization membership action
- **THEN** the action is denied regardless of service grants

### Requirement: Administrative UI and observable history
The browser SHALL provide admin-only organization settings and membership management with pending requests highlighted, explicit role selection, approval/denial, role changes, disable/re-enable and actionable conflict/last-admin errors. It SHALL preserve current styling and responsive/accessibility patterns. Membership/policy changes SHALL produce transactional audit records and post-commit structured events; operational logs MUST exclude emails, tokens and raw identity claims.

#### Scenario: Admin reviews access requests
- **WHEN** an admin opens Users
- **THEN** pending signed-in users are identifiable by email/name and can be approved with an explicit role or denied without an add-by-email workflow

#### Scenario: Failed transaction
- **WHEN** an approval or policy transaction fails
- **THEN** no successful mutation audit/event is recorded and the UI shows a safe actionable error

#### Scenario: Accessible role-limited UI
- **WHEN** users interact on desktop or mobile with keyboard or assistive technology
- **THEN** management forms and waiting status are labeled and navigable, and unavailable role actions are not offered

### Requirement: Guest-ready access boundary
The system SHALL keep ordinary membership roles separate from future guest identity/resource-grant concepts. This release MUST NOT grant external evidence access via a guest category or treat a guest as a viewer. Future invitations SHALL require a separate explicitly implemented acceptance and scoped-grant workflow.

#### Scenario: Guest principal without implemented grants
- **WHEN** a guest or unsupported membership category reaches authorization
- **THEN** organization and evidence access is denied rather than inherited from viewer defaults

### Requirement: Migration and operator recovery
Membership rollout SHALL preserve immutable evidence, audit attribution, identity IDs and service ownership. It SHALL include a documented, explicit audited operator recovery command, and MUST NOT use repeated bootstrap as recovery. Rollback documentation SHALL address old binaries lacking membership enforcement.

#### Scenario: Recover from provider policy lockout
- **WHEN** an operator invokes the documented recovery command with database authority
- **THEN** the explicit administrator/policy repair is audited without resetting evidence or restoring unrelated grants

#### Scenario: Rollout verification
- **WHEN** migration is verified
- **THEN** history counts/hashes and identity ownership remain unchanged, and live sign-in, jail, approval and role restrictions are recorded before rollout acceptance is marked complete
