# Service logging

Fed uses zerolog operational events alongside its durable database audit history.
`KSI_LOG_LEVEL` accepts `debug`, `info` (default), `warn`, `error` case-insensitively.
`KSI_LOG_FORMAT` accepts `json` (default) or `console` for local development.
JSON is written to stdout for the existing ECS awslogs integration. Settings take
effect on process restart; no logging sidecar or database migration is required.

## Event ownership

| Boundary | Event family | Owner and fields |
| --- | --- | --- |
| Process and administrative commands | `runtime.*` | Entry point; mode, stage, outcome |
| HTTP requests | `http.request.completed` | Outer Echo middleware; request ID, normalized inner route, status, duration |
| Browser errors | `browser.request.failed` | Error response helper; diagnostic reference, code, request ID |
| MCP methods | `mcp.method.*` | SDK receiving middleware; known method, operation ID, outcome |
| MCP tools | `mcp.tool.*` | Shared tool wrapper; registered tool, operation ID, error code |
| Service/profile/scope and evidence/review/export changes | `assessment.*` | Shared service after commit; resolved IDs, revisions |
| Agent credentials | `auth.token.*` | Auth service after commit; issuer/service, lifetime, permissions; never token/hash |
| Provider connections | `connection.*` | Shared connection service; IDs, generation, coverage counts |
| Collection | `collection.job.*` | Queue/worker after state commit; job/batch ID, attempt, outcome |
| Provider calls | `provider.operation.*` | Provider boundary; provider, known API operation, duration, code |

INFO records normal activity and successful durable changes. WARN records expected
rejection, conflicts, unavailable resources, degraded access and retries. ERROR
records unexpected failures and exhausted work. DEBUG adds operation stages;
successful health/static requests and idle polling appear only at DEBUG. Errors
are not sampled. Request, diagnostic and mutation events serve distinct purposes.

Fields exclude request/response bodies, raw URLs/query strings, headers, cookies,
tokens and token hashes, OAuth state/codes/verifiers, external IDs, keys, DSNs,
evidence contents and arbitrary upstream errors. DEBUG has the same exclusions.
Do not add arbitrary object serialization or `.Err(err)` at logging call sites.
Use known codes and resolved identifiers instead. Unmatched routes are logged as
`unmatched`. Application `service_id` always means an assessment service.

Some MCP malformed-parameter errors bypass SDK hooks. A bounded JSON response
observer extracts only their numeric `protocol_code`; it never emits response
contents. Responses over 64 KiB are marked `mcp.response.uninspected` at DEBUG;
SSE responses pass through without observation. Method and tool events remain
available independently of this observer.

## CloudWatch Logs Insights

Find a browser diagnostic reference:

```text
fields @timestamp, event, request_id, reference, route, status, error_code
| filter reference = "DIAGNOSTIC_REFERENCE"
| sort @timestamp desc
```

Follow a request (copy its request ID from the result or X-Request-ID header):

```text
fields @timestamp, event, operation_id, tool, outcome, error_code, duration_ms
| filter request_id = "REQUEST_ID"
| sort @timestamp asc
```

Inspect MCP activity:

```text
fields @timestamp, event, method, tool, request_id, operation_id, outcome, error_code
| filter component = "mcp"
| sort @timestamp desc
```

Follow a job across attempts/process restarts:

```text
fields @timestamp, event, job_id, batch_id, attempt, outcome, error_code
| filter job_id = "JOB_ID"
| sort @timestamp asc
```

Provider diagnosis:

```text
fields @timestamp, event, provider, connection_id, operation_id, outcome, error_code
| filter component = "provider" or component = "connections"
| sort @timestamp desc
```

Deploy INFO/JSON first. Temporarily enable DEBUG for diagnosis and return to INFO
afterwards. Retention is managed by infrastructure. Rollback uses the prior image;
logging does not rewrite evidence or replace authorization/audit records. Verify
actual browser references, MCP results and worker attempts in CloudWatch after
deployment; local capture tests alone do not establish production visibility.
