## ADDED Requirements

### Requirement: Connection-first source navigation
The Sources workspace SHALL provide connection cards/list, connection details with child sources and an all-sources view. Connection summaries SHALL show provider/account, access mode, attributed health/time, discovered and selected resource counts and a relevant next action. Existing source links SHALL resolve to their corresponding source or imported connection context.

#### Scenario: Open an existing service
- **WHEN** an operator opens Sources after migration
- **THEN** imported sources are grouped under understandable connections, with unknown access labeled explicitly and existing source details still reachable

### Requirement: Step-by-step setup for each provider
The add-connection workflow SHALL guide GitHub, Google Workspace, AWS and Kubernetes setup from prerequisites through authorization/configuration, testing, discovery and source selection. Guides SHALL expose the correct execution environment, read permissions, app registration or credential prerequisites, exact callback URLs where relevant, and supported source/evidence types. The UI SHALL use labeled controls and selectable resources rather than raw JSON.

#### Scenario: First GitHub setup
- **WHEN** the operator chooses GitHub and no App is configured
- **THEN** the workflow explains how to register/configure the App, install it for the intended repositories and return to test and discover sources

#### Scenario: Configure AWS or Kubernetes
- **WHEN** the operator selects AWS or Kubernetes
- **THEN** provider-specific profile/role or context/kubeconfig instructions and fields are shown, followed by access tests and account/cluster/namespace selection

#### Scenario: Google Workspace setup is restricted by administrator policy
- **WHEN** Workspace authorization is blocked by an admin policy or missing Directory permission
- **THEN** the guide identifies the required administrator action and preserves the setup progress

### Requirement: Usable lifecycle and recovery controls
Authorized users SHALL be able to configure, renew, disconnect and delete connections and configure, disable, refresh, delete or explicitly restore sources. Controls SHALL distinguish credential renewal, inventory discovery and evidence refresh, show pending/success/failure outcomes, preserve inputs on conflicts and identify effects on child sources and retained history. Access-error links SHALL open the relevant connection or preselected provider setup.

#### Scenario: GitHub collector reports missing access
- **WHEN** the operator follows the recovery action from the failed collection
- **THEN** the correct GitHub connection or setup guide opens with the affected repository context and a working test/retry path

#### Scenario: Partial discovery
- **WHEN** some resource kinds can be discovered and others fail
- **THEN** successful resources remain selectable and failed kinds show their own next steps rather than a blanket connected/failed claim

### Requirement: Searchable responsive source selection
Source discovery and selection SHALL support server-side pagination, search and selection-state filters, meaningful labels, provider identity, access/last-seen metadata and scope differences. The workflow SHALL preserve selections across pages and prevent accidental selection of unseen resources. It SHALL be keyboard-accessible and usable at 390px, 768px and 1440px widths.

#### Scenario: Select across pages
- **WHEN** the operator selects resources on multiple discovery pages
- **THEN** the final selection identifies exactly those resources and any required scope/pinning fields before applying them

### Requirement: Honest setup and health states
The UI SHALL distinguish missing configuration, awaiting authorization, cancelled/expired authorization, verified access, agent-reported access, stale checks, partial discovery, no discovered resources, filtered-empty results and disconnected/deleted connections. Authorization success SHALL NOT imply completed evidence collection or assessment readiness.

#### Scenario: Consent finishes with no accessible repositories
- **WHEN** provider authorization succeeds but no repositories are available to the installation
- **THEN** the UI shows the verified identity, zero available sources and repository-selection guidance instead of evidence completion
