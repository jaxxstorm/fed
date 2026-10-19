package guidancecontent

import (
	"fmt"
	"io/fs"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

// INR is authored separately from CED: after-action reporting, procedure review,
// and historical pattern review have different prerequisites and proof.
func INR(rules, markdown fs.FS) (readiness.Package, error) {
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return readiness.Package{}, err
	}
	p := readiness.Package{Format: readiness.Format, CatalogID: catalog.PinnedCommit + ":" + storage.Hash(raw), Revision: 1, Sources: map[string]readiness.Source{"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit}, "site": {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"}}}
	const aar = "Incident after action reports are generated and lessons learned are persistently incorporated."
	const rir = "The effectiveness of documented incident response procedures is persistently reviewed."
	const rpi = "Past incidents are persistently reviewed for patterns or vulnerabilities that were not previously apparent or identified."
	for _, ref := range []struct{ id, source, path, anchor, excerpt string }{
		{"aar", "rules", "rules.json", "/KSI/INR/indicators/KSI-INR-AAR/statement", aar},
		{"rir", "rules", "rules.json", "/KSI/INR/indicators/KSI-INR-RIR/statement", rir},
		{"rpi", "rules", "rules.json", "/KSI/INR/indicators/KSI-INR-RPI/statement", rpi},
		{"aar-page", "site", "providers/20x/key-security-indicators/incident-response.md", "generating-after-action-reports", aar},
		{"rir-page", "site", "providers/20x/key-security-indicators/incident-response.md", "reviewing-incident-response-procedures", rir},
		{"rpi-page", "site", "providers/20x/key-security-indicators/incident-response.md", "reviewing-past-incidents", rpi},
		{"persistent", "site", "definitions.md", "persistently", persistentDefinition},
		{"incident", "site", "definitions.md", "incident", `Has the meaning given in 44 USC § 3552 (b)(2) which is "an occurrence that (A) actually or imminently jeopardizes, without lawful authority, the integrity, confidentiality, or availability of information or an information system; or (B) constitutes a violation or imminent threat of violation of law, security policies, security procedures, or acceptable use policies."`},
		{"vulnerability", "site", "definitions.md", "vulnerability", "This includes gaps in Rev5 Controls and 20x Key Security Indicators, software vulnerabilities, misconfigurations, exposures, weak credentials, insecure services, and all other such potential weaknesses in protection (intentional or unintentional)."},
		{"vulnerability-response", "site", "definitions.md", "vulnerability-response", "The systematic process of tracking, evaluating, mitigating, monitoring, remediating, assessing exploitation, reporting, and otherwise managing detected vulnerabilities."},
		{"class-a", "site", "providers/20x/rules/fedramp-certification.md", "mandatory-fedramp-rules-for-class-a", "Providers seeking a Class A FedRAMP Certification [MUST](../../../definitions/#must){ data-preview } address all rules in this FedRAMP Class A Certification subset (FRC-CLA) AND the following additional FedRAMP Class A rules; the appropriate artifacts or information mapping for all rules [MUST](../../../definitions/#must){ data-preview } be supplied in the FedRAMP Certification Package."},
		{"class-a-rir", "site", "providers/20x/rules/fedramp-certification.md", "mandatory-fedramp-rules-for-class-a", "Key Security Indicators: [KSI-INR-RIR (Reviewing Incident Response Procedures)](../key-security-indicators/incident-response.md#reviewing-incident-response-procedures){ data-preview }"},
	} {
		root := markdown
		if ref.source == "rules" {
			root = rules
		}
		c, e := citation(root, ref.id, ref.source, ref.path, ref.anchor, ref.excerpt)
		if e != nil {
			return p, e
		}
		p.Citations = append(p.Citations, c)
	}
	official := func(text string, refs ...string) readiness.Statement {
		return readiness.Statement{Text: text, Basis: readiness.Official, Citations: refs}
	}
	recommend := func(text, why string, refs ...string) readiness.Statement {
		return readiness.Statement{Text: text, Basis: readiness.Recommendation, Citations: refs, Rationale: why}
	}
	step := func(id, owner, action, completion, ref string) readiness.Step {
		return readiness.Step{ID: id, OwnerRole: owner, Trigger: "one_time", Action: recommend(action, "This makes the incident-related obligation assessable through retained records, without requiring a particular incident-management vendor.", ref), CompletionCriteria: []readiness.Statement{recommend(completion, "The record must connect the conclusion to the actual incident, procedure version or historical period evaluated.", ref)}}
	}
	p.Capabilities = []readiness.Capability{
		{ID: "incident-history", Name: "Retained incident history", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-incident-history", "Incident response owner", "Locate the service's incident register and retained investigation records. Record the period and service boundary covered, including any claimed no-incident period.", "The inventory identifies actual incidents or the supported no-incident period; missing access or absent records are not treated as no incidents.", "incident")}}},
		{ID: "incident-procedures", Name: "Documented incident response procedures", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-response-procedures", "Incident response owner", "Locate the current documented incident response procedures, accountable owners and version history for the service.", "The procedure version, service boundary and responsible owners are identifiable; a document's existence is not proof that its effectiveness was reviewed.", "rir")}}},
	}
	timing := official(persistentDefinition, "persistent")
	type assertionRecipe struct{ id, clause, obligation, fact, records, insufficient string }
	for _, recipe := range []struct {
		id, ref, statement, purpose, capability, dependency, collect, criteria string
		required                                                               []string
		unresolved, excluded                                                   string
		assertions                                                             []assertionRecipe
	}{
		{"KSI-INR-AAR", "aar", aar, "After an incident, retain an after-action report and show that lessons are repeatedly incorporated into how the service operates. Writing a report is not the same as incorporating its lessons.", "incident-history", "After-action reporting needs an identifiable incident and its retained investigation record; this inferred dependency does not mandate a ticketing product.", "Collect the incident inventory, after-action reports and records showing how identified lessons were incorporated over time.", "Separate report generation from lesson incorporation. Link each claimed lesson to its disposition and resulting practice; preserve open lessons rather than treating a report as completion.", []string{"fdcf18d017762f0ad1d66a5a1cac6557f48c961a03a1a62c88659b66fce3ac1f"}, "a9cfb736191940ba3c1c5433bf98cd3dc28af909a099f7234f02a9055d3c1001", "e90bc59a9e3d6f764e244adbd30242307629a93c92cf52feb6e1c61f110396d3", []assertionRecipe{
			{"after-action-reports", "Incident after action reports are generated", "Generate incident after-action reports.", "Actual incidents have attributable after-action reports.", "Incident identifiers, investigation timeline and attributable after-action reports reconciled to the incident inventory.", "An empty report template, or a claim of no incidents based only on missing records."},
			{"incorporated-lessons", "and lessons learned are persistently incorporated", "Persistently incorporate lessons learned.", "Lessons from incidents are incorporated repeatedly, with current activity status known.", "Lesson/action history linked to reports, dispositions and dated changes to procedures or operating practices; explain intentional interruptions.", "A list of lessons or open action items without evidence of incorporation."},
		}},
		{"KSI-INR-RIR", "rir", rir, "Review whether the documented response procedures work effectively. A current runbook alone is not proof of review, and a review record alone does not guarantee every future response will succeed.", "incident-procedures", "A documented procedure with an identifiable version is necessary to assess whether its effectiveness was reviewed; the indicator does not prescribe a specific format.", "Collect the current response procedures and dated effectiveness reviews from actual responses, exercises or other justified evaluation methods.", "Cite the procedure version, evaluation method, findings and conclusions. Explain repeated review and known activity status; do not substitute attendance or a document approval for effectiveness review.", []string{"18b07ca12e87629e048bb4606cf757bff16b2d8137da65c978c17a60428542be", "39d1bfd1c0c5e3edac2b2b8a05fccc443502e30465fe97657df7cee093a0b008"}, "3d77e33508b3a1e98bdde7bea08640910aedaf62f5ae87b82b3f70d83dd1ea3d", "", []assertionRecipe{
			{"documented-procedure-review", rir, "Persistently review the effectiveness of documented incident response procedures.", "Documented procedures have attributable repeated effectiveness reviews.", "Procedure versions, dated review history, evaluation methods and findings, current review status, and dispositions or rationale for findings.", "A runbook, document-approval signature, or exercise attendance list without effectiveness conclusions."},
		}},
		{"KSI-INR-RPI", "rpi", rpi, "Revisit the service's incident history to find patterns or weaknesses that were not apparent earlier. This is a retrospective review, not just investigation of the latest incident.", "incident-history", "Historical pattern review needs accessible prior incident records and an identified historical period; this inferred prerequisite does not require a particular analytics product.", "Collect the incident-history inventory and dated retrospective reviews explaining newly considered patterns or vulnerabilities, their scope and conclusions.", "Identify the historical incident set reviewed, analytical method and findings, including justified no-new-finding conclusions. Demonstrate repetition and known status rather than presenting one incident ticket as the whole historical review.", []string{"610cdf5a6ad2f5d6e568d770273ccb555260e75ffdd761624893ebac3b537a8f"}, "27da685c9949027d91b2b69588299116f0bf6d671e1e23463ffaf83962128c50", "59cf3594b8782b26941c73284557d5623d2808981360c712a518bca7f574caa5", []assertionRecipe{
			{"retrospective-pattern-review", rpi, "Persistently review past incidents for previously unrecognized patterns or vulnerabilities.", "Repeated historical reviews look for previously unrecognized patterns or vulnerabilities and retain their conclusions.", "Historical incident-set inventory, review dates and methods, cross-incident pattern/vulnerability analysis and findings or justified no-new-finding conclusions.", "Only the latest incident ticket, an automated scan unconnected to incident history, or an assertion that no new findings proves no review was needed."},
		}},
	} {
		refs := []string{recipe.ref, recipe.ref + "-page", "incident", "persistent"}
		if recipe.ref == "rpi" {
			refs = append(refs, "vulnerability")
		}
		if recipe.ref == "rir" {
			refs = append(refs, "vulnerability-response")
		}
		book := readiness.Playbook{Kind: "actionable", ID: recipe.ref + "-required", Revision: 1, RequirementID: recipe.id, VariantIDs: recipe.required, OfficialStatement: recipe.statement,
			Purpose:       recommend(recipe.purpose, "This explanation distinguishes the specific incident-review outcome from records that only show setup or participation.", refs...),
			Applicability: recommend("This exact selected catalog variant resolves the indicator as required. Preserve the saved profile's class and certification path when assessing it.", "Applicability is bound to the inventoried variants rather than inferred from the incident-management implementation.", recipe.ref, "class-a", "class-a-rir"),
			Prerequisites: []readiness.Dependency{{CapabilityID: recipe.capability, Reason: recommend(recipe.dependency, "The dependency supplies the object of this indicator's review; it is not a new separately quoted mandate.", recipe.ref)}},
			Setup:         readiness.Steps{NoneReason: "Use the linked shared capability setup; this indicator prescribes no additional product integration."},
			Collect:       readiness.Steps{Items: []readiness.Step{step("collect-"+recipe.ref, "Incident response owner", recipe.collect, recipe.criteria, recipe.ref)}},
			ReviewSteps:   readiness.Steps{Items: []readiness.Step{step("assess-"+recipe.ref, "Service assessment owner", recipe.criteria, "The assessment cites exact retained revisions, covers the relevant period and service boundary, and preserves unsupported assertions and limitations for separate human evidence review.", recipe.ref)}},
		}
		for _, a := range recipe.assertions {
			operation := readiness.Step{ID: a.id, OwnerRole: "Incident response owner", Trigger: "recurring", Timing: &timing, Action: official(a.obligation, refs...), CompletionCriteria: []readiness.Statement{recommend(a.records, "These records make the specific operating fact assessable; records are candidates until evaluated with exact citations.", recipe.ref)}}
			if a.id == "after-action-reports" {
				event := recommend("When an actual incident supplies the subject of an after-action report; this statement supplies no numeric reporting deadline.", "After-action reporting is event-dependent. Preserve any separately applicable incident communication deadlines under their own rules.", "aar", "incident")
				operation.Trigger = "event"
				operation.Timing = &event
			}
			book.Operate.Items = append(book.Operate.Items, operation)
			acceptance := readiness.Acceptance{Fact: a.fact, Scope: recommend("Identify the service boundary and actual incident set or procedure versions evaluated, including differences in production, development and shared responsibilities where relevant.", "One incident or one team's procedure is not automatically coverage of the entire relevant service history.", recipe.ref), TimePeriod: recommend("Retain review dates, historical period, actual cycles and current activity status. Explain intentional gaps; do not invent a numeric persistent-review interval.", "FRD-PER permits irregular cycles and documented interruptions.", "persistent"), Provenance: recommend("Retain attributable incident/report or procedure identifiers, version dates, reviewer, method, findings and conclusions.", "Provenance connects the conclusion to the actual material evaluated.", recipe.ref), Insufficient: []string{a.insufficient, "A declaration, empty template or successful collection without sufficiency assessment."}}
			for _, route := range []struct{ id, artifact, proves, limits string }{
				{"system-records", "Authorized incident-management or document-system exports, plus the associated review/conclusion records and scope inventory.", "System provenance and the recorded review or incorporation activity for the identified incidents or procedures.", "A ticket's existence or closed status alone does not establish the review's substance or incorporation of lessons."},
				{"retained-documents", "Dated, attributable reports or review minutes, versioned procedures/action records as relevant, and the corresponding scope/history inventory.", "The same review/reporting facts without requiring a particular incident platform.", "Documentary accounts require traceable incident/procedure references; a template or unsupported retrospective claim is insufficient."},
			} {
				acceptance.Alternatives = append(acceptance.Alternatives, readiness.EvidenceRoute{ID: route.id, Artifacts: []readiness.Statement{recommend(route.artifact, "These are alternative evidence combinations, not mandatory tooling.", recipe.ref), recommend(a.records, "This material addresses the assertion's specific operating fact.", recipe.ref)}, Proves: route.proves, Limitations: route.limits})
			}
			book.Assertions = append(book.Assertions, readiness.Assertion{ID: a.id, Obligation: official(a.obligation, refs...), Clauses: []string{a.clause}, Acceptance: acceptance})
		}
		p.Playbooks = append(p.Playbooks, book)
		decision := readiness.Playbook{Kind: "needs_decision", ID: recipe.ref + "-path-decision", Revision: 1, RequirementID: recipe.id, VariantIDs: []string{recipe.unresolved}, OfficialStatement: recipe.statement, Purpose: book.Purpose, Applicability: recommend("The saved certification path does not resolve this KSI's applicability; no implementation checklist is imposed by this variant.", "The inventory explicitly retains an unresolved path outcome.", recipe.ref, "class-a"), DecisionOwnerRole: "Service assessment owner", DecisionQuestion: "Which certification path and applicable KSI rule set are being assessed for this service? The saved path leaves this indicator unresolved."}
		p.Playbooks = append(p.Playbooks, decision)
		if recipe.excluded != "" {
			excluded := decision
			excluded.Kind = "excluded"
			excluded.ID = recipe.ref + "-class-a-excluded"
			excluded.VariantIDs = []string{recipe.excluded}
			excluded.DecisionOwnerRole = ""
			excluded.DecisionQuestion = ""
			excluded.Applicability = recommend("The selected Class A enumerated obligations do not include this indicator. This variant explains exclusion rather than asserting that incident review is unnecessary or that the service complies.", "Retain the exact not-applicable inventory variant; other classes or paths must resolve separately.", recipe.ref, "class-a")
			p.Playbooks = append(p.Playbooks, excluded)
		}
	}
	if err = readiness.ValidateDraft(p); err != nil {
		return p, fmt.Errorf("INR draft: %w", err)
	}
	return p, readiness.VerifySources(p, rules, markdown)
}
