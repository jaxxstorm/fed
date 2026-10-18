## Why

Fed currently emits little operational information, making deployed API failures, MCP activity and background work difficult to diagnose. Even browser 404 diagnostic references cannot be found in logs, so operators need consistent, correlated logging before expanding production use.

## What Changes

- Introduce zerolog-backed structured JSON logging with configurable DEBUG, INFO, WARN and ERROR levels and INFO as the default.
- Record HTTP/API requests, MCP protocol/tool activity, authorization outcomes and diagnostic references with durations and contextual identifiers.
- Instrument product lifecycle operations: service configuration, agent tokens, provider onboarding/access/discovery, evidence submission/collection, assessment review and exports.
- Record worker job transitions, retries, provider failures, startup, configuration readiness and shutdown in server, worker, all and administrative command modes.
- Define bounded, secret-free event fields, level conventions and noise controls so ECS CloudWatch logs are useful without recording credentials or evidence contents.
- Add regression coverage and operator guidance for querying requests, tools, jobs and diagnostic references.

## Capabilities

### New Capabilities
- `structured-runtime-logging`: Logger configuration, event schema, lifecycle events, level policy and safe field handling.
- `request-operation-observability`: Correlated HTTP/MCP activity, browser diagnostics, product operations and background job logging.

### Modified Capabilities

None. There are currently no main specs in `openspec/specs/`; this change adds operational visibility without changing assessment, authorization or evidence semantics.

## Impact

- Add zerolog and a shared internal logging package; replace existing isolated slog calls.
- Touch runtime configuration/startup, Echo and net/http adapters, MCP dispatch, auth, connection services, assessment/observation operations and collection workers.
- Preserve MCP streaming and browser/API response contracts, with additive request correlation headers. Operational logs supplement existing durable audit records.
- Emit JSON to container output for the existing ECS awslogs integration; no new logging service or database is required. Document log-level deployment settings and CloudWatch queries.
