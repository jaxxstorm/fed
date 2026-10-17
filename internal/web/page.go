package web

import "github.com/jaxxstorm/fed/internal/assessment"

// Page is the browser adapter's typed view model. Mutations use the same domain
// services as MCP; the React workspace owns rendering and interactive state.
type Field struct {
	Name, Label, Value, Type string
	Options                  []string
}
type ReferenceOption struct {
	ID       string
	Revision int
	Label    string
	Kind     string
}
type Form struct {
	Action, Title, Button, CSRF string
	Fields                      []Field
	Hints                       []string          `json:"Hints,omitempty"`
	References                  []ReferenceOption `json:"References,omitempty"`
}
type Link struct{ Text, URL string }
type Row struct{ ID, Name, Applicability, Technical, Freshness, Review, URL string }
type EvidenceCard struct{ Title, Source, Observed, Target, URL, Action, Metadata string }
type Metric struct{ Label, Value string }
type CitationCard struct {
	Evidence           assessment.Record
	StartLine, EndLine int
	JSONPointer        *string
	Label              string
}
type Section struct{ Title, Text string }
type Page struct {
	Primary              *assessment.RequirementPresentation
	Citations            []CitationCard
	Title, Notice, Error string
	Links                []Link
	Forms                []Form
	Rows                 []Row
	Sections             []Section
	Detail               bool
	Metrics              []Metric
	Evidence             []EvidenceCard
	Findings             []Section
	Diagnostics          []Section
	Activities           []Link
	Pagination           []Link
}
