## Context

Go/Echo/PostgreSQL own connection lifecycle, encrypted grants, single-use browser handoffs and generation fencing; React and MCP share those services. AWS already supports optional AssumeRole but can use a profile or ambient credentials directly. Workspace already exchanges offline OAuth grants but requires a declared customer ID. GitHub already checks App and user installation access but standard setup requires copying an installation ID. These foundations should be extended, not replaced.

The reference screenshots describe interaction patterns, not permission or region requirements. Fed remains read-only and AWS inspection stays in us-east-1. ECS uses an application task role distinct from its image-pulling execution role. The production browser origin is https://fedramp.ignistech.io behind approved private connectivity.

## Goals / Non-Goals

**Goals:**
- Explicit, verifiable provider delegation suitable for hosted deployments.
- Step-by-step setup with actionable operator versus customer prerequisites, copyable configuration and resumable consent.
- Browser/MCP equivalence, tenant isolation and immutable historical evidence.
- Clear migration for existing connections and deployment documentation for provider registrations.

**Non-Goals:**
- Infrastructure remediation/autofixes, repository writes, automatic customer IAM creation by Fed, new AWS regions or expanded Workspace resource coverage.
- Replacing Fed's application-login OIDC, embedded agent execution, or deploying the separate infrastructure project in this change.
- Copying Oneleet branding, UI, account numbers, external IDs or permission defaults.

## Decisions

### 1. Persist setup independently from evidence and verified connections

Add service-owned `connection_setups` with provider, initiating actor, optional initiating token reference, step, sanitized input, revision, idempotency digest, expiry, resulting connection ID and authorization-attempt references. Use existing credential encryption for staged grants. Add setup-owned authorization attempts referencing the setup rather than a connection, sharing state/PKCE/session authorization helpers with existing attempts. Bind each attempt to setup revision and redirect stage; reconnect finalization additionally checks the expected connection generation. Keep existing connection-bound attempts for legacy flows. A setup may exist before a verified provider tenant; it is not selectable or collectable. Finalization verifies identity and atomically creates/binds a connection under the service lock. Existing connections retain expected revision/generation checks. Completion retries return the same connection.

This avoids placeholder tenant IDs in the current connection contracts. Existing configured connections can enter reauthorization; their verified tenant cannot silently change. Setup expires after 24 hours; browser authorization attempts retain their shorter existing lifetime. Expired setup grants are removed; sanitized audit receipts remain. Abandoned setup never creates active sources.

### 2. Separate deployment readiness from customer consent

Extend provider catalog readiness with missing configuration identifiers, supported flow, required permissions, setup version and operator/customer actions. Never expose app secrets or private keys. Missing one provider configuration must not disable other integrations. Invalid configured secrets remain actionable startup/configuration errors.

Proposed AWS deployment setting `KSI_AWS_FED_ROLE_ARN` identifies the stable application IAM role that customers trust. Verify actual STS caller account/role against that configured principal before rendering executable trust material or collecting through delegated connections. Do not derive a trusted role ARN by lossy string replacement of an STS ARN; handle IAM role paths and assumed-role sessions explicitly. A mismatched runtime fails readiness. Operators provision the ECS task role with AssumeRole permissions for approved target role resources; ECS execution-role permissions remain separate.

Google uses existing `KSI_GOOGLE_CLIENT_ID/SECRET`; GitHub uses existing App ID, slug, client ID/secret and private key settings. Registration happens once per deployment. End users authorize access; they do not upload application secrets.

### 3. AWS delegation is an explicit access strategy

Add an explicit delegated-role strategy rather than inferring it from an optional role ARN. Standard setup: prerequisites → CLI or manual → account ID → generated policy/trust instructions → role ARN → test → discovery. Account IDs are exactly 12 digits; ARN partition/account/role and allowed region are validated. Fed generates a cryptographically random external ID per setup, binds it to service/connection/account, and retains it across retries. Reveal it only through authorized setup material, not ordinary listings, evidence or logs. It is a trust discriminator, not an AWS access key.

Trust policy pins the stable Fed IAM role and `sts:ExternalId`. Target read permissions come from a versioned manifest of enabled collector actions, with resource constraints where supported; do not blindly attach the screenshot's broad SecurityAudit policy. The rendered CLI and manual paths use the same documents. CLI validates the active account before writes, uses safely encoded JSON and validated names, verifies existing roles rather than silently skipping conflicting trust, and does not overwrite unrelated roles. Only the customer's administrator executes provisioning commands. Fed's runtime invokes STS and read operations only.

Every delegated request must successfully AssumeRole with the stored external ID, then verify the target account before reads. Cache/renew sessions by service, connection and generation; a failure never falls back to base credentials. External-ID replacement is explicit, invalidates in-flight access and requires updating target trust and retesting. Local disconnect fences cached sessions; remote role deletion/revocation remains a separate operator action.

Retain explicit legacy profile/environment strategy and agent-managed mode. Existing data is not silently converted. If an optional broker role is wanted later, it is a separate feature; initial deployment uses the Fed application task role directly as the base principal.

### 4. Google consent discovers the customer identity

Create a Workspace setup without demanding a customer ID. Connect redirects to Google's authorization endpoint with offline consent, PKCE and service-bound state. The deployment registers an OAuth web client, enables Admin SDK in its Google Cloud project, configures consent audience/test users or publishing, and sets the exact `/connections/callback/google_workspace` URI. The administrator grants the existing customer/org-unit/group read-only scopes. API enablement belongs to the application project; Workspace administrators may also need to approve API access under their tenant policies.

Resolve `customers/my_customer` using the returned token. An initial verified customer is shown for explicit confirmation before finalization; an expected customer on a reconnect must match. Retain existing refresh tokens if renewal omits a replacement; initial offline consent must yield a renewable grant. Show denial, missing permissions, wrong customer and policy restrictions distinctly. No email-domain-only identity inference. Existing org-unit/group metadata coverage remains explicit; user/mail/Drive collection is not implied.

### 5. GitHub installation is a staged handoff

A Fed-owned GitHub App declares Metadata and Contents read permissions. Standard setup starts a provider installation URL for the configured App slug with opaque setup state; GitHub handles organization and repository choice. The setup callback receives an installation hint, not proof of ownership. Follow with user authorization and a distinct single-use OAuth state, then verify App identity, installation account, suspension, permissions and authorizing-user installation access via GitHub APIs. Existing installations are selectable only from installations verified accessible to that user and App.

Use `/connections/setup/github` for installation return and `/connections/callback/github` for user authorization. Preserve the setup across redirects with independent stage tokens rather than reusing consumed OAuth state. Administrator approval pending is a resumable state, not a successful connection. Do not persist user OAuth credentials once installation verification finishes unless required by a documented renewal operation. Repository installation grants define maximum access; selecting assessment sources/commits/paths remains explicit. Tests and reads revalidate removed/suspended installations; no webhook/public ingress is required for this change.

### 6. Shared setup API and original UI

Expose begin/get/update/render-material/authorize/verify/complete/cancel operations through shared services. Browser mutations require session/CSRF; MCP requires service-scoped connect permission, and source scope changes additionally require edit. MCP returns same-origin handoff links for human consent. Read-only callers receive redacted status, not external IDs or staged credentials. Recheck actor/token authorization at finalization and fence disconnect/delete/reconfigure races.

Provider-specific wizards replace the generic standard-mode forms. Display progress, back/resume, copy/download for AWS material, prerequisites with external documentation links, exact verified identity, partial discovery and recoverable failures. Keep advanced legacy/agent modes explicit. Show concrete next actions when deployment registration is absent. A successful connect proceeds to discovery/selection without claiming evidence collection or control completion. Generated OpenAPI contracts remain the source for TypeScript.

## Risks / Trade-offs

- Private hostname prevents public callbacks → these flows return through the user's browser; require browser private access and server outbound provider access. Do not introduce a public webhook requirement.
- Provider approval/consent behavior varies → preserve pending/cancelled states and validate with mocked redirects plus operator-owned smoke runs.
- Trust material can become stale after operator-role changes → version the manifest, bind it to the configured principal, fail readiness on mismatch and require retesting.
- Existing setup and authorization code overlaps → extend shared services and retain their authorization/generation tests rather than duplicating routes.
- Role propagation is eventually consistent → bounded retry with explicit retryable trust error; never treat failed access as connected.
- Legacy binaries may misinterpret new strategies → require connection-aware rollout and keep workers stopped during rollback to incompatible binaries.

## Migration Plan

1. Capture connection/source/evidence and artifact baselines; inventory existing profile, role, OAuth and imported bindings.
2. Apply additive setup/strategy migrations and backfill explicit legacy strategy without changing tenant IDs, source revisions, grants or evidence.
3. Deploy shared APIs/adapters, then generated browser contracts and wizards. Enable standard delegated setup only when principal/app readiness passes.
4. Publish operator registration instructions, IAM action manifest and approved private callback paths. Provision registrations outside the application.
5. Exercise provider mocks, browser/MCP parity, expired state, replay, wrong tenant, revoke/renew races and preservation checks. Run live smoke journeys when operator registrations are available; report blockers explicitly.
6. Roll back UI to legacy setup only with a backend that understands persisted strategies. Retain additive schema and encryption keys; do not let an old worker reinterpret delegated connections as ambient credentials.

## Open Questions

- Operator must supply the deployed Fed task-role ARN, Google OAuth audience/client registration, GitHub App slug/registration and private browser connectivity. These are deployment inputs, not reasons to block mock-backed implementation.
- Confirm the permission manifest against all enabled collectors during implementation; any added permission must be tied to an existing read operation, with tests and a manifest version change.
