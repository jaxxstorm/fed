package providers

import (
	"encoding/json"
	"os"

	"github.com/jaxxstorm/fed/internal/connections"
)

// FromEnv exposes only successfully configured adapters. Missing applications
// remain visible in the catalog with server_adapter_available=false.
func FromEnv(get func(string) string) (map[connections.Provider]connections.Adapter, error) {
	adapters := map[connections.Provider]connections.Adapter{connections.AWS: AWS{}}
	if get("KSI_KUBERNETES_ALLOWED_SERVERS") != "" || get("KSI_KUBERNETES_CONTEXTS") != "" {
		var servers, contexts []string
		if raw := get("KSI_KUBERNETES_ALLOWED_SERVERS"); raw != "" && json.Unmarshal([]byte(raw), &servers) != nil {
			return nil, ProviderError("kubernetes_endpoint_configuration_invalid")
		}
		if raw := get("KSI_KUBERNETES_ALLOWED_CONTEXTS"); raw != "" {
			if json.Unmarshal([]byte(raw), &contexts) != nil {
				return nil, ProviderError("kubernetes_context_configuration_invalid")
			}
		}
		var legacyContexts, profiles map[string]string
		if raw := get("KSI_KUBERNETES_CONTEXTS"); raw != "" {
			if json.Unmarshal([]byte(raw), &legacyContexts) != nil {
				return nil, ProviderError("kubernetes_context_configuration_invalid")
			}
			for _, name := range legacyContexts {
				contexts = append(contexts, name)
			}
		}
		if raw := get("KSI_AWS_ACCOUNT_PROFILES"); raw != "" {
			if json.Unmarshal([]byte(raw), &profiles) != nil {
				return nil, ProviderError("aws_profile_configuration_invalid")
			}
		}
		adapters[connections.Kubernetes] = Kubernetes{AllowedServers: servers, AllowedContexts: contexts, LegacyProfiles: profiles}
	}
	if get("KSI_GITHUB_APP_ID") != "" || get("KSI_GITHUB_CLIENT_ID") != "" || get("KSI_GITHUB_PRIVATE_KEY_FILE") != "" {
		key, err := os.ReadFile(get("KSI_GITHUB_PRIVATE_KEY_FILE"))
		if err != nil {
			return nil, ProviderError("github_application_configuration_invalid")
		}
		defer clear(key)
		adapter, err := NewGitHub(GitHubConfig{AppID: get("KSI_GITHUB_APP_ID"), Slug: get("KSI_GITHUB_APP_SLUG"), ClientID: get("KSI_GITHUB_CLIENT_ID"), ClientSecret: get("KSI_GITHUB_CLIENT_SECRET"), PrivateKey: key})
		if err != nil {
			return nil, ProviderError("github_application_configuration_invalid")
		}
		adapters[connections.GitHub] = adapter
	}
	if get("KSI_GOOGLE_CLIENT_ID") != "" || get("KSI_GOOGLE_CLIENT_SECRET") != "" {
		adapter, err := NewWorkspace(get("KSI_GOOGLE_CLIENT_ID"), get("KSI_GOOGLE_CLIENT_SECRET"), nil)
		if err != nil {
			return nil, ProviderError("google_application_configuration_invalid")
		}
		adapters[connections.Workspace] = adapter
	}
	return adapters, nil
}
