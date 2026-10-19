package guidancecontent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

// This assembler only handles the four explicitly authored recipe tables. The
// payload pin prevents new rules or changed clauses acquiring old guidance merely
// because they have a familiar ID. Inventory supplies exact resolver variants,
// including empty selected statements; it does not author implementation advice.
type communicationRecipe struct {
	cap, owner, purpose, setup, operate, records, review, scope, timing, insufficient, trigger string
}

type communicationCapability struct{ id, name, owner, setup, completion string }

const communicationRulesHash = "fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4"

var communicationPageHashes = map[string]string{
	"CCM": "72b914c96693ac2288135bc9e99a8a660ebd362b601ee4323b155845ba281f8a",
	"CMU": "7f86e0728fb00c4b5b03c8339c377c999007731b81843746a72b6738901dbc25",
	"CPO": "f4349d31d1ac022b1c98350546f90b25c253393922461443635f4579205aee47",
	"SCN": "8f95eaf6d6883dd7852d99ec423cd5dd908fcfc0c9a807bb8479da4003ccb333",
}

func communicationDraft(rules, markdown fs.FS, domain, page string, count int, recipes map[string]communicationRecipe, capabilities []communicationCapability, terms []string) (readiness.Package, error) {
	p := readiness.Package{Format: readiness.Format, Revision: 1, Sources: map[string]readiness.Source{
		"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit},
		"site":  {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"},
	}}
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return p, err
	}
	if storage.Hash(raw) != communicationRulesHash {
		return p, fmt.Errorf("%s guidance requires reauthoring for changed rules payload", domain)
	}
	schema, err := fs.ReadFile(rules, "schema.json")
	if err != nil {
		return p, err
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		return p, err
	}
	inventory, err := draftInventory(snapshot)
	if err != nil {
		return p, err
	}
	p.CatalogID = inventory.CatalogID
	pagePath := "reference/" + page + ".md"
	pageRaw, err := fs.ReadFile(markdown, pagePath)
	if err != nil {
		return p, err
	}
	if storage.Hash(pageRaw) != communicationPageHashes[domain] {
		return p, fmt.Errorf("%s Markdown source changed; revalidate guidance", domain)
	}
	definitionRaw, err := fs.ReadFile(markdown, "definitions.md")
	if err != nil {
		return p, err
	}
	if storage.Hash(definitionRaw) != "c20cc9acb9388f7c19465b0a0677bfd5486f0c2a4cb1c0c63e25d95fe072bd64" {
		return p, fmt.Errorf("definitions changed; revalidate %s guidance", domain)
	}
	add := func(root fs.FS, id, source, path, anchor, excerpt string) error {
		c, e := citation(root, id, source, path, anchor, excerpt)
		if e == nil {
			p.Citations = append(p.Citations, c)
		}
		return e
	}
	// RawMessage keeps exact source bytes (including escaped JSON) for source
	// verification rather than reserializing an object into a non-source excerpt.
	var tree map[string]map[string]json.RawMessage
	if err = json.Unmarshal(raw, &tree); err != nil {
		return p, err
	}
	var document struct {
		Info json.RawMessage                                  `json:"info"`
		Data map[string]map[string]map[string]json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(tree["FRR"][domain], &document); err != nil {
		return p, err
	}
	meta := strings.ToLower(domain) + "-applicability"
	if err = add(rules, meta, "rules", "rules.json", "/FRR/"+domain+"/info", string(document.Info)); err != nil {
		return p, err
	}
	pageMeta := strings.ToLower(domain) + "-page-applicability"
	intro := string(pageRaw)
	if end := strings.Index(intro, "\n## "); end >= 0 {
		intro = intro[:end]
	}
	if err = add(markdown, pageMeta, "site", pagePath, page, intro); err != nil {
		return p, err
	}
	terms = append(append([]string{}, terms...), "MUST", "MUST NOT", "SHOULD", "SHOULD NOT", "MAY", "Certification Package", "Certification Data")
	definitionRefs := []string{}
	seenTerms := map[string]bool{}
	for _, term := range terms {
		if seenTerms[term] {
			continue
		}
		seenTerms[term] = true
		excerpt, e := communicationSection(string(definitionRaw), "## "+term+"\n", "\n## ")
		if e != nil {
			return p, e
		}
		id := strings.ToLower(domain) + "-definition-" + communicationSlug(term)
		if err = add(markdown, id, "site", "definitions.md", communicationSlug(term), excerpt); err != nil {
			return p, err
		}
		definitionRefs = append(definitionRefs, id)
	}
	// The narrative explains the outcomes/automation context; detailed duties and
	// dates below remain bound to the rule and its exact selected statement.
	const narrative = "FedRAMP is a security framework for businesses to set security goals for themselves, continuously validate the effectiveness of the capabilities used to meet those goals, measure their performance against those goals, and ensure security and engineering teams have the resources necessary to meet those goals. It should not be treated like a traditional compliance framework."
	narrativeRef := strings.ToLower(domain) + "-provider-context"
	if err = add(markdown, narrativeRef, "site", "providers/index.md", "cloud-service-providers", narrative); err != nil {
		return p, err
	}
	refsByRequirement := map[string][]string{}
	ids := make([]string, 0, len(recipes))
	for id := range recipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var ruleRaw json.RawMessage
		var pointer string
		for typ, subsets := range document.Data {
			for subset, entries := range subsets {
				if value, ok := entries[id]; ok {
					ruleRaw, pointer = value, "/FRR/"+domain+"/data/"+typ+"/"+subset+"/"+id
				}
			}
		}
		if len(ruleRaw) == 0 {
			return p, fmt.Errorf("authored rule %s absent", id)
		}
		ruleRef, pageRef := strings.ToLower(id)+"-source", strings.ToLower(id)+"-page"
		if err = add(rules, ruleRef, "rules", "rules.json", pointer, string(ruleRaw)); err != nil {
			return p, err
		}
		text := string(pageRaw)
		index := strings.Index(text, "??? abstract \""+id+"\"")
		if index < 0 {
			return p, fmt.Errorf("Markdown missing %s", id)
		}
		start := strings.LastIndex(text[:index], "\n### ")
		if start < 0 {
			return p, fmt.Errorf("Markdown heading missing %s", id)
		}
		section := text[start+1:]
		if end := strings.Index(section, "\n##"); end >= 0 {
			section = section[:end]
		}
		heading := strings.SplitN(section, "\n", 2)[0]
		if err = add(markdown, pageRef, "site", pagePath, communicationSlug(strings.TrimPrefix(heading, "### ")), section); err != nil {
			return p, err
		}
		refsByRequirement[id] = append([]string{ruleRef, pageRef, meta, pageMeta, narrativeRef}, definitionRefs...)
	}
	recommend := func(text, rationale string, refs []string) readiness.Statement {
		return readiness.Statement{Text: text, Basis: readiness.Recommendation, Rationale: rationale, Citations: refs}
	}
	step := func(id, owner, action, completion string, refs []string) readiness.Step {
		return readiness.Step{ID: id, OwnerRole: owner, Trigger: "one_time", Action: recommend(action, "Implementation advice for the cited outcome; the selected official statement controls actor, force, conditions and timing.", refs), CompletionCriteria: []readiness.Statement{recommend(completion, "These attributable records distinguish the actual activity from a planned process or unsupported declaration.", refs)}}
	}
	for _, c := range capabilities {
		refs := []string{meta, pageMeta}
		for _, id := range ids {
			if recipes[id].cap == c.id {
				refs = append(refs, refsByRequirement[id][0], refsByRequirement[id][1])
			}
		}
		refs = append(refs, definitionRefs...)
		p.Capabilities = append(p.Capabilities, readiness.Capability{ID: c.id, Name: c.name, ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("establish-"+c.id, c.owner, c.setup, c.completion, refs)}}})
	}
	for _, v := range inventory.Variants {
		if v.Document != domain {
			continue
		}
		r, ok := recipes[v.RequirementID]
		if !ok {
			return p, fmt.Errorf("no authored recipe for %s", v.RequirementID)
		}
		if v.RequirementID == "CCM-QTR-MTG" {
			r.records += " Preserve the selected JSON artifact wording: B/C/D name the Ongoing Certification Report cycle rather than the Quarterly Review cycle. Retain the OCR ordinal recurrence alongside the meeting recurrence (and, for B, the source's alternative explanation if OCRs are not delivered), and flag this source mismatch for clarification. That artifact wording does not waive the separate OCR supply requirement."
		}
		refs := refsByRequirement[v.RequirementID]
		kind := "actionable"
		if v.Applicability == "not_applicable" {
			kind = "excluded"
		} else if v.Applicability == "unresolved" {
			kind = "needs_decision"
		}
		b := readiness.Playbook{ID: strings.ToLower(v.RequirementID) + "-" + v.ID, Kind: kind, Revision: 1, RequirementID: v.RequirementID, VariantIDs: []string{v.ID}, OfficialStatement: v.Statement,
			Purpose:       recommend(r.purpose, "This explains the specific rule's intended outcome without changing the selected actor or normative force.", refs),
			Applicability: recommend(communicationApplicability(v), "This explanation preserves the pinned inventory outcome and its source metadata; optional adoption is distinct from the rule's normative force.", refs),
		}
		if kind != "actionable" {
			if kind == "needs_decision" {
				b.DecisionOwnerRole = "Service certification owner"
				b.DecisionQuestion = communicationQuestion(v)
			}
			p.Playbooks = append(p.Playbooks, b)
			continue
		}
		if r.cap == "" || r.records == "" || r.setup == "" || r.review == "" || v.Statement == "" {
			return p, fmt.Errorf("incomplete actionable authoring for %s", v.RequirementID)
		}
		b.Prerequisites = []readiness.Dependency{{CapabilityID: r.cap, Reason: recommend("Use "+r.cap+" to identify the actual inputs, audience or event history for this activity: "+r.setup, "This is an inferred service-scoped prerequisite for evaluating the outcome, not a separate required product, document format or new official duty.", refs)}}
		b.Setup.Items = []readiness.Step{step("prepare", r.owner, r.setup, "Retain the service-specific input mapping and decisions: "+r.records, refs)}
		operation := step("perform", r.owner, r.operate, r.records, refs)
		operation.Trigger = r.trigger
		// Exact selected text carries every numeric deadline, actor and qualifier.
		operation.Timing = &readiness.Statement{Text: v.Statement, Basis: readiness.Official, Citations: refs}
		b.Operate.Items = []readiness.Step{operation}
		b.Collect.Items = []readiness.Step{step("retain-records", r.owner, "Collect exact retained revisions of: "+r.records, "Identify source system/document, responsible actor, event and publication times, service scope, retrieval time and any missing or inaccessible records. Preserve samples and no-event periods as such.", refs)}
		b.ReviewSteps.Items = []readiness.Step{step("assess-records", "Service evidence reviewer with "+r.owner, r.review, "Record a conclusion and exact artifact citations for each assertion, including coverage gaps, conditional nonapplicability and evidence limitations. Collection success is not a sufficiency assessment.", refs)}
		forceAdvice := ""
		if v.Force == "SHOULD" || v.Force == "SHOULD NOT" {
			forceAdvice = " For SHOULD/SHOULD NOT, retain the security-documentation decision explaining the treatment of the rule and carefully weighed implications, as required by the pinned definitions."
		} else if v.Force == "MAY" {
			forceAdvice = " Preserve the election or non-election of the permitted activity; permission does not mandate adoption. Under the MAY definition, parties SHOULD explain how they handle the rule in security documentation. Any MUST clauses within the selected statement remain applicable when their conditions hold."
		} else if v.Force == "MUST" || v.Force == "MUST NOT" {
			forceAdvice = " Address the requirement or prohibition in security documentation as well as meeting it, preserving the MUST/MUST NOT definition."
		}
		b.Operate.Items[0].CompletionCriteria[0].Text += forceAdvice
		acceptance := func(fact, records string) readiness.Acceptance {
			return readiness.Acceptance{Fact: fact,
				Scope:      recommend(r.scope, "This identifies the population and service boundary against which coverage is evaluated.", refs),
				TimePeriod: recommend(r.timing, "The rule's actual trigger or cadence governs; evidence dates must support that comparison without adding numeric deadlines.", refs),
				Provenance: recommend("For "+v.Name+", retain exact source/revision identifiers, the responsible "+r.owner+", actual activity and publication/delivery dates, extraction time and attributable review conclusions. "+records, "Provenance connects the claimed outcome to the actual service event or artifact rather than a generic template.", refs),
				Alternatives: []readiness.EvidenceRoute{
					{ID: "system-and-supporting-records", Artifacts: []readiness.Statement{recommend(records+forceAdvice, "The combination is tailored to this rule's outcome; no specific platform is required.", refs), recommend("Use authorized source-system exports with native identifiers, timestamps and version history, plus the scope mapping and attributable review/decision records described above.", "Exports support recorded events; human interpretation and content comparison still need their own evidence.", refs)}, Proves: "The identified operating or publication facts, conditional decisions and content for " + v.Name + " when reconciled to the stated scope and period.", Limitations: r.insufficient + " An export does not prove coverage beyond its source population or establish that unobserved events did not occur."},
					{ID: "retained-documentary-records", Artifacts: []readiness.Statement{recommend(records+forceAdvice, "These same substantive records can be retained without a vendor integration.", refs), recommend("Use dated signed or attributable records, exact published artifacts, correspondence or observation captures, and a reconciled index linking them to the service's underlying event/input history.", "Documentary evidence must establish the same content, timing and audience facts as system evidence.", refs)}, Proves: "The same " + v.Name + " facts through traceable retained documents and supporting observations.", Limitations: "A retrospective account, signature or screenshot without the underlying event/content provenance cannot establish completeness. " + r.insufficient},
				}, Insufficient: []string{r.insufficient, "A planned process, blank template, unsupported declaration or successful collection without assertion-specific sufficiency review."},
			}
		}
		clauses := []string{v.Statement}
		if v.RequirementID == "SCN-CSO-EMG" {
			clauses = strings.SplitAfter(v.Statement, ". ")
		}
		for n, clause := range clauses {
			b.Assertions = append(b.Assertions, readiness.Assertion{ID: fmt.Sprintf("obligation-%d", n+1), Clauses: []string{clause}, Obligation: readiness.Statement{Text: clause, Basis: readiness.Official, Citations: refs}, Acceptance: acceptance(clause, r.records)})
		}
		for n, check := range communicationClauseChecks[v.RequirementID] {
			if !strings.Contains(v.Statement, check.clause) {
				return p, fmt.Errorf("authored clause absent from %s: %s", v.RequirementID, check.clause)
			}
			ac := acceptance(check.fact, check.records)
			ac.Insufficient = append([]string{check.insufficient}, ac.Insufficient...)
			b.Assertions = append(b.Assertions, readiness.Assertion{ID: fmt.Sprintf("focused-clause-%d", n+1), Clauses: []string{check.clause}, Obligation: readiness.Statement{Text: check.clause, Basis: readiness.Official, Citations: refs}, Acceptance: ac})
		}
		switch v.RequirementID {
		case "SCN-ADP-NTF", "SCN-TRF-NIP", "SCN-TRF-NFP", "SCN-TRF-NAF", "SCN-TRF-NAV":
			b.Assertions = append(b.Assertions, readiness.Assertion{ID: "certification-data-notification", Clauses: []string{v.Statement}, Obligation: readiness.Statement{Text: "The rule's notification metadata specifies All Necessary Parties via an update to FedRAMP Certification Data.", Basis: readiness.Official, Citations: refs}, Acceptance: acceptance("The notice is reflected in FedRAMP Certification Data as well as notified to all necessary parties.", "Exact Certification Data update revision, publication timestamp, notice linkage and recipient delivery records. A private draft or an email without the corresponding Certification Data update does not establish this fact.")})
		}
		// following_information is normative content outside the catalog's selected
		// statement string. Keep each item separately assessable, including its own
		// conditional language, rather than treating the introductory colon as all
		// the rule requires. Clauses refer to that exact introductory statement.
		if items, ok := v.Details["following_information"].([]any); ok {
			for n, value := range items {
				item, ok := value.(string)
				if !ok {
					return p, fmt.Errorf("unexpected following_information in %s", v.RequirementID)
				}
				b.Assertions = append(b.Assertions, readiness.Assertion{ID: fmt.Sprintf("listed-content-%d", n+1), Clauses: []string{v.Statement}, Obligation: readiness.Statement{Text: "The rule's following_information includes: " + item, Basis: readiness.Official, Citations: refs}, Acceptance: acceptance("The rule's treatment of this exact listed item is supported: "+item, "Identify the exact field, section or evaluation record addressing '"+item+"', its underlying source revision and any stated applicability condition. For referenced rules in the overview, retain applicability and input mapping rather than requiring nonapplicable information. "+r.records)})
			}
		}
		p.Playbooks = append(p.Playbooks, b)
	}
	if len(p.Playbooks) != count {
		return p, fmt.Errorf("%s inventory changed: got %d, authored %d", domain, len(p.Playbooks), count)
	}
	if err = readiness.ValidateDraft(p); err != nil {
		return p, fmt.Errorf("%s: %w", domain, err)
	}
	if err = readiness.ValidateInventory(p, inventory); err != nil {
		return p, err
	}
	return p, readiness.VerifySources(p, rules, markdown)
}

func communicationSection(text, heading, delimiter string) (string, error) {
	start := strings.Index(text, heading)
	if start < 0 {
		return "", fmt.Errorf("missing source section %q", heading)
	}
	section := text[start:]
	if end := strings.Index(section[len(heading):], delimiter); end >= 0 {
		section = section[:len(heading)+end]
	}
	return section, nil
}

func communicationSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("(", "", ")", "", "/", "-", " ", "-").Replace(s)
	return s
}

func communicationApplicability(v catalog.GuidanceVariant) string {
	base := "The exact selected inventory outcome is " + v.Applicability + ": " + v.Reason + ". "
	switch v.Reason {
	case "Different affected audience":
		return base + "The rule addresses agencies or FedRAMP rather than the provider audience of the inventoried profiles. Retain that actor's rule as context; this exclusion creates no provider implementation duty."
	case "Class excluded by subset":
		return base + "The selected certification class is outside the source subset's class list. This does not exempt another class or assert that the service has met the substantive rule."
	case "Different certification type", "Different subset type or path":
		return base + "The selected type/path does not match the source rule or subset. In particular, the current inventory enumerates 20x profiles; Rev5 text is not substituted for 20x maintenance or applicability."
	case "Requirement not yet effective":
		return base + "The saved profile date precedes applicable adoption/effectiveness. This date-bound exclusion is not a permanent exemption; reassess when the profile date or context changes."
	case "Optional adoption before required effective date", "Within published transition grace period":
		return base + "Adoption is optional in this selected transition state. The actionable plan applies if adopted and preserves the selected statement's force; it does not make early adoption compulsory. Initial/ongoing context and published transition dates remain separate from operating deadlines."
	case "Transition grace depends on next independent assessment":
		return base + "The next-independent-assessment-start fact is unresolved for the transition. Determine that fact for the saved profile date and initial/ongoing context before imposing a checklist; no assessment start is inferred."
	case "Class variant conflicts with subset applicability":
		return base + "The source's class-specific text and subset class list conflict. Preserve both rather than resolving the conflict by choosing the stricter clause or treating Class A as automatically included."
	case "Source contains a condition requiring applicability review":
		return base + "The resolver leaves a source condition unresolved. Record whether the condition is satisfied for this service/event; retain the conditional text as context without fabricating a current duty."
	default:
		return base + "Apply only to the saved type, path, class, date and assessment context. Retain the selected statement's actor and " + v.Force + " force: a recommendation or permission is not upgraded to MUST."
	}
}

func communicationQuestion(v catalog.GuidanceVariant) string {
	switch v.Reason {
	case "Transition grace depends on next independent assessment":
		return "Has the first independent assessment after the source transition boundary started for this service, and what evidence establishes the date and initial/ongoing context?"
	case "Class variant conflicts with subset applicability":
		return "How should the conflict between the selected class text and the subset class list be resolved for this service? Obtain an authoritative clarification and retain the saved profile before selecting an actionable variant."
	default:
		return "Which source condition is satisfied for this service and event, and what attributable facts resolve it? Selected context: " + v.Statement
	}
}
