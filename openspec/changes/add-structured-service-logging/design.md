## Context

The Go/Echo process wraps net/http browser and MCP handlers and runs collectors in `worker` or `all` mode. ECS already captures container output in CloudWatch. Existing slog calls cover a few startup/cleanup failures; `uiFailure` generates references for all errors but logs only 500s. MCP's generic `add` wrapper converts domain errors into tool error results with successful HTTP transport, making HTTP status alone inadequate. Operational logs must complement existing database audit records and preserve immutable evidence semantics.

## Goals / Non-Goals

**Goals:**
- Make normal API/MCP usage and important product transitions visible at INFO, with actionable WARN/ERROR outcomes and opt-in DEBUG detail.
- Connect browser references, HTTP requests, tool calls, provider operations and persisted worker jobs through stable identifiers.
- Produce consistent JSON logs usable directly in CloudWatch, including server, worker, all and administrative commands.
- Preserve protocol behavior, authorization, application results and credential confidentiality at every level.

**Non-Goals:**
- Distributed tracing infrastructure, metrics backends, browser analytics or remote frontend log ingestion.
- Logging evidence contents, SQL statements/parameters, HTTP bodies or arbitrary tool arguments/results.
- Replacing durable audit events or changing assessment/collection behavior.

## Decisions

### 1. Central zerolog configuration

Add `internal/logging` with injectable writers, context helpers, typed event/field helpers and error classification. Use zerolog directly rather than introducing a slog bridge for the few existing calls. Parse `KSI_LOG_LEVEL` case-insensitively (`debug`, `info`, `warn`, `error`), default INFO, and `KSI_LOG_FORMAT` (`json`, `console`), default JSON. Initialize before database/provider startup; invalid settings produce a sanitized configuration error and a nonzero exit. Console is an explicit local-development option. Emit one JSON object per line to stdout; do not require an external log service.

Common fields: UTC `time`, `level`, stable `event`, human-readable `message`, `component`, `mode`, `version` when known. Contextual fields include `request_id`, `operation_id`, `service_id`, `actor_id`, `job_id`, `batch_id`, `connection_id`, `setup_id`, `provider`, `attempt`, `duration_ms`, `outcome`, `error_code` and `reference`. Never label the application name as `service_id`, which already identifies an assessment service. Test loggers remain instance-scoped to avoid global-level races.

### 2. Level policy and bounded detail

INFO: request completions, MCP dispatch/completions, committed product changes, job starts/completions and runtime lifecycle. WARN: rejected authentication/authorization, invalid input/not-found/conflict responses, degraded provider access and retry scheduling. ERROR: unexpected internal failures, exhausted jobs, failed startup and recovered panics. DEBUG: safe operation stages, successful health/static requests, provider request metadata and empty scheduler polls. Normal cancellation is a distinct outcome rather than an unexpected internal error. Explicit WARN/ERROR thresholds intentionally suppress INFO activity.

Use stable event names (for example `http.request.completed`, `mcp.tool.completed`, `connection.discovery.completed`, `collection.job.retry_scheduled`). No automatic sampling of failures or normal API/tool completions at enabled levels. Successful `/healthz`, `/readyz` and static asset requests are DEBUG to prevent probe noise; their failures remain WARN/ERROR. Document the event inventory and ownership to avoid duplicating the same completion across layers.

Field values are allowlisted and bounded. Never serialize request objects, headers, full URLs/query strings, cookies, bearer/session tokens or hashes, OAuth codes/state/verifiers, external IDs, connection keyrings, DSNs, provider credentials, evidence bodies, prompts, raw tool input/output or raw upstream error bodies. Prefer known error codes, wrapped error types and safe context over `err.Error()` for arbitrary errors. Normalize route templates; unmatched paths use an `unmatched` route marker rather than attacker-controlled path text. Internal identifiers come from authenticated/resolved context; arbitrary client strings are not promoted to trusted identifiers. DEBUG follows the same exclusions. Panic logging records safe stack locations, not arbitrary panic values.

### 3. Request correlation and response-safe middleware

Generate a fresh server request UUID for every ingress request and return `X-Request-ID`; do not trust client-supplied correlation headers. Attach mutable request-scoped observation metadata through context so downstream net/http routing/authentication can supply normalized routes and trusted identities to the outer Echo completion logger. Avoid reporting only the Echo catch-all `/api/ui/v1/*` when the inner route is known. Emit one HTTP completion with method, normalized route, status, elapsed time and outcome, including auth/CSRF failures and unknown routes.

Middleware must observe final status after Echo error handling and recovery, preserve flushing and supported ResponseWriter interfaces, and avoid consuming/buffering bodies. Long-lived MCP requests log ingress at DEBUG and completion on close, with independent tool events while active. Cancellation and recovered panic paths must still produce a completion event.

Pass request context to browser and connection error response helpers. Log every returned diagnostic reference with the corresponding error code and request ID, including 400/403/404/409; retain existing response fields. The separate HTTP completion and diagnostic event have distinct purposes and event names.

### 4. MCP protocol and tool instrumentation

Use the installed SDK's receiving middleware hooks for method dispatch, validation/protocol errors, initialization, listing and prompt operations. Instrument the generic tool execution boundary for start/completion, duration and application outcome, including `IsError` responses where HTTP status is 200. The pinned SDK rejects some malformed parameters before receiving hooks, without a logger callback. Approved implementation amendment: observe up to 64 KiB of JSON response bytes while passing writes through immediately, extracting only a numeric top-level JSON-RPC error code after completion. Never inspect request bodies or emit response contents, IDs or error messages. Preserve writer interfaces with httpsnoop; do not inspect SSE/non-JSON streams. Oversized JSON responses remain bounded and are reported at DEBUG as uninspected rather than interpreted as protocol success.

Generate server-owned operation IDs per invocation; do not log arbitrary JSON-RPC IDs or MCP session identifiers. Nested service/provider events inherit request and operation IDs. Log known method/tool names and authorized internal actor/service IDs, not argument/result payloads. Protocol validation errors and forbidden tool calls remain visible even if the domain function never executes. In-process SDK usage without HTTP gets an operation ID without fabricating an HTTP request ID.

### 5. Product and worker event boundaries

Instrument shared application-service boundaries so browser and agent operations have the same event vocabulary. Cover service creation; profile/scope updates; token issuance/revocation; connection setup, consent, verification, discovery/selection, renewal/disconnect; evidence submission/refresh; drafts/reviews; export preparation; and setup cleanup. Success events occur after commit, with IDs, revision/generation and bounded counts instead of content. Idempotent replays report `outcome=replayed` rather than claiming another mutation.

Workers log actual enqueue/claim, attempt start, completion, retry, terminal failure and lease recovery with job/batch/service IDs and provider/check identifiers. Use persisted job and batch IDs to bridge enqueue and execution across restarts; no schema migration is needed to persist HTTP correlation. Each execution gets its own operation ID. Empty queue polling is DEBUG. Provider calls emit safe API operation names, duration, result classification and retry context without URLs or payloads; AWS inspection remains read-only and us-east-1 only. Storage/database failures use safe classifications, not parameter dumps.

### 6. Testing and rollout

Capture JSON into injected writers and assert event fields, levels, correlation, outcomes and absence of sensitive sentinels. Include real adapter paths for HTTP, MCP and worker integration, not only logging-helper tests. Verify references on 404s, tool errors with HTTP 200, pre-handler validation failures, concurrent context isolation, streaming/flushing, panic recovery and retries. Logger write failures must not convert successful domain commits into failed API responses or crash workers; use a nonrecursive bounded stderr fallback for sink errors.

## Risks / Trade-offs

- Increased CloudWatch volume → INFO completion summaries and DEBUG-only probes/polling; document level tuning and retention outside the application.
- Credential leakage through errors or SDK payloads → allowlisted fields, safe classifications and DEBUG sentinel tests; no blanket request dumping.
- Middleware interferes with MCP or misses inner routes → preserve interfaces, test actual SDK requests, and share request metadata across adapter boundaries.
- Excessive duplicate events → designate transport, operation and durable-state event owners; each event answers a different question.
- Synchronous output can slow requests under log backpressure → small bounded events and existing ECS log transport; avoid unbounded application log queues and benchmark representative workloads.
- No durable HTTP-to-job trace chain → correlate via job/batch IDs present in enqueue and worker events; distributed tracing can be added separately.

## Migration Plan

1. Add logger/configuration and context support, then request/MCP instrumentation, then product and worker events.
2. Run logging, protocol, security/redaction and existing regression checks; build the production container.
3. Document environment options, event fields and CloudWatch queries. Deploy INFO/JSON using the existing awslogs setup; use DEBUG temporarily for diagnosis.
4. Verify a browser error reference, MCP success/error and a worker job are searchable in ECS logs. Record any unavailable live provider checks explicitly.
5. Roll back the image or adjust the level if necessary; no database migration or evidence rewrite is involved.

## Open Questions

The user approved bounded JSON response error-code observation to cover gaps in the pinned MCP SDK's hooks. Log retention and any infrastructure-level buffering settings remain owned by the existing ECS deployment.
