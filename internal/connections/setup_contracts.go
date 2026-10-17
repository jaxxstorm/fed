package connections

import (
	"encoding/json"
	"time"
)

// AccessStrategy selects credential semantics independently of execution mode.
// Legacy bindings retain their existing profile/optional-role semantics.
type AccessStrategy string

const (
	LegacyAccess       AccessStrategy = "legacy"
	DelegatedRole      AccessStrategy = "delegated_role"
	WorkspaceConsent   AccessStrategy = "workspace_consent"
	GitHubInstallation AccessStrategy = "github_installation"
)

func StandardStrategy(p Provider) (AccessStrategy, error) {
	switch p {
	case AWS:
		return DelegatedRole, nil
	case Workspace:
		return WorkspaceConsent, nil
	case GitHub:
		return GitHubInstallation, nil
	default:
		return "", ErrInvalid
	}
}

type SetupStep string

const (
	SetupPrerequisites SetupStep = "prerequisites"
	SetupConfigure     SetupStep = "configure"
	SetupAuthorize     SetupStep = "authorize"
	SetupConfirm       SetupStep = "confirm"
	SetupComplete      SetupStep = "complete"
	SetupCancelled     SetupStep = "cancelled"
	SetupExpired       SetupStep = "expired"
)

const SetupLifetime = 24 * time.Hour

// SetupInput contains only declarations, never provider grants or external IDs.
type SetupInput struct {
	Name      string `json:"name"`
	AccountID string `json:"account_id,omitempty"`
	RoleARN   string `json:"role_arn,omitempty"`
}

type Setup struct {
	ErrorCode          string         `json:"error_code,omitempty"`
	ID                 string         `json:"id"`
	ServiceID          string         `json:"service_id"`
	Provider           Provider       `json:"provider"`
	Strategy           AccessStrategy `json:"strategy"`
	Step               SetupStep      `json:"step"`
	Revision           int            `json:"revision"`
	Input              SetupInput     `json:"input"`
	VerifiedAccountID  string         `json:"verified_account_id,omitempty"`
	ConnectionID       string         `json:"connection_id,omitempty"`
	ExpectedGeneration int64          `json:"-"`
	ActorID            string         `json:"actor_id"`
	TokenHash          string         `json:"-"`
	ExpiresAt          time.Time      `json:"expires_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type SetupError string

func (e SetupError) Error() string        { return string(e) }
func (e SetupError) RecoveryCode() string { return string(e) }

func (s Setup) Terminal() bool {
	return s.Step == SetupComplete || s.Step == SetupCancelled || s.Step == SetupExpired
}

// Advance is internal: clients cannot claim authorization or verified identity.
// Persistence must authorize, lock, and invalidate old attempts on each revision.
func (s Setup) Advance(next SetupStep, expected int, now time.Time) (Setup, error) {
	if s.Revision != expected {
		return s, ErrConflict
	}
	if s.Terminal() {
		return s, ErrInactive
	}
	if !now.Before(s.ExpiresAt) && next != SetupExpired {
		return s, ErrInactive
	}
	allowed := next == SetupCancelled || next == SetupExpired && !now.Before(s.ExpiresAt)
	switch s.Step {
	case SetupPrerequisites:
		allowed = allowed || next == SetupConfigure
	case SetupConfigure:
		allowed = allowed || next == SetupAuthorize
	case SetupAuthorize:
		allowed = allowed || next == SetupConfigure || next == SetupConfirm && s.VerifiedAccountID != ""
	case SetupConfirm:
		allowed = allowed || next == SetupConfigure || next == SetupComplete && s.VerifiedAccountID != "" && s.ConnectionID != ""
	}
	if !allowed {
		return s, ErrInvalid
	}
	if next == SetupConfigure {
		s.VerifiedAccountID = ""
	}
	s.Step, s.Revision, s.UpdatedAt = next, s.Revision+1, now.UTC()
	return s, nil
}

type BeginSetupInput struct {
	ConnectionID     string     `json:"connection_id,omitempty"`
	ExpectedRevision int        `json:"expected_revision,omitempty"`
	Generation       int64      `json:"generation,omitempty"`
	ServiceID        string     `json:"service_id"`
	Provider         Provider   `json:"provider"`
	IdempotencyKey   string     `json:"idempotency_key"`
	Input            SetupInput `json:"input"`
}

type SetupOperationInput struct {
	ServiceID        string `json:"service_id"`
	SetupID          string `json:"setup_id"`
	ExpectedRevision int    `json:"expected_revision"`
	IdempotencyKey   string `json:"idempotency_key"`
}

type UpdateSetupInput struct {
	SetupOperationInput
	Input SetupInput `json:"input"`
}

type SetupReadiness struct {
	ErrorCode            string         `json:"error_code,omitempty"`
	Provider             Provider       `json:"provider"`
	Ready                bool           `json:"ready"`
	Version              string         `json:"version"`
	Strategy             AccessStrategy `json:"strategy"`
	MissingConfiguration []string       `json:"missing_configuration"`
	Permissions          []string       `json:"permissions"`
	OperatorActions      []string       `json:"operator_actions"`
	CustomerActions      []string       `json:"customer_actions"`
}

// SetupMaterial is returned only by connect-authorized material endpoints,
// never embedded in ordinary setup status, evidence, or operation receipts.
type SetupMaterial struct {
	RoleARN      string          `json:"role_arn"`
	Version      string          `json:"version"`
	TrustPolicy  json.RawMessage `json:"trust_policy"`
	ReadPolicy   json.RawMessage `json:"read_policy"`
	CLI          string          `json:"cli"`
	Instructions []string        `json:"instructions"`
}

func (SetupMaterial) String() string     { return "[restricted setup material]" }
func (m SetupMaterial) GoString() string { return m.String() }
