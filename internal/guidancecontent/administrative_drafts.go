package guidancecontent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

// These drafts use a closed, hash-pinned inventory, not an open-ended rule-to-
// checklist generator. Every actionable requirement needs an authored recipe.
// Enumerating the resolver preserves date, audience, class and transition variants.
type administrativeRecipe struct {
	purpose, owner, prerequisite, setup, operate, records, review, scope, timing, insufficient string
}

type administrativeDomain struct {
	count                         int
	inventoryHash, page, pageHash string
}

var administrativeDomains = map[string]administrativeDomain{
	"AFC": {49, "07aebda4933fa5146517bb8ef0b4a5662c009e541e29a61052495500a044ca2b", "addressing-fedramp-communication", "787ef59caee3fdc1fd7be352adb4b1c0270424fb31cb52eb6e0a2d8b25b69767"},
	"AGU": {20, "8b23393429e23e23130636f3867621c77bf1ab5afdc3c4a2c320ff2edfaa2921", "agency-use", "c7ecd031939952595570018f9e76a67a9285f97611543b919c7b002e4883d883"},
	"MAS": {29, "f06ac010887f5d7bc3d7ce8b414ada2f2299a9485e1a8550b675b08ec7bdeef7", "minimum-assessment-scope", "49eb39d90e2ef00857836be722d5cde334996f4bcfddfca5da81b8c7de06fa9b"},
	"MKT": {14, "b22cc7e722cf2b5ff2ff7780739a7c115d47590e4710556f5e64e35a719caa93", "marketplace-listing", "0cd8c17ee1c9ed183e059c986e13046f5eca2fdc228f3d4ca35bc464c3c75cbd"},
	"REC": {16, "3a3d3450e5a052c7d3656c0783084bbe92517c36c4e880b2b3ec2fbea149c807", "fedramp-recognition", "5da3b165e7585b86278edf4ef1d93fd2f73dd7a63d07b5f7233e427440bc4f03"},
	"SCG": {36, "6d29341f81e12449ad0c85035fff67ce9945fdda0069f1643b791d269948023c", "secure-configuration-guide", "480ddf651cd45c1f303a53b5e36a8fbe1c1ce6badce4f75f12d296562123cfc6"},
	"SDR": {34, "c0028948d8542fd211eb8e4a05fc9381dba529a1318dd533ccb4e1f3c7f12c92", "security-decision-record", "675e725416b8daa04cedf7f81612c37599269c47804d7d50273fe9d5fe7029a5"},
}

func administrativeDraft(domain string, rules, markdown fs.FS, recipes map[string]administrativeRecipe) (readiness.Package, error) {
	p := readiness.Package{}
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return p, err
	}
	if storage.Hash(raw) != "fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4" {
		return p, fmt.Errorf("%s authored rules need revalidation", domain)
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
	d := administrativeDomains[domain]
	var variants []catalog.GuidanceVariant
	var ids []string
	for _, v := range inventory.Variants {
		if v.Document == domain {
			variants = append(variants, v)
			ids = append(ids, v.ID)
		}
	}
	if len(ids) != d.count || storage.Hash([]byte(strings.Join(ids, "\n"))) != d.inventoryHash {
		return p, fmt.Errorf("%s exact variant inventory changed", domain)
	}
	p = readiness.Package{Format: readiness.Format, CatalogID: snapshot.ID, Revision: 1, Sources: map[string]readiness.Source{
		"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit},
		"site":  {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"},
	}}
	// RawMessage retains the exact published JSON, including notes, notifications,
	// artifacts, conditions and class-specific details omitted from Statement.
	var root struct {
		FRR map[string]struct {
			Info json.RawMessage
			Data map[string]map[string]map[string]json.RawMessage
		}
	}
	if err = json.Unmarshal(raw, &root); err != nil {
		return p, err
	}
	doc := root.FRR[domain]
	add := func(id, source, path, anchor, excerpt string) error {
		rootFS := markdown
		if source == "rules" {
			rootFS = rules
		}
		c, e := citation(rootFS, id, source, path, anchor, excerpt)
		if e != nil {
			return e
		}
		p.Citations = append(p.Citations, c)
		return nil
	}
	if err = add(domain+"-metadata", "rules", "rules.json", "/FRR/"+domain+"/info", string(doc.Info)); err != nil {
		return p, err
	}
	for _, source := range []struct{ id, path, hash string }{
		{domain + "-page", "reference/" + d.page + ".md", d.pageHash},
		{domain + "-definitions", "definitions.md", "c20cc9acb9388f7c19465b0a0677bfd5486f0c2a4cb1c0c63e25d95fe072bd64"},
	} {
		content, e := fs.ReadFile(markdown, source.path)
		if e != nil {
			return p, e
		}
		if storage.Hash(content) != source.hash {
			return p, fmt.Errorf("%s authored narrative needs revalidation", source.path)
		}
		anchor := d.page
		if source.path == "definitions.md" {
			anchor = "fedramp-definitions"
		} else if domain == "AGU" {
			anchor = "agency-use-of-fedramp-certified-cloud-services"
		} else if domain == "REC" {
			anchor = "fedramp-recognition-of-independent-assessment-services"
		}
		if err = add(source.id, "site", source.path, anchor, string(content)); err != nil {
			return p, err
		}
	}
	if err = administrativeNarrative(domain, markdown, &p); err != nil {
		return p, err
	}
	seen := map[string]bool{}
	// Cross-domain citations support only the explicitly referenced dependency or
	// Class A inclusion; no playbooks for those other domains are authored here.
	addRelated := func(id string) error {
		ref := id + "-related-source"
		if seen[ref] {
			return nil
		}
		domain := strings.Split(id, "-")[0]
		for track, subsets := range root.FRR[domain].Data {
			for subset, rules := range subsets {
				if raw, ok := rules[id]; ok {
					if err := add(ref, "rules", "rules.json", "/FRR/"+domain+"/data/"+track+"/"+subset+"/"+id, string(raw)); err != nil {
						return err
					}
					seen[ref] = true
					return nil
				}
			}
		}
		return fmt.Errorf("missing related source %s", id)
	}
	for _, v := range variants {
		ref := v.RequirementID + "-source"
		var rule json.RawMessage
		var pointer string
		for track, subsets := range doc.Data {
			for subset, rules := range subsets {
				if value, ok := rules[v.RequirementID]; ok {
					rule = value
					pointer = "/FRR/" + domain + "/data/" + track + "/" + subset + "/" + v.RequirementID
				}
			}
		}
		if len(rule) == 0 {
			return p, fmt.Errorf("missing authored rule %s", v.RequirementID)
		}
		if !seen[ref] {
			if err = add(ref, "rules", "rules.json", pointer, string(rule)); err != nil {
				return p, err
			}
			seen[ref] = true
		}
		refs := []string{ref, domain + "-metadata", domain + "-page", domain + "-definitions"}
		if domain == "AGU" || domain == "REC" || domain == "MKT" || domain == "MAS" {
			refs = append(refs, domain+"-narrative")
		}
		if v.IncludedBy != "" {
			if err = addRelated(v.IncludedBy); err != nil {
				return p, err
			}
			refs = append(refs, v.IncludedBy+"-related-source")
		}
		if v.RequirementID == "MKT-CSO-MLR" {
			if err = addRelated("CDS-CSO-PUB"); err != nil {
				return p, err
			}
			refs = append(refs, "CDS-CSO-PUB-related-source")
		}
		recommend := func(text string) readiness.Statement {
			return readiness.Statement{Text: text, Basis: readiness.Recommendation, Citations: refs, Rationale: "Implementation and evidence interpretation for this exact published rule and inventory variant; not an additional FedRAMP obligation."}
		}
		official := func(text string) readiness.Statement {
			return readiness.Statement{Text: text, Basis: readiness.Official, Citations: refs}
		}
		var source struct {
			Force   string
			ByClass map[string]struct {
				Statement string
				Following []string `json:"following_information"`
			} `json:"varies_by_class"`
			Following    []string `json:"following_information"`
			Notes        []string
			Note         string
			Corrective   []string `json:"corrective_actions"`
			Artifacts    map[string][]string
			Notification []struct{ Party, Method, Target, Name string }
		}
		if err = json.Unmarshal(rule, &source); err != nil {
			return p, err
		}
		// Class-specific list items live below varies_by_class in the raw rule.
		// Preserve the resolver-selected topics in applicability-only explanations
		// as well as actionable assertions (notably B/C metric history).
		if following, ok := v.Details["following_information"].([]any); ok {
			source.Following = nil
			for _, item := range following {
				if text, ok := item.(string); ok {
					source.Following = append(source.Following, text)
				}
			}
		}
		context := administrativeApplicability(domain, v, string(doc.Info))
		if v.Force == "" && source.Force != "" {
			context = strings.Replace(context, "Source force: .", "Source force: "+source.Force+" (published rule; resolver stopped before force selection).", 1)
		}
		if v.Force == "" && len(source.ByClass) > 0 {
			context = strings.Replace(context, "Source force: .", "Source force: class-dependent (resolver stopped before class force selection).", 1)
		}
		b := readiness.Playbook{Kind: "actionable", ID: strings.ToLower(v.RequirementID) + "-" + v.ID, Revision: 1, RequirementID: v.RequirementID, VariantIDs: []string{v.ID}, OfficialStatement: v.Statement, Applicability: recommend(context)}
		if v.Applicability == "not_applicable" || v.Applicability == "unresolved" {
			b.Kind = "excluded"
			if v.Applicability == "unresolved" {
				b.Kind = "needs_decision"
				b.DecisionOwnerRole = administrativeDecisionOwner(domain)
				b.DecisionQuestion = administrativeQuestion(domain, v)
			}
			b.Purpose = recommend("Preserve the source's " + v.Name + " decision for its named actor: " + v.Statement + " " + strings.Join(source.Following, " ") + " " + source.Note + " " + strings.Join(source.Notes, " ") + " " + strings.Join(source.Corrective, " ") + " This profile explanation is not evidence that the named actor has fulfilled that duty.")
			if v.Statement == "" {
				for _, class := range []string{"a", "b", "c", "d"} {
					if details, ok := source.ByClass[class]; ok {
						b.Purpose.Text += " Published Class " + strings.ToUpper(class) + " context (not a selected obligation): " + details.Statement + " " + strings.Join(details.Following, " ")
					}
				}
			}
			p.Playbooks = append(p.Playbooks, b)
			continue
		}
		r, ok := recipes[v.RequirementID]
		if !ok {
			return p, fmt.Errorf("no substantive recipe for %s", v.RequirementID)
		}
		b.Purpose = recommend(r.purpose)
		capID := strings.ToLower(v.RequirementID) + "-inputs"
		step := func(id, action, completion string) readiness.Step {
			return readiness.Step{ID: id, OwnerRole: r.owner, Trigger: "one_time", Action: recommend(action), CompletionCriteria: []readiness.Statement{recommend(completion)}}
		}
		if !seen[capID] {
			seen[capID] = true
			p.Capabilities = append(p.Capabilities, readiness.Capability{ID: capID, Name: v.Name + " source inputs", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("identify-inputs", r.prerequisite, "Identify the actual owner, source versions and coverage; unavailable inputs remain an explicit evidence gap.")}}})
		}
		b.Prerequisites = []readiness.Dependency{{CapabilityID: capID, Reason: recommend(r.prerequisite)}}
		b.Setup = readiness.Steps{Items: []readiness.Step{step("prepare", r.setup, r.review)}}
		timing := recommend(r.timing)
		operation := r.operate
		if v.Applicability == "optional" && v.Force != "MAY" {
			operation = "If adopting this rule during the published transition, " + operation + " Record the adoption decision; transition optionality does not rewrite the source force."
		}
		if v.Force == "SHOULD" {
			operation += " If declining the recommendation, document the reasons and carefully weighed implications in security documentation."
		}
		if v.Force == "MAY" {
			operation += " Record whether this optional capability is used; non-adoption is not an implementation failure."
		}
		b.Operate = readiness.Steps{Items: []readiness.Step{step("perform", operation, r.records)}}
		b.Operate.Items[0].Trigger = "event"
		b.Operate.Items[0].Timing = &timing
		collect := "Retain " + r.records
		for _, n := range source.Notification {
			collect += " Preserve the submission and receipt for " + n.Name + " to " + n.Party + " via " + n.Method + ": " + n.Target + "."
		}
		for _, a := range source.Artifacts["all"] {
			collect += " Source artifact: " + a
		}
		b.Collect = readiness.Steps{Items: []readiness.Step{step("retain-evidence", collect, "The retained versions establish "+r.scope+" Record gaps; successful collection is not a sufficiency judgment.")}}
		b.ReviewSteps = readiness.Steps{Items: []readiness.Step{step("evaluate", r.review, "Record an attributable sufficiency judgment against each assertion with cited artifact revisions. Implementation remains unknown where evidence is missing; do not infer success from a plan or a generated draft.")}}
		clauses := []string{v.Statement}
		if following, ok := v.Details["following_information"].([]any); ok {
			for _, line := range following {
				if s, ok := line.(string); ok {
					clauses = append(clauses, s)
				}
			}
		}
		for index, clause := range clauses {
			ac := readiness.Acceptance{Fact: clause, Scope: recommend(r.scope), TimePeriod: recommend(r.timing), Provenance: recommend("Retain source location, immutable revision or export, producer, actual capture/event time, responsible " + r.owner + ", and the reviewer's rationale connecting the records to this clause: " + clause), Insufficient: []string{r.insufficient, "Missing scope, event dates or attributable supporting evidence; a declaration or collection success alone."}}
			ac.Alternatives = []readiness.EvidenceRoute{
				{ID: "native-records", Artifacts: []readiness.Statement{recommend(r.records), recommend("Native authoritative exports or records plus a clause-to-record mapping and dated assessment conclusions for: " + clause)}, Proves: "The stated fact within the explicitly mapped scope and observed period.", Limitations: "An export does not prove the review conclusion or coverage beyond its documented population; every item in this route is needed."},
				{ID: "retained-observation", Artifacts: []readiness.Statement{recommend(r.records), recommend("An attributable witnessed inspection report with captured underlying records, source versions and clause-by-clause findings for: " + clause)}, Proves: "The same fact through documented direct observation with supporting records.", Limitations: "An interview alone is insufficient. This alternative changes the collection method, not any mandated JSON format, independent assessment, submission or source artifact."},
			}
			if v.Force == "SHOULD" || v.Force == "MAY" || strings.HasPrefix(clause, "Recommended:") {
				ac.Alternatives = append(ac.Alternatives, readiness.EvidenceRoute{ID: "documented-decision", Artifacts: []readiness.Statement{recommend("An attributable security-documentation decision on this recommendation or option, with circumstances, rationale and fully weighed implications for " + v.Name + ".")}, Proves: "The disposition of the recommendation or optional action, not implementation of the capability.", Limitations: "Does not waive a MUST embedded in the rule, establish working functionality, or replace missing evidence with an invented decision."})
			}
			b.Assertions = append(b.Assertions, readiness.Assertion{ID: fmt.Sprintf("clause-%d", index+1), Clauses: []string{v.Statement}, Obligation: official(clause), Acceptance: ac})
		}
		p.Playbooks = append(p.Playbooks, b)
	}
	if err = readiness.ValidateDraft(p); err != nil {
		return p, err
	}
	if err = readiness.ValidateInventory(p, inventory); err != nil {
		return p, err
	}
	return p, readiness.VerifySources(p, rules, markdown)
}

func administrativeDecisionOwner(domain string) string {
	if domain == "AGU" {
		return "Agency authorizing official and FedRAMP policy liaison"
	}
	return "Provider certification owner with independent assessor and FedRAMP"
}

func administrativeQuestion(domain string, v catalog.GuidanceVariant) string {
	if domain == "AGU" {
		return "How should the agency apply " + v.RequirementID + " under its governing authority while the pinned AGU ruleset is placeholder? Obtain an agency/FedRAMP determination rather than converting it into a provider duty."
	}
	if strings.Contains(v.Reason, "next independent assessment") {
		return "Has the first independent assessment after the published grace boundary started for this ongoing certification? Supply the actual start date and applicable transition decision for " + v.RequirementID + "."
	}
	return "Which source condition applies to " + v.RequirementID + " for this profile, and what FedRAMP determination resolves it? For Class D metrics, obtain the 20x Phase 4 Pilot specifics instead of inventing a threshold."
}

func administrativeApplicability(domain string, v catalog.GuidanceVariant, metadata string) string {
	text := v.RequirementID + ": " + v.Applicability + " — " + v.Reason + ". Source force: " + v.Force + ". "
	if v.IncludedBy != "" {
		text += "Explicit inclusion: " + v.IncludedBy + ". "
	}
	switch v.Reason {
	case "Different affected audience":
		text += "The source names FedRAMP, an agency, assessor or advisor rather than the provider selected by this inventory. This is an actor boundary, not a waiver for that named actor. "
	case "Different subset type or path":
		text += "The published subset's certification types or paths do not match this supported 20x provider profile; empty source lists are preserved as resolver exclusions, not interpreted as universal applicability. "
	case "Different certification type":
		text += "This is a Rev5 control record, not a 20x KSI record. "
	case "Class excluded by subset":
		text += "The selected class is outside this subset's enumerated classes unless an explicit Class A inclusion is recorded. "
	case "Requirement not yet effective":
		text += "The saved assessment date precedes the applicable published effective/adoption boundary. This is a date-specific exclusion, not permanent exemption. "
	case "Within published transition grace period", "Optional adoption before required effective date":
		text += "The saved date/context permits early adoption before the transition boundary; implementation steps apply when adopting and the published MUST, SHOULD or MAY force remains unchanged. "
	case "Source ruleset is not stable":
		text += "AGU is marked placeholder in the structured source even though the narrative prints requirements and effective dates. Preserve the agency's legal and policy responsibilities for resolution rather than asserting provider compliance or erasing them. "
	case "Transition grace depends on next independent assessment":
		text += "The ongoing-certification transition depends on whether the first independent assessment after the published boundary has started; missing assessment status stays unresolved. "
	case "Source contains a condition requiring applicability review":
		text += "The rule contains a condition the resolver cannot settle. A profile match alone cannot establish the condition or a Class D pilot criterion. "
	}
	// Including exact metadata makes every date, subset and audience visible without
	// paraphrasing grace dates into an invented deadline.
	return text + "Exact source applicability and timing metadata: " + metadata
}

func administrativeNarrative(domain string, markdown fs.FS, p *readiness.Package) error {
	var path, hash, anchor string
	switch domain {
	case "AGU":
		path, hash = "agencies/use/index.md", "6971326e4b0d6cd746c6d41b794e37a4f9e45fb65541d810100d93d9e622ada8"
		anchor = "using-a-fedramp-certified-cloud-service"
	case "REC":
		path, hash = "assessors/index.md", "9af0233eb92c86efc90dcc4fe78c1b2e4c39a5f21948ad66b5df9c7efb7aaf50"
		anchor = "independent-assessment-services"
	case "MKT":
		path, hash = "providers/implement/marketplace/index.md", "fa563900939dec43be3363582c1a976eba4dcb53734f6181b3189cee15d368ae"
		anchor = "getting-listed"
	case "MAS":
		path, hash = "scope.md", "7675451e38aa8046528dd88496253b88db07c5908e86d5bd1c78d2a208889ca4"
		anchor = "scope-of-fedramp"
	default:
		return nil
	}
	raw, err := fs.ReadFile(markdown, path)
	if err != nil {
		return err
	}
	if storage.Hash(raw) != hash {
		return fmt.Errorf("%s narrative needs revalidation", path)
	}
	c, err := citation(markdown, domain+"-narrative", "site", path, anchor, string(raw))
	if err != nil {
		return err
	}
	p.Citations = append(p.Citations, c)
	return nil
}
