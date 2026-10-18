# Structured service logging verification

## Result

25/26 tasks completed. Live ECS/CloudWatch acceptance (6.5) remains pending
publication and deployment of this implementation. Changes are currently local;
local container output is not evidence of production log visibility.

## Automated checks

- PostgreSQL-backed `go test -race ./...` passed, using isolated test schemas.
- `go vet ./...` passed.
- `bun run contracts:check` passed; browser API contracts remain compatible.
- `git diff --check` passed.
- `openspec validate add-structured-service-logging --strict` passed.
- Production Docker build passed, including frontend typecheck/build and embedded
  assets. Final local `fed:logging-check` image:
  `sha256:9e12bf5fb76246ff49d6988ba87d0a11727e0c04d5da40353727537b0dc329a7`.

Logging regressions cover level/schema validation, concurrent contexts, broken
sinks, commit failure/replay, real token issuance/revocation without credential
disclosure, browser 404 references, auth/CSRF rejection, conflicts, cancellation,
panic recovery, flushing, real MCP SDK calls and malformed parameters, concurrent
in-process tools, oversized JSON/SSE passthrough, worker lease recovery/retry/
exhaustion and partial provider discovery. Existing domain/provider/worker tests
also passed with instrumentation enabled in code.

The user approved an amendment for protocol errors before MCP middleware:
observe at most 64 KiB of JSON response data, forwarding writes immediately and
emitting only a numeric protocol error code. SSE is not inspected and bodies are
never emitted in logs.

## Container smoke test

Used a temporary PostgreSQL schema, migrated it with the container, and started
server-mode containers with INFO and DEBUG settings. Each used development
identity on an internal loopback listener; a separate curl container shared the
network namespace. Checked bootstrap response correlation, JSON log parsing,
level filtering, and graceful shutdown. Both passed: INFO emitted 8 events,
DEBUG emitted 9, and the largest event in this fixture was 357 bytes. Temporary
containers and schema were removed. The initial attempt with a non-loopback
development listener was correctly rejected by existing configuration validation.

## Local performance measurements

Apple M4 Max, Go benchmark output, discard sink (not CloudWatch throughput):

| Benchmark | Time/op | Allocation |
| --- | --- | --- |
| Structured completion serialization | 252.7 ns | 24 B, 1 alloc |
| HTTP fixture, INFO logging | 2,188 ns | 7,854 B, 28 allocs |
| HTTP fixture, INFO suppressed at ERROR | 1,819 ns | 7,823 B, 27 allocs |
| MCP method start/completion wrapper | 1,061 ns | 1,232 B, 9 allocs |

HTTP measurements include request/recorder construction. They are local fixture
comparisons, not end-to-end production latency estimates.

## Remaining deployment acceptance

Publish/deploy the new build to `fedramp-web-shared` in `ignistech-shared`, then
exercise a browser error and MCP success/error and inspect a collection job's
events in CloudWatch. Use `docs/service-logging.md` queries to confirm request,
diagnostic, operation and persisted job correlation. Default INFO/JSON requires
no new environment settings; DEBUG is optional. Keep task 6.5 open until those
actual deployed observations are recorded.
