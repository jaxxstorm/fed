package assessment

import "time"

const ImplementationAssessmentVersion = "implementation-assessment/v1"

const ImplementationInvestigationInstructions = `Use submit_implementation_assessment to draft the service's actual implementation, not a restatement of the requirement. Record responsible roles, mechanisms, procedures, operating frequency, boundary and exact assertion IDs. Mark unknowns explicitly with reasons; do not invent organizational decisions. Cite exact evidence revisions and distinguish documented intent, deployed configuration and operating records. Assess implementation and operating support separately. Select an explicit reporting period, record actual event dates and explain sampling, population and coverage limitations. Configuration alone is not proof of ongoing operations. Fed validates structure and citations, not the substantive truth of your conclusions. Human adoption and review remain separate. A structurally reviewable partial or negative result is not compliance or independent assurance.`

// NarrativeField preserves unknowns instead of requiring an agent to invent facts.
// State is known, unknown, or not_applicable; the latter two require Reason.
type NarrativeField struct {
	State  string `json:"state"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// AssessmentPeriod is a closed interval of event time, never retrieval time.
type AssessmentPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type ImplementationNarrative struct {
	Description      NarrativeField `json:"description"`
	ResponsibleRoles NarrativeField `json:"responsible_roles"`
	Mechanisms       NarrativeField `json:"mechanisms"`
	Procedures       NarrativeField `json:"procedures"`
	Frequency        NarrativeField `json:"frequency"`
	Boundary         NarrativeField `json:"boundary"`
}

// EvidenceSupport is an agent conclusion, not a server or reviewer attestation.
type EvidenceSupport struct {
	Outcome     string     `json:"outcome"`
	Rationale   string     `json:"rationale"`
	Citations   []Citation `json:"citations"`
	Limitations []string   `json:"limitations"`
}

type OperationalSample struct {
	Method                  string           `json:"method"`
	Population              *int             `json:"population,omitempty"`
	PopulationUnknownReason string           `json:"population_unknown_reason"`
	SampleSize              int              `json:"sample_size"`
	Coverage                string           `json:"coverage"`
	CoverageReason          string           `json:"coverage_reason"`
	Events                  []OperatingEvent `json:"events"`
}

type OperatingEvent struct {
	Citation    Citation  `json:"citation"`
	OccurredAt  time.Time `json:"occurred_at"`
	Description string    `json:"description"`
}

type ImplementationAssertion struct {
	AssertionID    string            `json:"assertion_id"`
	Claim          string            `json:"claim"`
	Implementation EvidenceSupport   `json:"implementation"`
	Operating      EvidenceSupport   `json:"operating"`
	Sample         OperationalSample `json:"sample"`
}

type ImplementationAssessment struct {
	Version    string                    `json:"version"`
	Narrative  ImplementationNarrative   `json:"narrative"`
	Period     AssessmentPeriod          `json:"period"`
	Assertions []ImplementationAssertion `json:"assertions"`
}

type ImplementationSubmission struct {
	ServiceID              string                   `json:"service_id"`
	RequirementID          string                   `json:"requirement_id"`
	IdempotencyKey         string                   `json:"idempotency_key"`
	Binding                InstructionBinding       `json:"binding"`
	ExpectedResultRevision int                      `json:"expected_result_revision"`
	ExpectedDraftRevision  int                      `json:"expected_draft_revision"`
	Assessment             ImplementationAssessment `json:"assessment"`
}

type AssessmentReadinessBlocker struct {
	Code        string `json:"code"`
	Path        string `json:"path"`
	Explanation string `json:"explanation"`
	NextAction  string `json:"next_action"`
}

type StructuralReviewReadiness struct {
	Ready    bool                         `json:"ready"`
	Basis    string                       `json:"basis"`
	Blockers []AssessmentReadinessBlocker `json:"blockers"`
}

// Decision inputs contain no actor, organization, or timestamp supplied by an
// agent. Those fields belong to a server-authenticated human decision record.
type AssessmentDecisionInput struct {
	Assessment     Ref                `json:"assessment"`
	Binding        InstructionBinding `json:"binding"`
	DependencyHash string             `json:"dependency_hash"`
	Capacity       string             `json:"capacity"`
	Conclusion     string             `json:"conclusion"`
	Procedures     string             `json:"procedures"`
	Sampling       string             `json:"sampling"`
	Notes          string             `json:"notes"`
}

type AssessmentDecision struct {
	CurrentFreshness     string                  `json:"current_freshness"`
	AssessorOrganization string                  `json:"assessor_organization,omitempty"`
	Version              string                  `json:"version"`
	Decision             AssessmentDecisionInput `json:"decision"`
	ActorID              string                  `json:"actor_id"`
	OrganizationID       string                  `json:"organization_id"`
	AssessorDesignation  *Ref                    `json:"assessor_designation,omitempty"`
	DecidedAt            time.Time               `json:"decided_at"`
	Evidence             []Ref                   `json:"evidence"`
}

type ImplementationReceipt struct {
	Result           Ref                       `json:"result"`
	Draft            Ref                       `json:"draft"`
	Readiness        StructuralReviewReadiness `json:"readiness"`
	CurrentFreshness string                    `json:"current_freshness"`
	Period           AssessmentPeriod          `json:"period"`
}

type ImplementationContext struct {
	Assessment       *ImplementationAssessment `json:"assessment,omitempty"`
	Draft            *Ref                      `json:"draft,omitempty"`
	Readiness        StructuralReviewReadiness `json:"readiness"`
	CurrentFreshness string                    `json:"current_freshness"`
	Review           string                    `json:"review"`
}
