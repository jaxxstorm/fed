package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// CMT returns source-backed drafts for all 12 change-management variants.
func CMT(rules, markdown fs.FS) (readiness.Package, error) {
	return technicalDraft(rules, markdown, "CMT", "change-management", 12, []technicalRecipe{
		{code: "LMC", statement: "Modifications to the cloud service offering are logged and monitored.",
			purpose: "Demonstrate both a record of offering modifications and observation of those modifications, including changes outside the standard deployment pipeline.", capability: "offering-change-history",
			scope:   "Modifications across the cloud service offering, including code, infrastructure, configuration, permissions and relevant non-machine service processes; identify emergency and out-of-band paths.",
			setup:   "Locate change sources, offering boundaries, deployment and administrative audit trails, monitoring ownership and change-to-observation correlations.",
			records: "Change inventories, actor/time/target/before-after or revision records, pipeline and administrative events, monitoring outputs and reconciliations of expected versus observed modifications.",
			review:  "Reconcile modifications from independent deployment and administration sources with log and monitoring coverage. Verify actual monitoring outputs, not just recording settings, and explain omitted change categories.", insufficient: "A source commit list, logging enabled setting or approved change ticket without monitoring evidence.",
			timing: "When modifications occur, retain their logged timestamps and associated monitoring observations. The statement supplies no numeric monitoring interval or retention duration.",
			facets: []technicalFacet{{"logged", "Offering modifications have attributable log records.", "Change identifiers, timestamps, actors, targets and deployed/configuration revisions.", "Approved plans without executed-change records."}, {"monitored", "Offering modifications are monitored.", "Monitoring observations and findings tied to actual change records and covered modification types.", "Logs retained but never monitored."}}},
		{code: "RMV", statement: "Changes to machine-based information resources are executed through the redeployment of version controlled resources rather than direct modification wherever reasonable.",
			purpose: "Assess whether machine-resource changes use redeployment of version-controlled resources wherever reasonable, preserving a reasoned direct-modification exception rather than inventing an absolute prohibition.", capability: "offering-change-history",
			scope:   "Machine-resource change paths and deployments across repositories, infrastructure, images and managed-service configurations, including emergency/direct modifications.",
			setup:   "Locate change/deployment history, version-controlled resource definitions, artifact provenance and rationale for change classes where redeployment is not reasonable.",
			records: "Resource commit/version IDs, build/artifact-to-deployment links, redeployment execution observations, direct-modification inventory and context-specific reasonableness decisions.",
			review:  "Trace changes from version-controlled definitions to redeployed resources. Reconcile direct modifications and examine why redeployment was unreasonable for each relevant class; do not treat IaC ownership as evidence every change used redeployment.", insufficient: "A Git repository alone, undeployed commits or an unexplained blanket exception for manual changes.",
			timing: "When machine-resource changes occur, identify the executed change method and source revision. Preserve contemporaneous or attributable rationale for the wherever-reasonable condition; no numeric deadline is specified."},
		{code: "RVP", statement: "The effectiveness of documented change management procedures is persistently reviewed.",
			purpose: "Review whether documented change procedures work in practice using actual changes or justified evaluations, rather than merely approving a procedure document.", capability: "offering-change-history",
			scope:   "Documented change-management procedure versions and the offering's normal, emergency, failed and rollback change paths to which they apply.",
			setup:   "Locate versioned change procedures, process owners, change outcome history and effectiveness-review methods and conclusions.",
			records: "Procedure revisions, examined change populations, success/failure/rollback observations or exercise results, evaluation methods and dated effectiveness findings and conclusions.",
			review:  "Determine whether reviews evaluate procedural effectiveness against actual outcomes and cover relevant change paths over time. Separate document currency, approval and training from evaluation of whether the procedures work.", insufficient: "An approved change policy or meeting attendance list with no procedure-specific effectiveness evaluation."},
		{code: "VTD", statement: "Persistent testing and validation of changes throughout deployment is automated.",
			purpose: "Evaluate automated repeated testing and objective validation across deployment stages, not only a pre-merge test or a successful release job.", capability: "offering-change-history",
			scope:   "Changes and their deployment stages from relevant predeployment checks through rollout and deployed-state validation; map repository, artifact and environment identities explicitly.",
			setup:   "Locate deployment-stage maps, automated test and validation definitions, change/artifact IDs and retained stage-by-stage execution history.",
			records: "Pipeline/controller revisions, stage coverage matrix, test definitions/results, objective expected-outcome validation criteria/results and dated runs linked to change and deployed artifact IDs.",
			review:  "Trace each evaluated change through stages and distinguish automated tests from validation that deployed capabilities support expected outcomes. Explain skipped stages, manual checks and persistent activity interruptions without executing deployments.", insufficient: "One pre-merge unit test, a manual release checklist or deployment success with no throughout-deployment validation.",
			timing: "When changes pass through deployment, preserve automated testing and validation observations throughout the relevant stages and repeated deployment history. No universal stage list or numeric cadence is specified.",
			facets: []technicalFacet{{"testing", "Repeated change testing throughout deployment is automated.", "Stage-to-test coverage, automated runs and change-specific results.", "Test configuration with no executed results."}, {"validation", "Repeated change validation throughout deployment is automated.", "Objective outcome criteria and automated validation results for deployment stages and deployed revisions.", "Pipeline exit code alone without validation of intended outcomes."}}},
	})
}
