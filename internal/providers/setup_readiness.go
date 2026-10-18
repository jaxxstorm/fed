package providers

import "github.com/jaxxstorm/fed/internal/connections"

func SetupReadiness(get func(string) string) map[connections.Provider]connections.SetupReadiness {
	registry, _ := connections.NewRegistry(nil)
	result := map[connections.Provider]connections.SetupReadiness{}
	required := map[connections.Provider][]string{
		connections.AWS:       {"KSI_AWS_FED_ROLE_ARN"},
		connections.Workspace: {"KSI_GOOGLE_CLIENT_ID", "KSI_GOOGLE_CLIENT_SECRET", "KSI_PUBLIC_URL"},
		connections.GitHub:    {"KSI_GITHUB_APP_ID", "KSI_GITHUB_APP_SLUG", "KSI_GITHUB_CLIENT_ID", "KSI_GITHUB_CLIENT_SECRET", "KSI_GITHUB_PRIVATE_KEY_FILE", "KSI_PUBLIC_URL"},
	}
	for provider, keys := range required {
		definition, _ := registry.Provider(provider)
		out := connections.SetupReadiness{Provider: provider, Permissions: definition.Permissions, MissingConfiguration: []string{}, OperatorActions: []string{}, CustomerActions: []string{}}
		for _, key := range keys {
			if key == "KSI_GITHUB_PRIVATE_KEY_FILE" && get("KSI_GITHUB_PRIVATE_KEY") != "" {
				continue
			}
			if get(key) == "" {
				out.MissingConfiguration = append(out.MissingConfiguration, key)
			}
		}
		switch provider {
		case connections.AWS:
			out.OperatorActions = []string{"Assign Fed an application task role, distinct from the ECS image-pulling execution role. Allow iam:GetRole on that role and sts:AssumeRole on approved customer role ARNs."}
			out.CustomerActions = []string{"Use customer administrator credentials to create the generated role and trust policy. Fed only reads us-east-1 resources."}
		case connections.Workspace:
			out.OperatorActions = []string{"Register a Google OAuth web client, enable Admin SDK in its Cloud project, configure consent audience/test users or publishing, and register " + get("KSI_PUBLIC_URL") + "/connections/callback/google_workspace."}
			out.CustomerActions = []string{"Sign in as a Workspace administrator and approve the read-only Directory customer, organizational-unit and group scopes. Your tenant may require API access approval. This does not collect Gmail, Drive, users or audit logs."}
		case connections.GitHub:
			out.OperatorActions = []string{"Register a GitHub App with Metadata and Contents read permissions. Set setup URL to " + get("KSI_PUBLIC_URL") + "/connections/setup/github and OAuth callback to " + get("KSI_PUBLIC_URL") + "/connections/callback/github. Disable 'Request user authorization during installation' so Fed can bind each redirect stage separately."}
			out.CustomerActions = []string{"Choose your organization and repositories in GitHub, obtain administrator approval if required, then authorize your user account. Repository grants do not automatically expand assessment scope."}
		}
		out.Ready = len(out.MissingConfiguration) == 0
		result[provider] = out
	}
	return result
}
