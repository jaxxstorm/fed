package observation

// Recovery is additive operator guidance. Code remains the machine-readable
// cause; historical observations without this object use the same catalog.
type Recovery struct {
	ConnectionProvider string `json:"connection_provider,omitempty"`
	Code               string `json:"code"`
	Title              string `json:"title"`
	Explanation        string `json:"explanation"`
	Target             string `json:"target"`
	Environment        string `json:"environment"`
	Action             string `json:"action"`
	Retryable          bool   `json:"retryable"`
}

func ExplainError(code, target, environment string) Recovery {
	if environment == "collector_observation" {
		environment = "Server collector"
	} else if environment == "agent_observation" {
		environment = "External agent"
	}
	r := Recovery{Code: code, Target: target, Environment: environment, Title: "Evidence collection could not finish", Explanation: "Inspect the source access and collection diagnostics for this target.", Action: "Open source settings"}
	switch code {
	case "expired_credentials":
		r.ConnectionProvider = "aws"
		r.Title = "AWS sign-in expired"
		r.Explanation = "Refresh AWS sign-in for the configured profile in the collection environment, then retry the read-only collection."
		r.Retryable = true
	case "credentials_or_account_validation_failed":
		r.ConnectionProvider = "aws"
		r.Title = "AWS access could not be verified"
		r.Explanation = "Check the configured AWS profile, sign-in and account binding in the collection environment, then retry."
		r.Retryable = true
	case "github_credentials_missing":
		r.ConnectionProvider = "github"
		r.Title = "GitHub access is missing"
		r.Explanation = "Open Connections → GitHub. Configure the GitHub App, authorize its installation for the affected repositories, and test access. Select full commits and evidence file paths, then rerun collection. Alternatively, an external agent with repository access can submit cited evidence and update the requirement assessment. Existing artifacts do not clear a failed technical check automatically."
		r.Retryable = true
	case "cluster_enumeration_failed", "namespace_enumeration_failed":
		r.ConnectionProvider = "kubernetes"
		r.Title = "Cluster resources could not be listed"
		r.Explanation = "Check the configured cluster context and read access. Use diagnostics to identify the failed API operation before retrying."
		r.Retryable = true
	case "access_denied":
		r.Title = "Read access denied"
		r.Explanation = "Grant the collection identity read access to the affected target, then retry."
		r.Retryable = true
	case "namespace_access_denied":
		r.Title = "Namespace listing denied"
		r.Explanation = "Allow the configured Kubernetes identity to list namespaces on this cluster, then retry the read-only investigation."
		r.Retryable = true
	case "kubernetes_context_binding_missing":
		r.ConnectionProvider = "kubernetes"
		r.Title = "Choose a Kubernetes context"
		r.Explanation = "Map this account, region and cluster to a context in the server's KSI_KUBERNETES_CONTEXTS configuration, or provide the context hint to an external agent."
	case "no_paths_configured":
		r.Title = "Choose repository paths"
		r.Explanation = "Add the repository paths that contain evidence to the saved source scope."
	case "repository_file_unavailable":
		r.ConnectionProvider = "github"
		r.Title = "Repository file is unavailable"
		r.Explanation = "Check read access and confirm the file exists at the pinned commit and path."
		r.Retryable = true
	case "connection_inactive", "connection_credentials_unavailable", "connection_binding_conflict", "connection_access_failed", "connection_adapter_unavailable", "connection_binding_requires_standalone_scope":
		r.Title = "Evidence connection needs attention"
		r.Explanation = "Open Connections to verify the selected source binding and access. Renewing access or discovering inventory does not refresh existing evidence; collect again after recovery."
		r.Action = "Open Connections"
	case "profile_sign_in_required", "role_trust_denied":
		r.ConnectionProvider = "aws"
		r.Title = "Verify provider account access"
		r.Explanation = "Check the connection identity, server profile and role trust before retrying. Successful reads from other targets remain available."
	}
	return r
}
