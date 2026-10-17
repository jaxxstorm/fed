## ADDED Requirements

### Requirement: Structured actionable errors
Browser-facing failures SHALL provide a stable code, readable title, short explanation, affected target/operation context, retryability, applicable field errors, recovery actions and a diagnostic reference where available. Known failures SHALL have specific remedies; unknown failures SHALL NOT invent a cause or expose raw internal errors as the primary message.

#### Scenario: AWS access expires
- **WHEN** collection reports a known expired-credential error
- **THEN** the UI identifies the relevant account/profile and execution environment, explains where to refresh access and offers the appropriate retry action

#### Scenario: Cluster enumeration denied
- **WHEN** a cluster read fails with known authorization context
- **THEN** the UI names the cluster and failed read, provides the known access requirement and links to source configuration or retry guidance

#### Scenario: Unexpected service error
- **WHEN** an operation fails with an unclassified internal error
- **THEN** the operator sees a concise failure message and diagnostic reference without SQL, stack traces or speculative credential instructions

### Requirement: Recoverable form and action states
Mutations SHALL show pending, success and failure feedback; preserve entered data on validation/conflict errors; prevent duplicate pending submissions; and offer actual authorized recovery actions. Retryable operations SHALL respect existing idempotency contracts. Acknowledging an error SHALL NOT mark its underlying problem resolved.

#### Scenario: Invalid source configuration
- **WHEN** the operator submits an invalid region, path or freshness setting
- **THEN** the relevant field explains how to correct the value and the rest of the form remains populated

#### Scenario: Partial collection
- **WHEN** some targets collect successfully and others fail
- **THEN** successful evidence remains visible and each failed target has a clear explanation and appropriate next action

### Requirement: Honest loading empty and stale states
Every data-driven screen SHALL distinguish initial loading, no data, filtered-empty results, unavailable data and stale cached data. Session expiry SHALL produce a clear sign-in action and SHALL NOT masquerade as a successful empty API response.

#### Scenario: Filter has no matches
- **WHEN** a filter produces zero evidence records
- **THEN** the UI offers to clear/change filters rather than claiming no evidence exists in the service

#### Scenario: Refresh fails after data was loaded
- **WHEN** a background refresh fails
- **THEN** the UI retains the last known data with its update time and a visible retryable warning rather than replacing it with zeros
