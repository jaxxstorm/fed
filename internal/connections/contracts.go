// Package connections defines provider access independently of evidence records.
package connections

import (
	"errors"
	"time"
)

var (
	ErrConflict = errors.New("connection revision or generation conflict")
	ErrInvalid  = errors.New("invalid connection operation")
	ErrInactive = errors.New("connection is inactive")
)

type Provider string

const (
	GitHub     Provider = "github"
	Workspace  Provider = "google_workspace"
	AWS        Provider = "aws"
	Kubernetes Provider = "kubernetes"
	Documents  Provider = "documents"
)

type AccessMode string

const (
	ServerManaged     AccessMode = "server_managed"
	ServerEnvironment AccessMode = "server_environment"
	AgentManaged      AccessMode = "agent_managed"
	Manual            AccessMode = "manual"
)

type Lifecycle string

const (
	Active       Lifecycle = "active"
	Disconnected Lifecycle = "disconnected"
	Deleted      Lifecycle = "deleted"
)

type AccessState string

const (
	NotConfigured         AccessState = "not_configured"
	AuthorizationRequired AccessState = "authorization_required"
	Checking              AccessState = "checking"
	Verified              AccessState = "verified"
	PartialAccess         AccessState = "partial"
	Expired               AccessState = "expired"
	Denied                AccessState = "denied"
	AccessError           AccessState = "error"
	AgentReported         AccessState = "agent_reported"
)

type DiscoveryState string

const (
	NeverRun         DiscoveryState = "never_run"
	Running          DiscoveryState = "running"
	Complete         DiscoveryState = "complete"
	PartialDiscovery DiscoveryState = "partial"
	Failed           DiscoveryState = "failed"
)

// Configuration contains declarations only. Credentials belong in the secret store.
type Configuration struct {
	KnownNamespaces []string `json:"known_namespaces,omitempty"`
	AccountID       string   `json:"account_id"`
	InstallationID  string   `json:"installation_id,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	RoleARN         string   `json:"role_arn,omitempty"`
	Region          string   `json:"region,omitempty"`
	Context         string   `json:"context,omitempty"`
	ClusterID       string   `json:"cluster_id,omitempty"`
	Server          string   `json:"server,omitempty"`
}

type Connection struct {
	VerifiedAccountID   string        `json:"verified_account_id,omitempty"`
	ID                  string        `json:"id"`
	ServiceID           string        `json:"service_id"`
	Provider            Provider      `json:"provider"`
	Name                string        `json:"name"`
	Mode                AccessMode    `json:"access_mode"`
	Configuration       Configuration `json:"configuration"`
	Lifecycle           Lifecycle     `json:"lifecycle"`
	Revision            int           `json:"revision"`
	Generation          int64         `json:"generation"`
	CredentialReference string        `json:"-"`
	ActorID             string        `json:"actor_id"`
	UpdatedAt           time.Time     `json:"updated_at"`
	Imported            bool          `json:"imported"`
}

// Tenant uses provider-verified identity when available while retaining the
// original setup declaration separately (for example an organization login).
func (c Connection) Tenant() string {
	if c.VerifiedAccountID != "" {
		return c.VerifiedAccountID
	}
	return c.Configuration.AccountID
}

type AccessHealth struct {
	Principal         string      `json:"principal,omitempty"`
	State             AccessState `json:"state"`
	VerifiedAccountID string      `json:"verified_account_id,omitempty"`
	Generation        int64       `json:"generation"`
	ActorID           string      `json:"actor_id,omitempty"`
	CheckedAt         *time.Time  `json:"checked_at,omitempty"`
	ValidUntil        *time.Time  `json:"valid_until,omitempty"`
	ErrorCode         string      `json:"error_code,omitempty"`
	Limitations       []string    `json:"limitations"`
}

func (h AccessHealth) Stale(now time.Time, generation int64) bool {
	return h.CheckedAt == nil || h.ValidUntil == nil || !now.Before(*h.ValidUntil) || h.Generation != generation
}

type SourceLifecycle string

const (
	Enabled  SourceLifecycle = "enabled"
	Disabled SourceLifecycle = "disabled"
	Retired  SourceLifecycle = "retired"
)

// ResourceKey excludes display names so renames do not duplicate selected sources.
type ResourceKey struct {
	Provider Provider `json:"provider"`
	Tenant   string   `json:"tenant"`
	Kind     string   `json:"kind"`
	ID       string   `json:"id"`
}

type SourceBinding struct {
	ServiceID    string          `json:"service_id"`
	SourceID     string          `json:"source_id"`
	ConnectionID string          `json:"connection_id"`
	Resource     ResourceKey     `json:"resource"`
	Lifecycle    SourceLifecycle `json:"lifecycle"`
	Revision     int             `json:"revision"`
}

type DiscoveredResource struct {
	DefaultBranch string          `json:"default_branch,omitempty"`
	Permissions   map[string]bool `json:"permissions,omitempty"`
	Resource      ResourceKey     `json:"resource"`
	Name          string          `json:"name"`
	Locator       string          `json:"locator"`
	SourceID      string          `json:"source_id,omitempty"`
	LastSeenAt    time.Time       `json:"last_seen_at"`
	Available     bool            `json:"available"`
}

type DiscoverySession struct {
	ID           string         `json:"id"`
	ServiceID    string         `json:"service_id"`
	ConnectionID string         `json:"connection_id"`
	Generation   int64          `json:"generation"`
	ActorID      string         `json:"actor_id"`
	Mode         AccessMode     `json:"access_mode"`
	State        DiscoveryState `json:"state"`
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at,omitempty"`
	Coverage     []KindCoverage `json:"coverage"`
	NextCursor   string         `json:"next_cursor,omitempty"`
}

type KindCoverage struct {
	Kind          string   `json:"kind"`
	Complete      bool     `json:"complete"`
	Authoritative bool     `json:"authoritative"`
	ErrorCode     string   `json:"error_code,omitempty"`
	Limitations   []string `json:"limitations"`
}

func (c KindCoverage) CanMarkAbsent() bool {
	return c.Complete && c.Authoritative && c.ErrorCode == "" && len(c.Limitations) == 0
}

// Transition changes metadata only. Evidence age and source revisions are not inputs.
// The persistence layer must lock the connection before invoking this operation.
func (c Connection) Transition(operation string, expected int, now time.Time) (Connection, error) {
	if c.Revision != expected {
		return c, ErrConflict
	}
	switch operation {
	case "configure":
		if c.Lifecycle != Active {
			return c, ErrInactive
		}
	case "disconnect":
		if c.Lifecycle != Active {
			return c, ErrInactive
		}
		c.Lifecycle = Disconnected
	case "delete":
		if c.Lifecycle == Deleted {
			return c, ErrInactive
		}
		c.Lifecycle = Deleted
		c.CredentialReference = ""
	case "restore":
		if c.Lifecycle != Deleted && c.Lifecycle != Disconnected {
			return c, ErrInvalid
		}
		c.Lifecycle = Active
	case "renew":
		if c.Lifecycle != Active {
			return c, ErrInactive
		}
		c.Generation++
		c.UpdatedAt = now
		return c, nil
	default:
		return c, ErrInvalid
	}
	c.Revision++
	c.Generation++
	c.UpdatedAt = now
	return c, nil
}

func (c Connection) CheckGeneration(generation int64) error {
	if c.Lifecycle != Active {
		return ErrInactive
	}
	if c.Generation != generation {
		return ErrConflict
	}
	return nil
}
