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

// certificationRecipe is deliberately authored per rule. Source expansion only
// preserves selected class statements, enumerated information and timing tables;
// it does not infer implementation advice from a rule's name.
type certificationRecipe struct {
	purpose, setup, records, review, scope, timing, insufficient string
}

const certificationRulesHash = "fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4"

func certificationDraft(rules, markdown fs.FS, domain string, recipes map[string]certificationRecipe, count int) (readiness.Package, error) {
	var p readiness.Package
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return p, err
	}
	if storage.Hash(raw) != certificationRulesHash {
		return p, fmt.Errorf("%s authored rules need revalidation", domain)
	}
	schema, err := fs.ReadFile(rules, "schema.json")
	if err != nil {
		return p, err
	}
	if storage.Hash(schema) != "eb4710bee0b06d0f8f803302ac2e88b934ef2bc942d0b548738c7304f0cb418f" {
		return p, fmt.Errorf("%s rules schema needs revalidation", domain)
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		return p, err
	}
	inventory, err := draftInventory(snapshot)
	if err != nil {
		return p, err
	}
	p = readiness.Package{Format: readiness.Format, CatalogID: snapshot.ID, Revision: 1, Sources: map[string]readiness.Source{"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit}, "site": {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"}}}
	add := func(root fs.FS, id, source, path, anchor, text string) error {
		content, e := fs.ReadFile(root, path)
		if e != nil {
			return e
		}
		if !strings.Contains(string(content), text) {
			return fmt.Errorf("%s exact source excerpt mismatch", id)
		}
		p.Citations = append(p.Citations, readiness.Citation{ID: id, Source: source, Path: path, Anchor: anchor, Excerpt: text, ContentHash: storage.Hash(content), ExcerptHash: storage.Hash([]byte(text))})
		return nil
	}
	// Retain exact rule objects, including lists, notes, notification destinations
	// and class tables; a top-level statement alone omits substantive obligations.
	objects, paths := map[string]json.RawMessage{}, map[string]string{}
	var walk func(json.RawMessage, string)
	walk = func(data json.RawMessage, path string) {
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) != nil {
			return
		}
		for k, v := range m {
			if strings.HasPrefix(k, domain+"-") {
				objects[k], paths[k] = v, path+"/"+k
			} else {
				walk(v, path+"/"+k)
			}
		}
	}
	walk(raw, "")
	web := map[string]string{"FRC": "fedramp-certification", "IEC": "incident-evaluation-and-communication", "IVV": "independent-verification-and-validation"}[domain]
	defs, err := fs.ReadFile(markdown, "definitions.md")
	if err != nil {
		return p, err
	}
	if storage.Hash(defs) != "c20cc9acb9388f7c19465b0a0677bfd5486f0c2a4cb1c0c63e25d95fe072bd64" {
		return p, fmt.Errorf("%s definitions need revalidation", domain)
	}
	pageHashes := map[string]string{"FRC": "2738732080c7aff976bc81f324d20869c1af138f527efca93f38ba2e69369186", "IEC": "f3c6eaccfc3d1f9d13e2cee6f9465c278aa615c2c026a6cf624bb7a76051bcd5", "IVV": "e09becd7e6bdebc37de921661f9a968788bd41d7dbd7dfef7ad7808272cc9af5"}
	contextID := strings.ToLower(domain) + "-context"
	contextPath := "reference/" + web + ".md"
	contextPage, err := fs.ReadFile(markdown, contextPath)
	if err != nil {
		return p, err
	}
	if storage.Hash(contextPage) != pageHashes[domain] {
		return p, fmt.Errorf("%s context needs revalidation", domain)
	}
	if err = add(markdown, contextID, "site", contextPath, web, string(contextPage)); err != nil {
		return p, err
	}
	definitionIDs := []string{}
	for _, title := range []string{"MUST", "MUST NOT", "SHOULD", "MAY", "Persistently", "Promptly", "Responsibly", "Verification", "Validation", "All Necessary Parties", "All Necessary Assessors", "All Affected Parties", "Likely", "Potential Agency Impact", "Minimal Customer Effect", "Narrow Customer Effect", "Disruptive Customer Effect", "Debilitating Customer Effect"} {
		section := certificationSection(string(defs), "## "+title)
		if section == "" {
			return p, fmt.Errorf("missing definition %s", title)
		}
		id := strings.ToLower(domain + "-definition-" + strings.ReplaceAll(title, " ", "-"))
		if err = add(markdown, id, "site", "definitions.md", strings.ToLower(strings.ReplaceAll(title, " ", "-")), section); err != nil {
			return p, err
		}
		definitionIDs = append(definitionIDs, id)
	}
	seen := map[string]bool{}
	for _, v := range inventory.Variants {
		if v.Document != domain {
			continue
		}
		r, ok := recipes[v.RequirementID]
		if !ok {
			return p, fmt.Errorf("unwritten %s recipe", v.RequirementID)
		}
		ref := strings.ToLower(v.RequirementID)
		pageRef := ref + "-page"
		var base map[string]any
		if err = json.Unmarshal(objects[v.RequirementID], &base); err != nil {
			return p, err
		}
		if !seen[ref] {
			seen[ref] = true
			if err = add(rules, ref, "rules", "rules.json", paths[v.RequirementID], string(objects[v.RequirementID])); err != nil {
				return p, err
			}
			// The reference page retains every audience and certification type,
			// including explanatory-only Rev5 and assessor inventory variants.
			path := "reference/" + web + ".md"
			page, e := fs.ReadFile(markdown, path)
			if e != nil {
				return p, e
			}
			if storage.Hash(page) != pageHashes[domain] {
				return p, fmt.Errorf("%s narrative needs revalidation", domain)
			}
			section := certificationSection(string(page), "### "+v.Name)
			if !strings.Contains(section, v.RequirementID) {
				return p, fmt.Errorf("%s narrative section mismatch", v.RequirementID)
			}
			if err = add(markdown, pageRef, "site", path, strings.ToLower(strings.ReplaceAll(v.Name, " ", "-")), section); err != nil {
				return p, err
			}
		}
		refs := append([]string{ref, pageRef, contextID}, definitionIDs...)
		rec := func(text string) readiness.Statement {
			return readiness.Statement{Text: text, Basis: readiness.Recommendation, Citations: refs, Rationale: "Evidence-first assessment guidance for " + v.RequirementID + "; artifact organization and review methods are recommendations, not additional FedRAMP obligations."}
		}
		off := func(text string) readiness.Statement {
			return readiness.Statement{Text: text, Basis: readiness.Official, Citations: []string{ref, pageRef}}
		}
		owner := "Provider certification owner"
		if domain == "IEC" {
			owner = "Provider incident response coordinator"
		}
		if domain == "IVV" {
			owner = "Provider assessment owner"
		}
		switch v.RequirementID {
		case "FRC-CSX-VVK", "FRC-CSX-VVR":
			owner = "Provider automation engineering owner and certification owner"
		case "FRC-CSO-JSN":
			owner = "Provider certification data publisher"
		case "IEC-CSO-AIR":
			owner = "Provider incident response coordinator and reporting automation owner"
		case "IVV-CSO-SEI", "IVV-CSO-SEE", "IVV-CSO-STE":
			owner = "Provider assessment owner and responsible technical leads"
		}
		if strings.Contains(v.RequirementID, "-IAS-") {
			owner = "Independent assessor"
		}
		if strings.Contains(v.RequirementID, "-FRP-") {
			owner = "FedRAMP program reviewer"
		}
		app := fmt.Sprintf("Catalog outcome: %s; selected force: %s. %s", v.Applicability, v.Force, v.Reason)
		if v.IncludedBy != "" {
			app += " Included by " + v.IncludedBy + "."
		}
		app += " Preserve the saved type, path, class, date, initial/ongoing context and next-assessment transition decision. Optional adoption does not turn the quoted rule's force into an unconditional duty; MAY remains permission and SHOULD remains recommendation. Unknown event or scope facts remain unknown, not evidence of failure."
		app += " SHOULD permits justified departures only after weighing the full implications and MUST be addressed by explaining the decision in security documentation. MAY is truly optional and SHOULD be addressed with a documented decision. These definition-level documentation duties do not make the optional activity mandatory."
		b := readiness.Playbook{Kind: "actionable", ID: ref + "-" + v.ID, Revision: 1, RequirementID: v.RequirementID, VariantIDs: []string{v.ID}, OfficialStatement: v.Statement, Purpose: rec(r.purpose), Applicability: rec(app)}
		switch v.Applicability {
		case "not_applicable":
			b.Kind = "excluded"
		case "unresolved":
			b.Kind = "needs_decision"
			b.DecisionOwnerRole = owner
			b.DecisionQuestion = "Resolve the saved certification path, class and effective-date/next-assessment facts: " + v.Reason + ". Which supported profile and transition decision apply?"
			if strings.Contains(v.Reason, "condition") {
				b.DecisionQuestion = "Establish the actual event, election and exception facts in this selected statement before deciding applicability: " + v.Statement + " Preserve unknown facts and explain whether its conditional boundary is met."
			}
			if strings.Contains(v.Reason, "conflicts") {
				b.DecisionQuestion = "The class-specific statement conflicts with subset applicability metadata. Obtain an authoritative resolution of that source conflict; do not assume the presence of a Class A statement overrides the subset or infer that it is universally excluded. " + v.Reason
			}
		}
		if b.Kind != "actionable" {
			p.Playbooks = append(p.Playbooks, b)
			continue
		}
		step := func(id, action, completion string) readiness.Step {
			return readiness.Step{ID: id, OwnerRole: owner, Trigger: "one_time", Action: rec(action), CompletionCriteria: []readiness.Statement{rec(completion)}}
		}
		capID := ref + "-evidence-context"
		found := false
		for _, c := range p.Capabilities {
			found = found || c.ID == capID
		}
		if !found {
			p.Capabilities = append(p.Capabilities, readiness.Capability{ID: capID, Name: v.Name + " evidence context", ScopeKind: "service", Setup: readiness.Steps{Items: []readiness.Step{step("locate-context", r.setup, "Record the actual owner, available revisions and access limitations for "+r.scope+". Missing access or records leave the assessment unknown; do not infer missing implementation or perform remediation.")}}})
		}
		b.Prerequisites = []readiness.Dependency{{CapabilityID: capID, Reason: rec(r.setup)}}
		if v.RequirementID == "IVV-CSO-DUS" {
			b.Prerequisites[0].Condition = &readiness.Condition{Fact: "scope.representative_samples", Equals: "true", Question: "Were representative samples actually used during verification or validation? Retain unknown until population and assessment records resolve this fact."}
		}
		b.Setup = readiness.Steps{NoneReason: "Use the shared " + capID + " capability to locate existing records and responsible people."}
		timing := r.timing
		// Selected details (not another class's table) supply supplemental policy.
		if value, ok := v.Details["pain_timeframes"]; ok {
			timing += " " + certificationPAINText(value)
		}
		if domain == "IEC" && v.Details["pain_timeframes"] != nil {
			timing += " bizdays means business days, not elapsed 24-hour periods. Match the actual class and PAIN history to this table. Retain the rating/default clock start and report transmission times; unresolved clock or business-calendar facts must be recorded for review."
		}
		timeStatement := rec(timing)
		operation := step("operating-duty", v.Statement, r.records)
		operation.Action = off(v.Statement)
		operation.Trigger = "event"
		operation.Timing = &timeStatement
		if table, ok := v.Details["pain_timeframes"]; ok {
			deadline := off(certificationPAINText(table))
			operation.Timing = &deadline
		}
		b.Operate = readiness.Steps{Items: []readiness.Step{operation}}
		b.Collect = readiness.Steps{Items: []readiness.Step{step("collect-records", "Collect retained revisions of: "+r.records, "Identify the record source, author, actual dates and service boundary. Keep unsuccessful or incomplete results and distinguish actual events from simulations.")}}
		b.ReviewSteps = readiness.Steps{Items: []readiness.Step{step("review-evidence", r.review, "Record a separate supported, contradicted or unknown conclusion for each assertion with exact artifact citations, uncovered scope and timing limitations. Collection success and this draft are not authenticated evidence review.")}}
		assert := func(id, obligation, fact, records, insufficient string) {
			a := readiness.Assertion{ID: id, Clauses: []string{v.Statement}, Obligation: off(obligation), Acceptance: readiness.Acceptance{Fact: fact, Scope: rec(r.scope), TimePeriod: timeStatement, Provenance: rec("Retain " + records + " Identify the originating organization/person or system, exact revision, service/incident/assessment identifier, collection date and original event dates; preserve recipient and transmission provenance when supplied externally."), Insufficient: []string{insufficient, "An unsupported declaration, blank template or successful export without a substantive comparison to this assertion."}}}
			a.Acceptance.Alternatives = []readiness.EvidenceRoute{
				{ID: "native-records", Artifacts: []readiness.Statement{rec("Native exports with identifiers and event/version history for: " + records)}, Proves: fact, Limitations: "Only the identified population and dates are established; exports still need the stated comparison and accountable interpretation."},
				{ID: "retained-records", Artifacts: []readiness.Statement{rec("Attributable retained reports or correspondence, with underlying source excerpts and scope reconciliation for: " + records)}, Proves: fact, Limitations: "A narrative without traceable underlying facts cannot establish implementation, effectiveness, delivery or timeliness; preserve unknowns and simulation boundaries."},
			}
			b.Assertions = append(b.Assertions, a)
		}
		fact, ok := certificationOperatingFacts[v.RequirementID]
		if !ok {
			return p, fmt.Errorf("missing observable fact for %s", v.RequirementID)
		}
		assert("operating-fact", v.Statement, fact, r.records, r.insufficient)
		if v.Force == "SHOULD" || v.Force == "MAY" {
			assert("document-disposition", v.Statement, "The security documentation explains the decision about this rule; for SHOULD, any departure includes the circumstances and carefully weighed implications. For MAY, documentation is recommended and non-election is permitted.", "Security-documentation entry, rule-specific disposition and reasons, including implications of any departure.", "Treating SHOULD as permission to silently ignore a rule, or treating MAY non-election as a failure.")
			b.Assertions[len(b.Assertions)-1].Obligation = rec("Apply the bound SHOULD/MAY definitions to the selected rule's disposition and documentation.")
		}
		for i, check := range certificationChecks[v.RequirementID] {
			if !strings.Contains(v.Statement, check.clause) {
				return p, fmt.Errorf("%s authored clause changed: %s", v.RequirementID, check.clause)
			}
			assert(fmt.Sprintf("clause-%d", i+1), v.Statement, check.fact, check.records, check.insufficient)
			b.Assertions[len(b.Assertions)-1].Clauses = []string{check.clause}
		}
		for _, key := range []string{"following_information", "following_information_bullets"} {
			if items, ok := v.Details[key].([]any); ok {
				for i, item := range items {
					text, ok := item.(string)
					if !ok {
						continue
					}
					fact := "The selected rule's inclusion, availability and applicability qualifiers are evaluated for: " + text
					insufficient := "A package/report that omits this applicable item or silently treats unavailable information as known: " + text
					if v.RequirementID == "FRC-CLA-ASF" || v.RequirementID == "FRC-CLA-EAM" {
						fact = "Only if this framework is selected, establish its completed process or required material set, retaining all if-applicable qualifiers: " + text + " Other listed frameworks are alternatives, not additional required certifications or document sets."
						insufficient = "Claiming the selected framework is covered without its completion/material evidence; non-selection of this alternative is not a failure."
					}
					assert(fmt.Sprintf("%s-%d", key, i+1), v.Statement+"\n"+text, fact, r.records+" Pinpoint the specific entry and supporting facts for: "+text, insufficient)
				}
			}
		}
		// Parent metadata remains applicable even where the resolver selects only
		// a class child. Explicitly retain notification and schema obligations.
		if notifications, ok := base["notification"].([]any); ok {
			for i, value := range notifications {
				n := value.(map[string]any)
				route := fmt.Sprintf("%s via %s: %s (%s)", n["party"], n["method"], n["target"], n["name"])
				assert(fmt.Sprintf("notification-%d", i+1), v.Statement, "The applicable recipient population received the selected report/application through the specified route: "+route+". Preserve the selected rule's force and event boundary.", "Recipient roster for "+route+", actual transmitted revision, transmission/update timestamp and delivery/access provenance. Compare population coverage to individual deliveries.", "An internal draft, another recipient's delivery, or a generic sent flag without the specified route and recipient coverage.")
			}
		}
		if value, ok := base["schema"].(map[string]any); ok {
			target := fmt.Sprintf("%s (%s)", value["name"], value["url"])
			assert("schema", v.Statement, "The actual supplied structured information is matched to the rule-linked schema "+target+"; validation results and any explicit rule exception are retained separately from substantive accuracy and delivery.", "Exact supplied payload revision, linked schema identifier/revision, validation results and any explicit exception. A schema association is not evidence of passing validation.", "A schema URL or syntactically valid JSON without validation of the supplied payload against its corresponding schema.")
			b.Assertions[len(b.Assertions)-1].Obligation = rec("Check the rule's linked schema association and retain corresponding payload-validation evidence; artifact checks do not prove factual accuracy.")
		}
		p.Playbooks = append(p.Playbooks, b)
	}
	if len(p.Playbooks) != count {
		return p, fmt.Errorf("%s denominator changed: %d", domain, len(p.Playbooks))
	}
	ids := make([]string, 0, len(p.Playbooks))
	for _, b := range p.Playbooks {
		ids = append(ids, b.VariantIDs...)
	}
	sort.Strings(ids)
	digests := map[string]string{"FRC": "24e830ab6da3657464136e21a9eae8deac6e98c6f026119e80a9b2668f61a430", "IEC": "1815a8475a54c3d055eb43d6a443b2e5981044b583ce1e53a7948fe507a90d5a", "IVV": "865779cb422abbe1ba57b7ab2d8b781a5c1132c80d87838cf67ca5b171c3884c"}
	if storage.Hash([]byte(strings.Join(ids, "\n"))) != digests[domain] {
		return p, fmt.Errorf("%s exact variants need revalidation", domain)
	}
	sort.Slice(p.Capabilities, func(i, j int) bool { return p.Capabilities[i].ID < p.Capabilities[j].ID })
	if err = readiness.ValidateDraft(p); err != nil {
		return p, err
	}
	if err = readiness.ValidateInventory(p, inventory); err != nil {
		return p, err
	}
	return p, readiness.VerifySources(p, rules, markdown)
}

func certificationPAINText(value any) string {
	// Source values are decoded catalog objects. JSON round-tripping normalizes
	// nested map types without selecting or combining another class's table.
	raw, _ := json.Marshal(value)
	var table map[string]map[string]struct {
		Number      float64 `json:"timeframe_num"`
		Unit        string  `json:"timeframe_type"`
		Description string  `json:"description"`
	}
	_ = json.Unmarshal(raw, &table)
	parts := []string{"Selected class reporting timeframes:"}
	for _, rating := range []string{"1", "2", "3", "4", "5"} {
		keys := []string{}
		for k := range table[rating] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			d := table[rating][k]
			unit := d.Unit
			if unit == "bizdays" {
				unit = "business days"
			}
			if d.Number == 1 {
				unit = strings.TrimSuffix(unit, "s")
			}
			parts = append(parts, fmt.Sprintf("PAIN-%s %s: %g %s.", rating, d.Description, d.Number, unit))
		}
	}
	return strings.Join(parts, " ")
}

func certificationSection(page, heading string) string {
	start := strings.Index(page, "\n"+heading+"\n")
	if start < 0 {
		return ""
	}
	start++
	rest := page[start+len(heading):]
	end := len(rest)
	for _, marker := range []string{"\n## ", "\n### "} {
		if n := strings.Index(rest, marker); n >= 0 && n < end {
			end = n
		}
	}
	return page[start : start+len(heading)+end]
}
