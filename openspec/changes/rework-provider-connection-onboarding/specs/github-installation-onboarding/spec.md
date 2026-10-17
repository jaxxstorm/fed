## ADDED Requirements

### Requirement: Fed App readiness and read-only setup
Fed SHALL present the configured GitHub App's readiness, required Metadata/Contents read permissions and installation prerequisites, with Connect launching the standard installation/authorization journey. Standard setup SHALL NOT require users to copy an installation ID or enable repository-write/autofix permissions.

#### Scenario: Missing App configuration
- **WHEN** App ID, slug, client credentials, signing key or callback configuration is unavailable
- **THEN** setup identifies the missing operator prerequisite without exposing credentials or presenting the App as installable

### Requirement: Verified multi-stage installation handoff
Fed SHALL persist installation setup and user authorization as distinct stages with separate single-use state, bind completion to the authorized browser and initiating service, and verify the installation against both the App and authorizing user before activation. Callback installation IDs SHALL be treated as untrusted hints.

#### Scenario: Installation returns successfully
- **WHEN** GitHub returns from organization/repository selection and user authorization proves access to the selected Fed App installation
- **THEN** Fed shows the verified account and finalizes or resumes the originating connection without manual ID entry

#### Scenario: Substitution or replay
- **WHEN** an attacker substitutes another installation ID, replays a consumed stage or completes with another actor/session
- **THEN** Fed rejects completion and does not bind credentials or resources

#### Scenario: Existing installation selection
- **WHEN** a user selects an existing installation
- **THEN** only installations verified accessible to that user and belonging to the configured Fed App can be bound

### Requirement: Pending approval and revocation recovery
Fed SHALL distinguish administrator approval pending, cancellation, missing repository permissions, suspended installation and removed installation, and SHALL provide resumable next actions.

#### Scenario: Administrator approval required
- **WHEN** installation requires organization approval that has not been granted
- **THEN** setup remains pending with an approval/retry action and does not claim verified access

#### Scenario: Installation removed after setup
- **WHEN** the App installation is removed or suspended
- **THEN** subsequent access checks and collection fail explicitly without using a global GitHub token and retained evidence remains available

### Requirement: Repository grant and evidence selection separation
Fed SHALL restrict repository discovery and reads to the verified installation grant. Choosing all or selected repositories in GitHub SHALL NOT automatically add assessment scope. Fed SHALL retain explicit source selection, full commit resolution and evidence path configuration.

#### Scenario: Installation authorizes several repositories
- **WHEN** the callback completes
- **THEN** the user can discover permitted repositories and explicitly select sources while no evidence is gathered or requirement marked complete solely by installation
