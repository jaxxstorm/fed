## 1. Logging foundation and runtime configuration

- [x] 1.1 Inventory HTTP, MCP, auth, domain, provider and worker boundaries; document stable event names, field ownership, severity and sensitive-field exclusions.
- [x] 1.2 Add zerolog and an injectable internal logging package with structured fields, context propagation, safe error classification and bounded sink-error fallback.
- [x] 1.3 Add KSI_LOG_LEVEL and KSI_LOG_FORMAT parsing with INFO/JSON defaults, case-insensitive levels and sanitized invalid-setting failures; initialize logging before runtime dependencies.
- [x] 1.4 Replace existing slog calls and instrument startup/readiness/shutdown and administrative command outcomes across server, worker and all modes.
- [x] 1.5 Verify JSON schema, level filtering, concurrency-safe logger isolation, sensitive-value exclusions and failed-writer behavior with captured output.

## 2. HTTP and browser diagnostics

- [x] 2.1 Add server-generated request IDs, X-Request-ID responses and context propagation across Echo, auth and net/http adapters, including early failures and unknown routes.
- [x] 2.2 Emit one HTTP completion event with inner route template, method, final status, elapsed time and outcome; enrich trusted actor context and implement probe/static noise rules.
- [x] 2.3 Preserve streaming and response-writer interfaces, observe final Echo error responses, and add safe panic/cancellation handling without body buffering.
- [x] 2.4 Make browser/connection error helpers context-aware and log all returned diagnostic references with request ID, status and safe error code, including 404s.
- [x] 2.5 Add HTTP integration coverage for success, auth/CSRF rejection, 404 reference lookup, conflicts, 500s, panic, cancellation, streaming and malicious path/header/query inputs.

## 3. MCP visibility

- [x] 3.1 Verify pinned MCP SDK receiving/error hooks and implement protocol method events for initialization, listing, prompts and pre-handler dispatch/validation failures.
- [x] 3.2 Instrument generic tool execution with operation IDs, start/completion events, durations and domain outcomes, classifying IsError results independently of HTTP status.
- [x] 3.3 Propagate authenticated actor and authorized service context into nested service calls without serializing arguments, results, RPC IDs or session credentials.
- [x] 3.4 Verify real SDK success/error requests, invalid methods/arguments, forbidden access, in-process calls and concurrent request/operation isolation with secret sentinel assertions.

## 4. Product operation events

- [x] 4.1 Instrument committed service/profile/scope changes and agent token issuance/revocation with safe actor, service, revision, lifetime and permission metadata.
- [x] 4.2 Instrument provider setup/consent/verification, discovery/source selection, renewal/disconnect and cleanup with safe outcomes, IDs, generations and coverage counts.
- [x] 4.3 Instrument evidence submission/refresh, draft/review and export preparation through shared service boundaries so browser and MCP adapters use the same domain events.
- [x] 4.4 Verify rolled-back mutations do not log success, idempotent replays are identified, and operational logging preserves durable audit/evidence and authorization behavior.

## 5. Workers and provider execution

- [x] 5.1 Instrument enqueue/claim, attempt start/completion, retry scheduling, terminal failure and lease recovery with persisted job/batch identifiers and attempt context.
- [x] 5.2 Add safe provider operation/duration/error events for registered adapters and collectors, including partial coverage and credential renewal failures; keep idle polling at DEBUG.
- [x] 5.3 Verify retry/restart correlation, provider denial, partial discovery, cancellation and concurrent workers without credentials, upstream response bodies or evidence in output.

## 6. Verification and operational guidance

- [x] 6.1 Run PostgreSQL-backed Go race/integration and vet checks plus logging regressions; confirm MCP/browser contracts and streaming behavior remain compatible.
- [x] 6.2 Build the production container and smoke-test INFO/JSON and DEBUG output; measure representative request/tool logging overhead and bounded event sizes.
- [x] 6.3 Document settings, event inventory, severity, redaction, CloudWatch queries and deployment/rollback steps for request IDs, diagnostic references, MCP tools and worker jobs.
- [x] 6.4 Run strict OpenSpec validation and record automated verification results and any deployment dependencies.
- [ ] 6.5 After deployment, verify browser diagnostic lookup, MCP success/error and worker job correlation in ECS CloudWatch; record unavailable live checks as blockers rather than inferring success from mocks.
