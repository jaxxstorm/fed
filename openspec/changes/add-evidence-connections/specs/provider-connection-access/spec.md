## ADDED Requirements

### Requirement: Four first-class provider integrations
The provider registry SHALL include GitHub, Google Workspace, AWS and Kubernetes with setup instructions, supported access modes, resource kinds, required read permissions, access testing, renewal and collection capabilities. Each provider SHALL distinguish unsupported capabilities and missing application configuration from successful connection status.

#### Scenario: Provider application registration is absent
- **WHEN** an operator chooses GitHub or Google Workspace before its application credentials are configured
- **THEN** setup displays the required registration steps, callback URLs and configuration fields and does not offer a falsely successful authorization

### Requirement: GitHub installation access powers collection
GitHub connections SHALL support GitHub App installations with read-only repository metadata and contents access. The system SHALL verify the authorizing user's access to the installation and bind the verified account identity. Repository discovery and existing server GitHub collectors SHALL use connection-scoped installation tokens with automatic short-lived renewal. A connection-bound source SHALL NOT silently fall back to unrelated process-wide credentials.

#### Scenario: Resolve missing GitHub access
- **WHEN** an operator installs the configured App for the intended organization, selects a repository source and retries its pinned file read
- **THEN** the collector uses that connection's installation credentials and returns evidence or a specific permission/path error rather than requiring a global GitHub token

#### Scenario: Installation ID is substituted
- **WHEN** a callback supplies an installation ID not accessible to the authorizing identity or inconsistent with the bound connection account
- **THEN** the connection is not authorized and the attempted binding is rejected

#### Scenario: Installation loses repository access
- **WHEN** a repository is removed from the installation's access
- **THEN** subsequent verification or collection marks that resource unavailable or denied while retaining prior evidence and access to unaffected repositories

### Requirement: Google Workspace Directory authorization and evidence
Google Workspace SHALL support OAuth web-server authorization with offline renewal and read-only Admin SDK Directory customer, org-unit and group scopes. Customer read access SHALL establish the actual Workspace customer identity before activation. The system SHALL discover organizational units and groups and produce immutable JSON metadata artifacts for selected supported resources. The UI and MCP SHALL state this Directory coverage without implying access to Gmail, Drive, devices or audit reports.

#### Scenario: Workspace administrator connects
- **WHEN** an authorized Workspace administrator grants the configured Directory scopes
- **THEN** the connection identifies the verified customer, lists accessible org units/groups and permits supported selected-resource reads

#### Scenario: Consent grants only some capabilities
- **WHEN** provider permissions permit groups but not organizational units
- **THEN** discovery retains successful group results and explains the missing org-unit permission as partial coverage

#### Scenario: Refresh token is invalid
- **WHEN** renewal returns an invalid or revoked refresh grant
- **THEN** the connection requests reauthorization and retains existing source/evidence history without repeated blind retries or a false connected state

### Requirement: AWS profile and assume-role integration
AWS connections SHALL support validated server profiles, assume-role bindings and agent-managed access. Server-managed access SHALL verify the actual account with STS and resolve credentials per connection for discovery and existing collectors. Guides SHALL identify the execution environment, role trust requirements, external-ID configuration when used and required read permissions. All AWS discovery and collection SHALL remain limited to `us-east-1`; the service SHALL NOT mutate IAM or infrastructure.

#### Scenario: Connect an AWS account using a role
- **WHEN** the operator supplies a permitted role binding and the server can assume it
- **THEN** the system validates the account identity, discovers allowed regional and EKS resources and uses that connection for supported collection

#### Scenario: Wrong account or unsupported region
- **WHEN** credentials resolve to a different account than configured or an operation requests a region other than us-east-1
- **THEN** discovery and collection are rejected without changing scope or reporting verified access

#### Scenario: AWS sign-in expires
- **WHEN** a server profile session expires or role assumption fails
- **THEN** the connection identifies the profile/account and execution environment, supplies the appropriate sign-in or trust-policy recovery and retains prior successful evidence

### Requirement: Kubernetes cluster and namespace integration
Kubernetes connections SHALL support trusted server contexts, securely supplied kubeconfig credentials and agent-managed access. They SHALL verify the configured cluster endpoint/TLS identity, discover namespaces and support explicit cluster-wide or namespace-selected source boundaries, including standalone non-AWS clusters. Read-only collector planning and evidence ingestion SHALL enforce those boundaries and report missing cluster-level coverage honestly.

#### Scenario: Discover and select namespaces
- **WHEN** a connection can list namespaces in a configured cluster and the operator selects two namespaces
- **THEN** the selected namespaces become stable child sources and new namespace-scoped collection does not read unrelated namespaces

#### Scenario: Listing is denied but a namespace is readable
- **WHEN** namespace enumeration is forbidden and a configured namespace can be read directly
- **THEN** discovery is labeled partial and the known namespace can remain a usable source without claiming full cluster coverage

#### Scenario: Uploaded kubeconfig contains executable authentication
- **WHEN** a kubeconfig upload contains exec commands, local file references, disabled TLS verification or an unapproved endpoint
- **THEN** setup rejects the unsupported configuration with field-specific instructions and executes no supplied command

#### Scenario: Namespace is recreated
- **WHEN** a namespace with the same name is recreated with a new UID
- **THEN** discovery distinguishes the new resource identity and preserves the old source's evidence history

### Requirement: Bound authorization and credential renewal
Provider authorization attempts SHALL be short-lived, single-use and bound to provider, service, connection generation and authorized actors. Callback handling SHALL validate state, redirect URI, expiry and applicable PKCE, and SHALL recheck authorization before saving grants. Concurrent refresh SHALL avoid overwriting newer credentials and SHALL preserve a valid stored refresh token when a response omits a replacement.

#### Scenario: Replayed or stale callback
- **WHEN** an authorization code/state is replayed, expired or belongs to another connection generation
- **THEN** the grant is not stored and the caller receives a recoverable authorization error

#### Scenario: Evidence read exceeds artifact limits
- **WHEN** a supported provider snapshot exceeds the existing artifact size limit
- **THEN** the operation reports the limit or an explicitly partial result without silently truncating evidence or claiming complete coverage
