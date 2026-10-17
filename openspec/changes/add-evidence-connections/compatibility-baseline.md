# Connection migration baseline

## Existing paths

- `internal/assessment/evidence_sources.go`: `SaveEvidenceSource` requires edit and the exact scope revision; `ReconcileEvidenceSources` also runs on reads and creates deterministic `scope-<hash>` IDs. A scope change appends source revisions, preserving enabled/freshness/hint settings. Retirement must be checked before either path writes.
- `internal/assessment/domain.go`: scope currently supports AWS accounts/regions/clusters and pinned repository paths. It accepts regions beyond us-east-1; new connection validation must enforce its narrower boundary without rewriting historical scope JSON.
- `internal/assessment/agent_rules.go`, `internal/mcp/agent_evidence.go`, `internal/web/app.go`: read/instruction/UI entry points invoke reconciliation. All must honor retirement.
- `internal/checks/collectors.go`: GitHub takes a static registry token; AWS resolves operator account/profile mappings and optional scope role ARN, then verifies STS identity. Bound sources must resolve access at execution and never fall back after revocation.
- `internal/checks/kubernetes.go`: contexts come from operator mappings; current collection verifies EKS endpoint/CA, requires AWS access, and operates cluster-wide. Standalone and namespace-scoped planning needs explicit new paths.
- `internal/auth/identity.go`: token actions are explicit read/edit/check/export; database permission constraints also need connect. Existing tokens must not gain it. Review rejects agents independently of permissions.
- `internal/storage/artifacts.go`: content-addressed blobs use SHA-256; domain records retain exact references. Credential storage must not use this immutable artifact store.

## Local pre-migration record baseline

Service: `dc0c7bef-7a80-4233-9ff2-cb69c6b0554f`. Captured before connection implementation. There are no snapshot/publication records in this database, so export preservation additionally requires an isolated integration fixture.

| Kind | Revisions | SHA-256 |
| --- | ---: | --- |
| evidence | 785 | `8e4442839072fb9aa74d1e032ced91f2941504831f1536bd160492ea8528c3d6` |
| evidence_source | 49 | `9ed4e762e10ea4e04438ce011229b482c6f0d9d5b3c77bc1c23e5aba305dabf2` |

Reproduce under the same database timezone using:

```sql
SELECT service_id, kind, count(*) AS revisions,
 encode(sha256(convert_to(string_agg(
   jsonb_build_array(id,revision,requirement_id,actor_id,data,created_at)::text,
   E'\n' ORDER BY id,revision),'UTF8')),'hex') AS sha256
FROM records
WHERE kind IN ('evidence_source','evidence','snapshot','publication')
GROUP BY service_id,kind ORDER BY service_id,kind;
```

Migration acceptance must additionally compare actual artifact bytes, frozen export bytes and exact-revision citations; metadata digests alone do not prove blob preservation. Compare the captured revision set when legitimate new evidence arrives after this baseline.

## Compatibility cases

1. Imported bindings leave source IDs, revision numbers, JSON and artifact references unchanged.
2. Legacy source URLs and MCP artifact submissions still resolve exact revisions.
3. Reconciliation after a scope edit preserves disabled state and cannot revive retired sources.
4. Credential renewal does not revise sources or freshness deadlines.
5. A retired connection rejects late callback/discovery commits and blocks bound collectors.
6. Duplicate discovery and repository renames retain identity; a different connection requires explicit rebinding.
7. Old tokens retain their action sets; connect permission cannot grant human review.
