## ADDED Requirements

### Requirement: Verified Fed runtime principal
Fed SHALL identify its stable application IAM role through deployment configuration and verify that the runtime STS identity corresponds to that role before generating customer trust material or executing delegated collection. The application task role SHALL be distinguished from the ECS execution role.

#### Scenario: Runtime mismatch
- **WHEN** the configured Fed role does not match the actual runtime identity
- **THEN** AWS delegated setup is unavailable with an operator remediation message and no customer trust material or resource reads are issued

### Requirement: Generated customer role setup
Fed SHALL offer CLI and manual AWS onboarding that use the same versioned read-permission and trust documents, validated customer account ID, stable Fed role principal and server-generated per-connection external ID. Only a customer administrator SHALL execute IAM provisioning; Fed SHALL NOT provision customer IAM resources itself.

#### Scenario: Repeatable setup
- **WHEN** an authorized actor resumes or retries setup for the same account
- **THEN** the generated external ID and role configuration remain stable and both setup methods show equivalent policies and the expected role ARN

#### Scenario: Existing role conflict
- **WHEN** generated CLI commands encounter a pre-existing role with incompatible trust or a caller in another account
- **THEN** the commands stop with an actionable explanation instead of silently overwriting or accepting that role

#### Scenario: Permission boundary
- **WHEN** Fed renders the read policy
- **THEN** actions derive from the enabled collector permission manifest, contain no infrastructure write grants, and monitoring remains explicitly limited to us-east-1

### Requirement: Delegated credentials cannot fall back
New standard server-managed AWS connections SHALL use an explicit delegated strategy requiring a valid target role ARN, stored external ID, successful AssumeRole and verified target account. All collectors and discovery SHALL resolve this strategy at execution and fence persisted results against its generation.

#### Scenario: Failed trust despite working base credentials
- **WHEN** AssumeRole fails but the base runtime role can directly read the target account
- **THEN** the operation reports a trust failure and performs no resource reads using the base credentials

#### Scenario: Account or region substitution
- **WHEN** a submitted role belongs to another declared account, the assumed identity differs, or a request names a region other than us-east-1
- **THEN** setup or execution rejects the request before resource collection

### Requirement: External ID and session lifecycle
Fed SHALL bind generated external IDs to the service, setup/connection and target account, expose them only to authorized setup actors, encrypt their stored representation and omit them from ordinary listings, logs and evidence. Session caches SHALL be isolated by connection and generation. Explicit external-ID replacement SHALL invalidate prior access and require trust updates and retesting.

#### Scenario: Cross-service access and disconnect
- **WHEN** another service attempts to retrieve setup material, or an in-flight operation completes after disconnect
- **THEN** material access is denied and the stale operation cannot commit evidence or restore access

#### Scenario: Refresh and rotation
- **WHEN** temporary AWS credentials expire or the external ID is explicitly rotated
- **THEN** ordinary session refresh uses the current bound role and external ID, while rotation advances the access generation and supplies new trust instructions without revising historical evidence

### Requirement: Legacy and agent mode compatibility
Fed SHALL preserve existing profile/environment and agent-managed connections as explicitly labeled alternatives without silently converting them to delegated access.

#### Scenario: Upgrade existing sources
- **WHEN** onboarding migrations run against existing AWS connections
- **THEN** source IDs, source revisions, evidence and access strategy semantics remain unchanged and new standard setup defaults to delegated roles
