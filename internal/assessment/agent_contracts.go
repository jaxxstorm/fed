package assessment

import (
	"errors"
	"time"

	"github.com/jaxxstorm/fed/internal/catalog"
)

const MaxAgentArtifactBytes = 1 << 20

var ErrViewChanged = errors.New("view_changed")

// EvidenceSource contains declarations used by external agents, never credentials.
type EvidenceSource struct {
	ClusterID      string   `json:"cluster_id,omitempty"`
	Namespace      string   `json:"namespace,omitempty"`
	NamespaceUID   string   `json:"namespace_uid,omitempty"`
	ConnectionID   string   `json:"connection_id,omitempty"`
	ResourceKind   string   `json:"resource_kind,omitempty"`
	ResourceID     string   `json:"resource_id,omitempty"`
	ProviderTenant string   `json:"provider_tenant,omitempty"`
	Lifecycle      string   `json:"lifecycle,omitempty"`
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	Locator        string   `json:"locator"`
	ScopeRevision  int      `json:"scope_revision"`
	AccountID      string   `json:"account_id,omitempty"`
	Region         string   `json:"region,omitempty"`
	Cluster        string   `json:"cluster,omitempty"`
	Repository     string   `json:"repository,omitempty"`
	Commit         string   `json:"commit,omitempty"`
	Paths          []string `json:"paths"`
	AccessHint     string   `json:"access_hint,omitempty"`
	RequirementIDs []string `json:"requirement_ids"`
	Enabled        bool     `json:"enabled"`
	FreshnessHours int      `json:"freshness_hours"`
}

type ArtifactSubmission struct {
	ServiceID      string    `json:"service_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Source         Ref       `json:"source"`
	ScopeRevision  int       `json:"scope_revision"`
	Locator        string    `json:"locator"`
	SourceVersion  string    `json:"source_version"`
	MediaType      string    `json:"media_type"`
	Content        string    `json:"content"`
	ObservedAt     time.Time `json:"observed_at"`
	RetrievedAt    time.Time `json:"retrieved_at"`
	Method         string    `json:"method"`
	Description    string    `json:"description"`
	Limitations    []string  `json:"limitations"`
}

type DocumentaryProvenance struct {
	ConnectionID   string    `json:"connection_id,omitempty"`
	ProviderTenant string    `json:"provider_tenant,omitempty"`
	ResourceID     string    `json:"resource_id,omitempty"`
	ContentHash    string    `json:"content_hash,omitempty"`
	ETag           string    `json:"etag,omitempty"`
	Source         Ref       `json:"source"`
	Locator        string    `json:"locator"`
	SourceVersion  string    `json:"source_version"`
	MediaType      string    `json:"media_type"`
	RetrievedAt    time.Time `json:"retrieved_at"`
	Method         string    `json:"method"`
	FreshnessHours int       `json:"freshness_hours"`
}

type Assertion struct {
	ID            string   `json:"id"`
	Statement     string   `json:"statement"`
	Required      bool     `json:"required"`
	EvidenceKinds []string `json:"evidence_kinds"`
}

type InstructionBinding struct {
	CatalogID       string `json:"catalog_id"`
	ProfileRevision int    `json:"profile_revision"`
	ScopeRevision   int    `json:"scope_revision"`
	Version         string `json:"version"`
	Sources         []Ref  `json:"sources"`
}

type RuleInstructions struct {
	ServiceID              string             `json:"service_id"`
	Rule                   catalog.Resolved   `json:"rule"`
	Binding                InstructionBinding `json:"binding"`
	GuidanceKind           string             `json:"guidance_kind"`
	Assertions             []Assertion        `json:"assertions"`
	Steps                  []string           `json:"steps"`
	Sources                []Record           `json:"sources"`
	ExistingEvidence       []Record           `json:"existing_evidence"`
	Work                   EvidenceWork       `json:"work"`
	ExpectedResultRevision int                `json:"expected_result_revision"`
	ExpectedDraftRevision  int                `json:"expected_draft_revision"`
}

type Citation struct {
	Evidence    Ref     `json:"evidence"`
	StartLine   int     `json:"start_line,omitempty"`
	EndLine     int     `json:"end_line,omitempty"`
	JSONPointer *string `json:"json_pointer,omitempty"`
}

type AssertionResult struct {
	AssertionID string     `json:"assertion_id"`
	Outcome     string     `json:"outcome"`
	Rationale   string     `json:"rationale"`
	Citations   []Citation `json:"citations"`
}

type RuleResultSubmission struct {
	ServiceID              string             `json:"service_id"`
	RequirementID          string             `json:"requirement_id"`
	IdempotencyKey         string             `json:"idempotency_key"`
	Binding                InstructionBinding `json:"binding"`
	ExpectedResultRevision int                `json:"expected_result_revision"`
	ExpectedDraftRevision  int                `json:"expected_draft_revision"`
	Assertions             []AssertionResult  `json:"assertions"`
	Blockers               []string           `json:"blockers"`
	Explanation            string             `json:"explanation"`
	RequestedState         string             `json:"requested_state"`
}

type EvidenceWork struct {
	State             string   `json:"state"`
	Assessment        string   `json:"assessment"`
	MissingAssertions []string `json:"missing_assertions"`
	Blockers          []string `json:"blockers"`
	Reasons           []string `json:"reasons"`
}

type RuleReceipt struct {
	Result   Ref          `json:"result"`
	Draft    Ref          `json:"draft"`
	Evidence []Ref        `json:"evidence"`
	Work     EvidenceWork `json:"work"`
	Review   string       `json:"review"`
}
