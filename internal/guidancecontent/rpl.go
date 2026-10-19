package guidancecontent

import (
	"io/fs"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

func RPL(rules, markdown fs.FS) (readiness.Package, error) {
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return readiness.Package{}, err
	}
	p := readiness.Package{Format: readiness.Format, CatalogID: catalog.PinnedCommit + ":" + storage.Hash(raw), Revision: 1, Sources: map[string]readiness.Source{"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit}, "site": {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"}}}
	const abo = "The alignment of machine-based information resource backups with defined recovery objectives is persistently reviewed."
	const arp = "The alignment of recovery plans with defined recovery objectives is persistently reviewed."
	const rro = "The desired Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO) are defined and persistently reviewed for alignment with the provider's business needs and capabilities."
	const trc = "The capability to recover from incidents and contingencies aligned with defined recovery objectives is persistently tested."
	for _, ref := range []struct{ id, source, path, anchor, excerpt string }{
		{"abo", "rules", "rules.json", "/KSI/RPL/indicators/KSI-RPL-ABO/statement", abo},
		{"arp", "rules", "rules.json", "/KSI/RPL/indicators/KSI-RPL-ARP/statement", arp},
		{"rro", "rules", "rules.json", "/KSI/RPL/indicators/KSI-RPL-RRO/statement", rro},
		{"trc", "rules", "rules.json", "/KSI/RPL/indicators/KSI-RPL-TRC/statement", trc},
		{"abo-page", "site", "providers/20x/key-security-indicators/recovery-planning.md", "aligning-backups-with-objectives", abo},
		{"arp-page", "site", "providers/20x/key-security-indicators/recovery-planning.md", "aligning-recovery-plan", arp},
		{"rro-page", "site", "providers/20x/key-security-indicators/recovery-planning.md", "reviewing-recovery-objectives", rro},
		{"trc-page", "site", "providers/20x/key-security-indicators/recovery-planning.md", "testing-recovery-capabilities", trc},
		{"persistent", "site", "definitions.md", "persistently", persistentDefinition},
		{"information-resource", "site", "definitions.md", "information-resource", `Has the meaning from 44 USC § 3502 (6): "information and related resources, such as personnel, equipment, funds, and information technology." This includes any aspect of the cloud service offering, both technical and managerial, including everything that makes up the business of the offering from non-machine-based information resources like organizational policies, procedures, employees, etc. to machine-based information resources like hardware, software, cloud services, code, etc.`},
		{"machine-based", "site", "definitions.md", "machine-based-information-resources", "Any information technology information resource—including systems, processes, software, hardware, services, cloud-native capabilities, and any other such capability, component, or resource—that relies primarily on mechanical or electronic devices (i.e. computers) for operation."},
		{"incident", "site", "definitions.md", "incident", `Has the meaning given in 44 USC § 3552 (b)(2) which is "an occurrence that (A) actually or imminently jeopardizes, without lawful authority, the integrity, confidentiality, or availability of information or an information system; or (B) constitutes a violation or imminent threat of violation of law, security policies, security procedures, or acceptable use policies."`},
		{"class-a", "site", "providers/20x/rules/fedramp-certification.md", "mandatory-fedramp-rules-for-class-a", "Providers seeking a Class A FedRAMP Certification [MUST](../../../definitions/#must){ data-preview } address all rules in this FedRAMP Class A Certification subset (FRC-CLA) AND the following additional FedRAMP Class A rules; the appropriate artifacts or information mapping for all rules [MUST](../../../definitions/#must){ data-preview } be supplied in the FedRAMP Certification Package."},
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
		return readiness.Step{ID: id, OwnerRole: owner, Trigger: "one_time", Action: recommend(action, "This identifies the actual recovery objective, configuration or operating record without prescribing a particular vendor or changing infrastructure.", ref), CompletionCriteria: []readiness.Statement{recommend(completion, "The evidence must identify the object and period assessed before it can support a recovery assertion.", ref)}}
	}
	p.Capabilities = []readiness.Capability{
		{ID: "recovery-objectives", Name: "Defined service recovery objectives", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-objectives", "Recovery owner and business owner", "Locate the desired Recovery Time Objectives and Recovery Point Objectives, their service/target coverage, and the business/capability rationale. If unavailable, record the gap rather than inventing targets.", "Retained objective versions identify the desired recovery targets and their applicable service functions; a declaration is not an alignment review.", "rro")}}},
		{ID: "backup-inventory", Name: "Machine-based resource and backup inventory", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-backup-coverage", "Recovery owner and resource owners", "Locate the machine-based resource inventory and backup records, including the actual controlling resource for inherited backup settings.", "The inventory maps resources and environments to backup arrangements or explicit gaps; declaring the inventory present does not establish that every resource has suitable backups.", "abo")}}},
		{ID: "recovery-plans", Name: "Identifiable recovery plan versions", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-recovery-plans", "Recovery owner", "Locate the recovery plans, current versions, service/function scope and responsible owners.", "Plan versions and their intended scope are retained; a plan's existence does not demonstrate alignment or successful recovery.", "arp")}}},
	}
	timing := official(persistentDefinition, "persistent")
	type assertionRecipe struct{ id, clause, obligation, fact, records, insufficient string }
	for _, recipe := range []struct {
		id, ref, statement, anchor, purpose, collect, review, required, unresolved, excluded string
		caps                                                                                 []string
		assertions                                                                           []assertionRecipe
	}{
		{"KSI-RPL-ABO", "abo", abo, "aligning-backups-with-objectives", "Review whether backups for machine-based resources match the defined recovery objectives. Backup retention settings alone do not demonstrate this comparison or a successful restore.", "Collect the resource-to-backup inventory, controlling-resource backup settings and operating records, objective versions, and dated alignment reviews.", "Compare the actual backup coverage and settings with the applicable objective version, including missing resources and environments. Preserve the review conclusion and its limits; do not treat a nonzero retention value as automatically sufficient.", "e05a5fc3154441c299993c34f6ae82d806b746e2423473c088a91658e67fc583", "7ef62e4485dd71fa70d838389650e89aa917d34517c94f54b3d48d3e682e1c9a", "8797c0db2877c8b64c4a1ea963c34ec1b9f1dfc4e09cb94bad4edada4f39183d", []string{"recovery-objectives", "backup-inventory"}, []assertionRecipe{
			{"backup-alignment-review", abo, "Persistently review machine-based resource backup alignment with defined recovery objectives.", "Backup arrangements are repeatedly compared with the applicable defined objectives.", "Resource/environment coverage, controlling backup configuration, relevant backup operating records, objective version, and dated comparisons with findings and conclusions.", "A retention-days value or snapshot-success indicator without objectives, scope reconciliation and a recorded comparison."},
		}},
		{"KSI-RPL-ARP", "arp", arp, "aligning-recovery-plan", "Review whether recovery plans match the defined recovery objectives. Approving a plan is not the same as evaluating its alignment, and neither proves recovery was successfully tested.", "Collect the recovery plan versions, defined objectives and dated reviews that compare the plans with those objectives.", "Assess the actual comparison of plan assumptions and recovery approach against each applicable objective, including unresolved gaps and later plan/objective changes.", "cbedbcc1fadd5eb6c42db3a0e4a49533528d67c44c6704278be89d6eeed36c75", "9c363d1fb38f99d996ab8ed36adb0151c19b552d550bacc4814b562b22da5aff", "a562759293f617b8f99c0cd0bf77d14827c1ea17b8062e3dda7da288bc07ca56", []string{"recovery-objectives", "recovery-plans"}, []assertionRecipe{
			{"plan-alignment-review", arp, "Persistently review recovery-plan alignment with defined recovery objectives.", "Recovery plan versions are repeatedly compared with the applicable defined objectives.", "Plan and objective versions, their mapped scope, review dates, comparison rationale, findings and conclusions, with current review status.", "A plan document or approval signature without a substantive objective-alignment review."},
		}},
		{"KSI-RPL-RRO", "rro", rro, "reviewing-recovery-objectives", "Define desired recovery-time and recovery-point objectives and repeatedly review their fit with business needs and capabilities. Do not replace these service-specific targets with invented FedRAMP numbers.", "Collect the defined desired RTO/RPO versions, business/capability rationale and dated reviews of their alignment.", "Assess definition and repeated alignment review separately. The objectives and review records do not by themselves demonstrate tested recovery capability; that is assessed under KSI-RPL-TRC.", "2363c56f0ccbd594794453ddaf77a751e698efb95f955b6f6cbad965f5575bdb", "94439fddf1f3ceedc4c05532bbbe8ddbbe51e5e6cb453c749985507017fff873", "bebcaa5f1c9348097d361d01ea03a192164a3814f193d325ac89f03160bd2d34", []string{"recovery-objectives"}, []assertionRecipe{
			{"defined-recovery-objectives", "The desired Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO) are defined", "Define desired Recovery Time Objectives and Recovery Point Objectives.", "The service's desired RTO and RPO are explicitly defined for their relevant scope.", "Attributable objective records identifying desired RTO/RPO, versions, service/function coverage and accountable owner.", "A backup frequency or an observed restore time presented as an undefined desired objective."},
			{"objective-alignment-review", "and persistently reviewed for alignment with the provider's business needs and capabilities", "Persistently review desired recovery objectives for alignment with business needs and capabilities.", "The defined objectives are repeatedly evaluated against the provider's business needs and capabilities.", "Objective versions, business/capability inputs, review dates, alignment rationale, findings/conclusions and known current review status.", "A target value or business-impact document without an actual alignment review and repeated activity."},
		}},
		{"KSI-RPL-TRC", "trc", trc, "testing-recovery-capabilities", "Test recovery capability against the defined objectives repeatedly. A backup setting, recovery plan or objective review alone does not demonstrate an executed recovery test.", "Collect dated recovery-test execution records, the scenarios and resources tested, actual observed recovery results, and comparisons with the applicable objective versions.", "Distinguish planned exercises from executed tests. Assess the incident/contingency scenarios and service scope covered, actual outcomes and objective comparisons; preserve failed results and untested dependencies.", "2e5c6e11ce0747f737521c43a32cc6c184a08bdd3dd881bbedad179704cc4a01", "648ac8e9567ab3a9006e9843b6f34c0a414a02b5e3056077105b8c0bfac5d0fc", "ca281377c518e3acbfa9773cfff001dbb6b7c926d5574b51e913bdda8dc7ff04", []string{"recovery-objectives"}, []assertionRecipe{
			{"tested-recovery-capability", trc, "Persistently test the capability to recover from incidents and contingencies aligned with defined recovery objectives.", "Recovery capability is actually tested repeatedly against the relevant defined objectives.", "Executed test scenarios, resource/environment scope, dates, observed results, objective versions and comparison conclusions, including failures and untested components.", "Backup configuration, a plan, an objective review, an unexecuted test script, or a tabletop discussion claimed as proof of an untested technical restore."},
		}},
	} {
		refs := []string{recipe.ref, recipe.ref + "-page", "persistent"}
		if recipe.ref == "abo" {
			refs = append(refs, "information-resource", "machine-based")
		}
		if recipe.ref == "trc" {
			refs = append(refs, "incident")
		}
		book := readiness.Playbook{Kind: "actionable", ID: recipe.ref + "-required", Revision: 1, RequirementID: recipe.id, VariantIDs: []string{recipe.required}, OfficialStatement: recipe.statement, Purpose: recommend(recipe.purpose, "The explanation preserves the distinct recovery outcome and the limits of configuration or documentary evidence.", refs...), Applicability: recommend("This exact selected profile variant resolves the recovery indicator as required; the separate Class A excluded and unresolved-path variants retain their own explanations.", "Do not reuse a required variant to override the catalog's exclusion or unresolved outcome.", recipe.ref, "class-a"), Setup: readiness.Steps{NoneReason: "Use the shared prerequisite records. No additional vendor integration or infrastructure change is prescribed by this guidance."}, Collect: readiness.Steps{Items: []readiness.Step{step("collect-"+recipe.ref, "Recovery owner", recipe.collect, recipe.review, recipe.ref)}}, ReviewSteps: readiness.Steps{Items: []readiness.Step{step("assess-"+recipe.ref, "Service assessment owner", recipe.review, "Exact retained citations and scope/period limitations are recorded for each assertion; human service evidence review remains separate.", recipe.ref)}}}
		for _, cap := range recipe.caps {
			book.Prerequisites = append(book.Prerequisites, readiness.Dependency{CapabilityID: cap, Reason: recommend("The "+cap+" records identify the objective or object needed for this recovery comparison or test.", "This is a prerequisite for evaluating the stated outcome, not a mandate to purchase or configure a particular product.", recipe.ref, "rro")})
		}
		for _, a := range recipe.assertions {
			if a.id != "defined-recovery-objectives" {
				book.Operate.Items = append(book.Operate.Items, readiness.Step{ID: a.id, OwnerRole: "Recovery owner", Trigger: "recurring", Timing: &timing, Action: official(a.obligation, refs...), CompletionCriteria: []readiness.Statement{recommend(a.records, "These records make the specific recovery activity and its actual outcome assessable.", recipe.ref)}})
			}
			acceptance := readiness.Acceptance{Fact: a.fact, Scope: recommend("Identify the actual service functions, resources, environments and dependencies addressed, and map them to the applicable recovery objective version.", "Evidence from one development resource does not automatically establish production or shared-dependency coverage.", recipe.ref), TimePeriod: recommend("Retain actual definition, review or test dates and versions. For persistent activity, show repeated cycles, current status and intentional interruptions without inventing a numeric deadline.", "The pinned definition permits intentional irregularity; initial activity alone is not long-term repetition.", "persistent"), Provenance: recommend("Identify the accountable owner, source system or document, objective version, reviewed/tested object, method and actual findings/results.", "These contextual facts distinguish stated intent from performed review or testing.", recipe.ref), Insufficient: []string{a.insufficient, "A declaration or successful collection without a cited sufficiency assessment."}}
			for _, route := range []struct{ id, artifact, proves, limits string }{
				{"system-and-review-records", "Authorized system exports or execution logs together with objective records, scope mapping and attributable review/test conclusions.", "Recorded configuration or execution facts together with the explicit comparison or test conclusions.", "Configuration alone does not demonstrate alignment review or successful execution; logs must identify the relevant objective and actual scope."},
				{"documented-operating-records", "Dated, attributable objective/review/test reports with their supporting configuration or execution evidence and scope mapping.", "The same defined-objective, comparison or test facts without requiring a particular provider integration.", "Narrative reports need traceable supporting facts; a blank template, planned test or unsupported claim cannot prove operation."},
			} {
				acceptance.Alternatives = append(acceptance.Alternatives, readiness.EvidenceRoute{ID: route.id, Artifacts: []readiness.Statement{recommend(route.artifact, "These are alternative evidence combinations, not mandated tooling.", recipe.ref), recommend(a.records, "This combination must establish this assertion's specific fact.", recipe.ref)}, Proves: route.proves, Limitations: route.limits})
			}
			book.Assertions = append(book.Assertions, readiness.Assertion{ID: a.id, Obligation: official(a.obligation, refs...), Clauses: []string{a.clause}, Acceptance: acceptance})
		}
		p.Playbooks = append(p.Playbooks, book)
		for _, variant := range []struct{ id, kind, suffix string }{{recipe.unresolved, "needs_decision", "path-decision"}, {recipe.excluded, "excluded", "class-a-excluded"}} {
			b := readiness.Playbook{Kind: variant.kind, ID: recipe.ref + "-" + variant.suffix, Revision: 1, RequirementID: recipe.id, VariantIDs: []string{variant.id}, OfficialStatement: recipe.statement, Purpose: book.Purpose}
			if variant.kind == "excluded" {
				b.Applicability = recommend("This indicator is not included in the selected Class A enumerated obligations. This exclusion is not a claim of recovery capability or an exemption for other profiles.", "Preserve the exact excluded inventory variant without creating an evidence checklist.", recipe.ref, "class-a")
			} else {
				b.Applicability = recommend("The saved certification path leaves this recovery indicator unresolved; no implementation checklist is imposed yet.", "The resolver's unknown path outcome must not become a fabricated required or excluded obligation.", recipe.ref, "class-a")
				b.DecisionOwnerRole = "Service assessment owner"
				b.DecisionQuestion = "Which certification path and applicable KSI rule set are being assessed? The saved path leaves this recovery indicator unresolved."
			}
			p.Playbooks = append(p.Playbooks, b)
		}
	}
	if err = readiness.ValidateDraft(p); err != nil {
		return p, err
	}
	return p, readiness.VerifySources(p, rules, markdown)
}
