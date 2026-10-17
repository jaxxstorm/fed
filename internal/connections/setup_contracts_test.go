package connections

import (
	"errors"
	"testing"
	"time"
)

func TestSetupCannotSkipVerificationOrRevive(t *testing.T) {
	now := time.Now().UTC()
	s := Setup{Step: SetupConfigure, Revision: 1, ExpiresAt: now.Add(SetupLifetime)}
	if _, err := s.Advance(SetupComplete, 1, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("skipped verification", err)
	}
	s, err := s.Advance(SetupAuthorize, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Advance(SetupConfirm, 2, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("unverified identity", err)
	}
	s.VerifiedAccountID = "customer"
	s, err = s.Advance(SetupConfirm, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Advance(SetupComplete, 3, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("missing connection", err)
	}
	s.ConnectionID = "connection"
	s, err = s.Advance(SetupComplete, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Advance(SetupConfigure, 4, now); !errors.Is(err, ErrInactive) {
		t.Fatal("revived completed setup", err)
	}
}

func TestSetupExpiryRevisionAndRetry(t *testing.T) {
	now := time.Now().UTC()
	s := Setup{Step: SetupConfirm, Revision: 3, VerifiedAccountID: "customer", ExpiresAt: now.Add(time.Hour)}
	if _, err := s.Advance(SetupConfigure, 2, now); !errors.Is(err, ErrConflict) {
		t.Fatal("stale write", err)
	}
	retry, err := s.Advance(SetupConfigure, 3, now)
	if err != nil || retry.VerifiedAccountID != "" {
		t.Fatal("retry retained verification", err)
	}
	if _, err = s.Advance(SetupExpired, 3, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("premature expiry", err)
	}
	if _, err = s.Advance(SetupConfigure, 3, s.ExpiresAt); !errors.Is(err, ErrInactive) {
		t.Fatal("expired setup accepted", err)
	}
	expired, err := s.Advance(SetupExpired, 3, s.ExpiresAt)
	if err != nil || !expired.Terminal() {
		t.Fatal("expiry failed", err)
	}
}

func TestStandardSetupStrategies(t *testing.T) {
	for provider, want := range map[Provider]AccessStrategy{AWS: DelegatedRole, Workspace: WorkspaceConsent, GitHub: GitHubInstallation} {
		if got, err := StandardStrategy(provider); err != nil || got != want {
			t.Fatal(provider, got, err)
		}
	}
	for _, provider := range []Provider{Kubernetes, Documents, "unknown"} {
		if _, err := StandardStrategy(provider); !errors.Is(err, ErrInvalid) {
			t.Fatal("legacy mode promoted to standard setup", provider, err)
		}
	}
}
