package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// SCR keeps identification, review, mitigation and upstream monitoring distinct.
func SCR(rules, markdown fs.FS) (readiness.Package, error) {
	return technicalDraft(rules, markdown, "SCR", "supply-chain-risk", 6, []technicalRecipe{
		{code: "MIT", statement: "Persistently identify, review, and mitigate potential supply chain risks.",
			purpose: "Trace repeated supply-chain risk identification through analysis to actual risk-reducing measures; a supplier questionnaire or risk register alone does not show mitigation.", capability: "supply-chain-dependency-boundary",
			scope:   "Potential supply-chain risks across suppliers, software dependencies, build/distribution channels and relevant outsourced services supporting the offering; do not limit the risk set to published software vulnerabilities.",
			setup:   "Locate supplier/dependency inventories, supply-chain risk methods and owners, risk assessments and retained mitigation outcome records.",
			records: "Supplier/dependency and delivery-chain mapping, dated risk identification inputs, likelihood/impact or contextual review findings, implemented mitigation records and repeated activity status.",
			review:  "Trace potential risks from source and affected dependency to review and implemented risk-reducing measures. Distinguish detection, acceptance and mitigation; preserve residual risk and unsupported claims. Inspect existing decisions and outcomes without changing suppliers or systems.", insufficient: "A questionnaire, risk acceptance signature, software bill of materials or mitigation plan with no implemented risk-reducing evidence.",
			facets: []technicalFacet{{"identify", "Potential supply-chain risks are repeatedly identified.", "Dated intelligence/supplier/dependency inputs and identified risks tied to the supply chain.", "Only a supplier name list."}, {"review", "Identified supply-chain risks receive repeated review.", "Risk analyses, affected-service mapping and attributable review decisions over time.", "A raw vulnerability feed without contextual risk review."}, {"mitigate", "Potential supply-chain risks are repeatedly mitigated.", "Risk-linked implemented safeguards or supplier/process changes and outcome evidence.", "A risk acceptance or planned action misrepresented as mitigation."}}},
		{code: "MON", statement: "Third party software information resources are automatically monitored for upstream vulnerabilities using mechanisms that may include contractual notification requirements or active monitoring services.",
			purpose: "Verify operating automated upstream-vulnerability monitoring for third-party software, allowing contractual notifications or active monitoring mechanisms without treating a signed clause alone as running monitoring.", capability: "supply-chain-dependency-boundary",
			scope:   "Third-party software resources and relevant upstream products/versions, including libraries, images, packages and externally supplied software services used by the offering.",
			setup:   "Locate authoritative third-party software/version inventory, upstream supplier identifiers, notification agreements/subscriptions, monitoring jobs and delivery/coverage records.",
			records: "Dependency-to-upstream mapping, version-aware monitoring configurations, automated run/ingestion history, advisories/notifications and delivery timestamps; for a contractual route retain notification obligations plus evidence the notification/monitoring mechanism operates.",
			review:  "Reconcile software versions with automatic upstream monitoring coverage and verify advisories reach the responsible workflow. Compare contractual notification and active monitoring evidence on actual coverage and operation; do not infer no vulnerabilities from an empty or disconnected feed.", insufficient: "A static SBOM, manual occasional vendor browsing, an empty findings list without feed health or a notification contract with no operating mechanism.",
			facets: []technicalFacet{{"automatically monitored for upstream vulnerabilities", "Upstream vulnerability monitoring is automatic for third-party software resources.", "Software/version inventory, automated monitoring configurations, run/ingestion results and resource coverage reconciliation.", "Manual periodic searches only."}, {"may include contractual notification requirements or active monitoring services", "The chosen mechanism supports actual automated monitoring; the listed mechanisms are examples, not jointly mandatory.", "For supplier notifications: agreement scope, automated delivery/ingestion observations and resource mapping. For active services: subscriptions, run results and coverage mapping.", "Requiring both routes, or treating a contract as proof notifications operate."}}},
	})
}
