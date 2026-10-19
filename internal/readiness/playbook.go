// Package readiness models product guidance separately from official policy
// and service assessments. A valid draft is not a reviewed compliance result.
package readiness

import "time"

const Format = "requirement-playbooks/v1"

type Basis string

const (
	Official       Basis = "official_obligation"
	Recommendation Basis = "implementation_recommendation"
	ServiceChoice  Basis = "service_choice"
)

type Source struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
}

type Citation struct {
	ID          string `json:"id"`
	Source      string `json:"source"` // key in Package.Sources
	Path        string `json:"path"`
	Anchor      string `json:"anchor"`
	Excerpt     string `json:"excerpt"`
	ContentHash string `json:"content_hash"` // full source file, not line positions
	ExcerptHash string `json:"excerpt_hash"`
}

type Statement struct {
	Text      string   `json:"text"`
	Basis     Basis    `json:"basis"`
	Citations []string `json:"citations"`
	Rationale string   `json:"rationale"`
}

// Condition has only equality predicates over named profile/scope/decision
// facts. An absent fact is unknown, not false. No executable policy expressions.
type Condition struct {
	Fact     string `json:"fact"`
	Equals   string `json:"equals"`
	Question string `json:"question"`
}

type Dependency struct {
	CapabilityID string     `json:"capability_id"`
	Condition    *Condition `json:"condition,omitempty"`
	Reason       Statement  `json:"reason"`
}

type Step struct {
	ID                 string      `json:"id"`
	Action             Statement   `json:"action"`
	OwnerRole          string      `json:"owner_role"`
	Trigger            string      `json:"trigger"` // one_time, recurring, event
	Timing             *Statement  `json:"timing,omitempty"`
	CompletionCriteria []Statement `json:"completion_criteria"`
}

type Steps struct {
	Items      []Step `json:"items"`
	NoneReason string `json:"none_reason,omitempty"`
}

type EvidenceRoute struct {
	ID          string      `json:"id"`
	Artifacts   []Statement `json:"artifacts"` // all items in this route are needed
	Proves      string      `json:"proves"`
	Limitations string      `json:"limitations"`
}

type Acceptance struct {
	Fact         string          `json:"fact"`
	Scope        Statement       `json:"scope"`
	TimePeriod   Statement       `json:"time_period"`
	Provenance   Statement       `json:"provenance"`
	Alternatives []EvidenceRoute `json:"alternatives"`
	Insufficient []string        `json:"insufficient"`
}

type Assertion struct {
	ID         string     `json:"id"`
	Obligation Statement  `json:"obligation"`
	Clauses    []string   `json:"clauses"` // exact excerpts of official statement
	Acceptance Acceptance `json:"acceptance"`
}

type Alternative struct {
	ID          string    `json:"id"`
	Explanation Statement `json:"explanation"`
	Steps       Steps     `json:"steps"`
}

type Capability struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	ScopeKind    string        `json:"scope_kind"` // service, account, cluster, namespace, repository
	Dependencies []Dependency  `json:"dependencies"`
	Setup        Steps         `json:"setup"`
	Alternatives []Alternative `json:"alternatives"`
}

type Review struct {
	ReviewerID   string           `json:"reviewer_id"`
	ReviewerType string           `json:"reviewer_type"`
	ReviewedAt   time.Time        `json:"reviewed_at"`
	ContentHash  string           `json:"content_hash"`
	Comparison   SourceComparison `json:"comparison"`
}

const AgentReviewLabel = "Agent-reviewed guidance"
const AgentReviewCaveat = "Checked against cited official FedRAMP sources. Interpretations may be incomplete or incorrect; this is not an official FedRAMP determination."

// ReviewFinding records the actual source comparison, not a checklist tick.
type ReviewFinding struct {
	Finding   string   `json:"finding"`
	Citations []string `json:"citations"`
}

type SourceComparison struct {
	Applicability ReviewFinding   `json:"applicability"`
	Obligations   ReviewFinding   `json:"obligations"`
	Prerequisites ReviewFinding   `json:"prerequisites"`
	Evidence      ReviewFinding   `json:"evidence"`
	Timing        ReviewFinding   `json:"timing"`
	Limitations   []string        `json:"limitations"`
	Uncertainties []ReviewFinding `json:"uncertainties"`
}

type Playbook struct {
	Kind                  string       `json:"kind"`
	DecisionQuestion      string       `json:"decision_question,omitempty"`
	DecisionOwnerRole     string       `json:"decision_owner_role,omitempty"`
	ID                    string       `json:"id"`
	Revision              int          `json:"revision"`
	RequirementID         string       `json:"requirement_id"`
	VariantIDs            []string     `json:"variant_ids"`
	OfficialStatement     string       `json:"official_statement"`
	Purpose               Statement    `json:"purpose"`
	Applicability         Statement    `json:"applicability"`
	Prerequisites         []Dependency `json:"prerequisites"`
	NoPrerequisitesReason string       `json:"no_prerequisites_reason,omitempty"`
	Setup                 Steps        `json:"setup"`
	Operate               Steps        `json:"operate"`
	Collect               Steps        `json:"collect"`
	ReviewSteps           Steps        `json:"review_steps"`
	Assertions            []Assertion  `json:"assertions"`
	Review                *Review      `json:"review,omitempty"`
}

type Package struct {
	Format       string            `json:"format"`
	CatalogID    string            `json:"catalog_id"`
	Revision     int               `json:"revision"`
	Sources      map[string]Source `json:"sources"`
	Citations    []Citation        `json:"citations"`
	Capabilities []Capability      `json:"capabilities"`
	Playbooks    []Playbook        `json:"playbooks"`
}
