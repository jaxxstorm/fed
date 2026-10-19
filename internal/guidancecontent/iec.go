package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// IEC preserves the exact class/PAIN reporting tables and transition variants.
func IEC(rules, markdown fs.FS) (readiness.Package, error) {
	return certificationDraft(rules, markdown, "IEC", iecRecipes, 76)
}

var iecRecipes = map[string]certificationRecipe{
	"IEC-CSO-AIR": {
		"Assess recommended automation that reduces human intervention in incident reporting to affected parties.",
		"Locate the reporting workflow, automation owner, incident data sources, recipient routing and manual handoffs.",
		"Versioned automation configuration/code, incident-to-report data mapping, delivery execution logs, recipient coverage, failures and records of remaining human intervention.",
		"Trace an incident through report generation and affected-party delivery; compare actual automated and manual steps. Preserve SHOULD; sending a hand-crafted email through an automated mail server alone does not establish minimized intervention.",
		"Reporting of FedRAMP Reportable Incidents to all affected parties, including FedRAMP and affected agency customers, not just internal alerts.",
		"During incident reporting; automation does not replace the applicable IIR/OIR/FIR timing. This recommendation supplies no separate numerical deadline.",
		"An incident-response automation product license with no report workflow or execution evidence.",
	},
	"IEC-CSO-DPR": {
		"Determine whether reportable incidents use PAIN-5 unless a prompt rule-conforming impact estimate exists.",
		"Locate incident evaluation timelines, PAIN estimation decisions, default-rating configuration and rating history.",
		"Reportability decision time, rating/default history, any IEC-CSO-EFI calculation and its time, applicable class reporting clock and report records.",
		"For each reportable incident, check for a prompt supported estimate. If absent, compare reporting behavior to PAIN-5; an unknown impact is not a low rating. Separate a later estimate from the initial default period.",
		"Every FedRAMP Reportable Incident and every period before a prompt qualifying estimate; an actual or clearly labeled simulated incident may demonstrate the process.",
		"Promptly means without unnecessary delay, not a fixed grace period. The workflow starts the reporting clock using estimated or default PAIN timeframes; retain the underlying decision times.",
		"An unexplained PAIN-1 value, missing history, or a default configuration without actual reporting behavior.",
	},
	"IEC-CSO-EFI": {
		"Assess the recommended prompt estimate of likely adverse agency impact and the rationale for the incident's PAIN rating.",
		"Locate the agency/customer impact map, incident facts, impact categories and incident-rating decision history.",
		"Incident evaluation log, affected-agency count, evidence of likely customer effect, N1-N5 calculation, assessor/decision owner, rating time and revised estimates.",
		"Compare both customer-effect severity and agency count with each N-rating definition: N1 minimal at one or more; N2 narrow at one or more; N3 disruptive at one; N4 debilitating at one or disruptive at more than one; N5 debilitating at more than one. Minimal is noticeable by some users; narrow interrupts some users for less than 12 hours or compromises an extremely limited amount and type of federal data; disruptive interrupts many users for less than 24 hours or compromises large amounts or many types; debilitating interrupts most users or compromises most federal data. Unknown adverse customer effect should be treated as debilitating until proven otherwise. Separate observed facts from likely impact estimates and assess cumulative agency effects. If not promptly estimated, evaluate default PAIN-5 under IEC-CSO-DPR.",
		"Likely adverse effects on all agency customers of the incident, including cumulative effects rather than only the first agency reporting trouble.",
		"Promptly means without unnecessary delay; preserve detection/evaluation/rating timestamps and contextual reasons for delay. No numeric estimation allowance is invented.",
		"A severity label copied from a vulnerability scanner without agency-effect and agency-count reasoning.",
	},
	"IEC-CSO-EFR": {
		"Evaluate prompt determination of actual or likely confidentiality/integrity effects on federal customer data and routing of reportable incidents.",
		"Locate incident intake/logs, federal-customer-data scope and the reportability evaluation procedure and owner.",
		"Incident log with detection and evaluation times, known facts, federal data affected or likely affected, confidentiality/integrity rationale, determination and downstream reporting references. Real, simulated or combined examples are permitted and must be labeled.",
		"Check both actual and likely confidentiality/integrity effects; do not wait for confirmed compromise. Reconcile reportable determinations to reporting records and non-reportable conclusions to evidence. Availability-only impact is not automatically the stated reporting trigger; investigate accompanying confidentiality/integrity likelihood.",
		"Incidents involving or likely to involve confidentiality or integrity of federal customer data in the offering; reconcile evaluated examples to the incident inventory.",
		"Promptly, without unnecessary delay. Preserve detection-to-evaluation chronology; numeric report deadlines are separate from the reportability evaluation.",
		"A procedure alone, or no-incident assertion inferred from an unavailable incident log.",
	},
	"IEC-CSO-IIR": {
		"Check initial reporting after reportability identification, including available information or current status for every required field.",
		"Locate incident/rating timeline, federal coordinator contacts, report schema, affected-party roster and delivery channels, including FedRAMP security email and agency-specific procedures.",
		"Initial report revision with coordinator, tracking ID, description, incident timeline, historical/current PAIN and rationale if applicable, federal confidentiality/integrity/data-type impacts, recovery estimates and likely affected agencies; schema validation, recipient-specific delivery timestamps and trust-center/USDA Connect update records.",
		"Reconcile each report field to what was known at reporting, including explicit unknown/current status rather than waiting for a completed investigation. Compare all affected-party delivery to selected class/PAIN deadlines; record class, rating/default and clock start. Examine responsible disclosure choices without withholding required reporting. Real/simulated reports may satisfy example-artifact expectations but simulation does not prove live timeliness.",
		"Each reportable incident and all affected parties. FedRAMP uses fedramp_security@fedramp.gov; agencies use their specific procedures; retain the specified trust-center or USDA Connect update context.",
		"After identifying a reportable incident. The workflow starts the reporting clock after estimated or default PAIN selection. Apply the selected official IIR PAIN table, preserving Class A SHOULD versus other classes MUST; do not substitute OIR or FIR intervals.",
		"A drafted report without delivery, missing recipient coverage, or a full later report claimed to prove timely initial notification.",
	},
	"IEC-CSO-OIR": {
		"Assess ongoing affected-party reports that update prior information, including no-change status, as new information becomes available.",
		"Locate initial report baseline, ongoing incident/rating history, reporting schedule and recipient routes.",
		"Ordered OIR revisions and transmission history, all prior-field updates or lack of updates, observed incident activity, indicators of compromise, applicable CVEs, root cause, response/recovery activity, PAIN changes and recipient coverage.",
		"Compare consecutive reports with incident knowledge at each time and the selected OIR table. Check no-change reporting and all previously reported fields rather than only new technical indicators. Label real versus simulated histories and preserve unknown root cause or inapplicable CVE without fabricating facts.",
		"The entire ongoing response period for the reportable incident and all affected parties, with class and PAIN history retained.",
		"As new information becomes available during response, including updates or lack of updates; use the selected OIR PAIN intervals until resolution. Preserve actual report and rating-change timestamps and Class A SHOULD.",
		"Only an initial and final report, or a mutable dashboard with no dated recipient/update history.",
	},
	"IEC-CSO-FIR": {
		"Assess final notification after both incident resolution and completed recovery, with final updates to prior reporting.",
		"Locate incident closure and recovery-completion evidence, earlier report revisions, final report and recipient delivery records.",
		"Resolution and recovery-completion timestamps, final report revision reconciling all earlier reported fields, final PAIN/impact history, schema validation and all affected-party transmission/update records.",
		"Check both resolution and recovery completion before treating a report as final. Compare final-field updates with IIR/OIR history and delivery with the selected FIR PAIN table. Class A FIR is MUST even though its IIR/OIR statements are SHOULD. Label simulated examples and their limitations.",
		"Each resolved reportable incident, its completed recovery, all previously reported information and every affected recipient.",
		"Once the incident is resolved and recovery complete, apply the selected FIR PAIN deadline. Preserve the later qualifying completion event and all transmissions; do not reuse the initial-report clock.",
		"A closed incident ticket without recovery evidence or without final affected-party notification.",
	},
	"IEC-FRP-ORV": {
		"Preserve FedRAMP's periodic review of provider incident-evaluation/communication implementation, including lack-of-reporting triggers.",
		"Locate FedRAMP review correspondence, reporting history and provider procedure/evaluation records.",
		"FedRAMP review records explaining trigger, provider implementation discussion, conclusions and any actual corrective-action request or grace-period communication.",
		"Distinguish FedRAMP's review duty from provider duties. Lack of reports can trigger review but does not establish no incidents or failed implementation. Retain any actual corrective-action/grace decision as evidence; do not initiate remediation from this assessment.",
		"FedRAMP reviews of providers based on lack of reporting or other information; the current provider inventory excludes the FedRAMP actor's duty.",
		"Periodically without a numeric review cadence. The source describes a 3 month grace period after a corrective-action request for unawareness or improper procedures; this is not an extension of incident reporting deadlines.",
		"A provider self-review impersonating a FedRAMP review or an empty reporting register treated as compliance.",
	},
}
