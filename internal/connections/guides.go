package connections

type SetupGuide struct {
	Version            string   `json:"version"`
	Execution          string   `json:"execution"`
	Steps              []string `json:"steps"`
	ConfigurationNames []string `json:"configuration_names"`
	CallbackPath       string   `json:"callback_path,omitempty"`
}

type Recovery struct {
	Code        string `json:"code"`
	Instruction string `json:"instruction"`
}

func RecoveryCatalog() []Recovery {
	return []Recovery{
		{"missing_application_configuration", "Complete the provider registration steps and configure the listed deployment secrets."},
		{"authorization_cancelled", "Resume setup and authorize the intended provider account."},
		{"authorization_expired", "Start a new authorization attempt from the connection."},
		{"insufficient_permissions", "Grant the read permissions listed for the affected resource kind, then test access."},
		{"wrong_tenant", "Use credentials for the configured account; changing a display name cannot change account identity."},
		{"grant_expired", "Renew this connection or complete a new browser authorization."},
		{"grant_revoked", "Reconnect this account and test its access before collecting."},
		{"rate_limited", "Resume discovery after the provider retry interval; successful pages are retained."},
		{"resource_unavailable", "Check provider resource selection and access; historical evidence is retained."},
		{"profile_sign_in_required", "Sign in to the named AWS profile in the server execution environment, then test access."},
		{"role_trust_denied", "Check the role trust policy, server principal and configured external ID, then test access."},
		{"cluster_identity_mismatch", "Check the configured cluster endpoint and CA against the intended cluster."},
		{"unsupported_kubeconfig", "Upload embedded certificates or bearer credentials with TLS verification; executable helpers and local file references are not supported."},
		{"provider_error", "Retry access testing and inspect the provider operation status."},
	}
}

func providerDefinitions() []ProviderDefinition {
	return []ProviderDefinition{
		{ID: GitHub, Name: "GitHub", AccessModes: []AccessMode{ServerManaged, AgentManaged}, ResourceKinds: []string{"repository"}, EvidenceOperations: []string{"pinned_file"}, Permissions: []string{"Metadata: read", "Contents: read"}, Guide: SetupGuide{
			Version: "1", Execution: "Server uses GitHub App installation tokens; agent-managed discovery uses the agent's own tools and credentials.",
			ConfigurationNames: []string{"KSI_PUBLIC_URL", "KSI_CONNECTION_KEY_ID", "KSI_CONNECTION_KEYS", "KSI_GITHUB_APP_ID", "KSI_GITHUB_APP_SLUG", "KSI_GITHUB_CLIENT_ID", "KSI_GITHUB_CLIENT_SECRET", "KSI_GITHUB_PRIVATE_KEY_FILE"}, CallbackPath: "/connections/callback/github",
			Steps: []string{"Register a GitHub App in the intended owner account with Metadata and Contents read-only repository permissions.", "Set its callback and setup URL using the configured public origin; configure App ID, slug, OAuth client and private key in the server deployment.", "Authorize your GitHub identity and install the App for the intended organization and repositories. The server verifies your access to that installation.", "Test access and discover repositories. Select repository sources with full commit hashes and evidence paths.", "Collect pinned files. Renew installation access when needed; uninstalling the App is a separate provider-side action."},
		}},
		{ID: Workspace, Name: "Google Workspace", AccessModes: []AccessMode{ServerManaged, AgentManaged}, ResourceKinds: []string{"org_unit", "group"}, EvidenceOperations: []string{"directory_snapshot"}, Permissions: []string{"https://www.googleapis.com/auth/admin.directory.customer.readonly", "https://www.googleapis.com/auth/admin.directory.orgunit.readonly", "https://www.googleapis.com/auth/admin.directory.group.readonly"}, Guide: SetupGuide{
			Version: "1", Execution: "Server uses administrator-authorized offline OAuth; agents may submit attributed Directory discovery using their own access.",
			ConfigurationNames: []string{"KSI_PUBLIC_URL", "KSI_CONNECTION_KEY_ID", "KSI_CONNECTION_KEYS", "KSI_GOOGLE_CLIENT_ID", "KSI_GOOGLE_CLIENT_SECRET"}, CallbackPath: "/connections/callback/google_workspace",
			Steps: []string{"Enable Admin SDK in your Google Cloud project. Configure the OAuth consent audience and test users or production publishing requirements.", "Register a Web application OAuth client with the exact callback URI shown by the service and configure its client ID and secret on the server.", "Have a Workspace administrator allow the requested read-only Directory scopes under the organization's API access policy and grant offline consent.", "Test customer identity, then discover and select organizational units and groups. Missing permissions remain visible per resource kind.", "Collect immutable Directory metadata snapshots. This integration does not cover Gmail, Drive, devices, membership or audit reports. Renew or reauthorize if the offline grant expires."},
		}},
		{ID: AWS, Name: "AWS", AccessModes: []AccessMode{ServerEnvironment, ServerManaged, AgentManaged}, ResourceKinds: []string{"account_region", "eks_cluster"}, EvidenceOperations: []string{"aws_configuration"}, Permissions: []string{"sts:GetCallerIdentity", "sts:AssumeRole (role mode)", "eks:ListClusters", "eks:DescribeCluster", "rds:DescribeDBInstances", "rds:DescribeDBClusters", "elasticache:DescribeCacheClusters", "elasticache:DescribeReplicationGroups", "guardduty:ListDetectors", "guardduty:GetDetector"}, Guide: SetupGuide{
			Version: "1", Execution: "Profiles and role sessions execute on the server. Agent-managed inventory executes with the agent's own AWS credentials. Only us-east-1 is supported.", ConfigurationNames: []string{"KSI_AWS_ACCOUNT_PROFILES"},
			Steps: []string{"Choose a named profile installed in the server environment or a role ARN in the intended account. Record the expected account ID and us-east-1 region.", "For assume-role access, allow the displayed server principal in the role trust policy and configure the required external ID. The server must also be allowed to assume that role.", "Grant discovery and enabled collectors only the required read permissions. IAM changes are performed by your administrator, not this service.", "Test STS identity before discovering the account/region and EKS clusters; other accounts require separately verified connections.", "Select sources and collect. Renew expired SSO with aws sso login --profile <configured-profile> in the server environment, or correct role trust and retest."},
		}},
		{ID: Kubernetes, Name: "Kubernetes", AccessModes: []AccessMode{ServerEnvironment, ServerManaged, AgentManaged}, ResourceKinds: []string{"cluster", "namespace"}, EvidenceOperations: []string{"kubernetes_configuration"}, Permissions: []string{"get/list namespaces", "get/list pods and networkpolicies in selected namespaces", "get daemonsets for cluster-wide enforcement coverage"}, Guide: SetupGuide{
			Version: "1", Execution: "Server contexts run on the server; uploaded kubeconfig is parsed as data. Agents may use their own cluster access.", ConfigurationNames: []string{"KSI_KUBERNETES_CONTEXTS", "KSI_KUBERNETES_ALLOWED_SERVERS", "KSI_KUBERNETES_ALLOWED_CONTEXTS", "KSI_CONNECTION_KEY_ID", "KSI_CONNECTION_KEYS"},
			Steps: []string{"Choose an operator-installed context or upload kubeconfig using the authenticated browser. Configure the stable cluster ID, API endpoint and trusted TLS CA.", "Allow the endpoint in the server deployment. Uploaded configuration must contain embedded certificates or bearer credentials, without exec/auth-provider commands, local file references or disabled TLS verification.", "Grant read-only RBAC for namespace discovery and supported evidence reads. Namespace-scoped roles cannot establish cluster-wide coverage.", "Test the cluster binding, then discover namespaces. If listing is denied, configure a known namespace and test a direct authorized read.", "Select cluster-wide or specific namespace sources. Rotate expired certificates/tokens through renewal and retest; no Kubernetes resources are modified."},
		}},
		{ID: Documents, Name: "Documents and operational records", AccessModes: []AccessMode{Manual, AgentManaged}, ResourceKinds: []string{"document", "operational_records"}, EvidenceOperations: []string{"agent_artifact"}, Permissions: []string{}, Guide: SetupGuide{Version: "1", Execution: "External agents retrieve evidence using their own tools; the service does not dispatch agents.", ConfigurationNames: []string{}, Steps: []string{"Declare the HTTPS source location and freshness policy.", "Submit scoped artifacts with observation times, provenance and limitations."}}},
	}
}
