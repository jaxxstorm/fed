package connections

import (
	"context"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func (s *Service) ProviderSetupReadiness(ctx context.Context, a auth.Actor, service string, provider Provider) (SetupReadiness, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return SetupReadiness{}, err
	}
	strategy, err := StandardStrategy(provider)
	if err != nil {
		return SetupReadiness{}, err
	}
	out := s.SetupReadiness[provider]
	out.Provider, out.Strategy, out.Version = provider, strategy, "provider-setup/v1"
	out.MissingConfiguration = append([]string{}, out.MissingConfiguration...)
	if s.Credentials.Keys == nil {
		out.MissingConfiguration = append(out.MissingConfiguration, "KSI_CONNECTION_KEYS", "KSI_CONNECTION_KEY_ID")
	}
	adapter, err := s.Registry.Adapter(provider)
	if err != nil {
		out.ErrorCode = "missing_application_configuration"
		out.Ready = false
		return out, nil
	}
	out.Ready = len(out.MissingConfiguration) == 0 && out.ErrorCode == ""
	if provider == AWS && out.Ready {
		p, ok := adapter.(AWSSetupProvider)
		if !ok {
			out.Ready = false
			out.ErrorCode = "missing_application_configuration"
			return out, nil
		}
		call, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if _, err = p.SetupPrincipal(call); err != nil {
			out.Ready = false
			out.ErrorCode = "fed_runtime_role_unverified"
			if code, ok := err.(interface{ RecoveryCode() string }); ok {
				out.ErrorCode = code.RecoveryCode()
			}
		}
	}
	return out, nil
}
