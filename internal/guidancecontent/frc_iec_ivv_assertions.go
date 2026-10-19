package guidancecontent

// These checks separate facts that a single report or package title cannot
// establish. Each keeps its exact clause within the selected official statement.
type certificationCheck struct{ clause, fact, records, insufficient string }

var certificationOperatingFacts = map[string]string{
	"FRC-CSO-FCP": "The offering has an identified target certification profile and evidence that its relevant practices are applied across the actual boundary.",
	"FRC-CSO-PKG": "The initial package supplied to FedRAMP includes the offering overview, per-relevant-rule implementation/validation/assessment information and a labeled real or example OCR.",
	"FRC-CSO-JSN": "Supplied rule-schema information uses corresponding-schema-valid JSON unless explicitly excepted, and public JSON supports ordinary browser-based web application consumption.",
	"FRC-CSO-MRA": "The provider retains accountability for the package and substantively reviews accuracy and completeness of its own and third-party contributions.",
	"FRC-CSO-POP": "The same offering is not simultaneously seeking both Rev5 Program and 20x Program certification; any Agency certification is distinguished from a Program pursuit.",
	"FRC-CLA-ASF": "One approved alternative-framework certification or equivalent process, with independent assessment if applicable, completed for the offering within the past 12 months.",
	"FRC-CLA-EAM": "All necessary parties were supplied the complete material set for the elected alternative framework, preserving conditional bridge/gap and supplemental items.",
	"FRC-CLA-MFR": "The Class A package contains appropriate artifacts or information mappings for FRC-CLA and every enumerated additional rule, with sufficient decision detail and included KSI information despite future Rev5 intentions.",
	"FRC-CLA-RFR": "Each applicable recommended Class A entry has an explained disposition and supporting information; any departure preserves weighed implications rather than silent omission.",
	"FRC-CLA-OFR": "Applicable optional Class A entries have identified elections or non-elections; elected entries are linked to their own evidence without treating non-election as a failure.",
	"FRC-CLA-IVV": "If elected, a recognized assessor completed both independent verification and validation of the Class A package before its submission.",
	"FRC-APP-MLF": "The offering was listed in the Marketplace before applying for certification and the four referenced listing requirements have individual mappings.",
	"FRC-APP-AFC": "The provider submitted a fully completed certification application form requesting initial FedRAMP assessment.",
	"FRC-APP-FCP": "The supplied initial package describes current offering status supported by both provider verification and validation performed in the preceding seven days.",
	"FRC-APP-FIA": "Where required or elected, the supplied initial independent assessment was completed by a recognized service within the previous three months, or an explicitly justified separate stale-update route is identified.",
	"FRC-APP-NTP": "The provider itself applied for certification; any third-party assistance was preparation rather than submission on the provider's behalf.",
	"FRC-APP-USA": "If the optional freshening route is elected, a recognized service reviewed original-to-current changes and the original stale assessment does not exceed nine months.",
	"FRC-CSX-VVK": "The selected class's optional, recommended or mandatory automated methods repeatedly assess KSI accuracy and completeness, with actual per-KSI method counts where specified and known activity status.",
	"FRC-CSX-VVR": "Where applicable and adopted, automated methods repeatedly assess SDR rule accuracy and completeness, preserving actual coverage, failures and known current status.",
	"FRC-CSX-MAS": "The disposition and coverage mapping address every KSI across every offering aspect within MAS, with any departure from the recommendation explained.",
	"IEC-CSO-EFR": "Incidents are promptly evaluated for actual or likely federal-data confidentiality/integrity effects, with reasoned determinations and reporting linkage for reportable incidents.",
	"IEC-CSO-DPR": "Reportable incidents use PAIN-5 reporting treatment unless a prompt EFI-conforming estimate supports another rating, retaining default-period history.",
	"IEC-CSO-EFI": "The incident's promptly estimated rating is supported by likely cumulative agency effect, severity and agency count, including unknown-effect treatment and the default when estimation is incomplete.",
	"IEC-CSO-IIR": "An initial report with available information/current status for all eight fields reaches the affected recipients by the selected class/PAIN interval, preserving Class A SHOULD and responsible disclosure.",
	"IEC-CSO-OIR": "Ongoing affected-party reports reconcile changed and unchanged prior fields and available additional information over the response period at the selected class/PAIN intervals, preserving Class A SHOULD.",
	"IEC-CSO-FIR": "After both resolution and completed recovery, all affected recipients receive a final report reconciling all previously reported information within the selected FIR interval, including mandatory Class A final reporting.",
	"IEC-CSO-AIR": "The reporting workflow uses automation to reduce human intervention across report preparation and affected-party delivery, with remaining human handoffs and the recommendation's disposition explained.",
	"IVV-CSO-FIA": "Where mandatory or elected, completed independent verification and validation covers every applicable rule at least annually with recognized-service or explicitly prioritized FedRAMP provenance and persistent history.",
	"IVV-CSO-SEI": "All necessary assessors receive objective evidence that the documented measures are actually implemented within the claimed scope.",
	"IVV-CSO-SEE": "All necessary assessors receive objective evidence evaluating whether implemented measures produce their intended security outcomes, including failed and untested outcomes.",
	"IVV-CSO-ICP": "Independent assessment results are present in the supplied package with findings, limitations and underlying intent intact despite any presentation changes.",
	"IVV-CSO-DUS": "When representative samples are actually used, both their use and representativeness are documented and explained against the assessed population; absent sampling does not trigger a sample duty.",
	"IVV-CSO-STE": "All necessary assessors receive the technical explanations, demonstrations and supporting information needed for effective verification and validation, or a reasoned SHOULD disposition explains departures.",
	"IVV-CSO-USR": "Where sampling is elected, selected samples are appropriate representatives of the identified verification/validation population, with limitations retained; non-election is permitted.",
	"IVV-CSO-RAA": "Elected assessor advice is assessed against the likely-compromise exception with actual relationship and independence facts rather than a generic declaration.",
	"IVV-CSX-AIA": "Class A assessments meet the underlying framework's expectations persistently; B/C/D completed FedRAMP independent assessments cover all KSIs at least once per year.",
}

var certificationChecks = map[string][]certificationCheck{
	"FRC-CSO-FCP": {
		{"identify a target FedRAMP Certification Profile", "The target type, path and class are identified for this offering.", "Saved target profile and accountable provider decision linked to the service boundary.", "A practice inventory with no target profile."},
		{"apply all relevant FedRAMP Practices", "Relevant practices are applied across the actual offering, including relevant third-party resources.", "Profile-specific practice inventory and implementation comparison against scoped information resources.", "Selecting a profile without applying its relevant practices."},
	},
	"FRC-CSO-MRA": {
		{"accuracy and completeness of all information", "Provider review addresses factual accuracy and missing information independently of contributor identity.", "Package claim-to-source checks, required-content reconciliation and provider dispositions of errors and omissions.", "A signed responsibility statement without substantive accuracy/completeness review."},
	},
	"FRC-CSX-VVR": {
		{"accuracy and completeness of the Security Decision Record", "Automation checks both factual accuracy and coverage of applicable SDR rule entries.", "SDR denominator, automated rule-to-source comparisons and missing-entry results, including exceptions.", "Schema validation alone or a repeated job that never compares SDR claims to actual facts."},
		{"persistently verify and validate", "Automation repeats with known status and evidence of verification and validation rather than mere execution.", "Dated run history, outcomes, failures, skipped scope and intentional interruptions for the selected automation.", "One successful run presented as persistent verification and validation."},
	},
	"IVV-CSO-STE": {
		{"all necessary assessors", "The technical explanation and demonstration reach every necessary assessor.", "FedRAMP and contracted recognized-assessor roster, supplied explanations, demonstrations and access records.", "Internal engineering meeting notes with no assessor delivery or access."},
		{"effectively complete verification and validation", "The supplied technical material addresses what assessors need for both implementation verification and outcome validation.", "Assessor questions, technical demonstrations, supporting information and resolved or outstanding information needs.", "A marketing deck or attendance record without technical assessment support."},
	},
	"IVV-CSO-RAA": {
		{"UNLESS doing so is likely to compromise the objectivity and integrity", "The elected advice relationship has no identified likely compromise of assessment objectivity and integrity, or is treated as outside this permission.", "Actual advice, decision ownership, conflict analysis and independent assessment judgments.", "A generic independence declaration that never evaluates the actual advice."},
	},
	"FRC-APP-FCP": {
		{"shows the current status of the cloud service offering", "Package content represents the actual service status at supply.", "Package-to-service comparison and intervening change inventory linked to the exact supplied package revision.", "Recent verification of an obsolete baseline with no change reconciliation."},
		{"verified and validated by the provider within the previous 7 days", "Both provider verification and validation are within the seven-day lookback.", "Separate verification and validation results with original execution dates and the supply timestamp; calculate both ages.", "Recent formatting or one recent verification with stale validation."},
	},
	"FRC-APP-FIA": {
		{"completed by a FedRAMP Recognized independent assessment service", "The completing assessment service has FedRAMP Recognized provenance.", "Completed report identifying the independent service and corresponding recognition record at assessment time.", "A recognized assessor's logo on an uncompleted engagement."},
		{"within the previous 3 months", "The completed initial assessment satisfies the selected three-month freshness provision when applicable/elected.", "Original completion date, supply date and calendar-month age calculation, plus any separately justified stale-assessment route.", "A current download timestamp on an older assessment."},
	},
	"FRC-APP-USA": {
		{"review any changes between the original assessment and the current status", "The recognized assessor's review covers changes across the complete original-to-current interval.", "Original assessed baseline, current baseline, intervening changes and assessor dispositions for each change.", "Review of only the latest release or provider-only gap attestation."},
		{"UNLESS the stale assessment is more than 9 months old", "The optional update route is not used beyond the original assessment's nine-month ceiling.", "Original completion and update dates with original-age calculation and route election.", "Resetting original age to the last delta-review date."},
	},
	"FRC-CSO-JSN": {
		{"valid against the corresponding JSON schema", "Each supplied JSON payload validates against its corresponding rule-linked schema, except explicit rule exceptions.", "Payload bytes/revision, schema identifier/version, validation output and any exact exception reference.", "Syntactically valid JSON checked against an unrelated schema."},
		{"public JSON data MUST be supplied in a manner compatible with modern web frameworks", "Public JSON is accessible to normal web applications with appropriate public-data response behavior.", "Public endpoint URL, actual browser-origin request/response, CORS results and Content-Type and nosniff headers.", "A command-line request that bypasses browser-origin behavior."},
	},
	"FRC-CSX-VVK": {
		{"automated methods", "Actual automated methods support the selected KSI verification/validation provision, including any class-specific per-KSI count.", "Per-KSI method register, distinct method purpose, code/config revision and executed results, with class-specific count reconciliation.", "Counting repeated executions of one method as different methods."},
		{"persistently verify and validate the accuracy and completeness", "Repeated automation evaluates both accuracy and completeness with known current status.", "Dated run history, comparison to underlying service facts, population denominators, errors, skips and intentional-interruption records.", "A green execution badge without evaluated accuracy/completeness or scope."},
	},
	"IEC-CSO-EFR": {
		{"promptly evaluate incidents", "Actual or simulated incidents have timely reportability evaluations with reasons.", "Detection and evaluation timestamps, decision author and reasons for the determination/delay; label simulations.", "An incident policy with no executed evaluation."},
		{"affect confidentiality or integrity of federal customer data or are likely to affect confidentiality or integrity", "Reportability reasoning considers both actual and likely confidentiality/integrity effects on federal data.", "Federal data/agency mapping, incident facts, likelihood reasoning and actual/likely effect conclusion.", "Waiting for confirmed exfiltration despite evidence of likely integrity or confidentiality effects."},
		{"must be reported following the FedRAMP Incident Evaluation and Communication rules", "Reportable determinations are linked to the applicable reporting lifecycle.", "Determination-to-IIR/OIR/FIR crosswalk with report revisions and delivery references, or explicit current lifecycle status.", "A reportable label with no reporting linkage."},
	},
	"IEC-CSO-DPR": {
		{"Potential Agency Impact N-rating (PAIN) of 5 UNLESS", "A reportable incident without a prompt conforming estimate is handled using PAIN-5.", "Rating history, incident decision times and resulting deadline/notification choices during the default period.", "Unknown impact recorded as PAIN-1 or unassigned while reporting is delayed."},
		{"promptly estimate the PAIN rating following the rule in IEC-CSO-EFI", "Any exception to the default has prompt, rule-conforming estimation provenance.", "Dated IEC-CSO-EFI impact calculation, agency count, effect category and rationale linked to the rating change.", "A later unsupported estimate retroactively used to justify initial low-priority reporting."},
	},
	"IEC-CSO-IIR": {
		{"responsibly notify all affected parties", "Initial reports reach the affected recipients through the applicable routes with responsible disclosure.", "Affected-party roster reconciled to actual transmissions, FedRAMP email, agency-specific delivery and trust-center/USDA Connect updates, including disclosure rationale.", "A report stored internally or sent only to FedRAMP while affected agencies are omitted."},
		{"as much of the following information that is available at the time of reporting and/or the current relevant status", "Initial report fields distinguish known information and current unknown status at the reporting time.", "Initial revision and field-by-field knowledge/status reconciliation against contemporaneous incident records.", "Waiting for root cause or all recovery details before initial reporting, or backfilling a report without revision history."},
	},
	"IEC-CSO-OIR": {
		{"including updates (or lack of updates) to all previously reported information", "Ongoing reports reconcile every previously reported field, including no-change information.", "Ordered report revisions and prior-field crosswalk with changed, unchanged and unknown statuses.", "Only new indicators reported with no accounting for prior fields."},
		{"as new information becomes available during incident response", "The ongoing reporting history is traceable to new incident knowledge and the selected reporting intervals.", "Incident knowledge timeline, PAIN history, OIR transmissions and interval calculations for the response period.", "A single latest report that overwrites the history."},
	},
	"IEC-CSO-FIR": {
		{"once the incident has been resolved and recovery is complete", "Both resolution and recovery completion underpin final reporting and the final-report clock.", "Separate resolution and recovery completion evidence/timestamps plus final report transmissions.", "Incident ticket closure alone or final reporting while recovery remains incomplete."},
		{"including final updates to all previously reported information", "Final reporting reconciles the complete previously reported information set.", "Final revision compared field-by-field with IIR/OIR history and final known dispositions.", "A closure announcement omitting previously reported impact or recovery information."},
	},
	"IVV-CSO-DUS": {
		{"document and explain the use of representative samples", "Used samples have both retained selection records and a representativeness explanation.", "Population and sample identifiers, selection method, coverage dimensions and explanation of exclusions/bias.", "Selected resource screenshots without a denominator or rationale."},
	},
	"IVV-CSO-FIA": {
		{"all applicable FedRAMP rules", "Completed independent assessment scope reconciles all applicable rules.", "Saved applicability inventory and rule-to-assessment-results crosswalk, including exclusions and missing coverage.", "A general assessment completion letter without rule coverage."},
		{"at least once per year", "Where required or elected, completed assessments meet the annual provision as part of persistent activity.", "Original assessment completion dates and cycle history, current status and documented interruptions, plus the Class A election where relevant.", "Annual contract renewals without completed independent assessment results."},
	},
	"IVV-CSO-ICP": {
		{"without inappropriate modification", "Assessment intent, failures, qualifications and findings remain intact in the supplied package.", "Original assessor results, transformed package content and semantic differences with formatting-only explanations.", "Deleting an adverse conclusion while keeping the original report title."},
	},
	"IVV-CSO-SEI": {
		{"evidence to all necessary assessors", "All necessary assessors can access the actual implementation evidence supplied.", "Necessary-assessor roster, evidence manifest and attributable delivery/access records for each assessor.", "Evidence retained only in a provider workspace that assessors cannot access."},
		{"implementation of the measures that have been documented", "Deployed measures match the documented security measures within the actual scope.", "Documented-measure-to-runtime comparison using deployed code/configuration and resource identifiers.", "An undeployed repository configuration or intended architecture alone."},
	},
	"IVV-CSO-SEE": {
		{"evidence to all necessary assessors", "All necessary assessors receive the relevant outcome-validation evidence.", "Necessary-assessor roster, validation-result manifest and delivery/access records for each assessor.", "Internal validation metrics without assessor access."},
		{"effectiveness of the measures that have been implemented", "Actual observations/tests evaluate whether implemented measures produce intended outcomes.", "Intended outcomes, executed validation method, scoped observations, actual results/failures and outcome comparison.", "Deployment success or configuration presence substituted for an outcome test."},
	},
}
