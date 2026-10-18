# Authorization and migration inventory

## Existing boundaries

- `internal/auth/identity.go`: `BootstrapAdmin` currently upserts a human and restores `*:admin` on every call. `Authorize` validates stored identity kind/organization, service ownership, token scope and permission rows. Token issuance has durable actor attribution in audit events but no issuer column.
- `internal/auth/browser.go`: callback already verifies OIDC token, nonce and one-use state/PKCE. It currently accepts only pre-existing human subjects. `Protect` validates sessions/CSRF but has no membership gate. Split restricted-session authentication from active-membership enforcement here.
- `cmd/ksi-service/main.go`: both development and production server bootstrap invoke `BootstrapAdmin`. `cmd/ksi-service/connections.go` also invokes it for maintenance; maintenance must resolve an existing authorized admin rather than restore grants.
- `internal/assessment/store.go`: creation calls shared admin authorization; service enumeration filters candidates with shared read authorization. Central membership resolution must cover both and return a pending denial before enumeration for jailed actors.
- `internal/assessment/collection.go`: scheduled collection creates `automation` identities with explicit per-service read/edit/check grants. Preserve this distinct internal identity kind; it is neither an organization member nor a guest.
- `internal/checks/runner.go`: jobs reauthorize stored actors/token hashes and fence evidence commits. Preserve this behavior when membership and token revocation are added.
- `cmd/ksi-service/runtime_role.go`: runtime grants are DML-only and must be refreshed after migrations for new tables. Existing tables already support organizations and immutable actor references.
- `internal/testutil/database.go`: tests migrate isolated schemas twice. Existing test fixtures sometimes call bootstrap to create a second admin; replace that fixture behavior with explicit membership approval when bootstrap becomes one-time.

## Preflight rules

1. Count occupied organizations (any identities or services). Reject more than one. Use the sole occupied organization, or the sole existing empty organization; reject ambiguous empty organization selection rather than guess. An empty database initializes `default`.
2. Backfill humans with an organization-wide `*:admin` grant as active admins. Other human grant sets require explicit mapping before migration activation; report identity IDs only. Do not convert service-limited roles to organization-wide access automatically.
3. Preserve agent and automation permissions exactly. Add nullable issuer attribution for legacy tokens; new token issuance supplies it. No inferred issuer mapping is required for legacy tokens.
4. Preserve identity IDs, organization/service ownership, evidence records and audit attribution. Expire browser sessions, not identity/history records. Serialize migration with the existing migration advisory lock.

## Local read-only inventory

The development database contains one occupied organization (`default`), one human with only `*:admin`, six total identities and one service. Its human grants have an unambiguous admin mapping. This is not a production inventory; deployed migration must run its own preflight.
