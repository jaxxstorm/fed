# Operator route parity inventory

Existing routes are registered in `internal/web/app.go`; authentication and
operational/MCP endpoints are registered in `cmd/ksi-service/main.go`.

| Existing route/action | Replacement experience | Preservation check |
| --- | --- | --- |
| GET /; POST /services | Service switcher/cards and creation form | Same organization-scoped services and admin permission |
| /services/:id | Overview | Profile/scope onboarding when incomplete |
| ?kind=KSI / ?kind=FRR / ?gaps=1 | Requirements with URL filters | Both KSI and broader obligations remain accessible |
| /requirements/:rule | Requirement workspace | Same rule and exact evidence references |
| ?view=sources; actions/evidence-source | Source directory/editor | Expected revision, scope validation and source history |
| ?view=collection; actions/collect | Activity/collection | Idempotent collect-now, batch and target outcomes |
| actions/collection-config | Collection settings | Cadence, pause, policy and freshness revisions |
| /runs/:id | Collection run detail | Existing run deep links |
| ?view=settings; actions/preview-profile, profile, scope | Setup/settings | Profile preview and scoped AWS/repository inputs |
| actions/token, revoke-token | Agent connection settings | Explicit token display once and revocation |
| actions/measure, check | Advanced requirement controls | Custom measure and registered-check functionality |
| actions/evidence, draft, review, gap | Evidence/draft/review/gap forms | Citations, revision checks and human-only review |
| actions/destination | Export destination settings | Existing optional destination configuration |
| ?view=history; actions/prepare | Exports and history | Immutable snapshot creation and audit history |
| /exports/:id/:format | Download | Original JSON/HTML bytes and authorization |
| /artifacts/:id | Artifact download/preview | Original bytes, hash and service authorization |
| /auth/login, callback, logout | Existing identity flow | Session and CSRF handling |
| /mcp | External-agent API | Never routed to the SPA |
| /healthz, /readyz | Operational endpoints | Never routed to the SPA |

Read models currently include per-rule `Status`, `EvidenceWork`, typed findings,
rule receipts, source records, immutable snapshots and collector jobs. There is
no persisted external-agent session or live agent progress stream. Activity must
use receipt/audit timestamps rather than simulate a running agent.

Current error responses mostly lose structured context: browser errors are HTML,
collector failures are often strings, and API-style sessions need JSON 401/403
responses. New adapters must preserve existing authorization and mutation
services while exposing actionable structured presentation data.

Each row remains a rollout checklist item until its new UI and deep-link/action
behavior are verified. This inventory does not claim replacement parity yet.
