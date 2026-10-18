## ADDED Requirements

### Requirement: Correlated HTTP request completion
Every HTTP request SHALL receive a server-generated request ID in context and `X-Request-ID` in its response. A completion event SHALL report method, normalized route, final status, duration and outcome at the configured severity, including authentication failures and unmatched routes. Known inner routes MUST be distinguished within Echo catch-all adapters. Client-supplied request IDs MUST NOT replace the server-generated identifier. Instrumentation MUST preserve response status/body, streaming, cancellation and supported response-writer interfaces.

#### Scenario: Browser API request
- **WHEN** an authenticated browser calls an API route
- **THEN** its response and completion log share a request ID and the event includes the normalized inner route and trusted actor context

#### Scenario: Unauthorized request
- **WHEN** authentication or CSRF validation rejects a request before the application handler
- **THEN** the request still produces a correlated completion with the rejection status and no fabricated authenticated actor

#### Scenario: Panic and streaming behavior
- **WHEN** a handler panics or streams a response
- **THEN** recovery produces a safe failure event and completion for a panic, while streaming responses continue to flush without logging consuming their body

### Requirement: Searchable browser diagnostic references
Every diagnostic reference returned in a browser error response SHALL appear in an associated log event with request ID, error code and status. This SHALL include 400, 403, 404, 409 and 500 responses without changing their existing response fields.

#### Scenario: Missing item
- **WHEN** the UI receives a not-found response with a diagnostic reference
- **THEN** an operator can locate that exact reference in logs and identify the request route and error classification

### Requirement: MCP method and tool outcomes
Fed SHALL log MCP protocol method activity and tool invocation start/completion with server-owned operation IDs, request correlation when available, known method/tool names, elapsed time and outcome. Application tool errors MUST be logged as failures even when HTTP transport succeeds. Validation/protocol failures before tool execution MUST be observable. Logs MUST NOT serialize tool arguments, results, prompts, arbitrary JSON-RPC IDs or session identifiers.

#### Scenario: Successful tool invocation
- **WHEN** an agent invokes an authorized registered tool
- **THEN** start and completion events identify the tool and operation, and nested service events share that operation context

#### Scenario: Tool error with successful transport
- **WHEN** a tool returns `IsError` with a forbidden or conflict result over HTTP 200
- **THEN** its completion records the application error code and WARN outcome rather than success

#### Scenario: Invalid protocol or input
- **WHEN** the MCP SDK rejects a method or tool input before calling the domain function
- **THEN** a correlated protocol/dispatch failure event is emitted without raw input

#### Scenario: Errors before SDK middleware
- **WHEN** malformed parameters produce a JSON-RPC error before receiving middleware executes
- **THEN** a response observer extracts only the numeric error code from at most 64 KiB of JSON while forwarding writes immediately, without logging response contents

#### Scenario: Large or streaming response
- **WHEN** a JSON response exceeds the observation bound or a response uses SSE
- **THEN** output remains unchanged, SSE is not inspected and oversized JSON is marked uninspected at DEBUG rather than classified as protocol success

#### Scenario: Parallel calls
- **WHEN** multiple agents invoke tools concurrently
- **THEN** each call retains distinct operation/request context without leaking another call's actor or service identifiers

### Requirement: Product operations visible across adapters
Shared services SHALL emit outcome events for service creation, profile/scope changes, token issuance/revocation, provider setup/consent/access/discovery/source selection/renewal/disconnect, evidence submission/refresh, assessment draft/review, export preparation and setup cleanup. Events SHALL use trusted identifiers, revisions/generations and counts where applicable. Mutation success MUST be logged only after commit, and idempotent replay MUST be distinguishable from a new mutation. Existing durable audit and evidence behavior MUST remain unchanged.

#### Scenario: Agent token issuance
- **WHEN** an administrator successfully issues a scoped agent token
- **THEN** an event records the issuer, authorized service, lifetime and permitted action names without the token or token hash

#### Scenario: Shared evidence submission
- **WHEN** evidence is submitted through either an authorized browser operation or MCP tool
- **THEN** the same domain event vocabulary reports committed record identity/revision without the evidence contents

#### Scenario: Rolled-back or replayed mutation
- **WHEN** a transaction rolls back or an idempotency receipt is replayed
- **THEN** logs distinguish failure or replay from a newly committed operation

### Requirement: Background and provider execution visibility
Workers SHALL log enqueue, claim, attempt start, completion, retry, terminal failure and lease recovery with available persisted job/batch/service IDs, attempt numbers and safe error codes. Provider operations SHALL emit safe operation metadata and outcome/duration without payloads. Enqueue and worker execution events MUST be correlatable by persisted identifiers across process restarts without requiring a live HTTP context.

#### Scenario: Retry across restarts
- **WHEN** a provider request fails and a collection job retries in another process
- **THEN** logs identify both attempts by the same persisted job ID with their attempt numbers, retry reason and final outcome

#### Scenario: Partial discovery
- **WHEN** provider discovery succeeds for some resources but access is denied for others
- **THEN** logs identify partial coverage and safe provider error codes rather than reporting complete success or dumping provider responses

### Requirement: Operator verification guidance
Documentation SHALL describe logging configuration, severity/event conventions, secret exclusions and CloudWatch queries for request IDs, diagnostic references, MCP tools, provider operations and jobs. Deployment verification SHALL distinguish automated logging tests from actual ECS log visibility.

#### Scenario: Production diagnosis
- **WHEN** an operator follows the logging guide with a browser diagnostic reference or job ID
- **THEN** the guide supplies a CloudWatch query and explains how to follow related request, operation or job events
