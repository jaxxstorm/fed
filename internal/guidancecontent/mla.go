package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// MLA preserves the optional Class B log-access model and all unresolved variants.
func MLA(rules, markdown fs.FS) (readiness.Package, error) {
	return technicalDraft(rules, markdown, "MLA", "monitoring-logging-and-auditing", 17, []technicalRecipe{
		{code: "ALA", statement: "A least-privileged, role and attribute-based, and just-in-time access authorization model is used and persistently reviewed for access to log data based on organizationally defined data sensitivity.",
			purpose: "Examine log-access authorization against the organization's log-data sensitivity definitions and verify least privilege, role/attribute decisions, JIT access and repeated review.", capability: "log-data-boundary",
			scope:   "Log data, search interfaces, archives, export destinations and administrative access, mapped to organizationally defined sensitivity and all accessing principals.",
			setup:   "Locate the log-store inventory, sensitivity definitions, access policy and grant history, including exported copies and privileged bypass paths.",
			records: "Versioned sensitivity classifications, log-to-classification mapping, effective role/attribute policies, least-privilege rationale, JIT grant/expiry records and dated access-model reviews.",
			review:  "Compare effective log access to sensitivity, justified need, role and attribute inputs, and grant lifetime. Examine review conclusions and include archive/export access rather than stopping at the SIEM UI.", insufficient: "SIEM roles alone, no log sensitivity definition, or persistent administrator access presented as JIT.",
			facets: []technicalFacet{{"least-privileged, role and attribute-based", "Log authorization uses least privilege, roles and attributes.", "Role/attribute decisions and effective permissions linked to log-access need.", "Role names without effective decisions."}, {"just-in-time", "Log authorization is just in time.", "Time-bounded log-access grants and observed expiry/revocation.", "Permanent search access."}, {"based on organizationally defined data sensitivity", "Log-access decisions reflect the organization's sensitivity definitions.", "Sensitivity taxonomy, log classifications and policy mapping with repeated review.", "A generic data classification policy with no log mapping."}}},
		{code: "EVC", statement: "The configuration of machine-based information resources, especially infrastructure as code, is persistently evaluated and tested.",
			purpose: "Distinguish configuration evaluation from actual tests, including infrastructure-as-code definitions and the resources they create; a successful deployment is not itself security evaluation.", capability: "configuration-evaluation-boundary",
			scope:   "Machine-resource configurations, particularly IaC repositories, modules and deployed revisions; also cover machine resources configured outside IaC.",
			setup:   "Locate resource/configuration inventory, IaC revision mappings, evaluation criteria, test suites and retained runs.",
			records: "Configuration/IaC revisions, evaluated criteria, test definitions and dated results, deployed-resource mapping, evaluation conclusions and repeated activity status.",
			review:  "Reconcile configurations with evaluation and test coverage. Confirm tests exercised relevant security properties of the actual revisions and distinguish untested configuration from passing results.", insufficient: "Source code existence or a syntax-only pipeline success presented as complete configuration evaluation and testing.",
			facets: []technicalFacet{{"evaluated", "Configurations receive repeated substantive evaluation.", "Evaluation criteria and dated findings tied to configuration revisions.", "An unreviewed scan export."}, {"tested", "Configurations receive repeated testing.", "Test definitions, inputs, run dates and results tied to configuration/IaC revisions.", "Review signatures without tests."}}},
		{code: "LET", statement: "A list of information resources and event types that will be logged, monitored, and audited is maintained and persistently reviewed to ensure these activities occur.",
			purpose: "Connect the maintained resource/event-type list to actual logging, monitoring and auditing, rather than mistaking an enabled log sink for complete event coverage.", capability: "log-data-boundary",
			scope:   "Listed information resources and event types, including relevant non-machine resources and administrative activities, with logging, monitoring and auditing responsibilities identified.",
			setup:   "Locate resource/event matrices, collection routes, monitoring ownership, audit records and maintained-list history.",
			records: "Versioned resource/event-type list, source-to-sink mapping, representative received events, monitoring outputs, audit records and repeated coverage-review findings.",
			review:  "Reconcile the maintained list with observed logging, monitoring and auditing separately; identify silent sources, unsupported event types and dated reviews of actual coverage.", insufficient: "A list of desired events without evidence that logging, monitoring and auditing occur.",
			facets: []technicalFacet{{"A list of information resources and event types", "A maintained list identifies the resource/event denominator.", "Versioned list with resource identities and event categories.", "A SIEM connector list without event types."}, {"logged, monitored, and audited", "Repeated list review checks all three activities actually occur.", "Received events, monitoring findings, audit records and list-to-activity review conclusions.", "Received logs alone, with no monitoring or audit evidence."}}},
		{code: "OSM", statement: "A Security Information and Event Management (SIEM) or similar system(s) is used and persistently reviewed for centralized, tamper-resistant logging of events, activities, and changes.",
			purpose: "Demonstrate an operating SIEM or equivalent combination of systems that centralizes and resists tampering with event, activity and change logs, with repeated review.", capability: "log-data-boundary",
			scope:   "Service log sources, central stores, pipelines and administrative paths for events, activities and changes, including alternate/archive stores.",
			setup:   "Locate source-to-central-store mappings, log protection settings, administrator permissions, ingestion observations and system-review history.",
			records: "Logging architecture and source inventory, dated ingestion records for events/activities/changes, retention/protection configuration, write/delete privilege evidence, integrity observations and repeated reviews.",
			review:  "Verify central availability and tamper resistance against actual writer/administrator privileges and observed protection behavior. Evaluate review history and preserve limitations of equivalent multi-system logging architectures.", insufficient: "A SIEM license, centralized dashboard or encrypted storage alone without evidence of tamper resistance and repeated review.",
			facets: []technicalFacet{{"centralized", "Logging provides centralized access to the relevant event/activity/change records.", "Source-to-store reconciliation and dated received records across the service.", "One application's logs generalized to the offering."}, {"tamper-resistant", "Logging resists tampering in the deployed access and storage model.", "Protection policy versions, effective modification/deletion privileges, integrity checks and review findings.", "Encryption at rest alone, while unrestricted administrators can alter records."}}},
		{code: "RVL", statement: "Logs are persistently reviewed and audited.",
			purpose: "Establish repeated review and audit of log contents with attributable findings; ingestion and storage success do not establish either activity.", capability: "log-data-boundary",
			scope:   "Service log datasets and review/audit populations, including declared exclusions and periods covered; reconcile against the log-resource/event inventory.",
			setup:   "Locate review queries or procedures, audit methods, reviewer ownership, findings and the log datasets/periods examined.",
			records: "Dated review outputs, audit working records, query/method versions, evaluated log windows, findings or justified no-finding conclusions and repeated activity status.",
			review:  "Trace review and audit conclusions to actual logs, methods and periods. Explain sampling and the distinct audit activity; an automated alert needs retained examination and conclusions to support a review claim.", insufficient: "An empty alert queue, stored logs or a dashboard login without review/audit conclusions.",
			facets: []technicalFacet{{"reviewed", "Logs receive repeated review.", "Review queries, log windows and dated conclusions.", "Ingestion health alone."}, {"audited", "Logs receive repeated audit.", "Audit method, examined log sets and attributable audit findings/conclusions.", "Alert triage presented as audit with no audit method or records."}}},
	})
}
