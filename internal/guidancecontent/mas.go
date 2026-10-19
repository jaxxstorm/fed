package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

func MAS(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("MAS", rules, markdown, map[string]administrativeRecipe{
		"MAS-CSO-IIR": {
			"Define the offering by likely handling of federal customer data or likely impact to its confidentiality, integrity or availability, rather than by a convenient cloud-account boundary.", "Provider boundary architect and resource owners",
			"Locate architecture, data-handling paths, technical and managerial resource inventories and their accountable owners; identify federal customer data separately from provider-generated metadata.",
			"Evaluate resources against both handling and security-impact criteria. Include personnel, policies and processes as well as machines; record justified exclusions, including separately delivered software not operated in a shared-responsibility model.",
			"Identify and maintain the assessed resource set in the certification package. Reconcile the machine-readable inventory with architecture and human explanations, retaining the generation code and its coverage limitations.",
			"machine-readable resource output, its human-readable derivation explanation, generation code/version, resource-owner mapping and inclusion/exclusion rationale",
			"Challenge omissions across technical and managerial resources and dependencies. Likely means a reasonable probability in context; lack of direct storage is not proof of no confidentiality, integrity or availability impact.",
			"All information resources likely to handle federal customer data or impact its confidentiality, integrity or availability, with explicit offering and excluded-resource boundaries.",
			"Retain the assessed inventory version and observation dates for the selected certification/assessment; update the boundary as the offering changes. MAS supplies no fixed inventory refresh deadline.",
			"A cloud-resource export that omits non-machine resources or an unexplained account boundary treated as the entire offering.",
		},
		"MAS-CSO-FLO": {
			"Explain permitted information flows and security categories for the entire identified offering, including resources with different appropriate categories.", "Provider boundary architect and data owners",
			"Obtain the MAS resource set, connection sources and information/security-category decisions for each resource or documented resource group.",
			"Map source, destination, handled information and permitted connections to inventory identities; explain FIPS 199 security categories without equating them to certification classes.",
			"Document and explain information flows and security categories for every resource or set. Preserve the allowance for categories to vary with information handled or impacted, including third-party resources.",
			"machine-readable permitted-connection output, human-readable derivation explanation, generating code, flow explanations and resource/group-to-security-category mapping",
			"Reconcile the flow/category map to all resources, inspect unexplained connections and group membership, and assess category rationale against handled or impacted information.",
			"ALL information resources or explicit resource sets in the offering, with third-party connections and category differences identified.",
			"Use flow, inventory and category versions corresponding to the assessed offering and record actual collection dates; no numeric update cadence is stated.",
			"A network diagram without information types, permitted connections, category rationale or full resource coverage.",
		},
		"MAS-CSO-TPR": {
			"Explain third-party resource impact and mitigations only where MAS-CSO-IIR applies; third-party means not entirely inside this offering's minimum scope.", "Provider boundary architect and third-party service owners",
			"Resolve MAS-CSO-IIR applicability and obtain the offering boundary and dependency inventory; identify resources not entirely included in that scope.",
			"For each applicable dependency record actual usage/configuration, why it is used, mitigation measures and compensating controls addressing federal customer data impact.",
			"When MAS-CSO-IIR applies, document the four named topics for each applicable third-party resource and keep the records aligned with actual configurations and impact paths.",
			"machine-readable third-party resource records, human-readable derivation explanation, generating code, and per-resource usage/configuration, justification, mitigation and compensating-control evidence",
			"Verify each third-party entry against the boundary and dependency paths. Assess mitigations and compensating controls separately; another service's certificate alone does not describe this usage or reduce its impact.",
			"Each third-party information resource used by the offering that meets MAS-CSO-IIR's handling/impact condition; preserve the ONLY IF gate.",
			"Use the actual dependency/configuration and scope versions under assessment. Retain dates of impact and mitigation evaluation without inventing a vendor-review interval.",
			"A vendor list, procurement record or third-party certificate without all four per-resource explanations and supporting configuration facts.",
		},
		"MAS-CSO-MDI": {
			"Include metadata when it meets MAS-CSO-IIR's applicability condition even though provider-generated metadata is not itself federal customer data under the definition.", "Provider data architect and telemetry owners",
			"Resolve MAS-CSO-IIR applicability and locate metadata stores, telemetry, account information, analytics and metadata about federal customer data.",
			"Evaluate metadata resources for handling and security impact and map qualifying metadata into the minimum scope with an explicit rationale.",
			"Include qualifying metadata, including metadata about federal customer data, in the minimum assessment scope when MAS-CSO-IIR applies.",
			"machine-readable metadata inventory, human-readable derivation explanation, generation code and metadata-to-boundary handling/impact rationale",
			"Check that metadata is neither categorically excluded because it is provider-generated nor indiscriminately equated with federal customer data; test the IIR condition for actual metadata resources.",
			"Metadata collected or maintained by the offering that meets the MAS-CSO-IIR condition, including metadata about federal customer data.",
			"Retain metadata and scope versions for the assessment and dates of relevant changes; no independent metadata review deadline is supplied.",
			"A statement that metadata is not federal customer data used as a blanket justification to omit its security impact.",
		},
		"MAS-CSO-SUP": {
			"Allow useful out-of-offering supplemental information without suggesting those resources are FedRAMP Certified.", "Provider certification package owner",
			"Locate the defined offering boundary and any proposed supplemental materials for resources outside it.",
			"Decide whether to include a supplement; if included, visibly mark and separate supplemental resources from the certified offering in package structure and labels.",
			"Optionally supply supplemental material; when supplied, clearly mark and separate the resources and state that they are not FedRAMP Certified.",
			"the offering boundary, supplement publication/version, visible separation and certification-status labels, or the documented non-use decision",
			"Read the package as a customer: verify that no supplemental resource is presented as certified. The MAY applies to inclusion; the MUST separation applies once included.",
			"All optional package supplements describing resources outside the cloud service offering.",
			"When a supplement is included or revised; retain the published version/date and boundary it accompanies. No periodic publication obligation is created.",
			"A combined resource list or marketing annex that obscures which resources are certified.",
		},
	})
}
