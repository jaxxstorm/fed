## Context

`EvidenceSource` currently mixes resource identity, scope binding and an agent access hint. `ReconcileEvidenceSources` materializes records from scope, including during MCP reads. This creates useful addresses but no provider authorization. The GitHub collector receives a process-wide token from `checks.Registry`; source configuration cannot supply its access. Google Workspace has no scope/resource adapter.

The Go/Echo/PostgreSQL application owns versioned domain operations and MCP. React/TypeScript uses session-authenticated browser APIs. Agents use their own tools and credentials. Existing source revisions, documentary freshness, pinned commits, observations, reviews and exports must remain resolvable. AWS inspection remains read-only and limited to `us-east-1`.

## Goals / Non-Goals

**Goals:**
- Connect a provider once and discover/select its child sources through either web or MCP.
- Make configure, renew, disconnect, delete and rediscover useful operations with truthful outcomes.
- Make GitHub installation credentials usable by the actual server collector.
- Deliver Google Workspace authorization, Directory resource discovery and usable directory evidence.
- Deliver AWS and Kubernetes as first-class integrations with provider-specific setup, credential resolution, discovery, read-only collection and lifecycle recovery.
- Guide users through provider registration, consent, resource selection and recovery without JSON editing.
- Preserve existing agent-owned access and evidence/history during incremental migration.

**Non-Goals:**
- Embedded LLMs, agent dispatch, infrastructure remediation or a new mandatory queue.
- Full Google Workspace coverage: Gmail, Drive documents, devices and audit-report ingestion are future adapter capabilities, not implied by Directory access.
- GitHub Enterprise Server or arbitrary OAuth endpoints in the first release.
- Automatic compliance passing from authentication or resource discovery.

## Decisions

### 1. Connection ownership, source identity and revisions

Use service-owned connections, authorized through the same organization/service boundary as evidence. A connection has stable ID, provider, display name, access mode (`server_managed`, `server_environment`, `agent_managed`, `manual`), configured provider account identity, credential reference, desired lifecycle, revision and actor/timestamps. Verified provider account identity is recorded separately from a user-entered hint. Do not share credentials implicitly across services.

Store immutable non-secret connection revisions and audit events, plus operational projections for access checks and discovery. Source records remain existing versioned `evidence_source` objects. Add an explicit source-binding table keyed by service/source ID with connection ID, canonical provider resource key and lifecycle. New source revisions include connection/resource metadata; historical payloads are unchanged. Binding an imported source without changing its evidence meaning does not bump its source revision.

Canonical keys are `(service, provider, verified-or-declared tenant, resource kind, provider resource ID)`. GitHub uses numeric repository IDs rather than names; Workspace uses customer ID and Directory resource ID; AWS uses account/region; Kubernetes uses configured cluster identity. A resource has one active source per service. Discovery through another connection returns the existing source and requires an explicit rebind to change its access path. Imported entries whose external IDs are unknown retain legacy keys until a verified or attributed match is applied.

Separate dimensions:
- Lifecycle: `active`, `disconnected`, `deleted`.
- Access: `not_configured`, `authorization_required`, `checking`, `verified`, `partial`, `expired`, `denied`, `error`, `agent_reported`.
- Discovery: `never_run`, `running`, `complete`, `partial`, `failed`.
- Source selection/access: selected or not, enabled/disabled/retired, and latest access observation.

An active configured connection is not automatically verified. Resource counts distinguish discovered resources, selected sources and readable sources. Health is timestamped, attributed and marked stale when its check interval expires.

Alternative: retain flat sources and copy credentials into each. Rejected because renewal, deletion and access explanations would diverge across child resources.

### 2. Lifecycle semantics and history

Configuration changes use expected revisions; operation retries use idempotency keys. Operations lock the connection/source during canonical updates. Access/discovery completion uses a generation fence so results from a deleted, reconfigured or superseded connection cannot reactivate sources.

- **Renew connection:** refresh provider access credentials or initiate reauthorization. GitHub normally mints a new installation token; Google refreshes or requests new consent. Environment/agent connections return exact instructions plus a test/agent-report operation. Credential rotation changes a credential generation, not evidence observation times or source revisions.
- **Refresh source:** read new evidence for supported server adapters, or return a scoped agent investigation instruction. Rediscovery refreshes inventory and is labeled separately. Neither operation silently extends old evidence freshness.
- **Disconnect:** immediately prohibit new server credential use and new connection-backed collection; preserve source identities and historical evidence. A source can later be explicitly rebound to another access mode.
- **Delete connection:** retire it and its child source configuration, fence active work, remove local secrets and retain non-secret tombstones/history. **Delete source:** retire that resource locally, preserve citations and suppress its recreation. This never deletes provider repositories, accounts, groups or clusters.
- Restore/reconnect is explicit and revision-aware. Discovery/reconciliation must respect tombstones, including when assessment scope still names the resource.

Retirement appends source lifecycle revisions when it changes source usability, allowing current evidence-work projections to request refreshed support while historical exports remain frozen. Credential renewal alone must not invalidate reviewed evidence. Local deletion is committed independently of remote token revocation; record revocation results honestly. GitHub installation removal is an explicit provider-side action guided by an uninstall link, not automatically performed for a service-local deletion. Google revocation identifies possible effects on other grants before an explicit remote revoke; local disconnection always works.

### 3. Credential storage and authorization handoffs

Introduce a credential-store interface, with encrypted PostgreSQL storage for connection grants using an operator-supplied versioned encryption key outside the database. Bind authenticated encryption to organization/service/connection/provider. Retain key IDs for controlled rotation; never store plaintext credentials in versioned records, audits, exports, browser bootstrap, MCP results or logs. Provider application secrets/private keys are deployment secret references; the admin setup guide reports readiness and exact configuration names without returning secret values. Missing encryption/application configuration blocks only the relevant provider setup with a concrete next step.

Use provider OAuth libraries where applicable. Persist short-lived, single-use authorization attempts bound to provider, connection, service, expected generation, initiating actor and the authenticated browser completing consent. Derive callbacks from configured public origin, not request Host. Check state, redirect URI, code replay, expiry and applicable PKCE. Clear callback parameters by redirecting to the connection page. Concurrent/late callbacks cannot overwrite a newer grant or resurrect a retired connection.

An MCP-initiated attempt returns a same-origin handoff URL and attempt ID. Opening it requires an authorized browser user; the server then binds the provider redirect to that browser. The external agent polls the attempt outcome and receives metadata, never the authorization code or token. This is normal provider consent, not a separate approval workflow for routine agent edits.

Add an explicit `connect` service capability allowed in agent token issuance. Humans with admin retain it; admins can issue connect-scoped tokens. `read` lists non-secret connections; `connect` manages connection/access/discovery and source retirement; `edit` manages source metadata and assessment scope; selection that changes scope requires both `connect` and `edit`; `check` permits supported collection. Recheck capabilities at execution/completion. Existing tokens keep their current actions and gain no access automatically. Human-only review remains unchanged.

### 4. Initial provider adapters

Use a provider registry describing access modes, setup steps, required permissions, resource kinds, supported evidence operations and error mappings. Shared services call adapters for account verification, discovery pages, token renewal and supported reads. Provider requests have deadlines, bounded pages/body sizes and fixed allowed API hosts; provider redirects never receive credentials for another host.

| Provider | Connection | Discovered sources | First-release evidence behavior |
| --- | --- | --- | --- |
| GitHub.com | GitHub App installation | Accessible repositories | Existing pinned workflow/path reads using installation credentials |
| Google Workspace | Admin-authorized OAuth web client | Directory organizational units and groups | On-demand JSON metadata snapshots for selected units/groups |
| AWS | Validated server profile, assume-role binding or agent-managed access | Connected account, allowed region and EKS clusters in `us-east-1` | Existing AWS collectors with connection-scoped credentials and agent submissions |
| Kubernetes | Validated server context, encrypted kubeconfig credential or agent-managed access | Connected cluster and its namespaces | Existing cluster checks plus namespace-scoped read-only evidence |
| Documents/operational records | Manual or agent-managed connection | Manually declared HTTPS locations | Existing documentary artifact submissions |

**GitHub:** register a GitHub App with Metadata read and Contents read permissions; avoid repository write permissions. Guide the operator through App ID, slug, private-key secret reference, setup/callback URLs and installation repository selection. Bind installation ownership using the authenticated GitHub user authorization and the installations accessible to that user; never trust a callback-supplied installation ID alone. Store verified installation/account IDs. Mint short-lived installation tokens on demand, scoped to selected repositories where supported; cache until expiry margin. Resolve the source's active connection at job execution, not in the static registry. For a connection-bound source, revoked/missing access must not silently fall back to a global token. Unbound legacy jobs can use the explicitly labeled existing token during migration.

Repository discovery includes stable ID, current name, permissions and available default branch. Selection resolves a full commit SHA and asks for paths; do not substitute a moving branch into existing evidence. Renames update display metadata without duplicating sources. Repository selection/access changes appear on the next test/discovery or collection error; webhooks are optional, not required to use this release.

**Google Workspace:** use a Google OAuth web client with offline access and initial read-only scopes `admin.directory.customer.readonly`, `admin.directory.orgunit.readonly` and `admin.directory.group.readonly` (under `https://www.googleapis.com/auth/`). Customer read access is used to resolve `customers.get(my_customer)` and bind the actual customer ID. The guide covers enabling Admin SDK, configuring consent audience/test users as applicable, exact redirect URI and Workspace administrator API access. An email domain/`hd` hint is not proof of customer identity. Enumerate org units and groups, handling permissions per resource kind; if customer identity cannot be verified, do not activate the server-managed connection. Display precisely which Directory capabilities are available; do not label the entire Workspace assessed.

Extend assessment scope with optional Workspace customer/resource selections and extend source locator validation with a canonical customer/kind/resource URI. Existing scope JSON still decodes. Metadata snapshots contain provider/customer/resource identity, observation/retrieval times, ETag when supplied and a content hash; they are immutable JSON artifacts with server-collector provenance. They use existing citation/review semantics and do not imply rule completion. Groups membership/user enumeration and broad document retrieval are not part of the initial Directory adapter.

For server-managed snapshots, share artifact persistence/validation with current submissions but use a trusted internal collector path; callers cannot choose server provenance in MCP payloads. Preserve the 1 MiB artifact limit and return explicit partial/too-large outcomes rather than silent truncation.

**AWS:** the setup guide supports an existing named server-side AWS profile or a role ARN assumed using the server's configured base identity. Show the server principal and the trust-policy/external-ID configuration needed for role access, alongside the read permissions required by enabled collectors. Store role ARN, external-ID secret reference where applicable, expected account and region restrictions as connection configuration; do not ask agents to paste long-lived access keys into MCP. Use STS GetCallerIdentity to validate the actual account before discovery or collection. AssumeRole sessions refresh through the AWS SDK credential provider; expired SSO/profile sessions return the exact environment/profile sign-in instruction. Test renewal and role trust/access-denied cases independently.

AWS discovery identifies the connected account and `us-east-1` resources, including EKS clusters, using paginated read-only APIs. Multiple accounts use separately validated account connections; AWS Organizations-wide enumeration is not a prerequisite or an implied grant to assume roles. The source picker can select account/region and cluster resources in one operation. Resolve AWS credentials per connection at execution rather than relying only on the process-wide account map. Existing profile mappings migrate as environment connections, with access unverified until tested. Configuring IAM trust/policies is an operator setup task described by the guide; the service does not mutate IAM or customer infrastructure.

**Kubernetes:** support a server context reference and a write-only kubeconfig upload through the authenticated web connection setup. Store uploaded credentials in the encrypted credential store and return only context/server identity metadata. Uploaded kubeconfig is parsed as data: reject exec/auth-provider commands, local file references and disabled TLS verification; allow validated embedded certificates or bearer credentials. Existing operator-installed server contexts can retain their trusted local exec authentication, which is never populated from an untrusted upload. Agents can create context-based/agent-managed connections or initiate a browser credential handoff without exposing kubeconfig secrets in MCP responses.

The guide supplies the read-only RBAC needs for namespace enumeration and supported evidence reads, explains where credentials execute, and distinguishes cluster-wide access from namespace-restricted access. Verify API server/TLS identity and configured cluster binding before discovery. Private endpoints are supported through operator-configured cluster endpoint allowlists, with DNS/address validation and redirect restrictions; untrusted kubeconfig URLs cannot become arbitrary server-side fetch targets. A stable configured cluster ID plus namespace UID identifies sources; do not equate same-name namespaces across clusters or recreations.

Extend scope additively with optional Kubernetes cluster/namespace selections. Existing AWS account/region/cluster scope continues to mean its prior cluster-wide boundary unless explicitly narrowed; new namespace-selected connections are limited to the selected namespaces. Standalone Kubernetes clusters use explicit server/cluster scope entries without fabricated AWS account IDs. Update observation target validation and collector planning to enforce both forms. Where a check needs cluster-wide reads, a namespace-only connection reports that missing coverage instead of a pass. Manual namespace entries remain usable when namespace listing is forbidden and a direct authorized read succeeds. Renew updates credentials/context and retests access; discovery never creates, updates or deletes Kubernetes resources.

Alternative: a classic GitHub OAuth App. Rejected for initial repository access because its private-repository `repo` scope is broader than the read-only requirements; GitHub Apps also fit background collection without depending on one user's continuing membership. Google domain-wide delegation is deferred in favor of the normal admin OAuth consent path.

References reviewed for this design:
- https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/deciding-when-to-build-a-github-app
- https://developers.google.com/identity/protocols/oauth2/web-server

### 5. Discovery, selection and external-agent participation

Persist bounded discovery sessions with connection/config generation, actor, execution mode, started/finished timestamps, resource-kind coverage, errors, next cursor and discovered resources. Server-managed adapters enumerate authorized resources using the connection. Agent-managed connections accept paginated discovery submissions with tool/method, observation time, declared tenant, stable resource IDs and limitations. Report these as agent observations; never claim server verification without a server read. Validate service scope and provider resource schemas regardless of submission mode.

Discovery is inventory, not assessment scope mutation or evidence completion. Show eligible, outside-scope, already-selected, missing-access and retired resources. Selection explicitly names resources and expected scope/source revisions; it can atomically add the requested allowed resources to scope when the actor has edit rights, returning the scope delta and resulting revisions. Existing out-of-scope resources remain unselected otherwise. No new approval stage is required. Reject other AWS regions. For GitHub require pinned commit/path selection; Workspace requires the bound customer and supported resource kinds. Do not widen a connection tenant by editing a label.

Use opaque cursors tied to connection, session and generation, default 25 and maximum 100 items. Make page submission and finalization idempotent with content-digest conflict detection. Only a complete, authoritative enumeration of a resource kind can mark unseen items unavailable. Partial scans, throttling and agent limitations preserve last-known inventory and successful pages. Unavailable resources remain visible with evidence history; no automatic deletion. Repeated discovery should not revise an unchanged selected source.

Persist page-level progress, enabling request-driven continuation through UI or MCP without a mandatory background queue. Existing workers may drive the same operations. Avoid holding database transactions during provider requests; commit results with revision/generation checks.

### 6. Shared API and MCP contract

Implement domain operations in a connection service over the existing assessment/auth/storage services, with provider code in a separate package. Browser endpoints under `/api/ui/v1/services/{service}/connections` expose catalog/guides, detail, configuration, authorization attempts, tests, renew/disconnect/delete, discovery pages, source selection and source collection/retirement. Provider callback routes are separately registered, with bound attempts and browser identity rather than MCP bearer authentication. Generate TypeScript contracts before UI integration.

Add MCP tools:
- `list_connection_providers`, `get_connection_setup_instructions`.
- `list_evidence_connections`, `get_evidence_connection`, `configure_evidence_connection`.
- `begin_connection_authorization`, `get_connection_operation`, `test_evidence_connection`, `renew_evidence_connection`.
- `disconnect_evidence_connection`, `delete_evidence_connection`.
- `discover_connection_sources`, `submit_connection_discovery`, `list_connection_sources`, `select_connection_sources`.
- `configure_evidence_source`, `delete_evidence_source`, `refresh_evidence_source`.

Keep existing `list_evidence_sources`, `get_rule_instructions`, `submit_evidence_artifact` and `submit_rule_result` compatible. Add connection/lifecycle/health metadata without removing legacy fields. Default active listings omit retired items, with explicit history filters. Exact revisions remain retrievable. Guides explain whether the next action runs on the server, uses the agent's own credentials, or requires provider browser consent. Responses include operation IDs, status, revisions, applied scope changes and next steps; they never imply an external agent is running.

### 7. Web UX and recovery

Retain Sources navigation and its existing deep links. Its default view becomes **Connections**, with an **All sources** view and connection child-resource views. Show provider/account, access mode, access check time, available/selected source counts and primary action. Clicking Add connection opens a provider catalog and a resumable sequence:

1. Choose provider and access mode.
2. Show prerequisites and deployment-app readiness, with exact callback URLs and copyable configuration instructions.
3. Authorize/connect or configure a profile/context/access hint.
4. Test access and discover resources, retaining partial successes.
5. Search/select sources, choose applicable scope details and evidence freshness.
6. Show connected identity, selected resources and the next collection/agent action.

Resource counts have drill-down filters. Configuration/deletion/renewal work from connection details; source detail distinguishes refresh evidence from rediscover inventory. Delete displays the affected local source count and retained-history semantics in its normal action context. Failed GitHub collection links directly to the relevant GitHub connection (or preselected setup for an unbound legacy repository). Error codes distinguish missing application config, consent cancellation, insufficient permissions, wrong tenant, expired/revoked grant, rate limiting, unavailable resources and unknown provider failure. Successful siblings remain visible.

MCP guidance and web step text derive from the same versioned provider catalog. Initial setup describes provider registration as well as user consent; it must not assume an OAuth client or GitHub App already exists. The agent can do all permitted local management and discovery steps without web form filling; only provider-required interactive authorization is handed to the browser.

### 8. Compatibility and verification

Migration groups legacy sources into explicit imported connections using known identity: GitHub owner, AWS account/profile binding, Kubernetes cluster/context, and document host/manual grouping. These start with unknown access unless a real check establishes it. Existing source IDs/revisions and artifact references are unchanged. An authorized operator can bind an imported GitHub group to a verified installation; match repository IDs before declaring reconciliation complete. Preserve unbound sources for partial matches.

Move legacy reconciliation behind a lifecycle-aware service; read-triggered reconciliation cannot bypass tombstones. Connection selection, source metadata changes, evidence policy changes and scope changes preserve existing revision/freshness invalidation rules. Existing jobs, source URLs, downloads and MCP tools remain usable throughout migration.

Test provider adapters with mock HTTP endpoints including cancellation, rate limits, permission subsets, missing refresh tokens, revoked tokens, pagination and replay. Integration tests use isolated PostgreSQL schemas and prove old citations/exports remain byte-identical, tenant isolation, atomically applied scope changes, credential generation fencing and no tombstone resurrection. End-to-end browser and MCP tests execute equivalent lifecycle journeys; capture and inspect 1440px/768px/390px screenshots in Docker Chromium and run accessibility checks. Live provider smoke tests use operator-supplied app registrations; unavailable external credentials must be reported as blocked verification rather than fabricated success.

## Risks / Trade-offs

- [Provider registration/consent may require tenant administrator access] → Make readiness and exact setup steps available before authorization; keep agent-managed mode usable independently.
- [OAuth storage increases operational responsibility] → Separate encrypted secret storage from immutable records, explicit key rotation, no credential-bearing API output and provider-specific renewal.
- [Discovery looks like assessment completion] → Separate access, inventory, selected scope and evidence-work statuses everywhere.
- [Partial scans appear to remove sources] → Track coverage per resource kind; reconcile absence only after complete scans and retain historical sources.
- [Disconnect races with workers or callbacks] → Check authorization and credential generation at execution and commit; reject late results.
- [Legacy grouping is ambiguous] → Import with declared identity and unknown health, requiring a verified identity match before credential binding.
- [Old binaries ignore retirement tables] → Gate rollout and prohibit active-worker rollback to pre-connection code after lifecycle mutations without restoring a coordinated backup.
- [Workspace scope requests trigger provider policy restrictions] → Request only initial Directory scopes and surface admin-policy/verification requirements in the guide.
- [Kubernetes configuration embeds executable helpers or arbitrary endpoints] → Parse uploads strictly, store secrets outside records, validate TLS/endpoints and retain exec support only for trusted operator-installed contexts.

## Migration Plan

1. Add schema and provider interfaces behind a disabled connection-management feature flag; retain old evidence reads and collection paths.
2. Backfill non-secret imported connections/bindings idempotently; record baseline source/evidence/export counts and hashes.
3. Install application registration and encryption-key configuration; expose provider readiness even when some registrations are missing.
4. Implement shared lifecycle/discovery, provider adapters, collector credential resolution and MCP/browser contracts; verify isolated migration and old-agent compatibility.
5. Enable the connection-first UI and agent tools locally after Docker browser verification. Test real GitHub access and Workspace authorization when registrations are available; retain honest partial/blocked results.
6. Document operational setup, token/key rotation, disconnect/delete behavior and provider revocation limitations. Keep compatibility endpoints and legacy environment credentials explicitly labeled until migration completes.

Before rollout, rollback can disable the feature flag and retain additive schema. After source retirement or credential rebinding, a rollback must use a connection-aware binary or stop workers and restore a coordinated pre-change database/application backup; do not run old reconciliation against new tombstones. Exported evidence remains immutable throughout.

## Open Questions

- Deployment-specific GitHub App ownership, Google OAuth client audience and public callback origin must be supplied during installation; they do not block artifact creation or mock-backed implementation.
- First-release Workspace coverage is deliberately Directory org units/groups. Additional Workspace APIs can be added through the provider registry with separately declared permissions and scope semantics.
- No architectural decision requires a new LLM provider, vendor-specific trust centre or mandatory job queue.
