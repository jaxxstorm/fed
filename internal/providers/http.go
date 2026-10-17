package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/jaxxstorm/fed/internal/connections"
	"golang.org/x/oauth2"
)

type oauthTransport struct{ client *connections.ProviderHTTP }

func (t oauthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t.client.Do(r) }
func oauthContext(ctx context.Context, client *connections.ProviderHTTP) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: oauthTransport{client}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
}
func requestJSON(ctx context.Context, client *connections.ProviderHTTP, method, endpoint, token string, input, output any) (string, error) {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return "", connections.ErrInvalid
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", connections.ErrInvalid
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return "", ProviderError("provider_error")
	}
	data, err := connections.ReadProviderBody(response.Body)
	if err != nil {
		return "", ProviderError("artifact_too_large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch response.StatusCode {
		case 401:
			return "", ProviderError("grant_expired")
		case 403:
			if response.Header.Get("X-RateLimit-Remaining") == "0" {
				return "", ProviderError("rate_limited")
			}
			if req.URL.Host == "admin.googleapis.com" {
				var failure struct {
					Error struct {
						Errors []struct {
							Reason string `json:"reason"`
						} `json:"errors"`
					} `json:"error"`
				}
				if json.Unmarshal(data, &failure) == nil {
					for _, item := range failure.Error.Errors {
						switch item.Reason {
						case "domainPolicy", "accessNotConfigured", "adminPolicyEnforced":
							return "", ProviderError("workspace_policy_restricted")
						}
					}
				}
			}
			return "", ProviderError("insufficient_permissions")
		case 404:
			return "", ProviderError("resource_unavailable")
		case 429:
			return "", ProviderError("rate_limited")
		default:
			return "", ProviderError("provider_error")
		}
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return "", ProviderError("invalid_provider_response")
	}
	return response.Header.Get("ETag"), nil
}
