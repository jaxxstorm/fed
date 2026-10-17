## ADDED Requirements

### Requirement: Durable asynchronous execution
The system SHALL execute registered checks through durable background jobs with leases, bounded retries, deadlines, and authorized scope. Enqueue SHALL return a run ID promptly and support idempotency keys. Run states SHALL include queued, running, succeeded, failed, and timed-out, separately from evaluation outcomes.

#### Scenario: Repeated enqueue
- **WHEN** a caller repeats the same request with the same idempotency key
- **THEN** the system returns the existing logical run rather than scheduling another run

#### Scenario: Worker interruption
- **WHEN** a worker stops while holding a lease
- **THEN** the system recovers the run after lease expiry within the retry policy and preserves previous attempt details

### Requirement: Reproducible observations and explicit coverage
Checks SHALL preserve input scope, collector/evaluator versions, observation times, evidence references, and expected versus observed coverage. Permission failures, absent observations, and partial enumeration SHALL never become a passing result for the missing scope.

#### Scenario: One region cannot be inspected
- **WHEN** collection succeeds in one configured region and is denied in another
- **THEN** the result retains successful observations, identifies the denied region, and exposes incomplete coverage and an unknown or error outcome for that scope

### Requirement: Initial read-only checks
The system SHALL register checks for AWS GuardDuty detector/features, AWS EKS control-plane logging, and GitHub Pulumi project/workflow evidence. Evaluations SHALL use versioned customer measures and SHALL distinguish repository intent from live observations. Checks SHALL NOT mutate customer infrastructure.

#### Scenario: EKS logging differs from the measure
- **WHEN** the live EKS response lacks a log type required by the configured customer measure
- **THEN** the check records a failure with the cluster, observed settings, measure version, and collection time

#### Scenario: Pulumi workflow exists
- **WHEN** repository inspection identifies Pulumi deployment configuration at a specific commit
- **THEN** the check records configuration-intent evidence without claiming recurring drift execution or live resource coverage

### Requirement: Registry-bound execution
The check runner SHALL accept only registered check identifiers and schema-valid parameters within configured integration scopes.

#### Scenario: Arbitrary execution request
- **WHEN** a caller supplies an unregistered check or shell command
- **THEN** the system rejects the request before execution
