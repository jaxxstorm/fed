## Why

The current source directory describes repositories, accounts and clusters but does not establish reusable provider access, leaving operators with errors such as “GitHub access is missing” and no guided way to connect it. Operators and external agents need to connect a provider once, discover its resources, and manage the resulting evidence sources through the same lifecycle.

## What Changes

- Introduce service-owned **connections** for provider identity, authorization, access health and discovery, with child **sources** for distinct repositories, accounts, clusters and Workspace resources.
- Provide web and MCP operations to create, configure, test, renew, disconnect and delete connections; discover, select, configure, refresh, disable and delete sources. Deletion retires configuration while preserving evidence and immutable history.
- Add step-by-step provider setup guides, explicit credential/configuration prerequisites, recoverable authorization status and a connection-first source picker.
- Implement GitHub App installation access for read-only repository discovery and existing GitHub collection, replacing reliance on a process-wide token for newly connected repositories.
- Add Google Workspace OAuth authorization and initial Admin SDK Directory discovery for organizational units and groups, with directory evidence retrieval. Clearly describe the initial coverage in the UI and MCP guides.
- Deliver first-class AWS connections using validated profiles or assume-role access, including identity checks and account/region/EKS discovery, plus Kubernetes connections using contexts or securely supplied kubeconfig credentials with cluster/namespace discovery. Include guided setup, lifecycle management and collector integration for both.
- Support agent-managed access across all four providers; external agents can initiate setup, submit discovered resources and manage selected sources using their own tools. Interactive provider consent remains a browser step when the provider requires it.
- Separate resource discovery from assessment scope selection. Preserve explicit pinned GitHub commits/paths, AWS `us-east-1` limits, provenance and source freshness policy.
- Migrate existing source entries without losing their IDs, revisions or citations; prevent retired sources from being silently recreated by discovery or legacy reconciliation.

## Capabilities

### New Capabilities

- `evidence-connection-lifecycle`: Connection/source ownership, lifecycle, credential isolation, renewal semantics and compatibility migration.
- `provider-connection-access`: First-class GitHub, Google Workspace, AWS and Kubernetes setup, authorization, renewal and read-only server collection using connection credentials.
- `connection-source-discovery`: Paginated server/agent discovery, resource identity, scope-aware selection and stable reconciliation.
- `guided-connection-experience`: Connection-first web UX, setup guides, source picker, access recovery and lifecycle controls.
- `agent-connection-management`: MCP provider guidance, connection/source operations, authorization handoff and agent discovery receipts.

### Modified Capabilities

None in the main spec tree, which is currently empty. Coordinate with the unarchived `enable-agent-evidence-workflow` and `redesign-assessment-experience` contracts and preserve their existing transports. This change does not declare outstanding `automate-evidence-collection` verification complete.

## Impact

- PostgreSQL migrations for connection metadata, secret references, authorization attempts, discovery sessions and source retirement/bindings.
- Shared Go application services, provider adapters, credential resolution in collectors, scope/source validation and artifact provenance.
- Auth capability/token changes; additive MCP tools and generated browser OpenAPI contracts; provider callback routes distinct from application sign-in.
- React connection catalog, guided setup/detail/source-selection journeys and contextual links from existing access errors.
- Deployment configuration for GitHub App registration, Google OAuth client and encrypted credential storage; provider mock tests, migration and MCP/browser parity tests, Docker Chromium accessibility and visual verification.
