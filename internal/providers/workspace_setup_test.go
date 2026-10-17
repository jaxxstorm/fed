package providers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jaxxstorm/fed/internal/connections"
)

func TestWorkspaceSetupRecoveryCodes(t *testing.T) {
	for _, scenario := range []struct {
		name, token, customer, want string
		status                      int
	}{
		{"initial", `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer"}`, `{"id":"C123"}`, "", 200},
		{"offline", `{"access_token":"access","token_type":"Bearer"}`, `{"id":"C123"}`, "offline_consent_required", 200},
		{"identity", `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer"}`, `{}`, "customer_identity_permission_required", 403},
		{"policy", `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer"}`, `{"error":{"errors":[{"reason":"domainPolicy"}]}}`, "workspace_policy_restricted", 403},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			p, err := NewWorkspace("client", "secret", roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/token" {
					return jsonResponse(200, scenario.token), nil
				}
				return jsonResponse(scenario.status, scenario.customer), nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			grant, err := p.ExchangeAuthorization(context.Background(), connections.Connection{Provider: connections.Workspace, Strategy: connections.WorkspaceConsent}, connections.AuthorizationClaim{RedirectURI: "https://fed.example/connections/callback/google_workspace", Verifier: connections.NewCredentials(map[string]string{"verifier": "test"})}, connections.AuthorizationResponse{Code: "code"})
			if scenario.want == "" {
				if err != nil || grant.AccountID != "C123" {
					t.Fatal("consent-first identity", err)
				}
			} else if err == nil || err.Error() != scenario.want {
				t.Fatal("recovery", err, scenario.want)
			}
		})
	}
}
