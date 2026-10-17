# Provider onboarding compatibility baseline

## Existing paths and contracts

- `internal/connections/service.go` owns service-authorized, revision-aware configuration. `contracts.go` separates metadata revision, credential generation, access mode and provider-verified tenant identity.
- `authorization_attempts.go` binds consent to a persisted connection, generation, initiating actor/token and completing browser session. State is consumed before code exchange. `credentials.go` encrypts credentials with connection/generation-bound authenticated data.
- `internal/web/connection_authorization.go` resolves the connection before redirecting. AWS currently accepts an external ID upload; Workspace and GitHub both expect declared tenant configuration before consent.
- `internal/providers/aws.go` verifies the base STS identity, optionally assumes a role, then checks the target account. The new delegated strategy must make role assumption mandatory without reinterpreting legacy profile bindings.
- `internal/providers/workspace.go` resolves `customers/my_customer`, requires an initial offline grant, and currently rejects an identity differing from the preconfigured customer.
- `internal/providers/github.go` verifies App/user installation access; the new staged install return must treat installation IDs as hints rather than authorization.
- `internal/checks/connection_collectors.go` resolves bound credentials for AWS and GitHub and records collection-access fences. Only unbound legacy sources use legacy credential paths. Imported EKS and standalone Kubernetes retain separate configuration paths.
- The five unarchived `add-evidence-connections` specs remain the compatibility contract: immutable history and retirement; service-scoped connect/edit permissions and agent handoffs; bounded discovery and explicit selection; honest UI health; provider identity verification and renewal. New setup must not bypass these contracts or imply completion of the predecessor's outstanding live acceptance task.

## Observed local baseline

Captured before onboarding implementation for service `dc0c7bef-7a80-4233-9ff2-cb69c6b0554f`. The local database has no snapshot/publication records; frozen-export coverage therefore uses the existing isolated fixture below.

| Data | Rows/revisions | SHA-256 |
| --- | ---: | --- |
| Evidence | 785 | `8e4442839072fb9aa74d1e032ced91f2941504831f1536bd160492ea8528c3d6` |
| Evidence sources | 58 | `359bfa95b6a23fa3d1396c443cfef5d8b109008e1aa48e3964abef34fd0b1566` |
| Connections | 9 | `96ee21a695a50160c1a59164c7aaca85c5cf21e40c3c95cf834a226ad7afa266` |
| Connection revisions | 9 | `e6ff43c855952f58da3b10e977044e99d837796094fa46d7b2990b0175f17158` |
| Source bindings | 49 | `1345d8dd009a76e00626ab3d6280526627c91e3e732be005383b4da0c34c498b` |

Evidence/source digests use the query in `../add-evidence-connections/compatibility-baseline.md`. Connection-table digests hash UTF-8 `string_agg(to_jsonb(row)::text, E'\n' ORDER BY service_id,id)`; revisions use `(service_id,connection_id,revision)` and bindings use `(service_id,source_id)` ordering. Preserve the database timezone when comparing timestamps. Operational health may legitimately change; immutable revisions and evidence require exact preservation.

The source revision count has increased from the predecessor's 49 to 58 before this change. Use this captured baseline rather than treating those existing revisions as onboarding changes.

All eight files in `.data/artifacts` hash to their content-addressed filenames:

```text
02735c21e0b19cee2e22e62ddb19d714e22fcc33a7939c14b871777516d72fba
1b8f4d6d1a178fda549d0ce8b57785d0c93187796d77c4a2d420acecd3da92f4
8bf53a5585f44eaa87b47bcf08665cf9c39c9e6aa87b31256c38c920efc66e2c
d2a24d3f4ae271306a888e63a30b137d23efb1abff20925baf7afa5b649eb273
da689790dd6f54775f4497b1afcd8e6396bb5082660f36eb50606d0d4daff92a
e2d3de267112cf6c24f594e5043b7c5fa183da960ab6651ab3f8e85719b37847
e925ef9be665d3c5d686782d2fe66d06dde939357477f1683180705d3fdf9c65
f80d723a3a62bb6b847c42ee41045f2c810475adba01dd52e1c5f16e60eadb09
```

## Executable compatibility fixtures

Existing `internal/connections/import_test.go` fixtures preserve historical record payloads, exact-revision documentary references, artifact bytes and JSON/HTML frozen exports. Renewal and discovery fixtures preserve source revisions, refresh tokens, imported identities, renames and retirement.

Passed with the local PostgreSQL test URL configured:

```sh
go test -race ./internal/connections -run 'TestImportPreserves|TestRenewalPreserves|TestDiscoveryMatchesImported' -count=1
```

These tests establish the predecessor baseline, not acceptance of the forthcoming onboarding migration.

## Design issue resolved

Design decision 1 calls for reusing existing authorization attempts before a connection exists. Migration `007_evidence_connections.sql` makes `connection_operations.connection_id` mandatory and foreign-keyed to `evidence_connections`; `connection_authorizations` references that connection-bound operation. Browser binding, claim consumption and credential authenticated data also require an active connection generation.

The user approved setup-owned authorization attempts referencing `connection_setups`, sharing cryptographic/browser authorization helpers and retaining existing connection-bound attempts for legacy flows. Attempts bind setup revision and stage; reconnect finalization must additionally check the expected connection generation. Design decision 1 now records this additive approach.
