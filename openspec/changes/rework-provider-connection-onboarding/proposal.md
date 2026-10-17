## Why

Fed's current connection forms expose implementation details and depend on operator knowledge of local profiles, installation IDs and provider registrations. Production ECS deployments need a predictable onboarding experience: delegate AWS access to Fed's execution role, grant Workspace consent to a Fed-owned OAuth client, and install a Fed-owned GitHub App with verified return to the originating service.

## What Changes

- Add resumable provider-specific setup journeys with prerequisites, deployment readiness, explicit permissions, identity verification, recovery and resource discovery.
- Make delegated AssumeRole access the default for new server-managed AWS connections: a configured Fed IAM role, generated per-connection external ID, customer account ID, generated CLI/manual role setup, submitted role ARN and verified account identity.
- Preserve existing profile-based and agent-managed connections as explicit advanced/legacy modes. New delegated connections must never fall back to ambient account access.
- Complete Google Workspace's Connect → account selection/consent → verified customer → discovery journey, with operator OAuth registration clearly separated from administrator consent.
- Complete GitHub's Connect → App installation/selection → user authorization → verified installation journey without requiring users to copy installation IDs in the standard flow.
- Expose equivalent setup/status/generated configuration through authenticated MCP and browser APIs; provider consent remains an authorized browser step.
- Retain read-only collectors, AWS us-east-1 boundaries, explicit assessment scope selection, immutable evidence, separate access/discovery/collection states and existing lifecycle fencing.
- Document the ECS task-role, OAuth/App registrations, callbacks and private-hostname requirements for fedramp.ignistech.io. Use original Fed styling inspired by the supplied flow, with read-only permissions rather than the example's autofix privileges.

## Capabilities

### New Capabilities
- `delegated-aws-onboarding`: Fed principal readiness, generated trust setup, external-ID lifecycle and verified cross-account access.
- `workspace-consent-onboarding`: Deployment OAuth prerequisites and resumable administrator consent with customer verification.
- `github-installation-onboarding`: Fed App installation selection, verified authorization and repository selection handoff.
- `provider-setup-journeys`: Shared setup state, readiness, browser/MCP parity, recovery and compatibility.

### Modified Capabilities

None in the main spec tree, which is empty. These additive contracts extend the unarchived `add-evidence-connections` capability contracts; implementation must reconcile their overlapping interfaces without dropping lifecycle, authorization or evidence guarantees. This change does not close that change's outstanding live verification task.

## Impact

- `internal/connections`: setup persistence, external-ID provisioning, identity finalization, authorization stages and shared guide DTOs.
- `internal/providers`: deployment principal checks, AssumeRole enforcement and provider authorization/installation orchestration.
- `internal/web`, `internal/mcp`, generated contracts and React: provider wizards, generated setup material, callback resume and actionable failures.
- Additive PostgreSQL migrations; operator environment configuration and ECS deployment documentation; no assessment/evidence rewrite.
- Provider mocks, SDK request assertions, session/race tests and Docker browser journeys. Real cloud registration and consent remain operator prerequisites for live acceptance.
