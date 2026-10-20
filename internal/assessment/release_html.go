package assessment

const releaseHTMLTemplate = `<!doctype html><html lang="en"><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Public.ServiceName}} assessment</title>
<style>body{font:16px/1.6 system-ui;max-width:1000px;margin:2rem auto;padding:1rem}section{border-top:1px solid #aaa;padding:1rem 0}details{margin:1rem 0}pre{white-space:pre-wrap}a{overflow-wrap:anywhere}</style>
<main><h1>{{.Public.ServiceName}} assessment package</h1><p>{{.Label}}</p>
<p>Period: {{.Period.Start}} – {{.Period.End}}. Prepared: {{.PreparedAt}}</p>
{{if .Incomplete}}<p><strong>Incomplete assessment: unassessed or unsupported assertions remain.</strong></p>{{end}}
<h2>Service and boundary</h2><p>{{.Context.Description}}</p><p>{{.Context.Boundary}}</p><p>{{.Context.Responsibilities}}</p>
{{range .Context.Contacts}}<p>{{.}}</p>{{end}}
{{range .Blockers}}<p>Publication blocker: {{.}}</p>{{end}}
{{range .Requirements}}<section><h2>{{.Requirement.Requirement.ID}}</h2>
<p>Applicability: {{.Requirement.Applicability}}</p><p>{{.Requirement.Statement}}</p>
{{range .ExpectedDeliverables}}<p>Expected supporting material: {{.}}</p>{{end}}
{{range .UnresolvedDeliverables}}<p>Action Items: {{.}}</p>{{end}}
{{range .Deliverables}}<p>{{.Description}}: <a href="{{evidencePath $ .Evidence}}">{{.Evidence.ID}} revision {{.Evidence.Revision}}</a></p>{{end}}
{{if .Implementation}}{{if .Implementation.Assessment}}
<p>Current freshness: {{.Implementation.CurrentFreshness}}. Period support does not establish current operations.</p>
{{with .Implementation.Assessment.Narrative}}
<p>{{.Description.Text}} {{.Description.Reason}}</p>
<h3>Responsible roles</h3><p>{{.ResponsibleRoles.Text}} {{.ResponsibleRoles.Reason}}</p>
<h3>Mechanisms and procedures</h3><p>{{.Mechanisms.Text}} {{.Mechanisms.Reason}}</p><p>{{.Procedures.Text}} {{.Procedures.Reason}}</p>
<p>Frequency: {{.Frequency.Text}} {{.Frequency.Reason}}</p><p>Boundary: {{.Boundary.Text}} {{.Boundary.Reason}}</p>{{end}}
{{range .Implementation.Assessment.Assertions}}<h3>{{.Claim}}</h3>
<p>Implementation: {{.Implementation.Outcome}} — {{.Implementation.Rationale}}</p>
{{range .Implementation.Limitations}}<p>Limitation: {{.}}</p>{{end}}
{{range .Implementation.Citations}}<p>Evidence: <a href="{{evidencePath $ .Evidence}}">{{.Evidence.ID}} revision {{.Evidence.Revision}}</a>{{if .StartLine}}, lines {{.StartLine}}–{{.EndLine}}{{else}}, JSON selector: {{.JSONPointer}}{{end}}</p>{{end}}
<p>Operating support: {{.Operating.Outcome}} — {{.Operating.Rationale}}</p>
{{range .Operating.Limitations}}<p>Limitation: {{.}}</p>{{end}}
{{range .Operating.Citations}}<p>Evidence: <a href="{{evidencePath $ .Evidence}}">{{.Evidence.ID}} revision {{.Evidence.Revision}}</a>{{if .StartLine}}, lines {{.StartLine}}–{{.EndLine}}{{else}}, JSON selector: {{.JSONPointer}}{{end}}</p>{{end}}
<p>Sample: {{.Sample.SampleSize}}, {{.Sample.Method}}. Coverage: {{.Sample.Coverage}}. {{.Sample.CoverageReason}} {{.Sample.PopulationUnknownReason}}</p>
{{range .Sample.Events}}<p>{{.OccurredAt}}: {{.Description}} — <a href="{{evidencePath $ .Citation.Evidence}}">dated evidence</a></p>{{end}}
{{end}}{{else}}<p>Not assessed with a structured narrative.</p>{{end}}{{else}}<p>Not assessed.</p>{{end}}
{{if .Reviews}}<h3>Human decisions</h3><p>Adoption: {{.Reviews.Adoption}}; internal review: {{.Reviews.InternalReview}}; independent review: {{.Reviews.IndependentReview}}</p>
{{range .Reviews.Decisions}}{{$decision := decision .}}<details><summary>{{$decision.Decision.Capacity}}: {{$decision.Decision.Conclusion}}</summary>
<p>{{$decision.Decision.Procedures}}</p><p>{{$decision.Decision.Sampling}}</p><p>{{$decision.Decision.Notes}}</p><p>Organization: {{$decision.OrganizationID}}. Assessor organization: {{$decision.AssessorOrganization}}. Recorded {{$decision.DecidedAt}}.</p></details>{{end}}{{end}}
{{range .Issues}}{{$issue := issue .}}<h3>Action Items · {{$issue.State}}</h3><p>{{$issue.Explanation}}</p><p>{{$issue.Disposition}}</p>
{{range $issue.Milestones}}<p>{{.Description}}</p>{{end}}{{end}}
<details><summary>Technical diagnostics</summary><p>{{.Collection.Technical}}; freshness {{.Collection.Freshness}}</p>
{{range .Collection.Errors}}<p>Evidence blocked: {{.}}</p>{{end}}
{{if .Collection.MissingMeasures}}<p>{{.Collection.MissingMeasures}} configured measures have no current observations.</p>{{end}}
{{with .Collection.Inventory}}<p>{{.CurrentResources}} retained resources, {{.RetiredResources}} retired identities, {{.UnfencedResources}} without verified inventory provenance.</p><p>{{.Definition}} {{.Limitations}}</p><p>Observation window {{.FirstObservedAt}} – {{.LastObservedAt}}</p>{{end}}
{{range .Collection.Findings}}<p>{{.Condition}}: {{.Explanation}}</p>{{end}}</details></section>{{end}}
<h2>Evidence index</h2>{{range .Evidence}}<p><a href="{{.Path}}">{{.Reference.ID}} revision {{.Reference.Revision}}</a> — SHA-256 {{.Hash}}</p>{{end}}
<p>manifest.json contains exact citation selectors and evidence provenance. Verify files using SHA256SUMS. Hashes establish integrity, not assessor signatures.</p></main></html>`
