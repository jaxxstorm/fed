package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

func SDR(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("SDR", rules, markdown, map[string]administrativeRecipe{
		"SDR-CSO-FRR": {
			"Maintain a traceable decision record for each applicable rule, separating implementation explanation, objective verification, working validation and independent assessment.", "Provider security decision record owner with senior official and independent assessor",
			"Resolve applicable rules and obtain implementation/risk records, objective evidence, senior-official decisions where invoked, and actual independent verification/validation findings.",
			"Map each applicable rule into human-readable and JSON SDR entries with all seven listed topics. Preserve nonimplementation rationale/customer risk and senior-official acceptance alternatives without fabricating approvals or independent findings.",
			"Supply both SDR formats and maintain rule explanations, verification, validation, independent verification/validation, responses to assessor comments and rule-specific artifacts where applicable. Keep missing evidence or acceptance explicitly unknown.",
			"human-readable and JSON SDR revisions, applicability mapping, objective verification/validation evidence, attributable senior-official acceptance where used, independent findings, clarification history and applicable rule artifacts",
			"Reconcile all applicable rules and both formats. Evaluate appropriateness separately from operation, and provider findings separately from independent findings. A senior-official nonimplementation acceptance does not create independent validation or prove an implementation exists.",
			"Each applicable FedRAMP rule in this offering's resolved profile; all seven required information topics with rule-specific artifact applicability explicit.",
			"Retain actual decision, evidence and assessment dates and linked revisions. The SDR definition requires persistent maintenance, verification and validation; record cycles/status/intentional interruptions without inventing a numeric schedule.",
			"A generated rule checklist, empty JSON template, unsupported implemented claim or fabricated assessor/senior-official approval.",
		},
		"SDR-CSO-MTD": {
			"Make SDR updates attributable and distinguish record versions from the dates of underlying evidence.", "Provider security decision record maintainer",
			"Locate the SDR revision mechanism and authoritative update events, including the person or system producing each update.",
			"Attach version, last-update date/time and update source to SDR publication; ensure the metadata describes the released record rather than a later export.",
			"Include version, date and time of last update, and source of update in the SDR, preserving those fields across human-readable and JSON representations.",
			"SDR revisions with version/last-update/source fields and correlated change history or authoritative update records",
			"Compare each field to the actual record change. A file download time is not necessarily its last update, and a tool name without attribution may not establish the update source.",
			"The SDR version supplied for this offering, with all three metadata fields and its corresponding update event.",
			"At each SDR update; record actual date AND time of last update, not an invented periodic refresh timestamp.",
			"A filename version alone, export timestamp substituted for last update, or metadata without source of update.",
		},
		"SDR-CSX-KSI": {
			"Explain how each applicable KSI is demonstrated, including measure objectives, persistent cycles, automation sufficiency and validated production of measures.", "Provider measurement owner and security decision record owner",
			"Resolve applicable KSIs; locate measure definitions/objectives, production mechanisms, automation logic, observed results and documented rationale for absent measures or unnecessary automation.",
			"Create concise per-KSI entries separating the five source topics. Map measures to objectives and actual production evidence; document persistent cycles only where applicable.",
			"Supply short high-level KSI summaries covering measures/objectives or absence/customer risk, persistent cycles, measure verification, automation accuracy/sufficiency or non-necessity, and validation of accurate production and operation or valid absence reasons.",
			"per-KSI SDR entries, measure/objective definitions, persistent-cycle/status records, automation verification with accuracy/coverage findings, validation results and attributable alternative rationale/acceptance",
			"Assess each of the five topics separately. A metric value does not establish that the measure demonstrates the KSI, that the automation is sufficient, or that absence reasons are accepted and valid.",
			"Each applicable 20x KSI and every measure/automation path used to demonstrate it, with missing measures and nonautomated choices visible.",
			"Retain actual measure production and review periods. For persistent measures document intentional cycles, interruptions and known status under the official definition; no universal collection interval is added.",
			"A dashboard number, assertion of automation or copied KSI text without objectives and separate verification/validation findings.",
		},
		"SDR-CSX-KMT": {
			"Preserve class-specific historical metric depth and the initial-certification accommodation; missing history must not be backfilled with invented data.", "Provider measurement owner and certification owner",
			"Identify the selected 20x class, applicable KSIs, metric definitions, available historical start dates and initial/ongoing status; for Class D obtain the actual Phase 4 Pilot specifics from FedRAMP.",
			"Map available history to the selected class: A MAY include history; B SHOULD include 30-day summaries and up-to-year summaries where available; C MUST include those summaries plus all daily data including persistent-validation status up to a year where available. Do not substitute lower-class criteria for Class D pilot specifics.",
			"Supply the selected class's history, retaining actual coverage and gaps. For initial certification lacking the required operating history, retain mechanisms in place and agreement to meet the requirement. For a Class D optional-adoption variant, record the adoption decision and obtain pilot specifics; readiness remains unknown until those criteria and evidence exist.",
			"class/profile record, per-KSI metric definitions and historical coverage, applicable summaries and daily records with persistent-validation status, available-history explanations, and initial-certification mechanisms/agreement or Class D pilot criteria where relevant",
			"Recompute summaries against attributable source history and verify daily coverage only where required. Preserve Class B SHOULD, Class A MAY and where-available qualifiers. Do not treat the initial accommodation as permission to fabricate history or a Class D request as a satisfied requirement.",
			"Each applicable KSI's metrics for the selected class; distinguish Class A optional history, Class B recommended summaries, Class C required summaries/daily data, and unresolved Class D specifics.",
			"Class B/C: summary of each metric over the past 30 days and summary up to the past year where available. Class C additionally: all daily metric data including persistent-validation status up to the past year where available. Initial certification may use mechanisms plus agreement when history is unavailable. No Class D numeric minimum is published here.",
			"A current metric screenshot, invented historical points, missing daily validation status for Class C, or lower-class metrics asserted to satisfy unspecified Class D pilot criteria.",
		},
	})
}
