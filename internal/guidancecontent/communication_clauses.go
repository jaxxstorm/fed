package guidancecontent

// Focused checks make conjunctions independently assessable. The complete
// selected-statement assertion retains the actor, force and qualifying context;
// these exact fragments must not be read as freestanding unconditional mandates.
type communicationClauseCheck struct{ clause, fact, records, insufficient string }

var communicationClauseChecks = map[string][]communicationClauseCheck{
	"CCM-QTR-RTR": {
		{"record or transcribe Quarterly Reviews", "The selected recording OR transcription captures the actual review, subject to the documented SHOULD decision.", "Meeting occurrence record and attributable recording or transcript with content coverage review.", "An agenda or calendar invite represented as a meeting record."},
		{"supply them to all necessary parties", "The retained recording or transcript is supplied to the necessary-party audience, subject to the documented SHOULD decision.", "Released artifact, recipient reconciliation, delivery timestamps and recipient access checks.", "A host-only recording with no distribution evidence."},
	},
	"SCN-CSO-MAR": {
		{"maintain auditable records", "Evaluation decisions and their rationale remain auditable.", "Versioned evaluation history tied to change identifiers and attributable decision makers.", "Notifications alone without evaluation history."},
		{"make them available to FedRAMP as requested", "FedRAMP can obtain the records and actual requests are fulfilled.", "Retrieval instructions and actual request/response records where requests exist.", "An internal repository without a FedRAMP retrieval route."},
	},
	"SCN-CSO-NOM": {
		{"clearly documented in the FedRAMP Certification Package", "The elected notification mechanism is clearly documented in the package.", "Published package instructions mapped to the current notification mechanism list.", "An undocumented operational email practice."},
		{"easily accessible", "Intended recipients can readily access the elected mechanism.", "Recipient access observations following the published instructions and recorded access failures.", "Instructions for an inaccessible portal."},
	},
	"SCN-TRF-NIP": {
		{"including a summary of any likely security impacts or changes in risk", "Initial notice content explains likely security impacts or risk changes independently of timely delivery.", "Delivered initial notice and underlying prospective impact/risk analysis with content reconciliation.", "A timely announcement containing only the deployment date."},
	},
	"SCN-TRF-NFP": {
		{"including updates to all previously sent information", "The final notice updates the complete previously sent information set.", "Initial/final revisions and field-by-field reconciliation against final plans.", "A timely final notice with stale initial-plan information."},
	},
	"SCN-TRF-NAF": {
		{"including updates to all previously sent information", "The post-change notice reconciles all previously supplied information to the finished change.", "Initial, final and completion notices compared with actual deployed state.", "A timely completion email without updates to prior information."},
	},
	"CPO-CSX-CPM": {
		{"up to date", "Package contents track actual service and input changes, rather than merely acquiring new timestamps.", "Service/input change history reconciled to package content revisions and stale-content findings.", "A recent generation timestamp on stale information."},
		{"complete", "The maintained package includes all applicable certification materials.", "Applicable-material manifest and retrievable artifact revisions with missing-input reconciliation.", "Fresh copies of only a subset of required package materials."},
	},
	"CCM-OCR-AVL": {
		{"to all necessary parties every 3 months", "Each required report is supplied on the stated cycle to the necessary-party population.", "Successive report delivery timestamps and recipient reconciliation including FedRAMP, all using agency customers and additional parties required by agreements.", "A report saved internally without evidence that it was supplied to necessary parties."},
		{"covering the entire period since the previous summary", "Report coverage joins the previous summary boundary without unexplained gaps.", "Previous/current period boundaries and reconciliation to certification-data, change, vulnerability and incident histories over the entire interval.", "Only the most recent week's data in a report labeled quarterly."},
		{"in a consistent format that is human readable", "Delivered reports have a consistent human-readable form.", "Actual delivered report revisions with readable narrative/structure and a comparison across the series.", "Machine-only telemetry or a new incompatible layout with no consistent report presentation."},
	},
	"CCM-OCR-AFS": {
		{"an anonymized and desensitized summary", "The supplied summary removes identifying and sensitive details while conveying the feedback's substance.", "Original feedback, questions and answers; transformation/redaction decisions; exact released summary and attributable disclosure review.", "Raw correspondence with agency names removed but sensitive technical detail still exposed."},
		{"as an addendum to the Ongoing Certification Report OR in the next Ongoing Certification Report", "The summary is supplied using one of the two stated report destinations.", "Originating report ID, released addendum or next-report revision containing the summary, and delivery/publication evidence.", "An internal summary disconnected from either permitted report destination."},
	},
	"CCM-QTR-MTG": {
		{"host a synchronous Quarterly Review every 3 months", "When required or adopted for the selected class, synchronous reviews actually occur on the specified cycle.", "Class-specific decision, selected recurrence and meeting occurrence records with dates; where permitted, documented reasons for non-hosting.", "A recurring calendar invite without actual hosting or a written report substituted for a synchronous meeting."},
		{"open to all necessary parties", "The review is accessible to the complete necessary-party audience.", "Recipient/permission reconciliation including FedRAMP, using agency customers and agreement-required parties; meeting access instructions and access observations.", "Attendance limited to those who happened to receive an invitation, without an audience denominator."},
		{"the most recent Ongoing Certification Reports", "The discussion is grounded in the recent report material the provider considers most relevant to agencies.", "Meeting agenda/content linked to exact recent report revisions and topic relevance judgments.", "A sales presentation with no connection to the recent report series."},
	},
	"CCM-QTR-SRR": {
		{"removes all agency information (comments, questions, names, etc.)", "If sharing is elected, every form of agency information is removed from the released recording/transcript.", "Original/released media comparison covering audio, video, speaker labels, participant lists, captions, comments, questions, names and metadata; redaction review and release hash.", "Removing names from a transcript while leaving agency-identifying audio, questions or metadata."},
		{"determines doing so will NOT likely have an adverse effect on the cloud service offering", "The provider makes a content- and audience-specific no-likely-adverse-effect determination before optional release.", "Attributable release assessment tied to the exact redacted recording/transcript and target public or third-party audience.", "Agency anonymization alone without an assessment of remaining harmful technical disclosures."},
	},
	"CCM-QTR-SAR": {
		{"at least 3 business days after releasing an Ongoing Certification Report", "The recommendation's lower preparation-time bound is met or its treatment is documented with weighed implications.", "Actual report release time, scheduled review time, business-day calendar/count and security-documentation decision for any departure.", "Counting from report draft creation rather than release."},
		{"within 10 business days of such release", "The recommendation's upper freshness bound is met or its treatment is documented with weighed implications.", "Report-release/review pair, business-day count and documented rationale for any departure.", "Checking only that the review is not too early."},
	},
	"CMU-CSO-CMD": {
		{"used in each service (or groups of services that use the same modules)", "The module inventory covers each federal-data cryptographic service or justified same-module group.", "Federal-data protection flow map, service inventory, deployed module/version observations and explicit grouping rationale.", "One enterprise-wide module name without service-level coverage or grouping evidence."},
		{"including whether these modules are validated under the NIST Cryptographic Module Validation Program or are update streams of such modules", "Every documented module has an explicit supported validation or update-stream status.", "Module entries with CMVP references or update-stream lineage and explicit unvalidated/unknown status where appropriate.", "Encryption-enabled booleans or an unqualified FIPS label in place of per-module status."},
	},
	"CMU-CSO-UVM": {
		{"with active validations under the NIST Cryptographic Module Validation Program", "Where validated use is required or elected, the claimed validation is active and applies to the actual module/algorithm scope.", "CMVP status and observation date, deployed module/version/environment, vendor update lineage and algorithm comparison excluding new never-validated algorithms.", "A historical certificate, validation for a different module or new algorithms treated as inherited validation."},
		{"when using cryptographic services to protect federal customer data", "The decision covers actual federal-customer-data protection uses, not unrelated test encryption.", "Federal customer data flows and storage/protection paths mapped to effective runtime cryptographic services and module versions.", "A validated test environment with unexamined production data-protection paths."},
	},
	"CMU-CSO-CAT": {
		{"configure agency tenants by default", "The recommendation is evaluated for default agency provisioning rather than isolated opt-in configurations.", "Agency provisioning templates, policy inheritance, default configuration versions and observed newly provisioned tenant settings, plus the documented SHOULD decision.", "A hand-configured demonstration tenant or customer opt-in setting."},
		{"when such modules are available", "Availability is assessed for the relevant service and default rather than assumed universally.", "Available module/service options, active-validation/update-stream provenance and supported availability limitations tied to the agency-tenant default decision.", "Treating module unavailability as an undocumented blanket exemption for every service."},
	},
	"CPO-CSO-OVR": {
		{"within their FedRAMP Certification Package", "The overview is included in the actual service package.", "Published package index/manifest and retrievable overview revision referenced by that package.", "A standalone overview draft outside the supplied package."},
		{"in both human-readable and JSON formats", "Both representations are available and contain equivalent overview information.", "Paired published artifacts and field/content comparison, including exact version and update provenance.", "One representation or syntactically valid JSON with missing substantive content."},
	},
	"CPO-CSO-OSA": {
		{"supplied by the assessor per IVV-IAS-OSA (Overall Summary of Assessment)", "The overview's overall summary is attributable to the assessor and correct independent assessment.", "Assessor transmittal, exact summary revision, assessment scope/period and comparison to the included overview material.", "Provider-authored replacement language or a summary from another service's assessment."},
	},
	"SCN-CSO-EMG": {
		{"during an emergency or incident", "The event supports use of the emergency permission rather than ordinary late planning.", "Attributable incident/emergency record, response timeline and linkage to the deployed significant change.", "A change marked emergency solely because the normal advance-notice window was missed."},
		{"follow all relevant procedures", "Emergency execution follows the procedures relevant to the actual incident/change.", "Applicable procedure versions, incident command/change records and execution observations, with deviations preserved.", "A procedure document without evidence of its use during the event."},
		{"notify all necessary parties", "Emergency follow-through includes the necessary-party population.", "Emergency-change notice payload and delivery history reconciled to FedRAMP, using agency customers and agreement-required parties.", "Only internal incident escalation or notification of a single customer."},
		{"retroactively provide all Significant Change Notification materials", "The advance-process permission is followed by complete retroactive SCN materials.", "Change classification, required information, applicable plan/update/impact material and actual provision trail linked to the emergency.", "An incident report or one-line change announcement used in place of the complete retroactive materials."},
		{"complete appropriate assessment after the incident", "Appropriate post-incident assessment is actually completed.", "Incident conclusion milestone, justified assessment scope, executed verification/validation/assessment evidence and results after the incident.", "An assessment plan, incident closure status or deployment success without post-incident assessment."},
	},
	"SCN-CSO-HRM": {
		{"ALL Significant Change Notifications and related audit records", "The formats requirement covers both record populations completely.", "Notification and evaluation-audit inventories reconciled to paired published human-readable and JSON revisions, with explicit missing-record findings.", "Exporting notifications while leaving evaluation audit records inaccessible or in only one format."},
	},
	"SCN-TRF-TPR": {
		{"before starting transformative changes if human validation is necessary", "The necessity decision and, where recommended, assessor review precede change start.", "Security-decision necessity analysis, dated assessor scope/impact review or documented non-engagement explanation, and actual change-start milestone.", "Assessor engagement after deployment or unsupported assertions of no need for human validation."},
		{"limited to security decisions that require human validation", "The recommendation is scoped to security decisions requiring human judgment.", "Mapping of planned security decisions to human-validation need and actual assessor review scope; documentation of any weighed departure.", "A blanket assessment scope with no distinction between human security judgment and machine-verifiable facts."},
	},
}
