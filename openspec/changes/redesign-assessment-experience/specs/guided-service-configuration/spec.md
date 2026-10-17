## ADDED Requirements

### Requirement: Guided profile and source onboarding
Operators SHALL be able to select a service/profile, define scope, configure sources and obtain agent connection instructions through labeled controls and a clear setup checklist. Existing scope/profile constraints and profile-change previews SHALL remain enforced. Ordinary configuration SHALL NOT require raw JSON editing.

#### Scenario: New service setup
- **WHEN** an operator creates a service
- **THEN** onboarding identifies the next incomplete setup step and provides forms for profile, scope and evidence sources before offering a scoped agent prompt

#### Scenario: Configure AWS and repository sources
- **WHEN** an operator edits account regions or repository commit/path bindings
- **THEN** typed controls explain valid values, show affected scope and enforce the saved boundary through shared services

### Requirement: Source health and agent handoff
Source views SHALL show configured identity, non-secret access hints, latest known access result with timestamp, affected work and next steps. Unknown access SHALL be labeled as not yet checked. The UI SHALL provide copyable service-specific MCP connection instructions and investigation prompts, with explicit token issuance/revocation controls for authorized operators.

#### Scenario: Source has never been read
- **WHEN** a configured source has no access result
- **THEN** it is shown as not yet checked rather than connected or healthy

#### Scenario: External agent needs access
- **WHEN** an agent result reports missing source access
- **THEN** the operator can identify which agent-side source/access hint to configure and copy a relevant retry instruction without exposing stored credentials

### Requirement: Usable collection controls and run history
Operators SHALL be able to start supported collection, pause/resume refresh, edit cadence/policy inputs and inspect per-target run results through normal controls. Run views SHALL summarize successes, failures, resulting evidence and next actions instead of rendering raw result JSON by default.

#### Scenario: Retry a failed run
- **WHEN** an authorized operator retries collection after correcting an access problem
- **THEN** the UI shows the new run's pending and final state, links collected evidence and retains the prior failure in history

### Requirement: Understandable exports and history
Export/history views SHALL identify snapshot date, profile, progress at preparation, outstanding items and download actions. Current progress SHALL be visually distinct from frozen snapshot progress, and existing downloadable artifacts/export links SHALL remain usable.

#### Scenario: Download an earlier snapshot
- **WHEN** the operator selects an export created before later evidence updates
- **THEN** the UI shows that snapshot's recorded progress and downloads its original immutable content
