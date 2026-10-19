package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

func MKT(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("MKT", rules, markdown, map[string]administrativeRecipe{
		"MKT-CSO-MLR": {
			"Prepare the required public certification information before requesting a new or updated Marketplace listing.", "Provider Marketplace and public-information owner",
			"Obtain the current CDS-CSO-PUB public-information requirements, offering identity and public human-readable/machine-readable endpoints.",
			"Map the listing application to CDS-CSO-PUB and verify available/applicable fields: FedRAMP ID, service/deployment models, business category, UEI, sales/security contacts, website, viewable logo image, service description and service/security-category list, secure configuration guide, documentation overview, Trust Center access instructions, next OCR date and recognized assessor. Validate the referenced JSON schema and correct inconsistent facts before submission.",
			"Address CDS-CSO-PUB Public Information when applying for a new listing or requesting an existing listing update; preserve the version of public information used for the request.",
			"public-information endpoints and captured contents, applicable schema/version and validation results, CDS-CSO-PUB requirement mapping, and the listing request/update record",
			"Check the public-information rule's available/applicable fields and human/JSON consistency. Verify that common web-based tools can consume the public JSON and the logo is a directly viewable image. A populated form does not establish CDS-CSO-PUB; the listing narrative expects complete applications.",
			"The exact offering and public-information revision used for each new listing or update request.",
			"At application for a new listing OR request to update an existing listing. This clause creates no periodic listing-update deadline.",
			"A request receipt or landing page without the required public information and its machine-readable counterpart.",
		},
		"MKT-CSO-PML": {
			"Use the official provider listing request channel so FedRAMP receives a complete identifiable request.", "Provider Marketplace listing owner",
			"Locate the service's listing information and authorized submitter; obtain the official FedRAMP Marketplace Provider Listing Request Form.",
			"Prepare the form with the offering identity and requested listing, reconciling its data with the public-information package.",
			"Notify FedRAMP using the published Marketplace Provider Listing Request Form to request listing. FedRAMP does not accept listing applications via email.",
			"the submitted form contents, offering identity, form URL and timestamped submission confirmation or ticket",
			"Verify submission used the provider form and matches the offering; distinguish requesting a listing from FedRAMP approving it or certifying the service.",
			"This provider's request for a Marketplace listing, not assessor/advisor requests or a FedRAMP certification determination.",
			"When requesting listing; retain the actual submission timestamp. The rule does not supply a FedRAMP response-time guarantee.",
			"An emailed application, draft form, or submission receipt represented as approved listing or certification.",
		},
	})
}
