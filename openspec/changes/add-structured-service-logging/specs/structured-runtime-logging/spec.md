## ADDED Requirements

### Requirement: Configurable structured logger
Fed SHALL use a shared zerolog-backed logger in server, worker, all and administrative command modes. `KSI_LOG_LEVEL` SHALL accept debug, info, warn and error case-insensitively and default to info. `KSI_LOG_FORMAT` SHALL accept json and console and default to json. Invalid settings MUST fail startup with a sanitized configuration error. JSON output SHALL contain one object per line on stdout with UTC time, level, event, message, component and runtime mode.

#### Scenario: Default deployment
- **WHEN** Fed starts without logging settings
- **THEN** INFO, WARN and ERROR events are emitted as parseable JSON and DEBUG events are suppressed

#### Scenario: Debug diagnosis
- **WHEN** `KSI_LOG_LEVEL=DEBUG` is configured
- **THEN** safe DEBUG events and all higher levels are emitted using the same event schema

#### Scenario: Invalid configuration
- **WHEN** an unsupported level or format is configured
- **THEN** startup exits unsuccessfully with a configuration error without dumping environment values or credentials

### Requirement: Consistent severity and lifecycle visibility
Fed SHALL log startup, readiness summaries and shutdown at INFO, expected rejected/degraded operations at WARN, unexpected failures at ERROR and detailed stages at DEBUG. Lifecycle events MUST distinguish startup failure from successful readiness and identify the process mode. Cancellation SHALL be distinguishable from an unexpected failure.

#### Scenario: Database initialization failure
- **WHEN** a process cannot initialize its database connection
- **THEN** an ERROR event identifies the failed stage and safe error classification without exposing the connection string

#### Scenario: Graceful shutdown
- **WHEN** a running process receives a termination signal
- **THEN** lifecycle events identify shutdown initiation and its successful or failed outcome

### Requirement: Safe bounded event fields
Logs at every level MUST use bounded allowlisted metadata and MUST NOT include credentials, token hashes, cookies, authorization headers, connection strings, OAuth codes/state/verifiers, external IDs, keyrings, raw query strings, HTTP/tool payloads, evidence content or raw provider error bodies. Arbitrary errors and panic values MUST NOT bypass these exclusions. Diagnostic identifiers SHALL use server-owned or trusted internal values.

#### Scenario: Sensitive inputs at debug level
- **WHEN** requests, provider responses and wrapped errors contain credential and evidence sentinel values with DEBUG enabled
- **THEN** captured output contains safe event metadata and none of those sentinel values

#### Scenario: Unknown request path
- **WHEN** an unmatched URL contains sensitive or oversized path/query values
- **THEN** logs record a bounded unmatched route marker without the raw URL or query

### Requirement: Useful volume and resilient output
Successful health/readiness probes, static asset requests and idle scheduler polls SHALL be DEBUG-only. Their failures SHALL remain visible at WARN or ERROR. Enabled API/tool completions and failures MUST NOT be silently sampled away. Log write failures MUST NOT change committed application results or crash workers, and fallback reporting MUST be bounded and nonrecursive.

#### Scenario: Routine health probes
- **WHEN** health probes succeed at the default INFO level
- **THEN** they do not flood the output and a failed readiness probe still produces an error event

#### Scenario: Failed writer
- **WHEN** the configured writer fails during a successful operation
- **THEN** the operation retains its successful result and logging failure handling does not recurse or terminate the process
