## 1. Membership schema and migration

- [x] 1.1 Inventory auth/permission queries, bootstrap/provisioning paths and existing organizations/human grants; define migration preflight and report ambiguous mappings without widening access.
- [x] 1.2 Add singleton organization policy, revisioned identity-bound memberships with nullable pending role, bootstrap completion, organization audit history and token issuer metadata using additive migrations.
- [x] 1.3 Backfill existing organization-wide admins preserving identity IDs; reject multiple occupied organizations and unresolved legacy grants; expire legacy browser sessions for reauthentication.
- [x] 1.4 Verify fresh/repeated migrations, uniqueness, pending role constraints, legacy compatibility and preservation of immutable record counts/hashes and service ownership.

## 2. OIDC admission and one-time bootstrap

- [x] 2.1 Implement normalized email/domain validation, exact domain allowlists and paired optional signed tenant-claim configuration with sanitized validation errors.
- [x] 2.2 Add KSI_ADMIN_EMAIL and transitional subject configuration; remove recurring startup admin grants and persist one-time bootstrap across server/provision-runtime/development paths.
- [x] 2.3 Implement verified-email callback bootstrap with state/nonce and token validation, transactional concurrency protection, stable issuer/subject binding and conflict handling.
- [x] 2.4 Implement first eligible sign-in creating one pending membership without a role; reuse pending records on repeat login and deny disabled or policy-ineligible identities.
- [x] 2.5 Implement migrated subject-only compatibility, verified email attachment/change rules and tenant-policy session fingerprint invalidation.
- [x] 2.6 Test bootstrap races/restarts, incorrect/unverified claims, domain lookalikes, email collisions, changed emails, pending repeat login and disabled re-login with a test OIDC provider.

## 3. Jailed sessions and centralized authorization

- [x] 3.1 Separate authenticated session handling from active membership guards; expose only own minimal approval status and logout to pending sessions, with stable membership_pending API errors and waiting-page redirects.
- [x] 3.2 Centralize organization-scoped admin/user/viewer resolution and replace human permission-table assumptions in service listing/creation, assessment, connection and browser capability paths.
- [x] 3.3 Enforce live membership and current policy on sessions and shared operation authorization; ensure old human grants cannot bypass pending/disabled state or demotion.
- [x] 3.4 Preserve explicit agent service/action scopes, block agent organization management and atomically revoke attributable issued tokens on admin demotion/disable; recheck queued/running work authorization.
- [x] 3.5 Test jailed direct API/bootstrap/MCP bypass attempts, all role actions, cross-organization access, existing-session role changes and token/job revocation boundaries.

## 4. Administrative membership services and API

- [x] 4.1 Implement admin-only organization policy reads/updates and paginated membership listing, with expected revisions and current eligibility validation.
- [x] 4.2 Implement approval with explicit role, denial/disable, role changes and re-enable-to-pending; preserve identities and never restore revoked tokens implicitly.
- [x] 4.3 Serialize admin-changing operations and enforce last-eligible-active-admin protection across concurrent mutations, self-removal and domain-policy changes.
- [x] 4.4 Add transactional organization audit records and post-commit structured events with safe IDs/codes; include pending-request creation and approval/denial without logging email or raw claims.
- [x] 4.5 Wire CSRF-protected browser endpoints, safe conflict/denial responses and generated contracts including minimal approval status and organization capabilities.
- [x] 4.6 Test stale revisions, concurrent approvals/admin removals, policy changes before approval, rejected transactions and unsupported guest category denial.

## 5. Browser approval and user management experience

- [x] 5.1 Build standalone waiting-for-approval screen with own status, refresh and sign-out; avoid assessment navigation/bootstrap requests and transition to product access after approval.
- [x] 5.2 Add admin Organization settings and Users table highlighting pending signed-in users, with approve-and-assign-role, deny, change-role and disable/re-enable forms; include no pre-add email flow.
- [x] 5.3 Add domain policy editing and optional tenant restriction explanation, revision/last-admin feedback and immediate UI response to own demotion or disable.
- [x] 5.4 Apply role-driven navigation/actions and read-only viewer behavior consistently while preserving Fed styling and existing one-time token disclosure behavior.
- [x] 5.5 Add responsive/accessibility browser coverage for sign-in jail, direct navigation denial, admin approval/denial, post-approval access, all roles and conflict handling.

## 6. Recovery, documentation and verification

- [x] 6.1 Implement explicit audited database-authorized recovery command for administrator/policy repair without resetting bootstrap, evidence or unrelated grants; test recovery outcomes.
- [x] 6.2 Document role matrix, email bootstrap, provider claim requirements, sign-in-first approval, domain restrictions, legacy token/grant migration, guest extension boundary and safe rollback/recovery.
- [x] 6.3 Run PostgreSQL-backed race/integration tests, go vet, generated contract checks, frontend build/lint/unit checks and relevant browser workflows; record results.
- [x] 6.4 Build the production container and verify fresh and existing-installation migrations plus jail/approval behavior with a test identity provider; run strict OpenSpec validation.
- [ ] 6.5 Verify deployed OIDC email bootstrap or migrated admin, second-user sign-in jail, approval/denial, role enforcement and durable audit/log visibility; record any provider/deployment blockers without marking live acceptance complete.
