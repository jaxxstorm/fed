// Package guidancecontent contains explicitly authored guidance. Builders pin
// source excerpts and emit drafts; they never manufacture authenticated reviews.
package guidancecontent

import (
	"fmt"
	"io/fs"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

const trainingStatement = "The effectiveness of relevant cybersecurity education and training is persistently reviewed, including at least general training for all employees, role-specific training for employees in high risk roles, training for development and engineering staff on secure software delivery, and training for staff involved with incident response or disaster recovery."
const persistentDefinition = "Occurring in a firm, steady way that is repeated over a long period of time in spite of obstacles or difficulties. Persistent activities may vary between actors, may occur irregularly, and may include interruptions or waiting periods between cycles. These attributes of persistent activities should be intentional, understood, and documented; the status of persistent activities will always be known."

func citation(root fs.FS, id, source, path, anchor, excerpt string) (readiness.Citation, error) {
	raw, err := fs.ReadFile(root, path)
	if err != nil {
		return readiness.Citation{}, err
	}
	if !readiness.SourceExcerptMatches(raw, source, anchor, excerpt) {
		return readiness.Citation{}, fmt.Errorf("authored citation %s no longer matches the pinned source", id)
	}
	return readiness.Citation{ID: id, Source: source, Path: path, Anchor: anchor, Excerpt: excerpt, ContentHash: storage.Hash(raw), ExcerptHash: storage.Hash([]byte(excerpt))}, nil
}

func CED(rules, markdown fs.FS) (readiness.Package, error) {
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return readiness.Package{}, err
	}
	p := readiness.Package{Format: readiness.Format, CatalogID: catalog.PinnedCommit + ":" + storage.Hash(raw), Revision: 1, Sources: map[string]readiness.Source{
		"rules": {Repository: "FedRAMP/rules", Commit: catalog.PinnedCommit},
		"site":  {Repository: "FedRAMP/2026", Commit: "8404168e6c150b40e4d0e45152d18965ab27e001"},
	}}
	for _, ref := range []struct{ id, source, path, anchor, excerpt string }{
		{"training", "rules", "rules.json", "/KSI/CED/indicators/KSI-CED-RAT/statement", trainingStatement},
		{"training-page", "site", "providers/20x/key-security-indicators/cybersecurity-education.md", "reviewing-all-training", trainingStatement},
		{"persistent", "site", "definitions.md", "persistently", persistentDefinition},
		{"class-a", "site", "providers/20x/rules/fedramp-certification.md", "mandatory-fedramp-rules-for-class-a", "Key Security Indicators: [KSI-CED-RAT (Reviewing All Training)](../key-security-indicators/cybersecurity-education.md#reviewing-all-training){ data-preview }"},
		{"incident", "site", "definitions.md", "incident", `Has the meaning given in 44 USC § 3552 (b)(2) which is "an occurrence that (A) actually or imminently jeopardizes, without lawful authority, the integrity, confidentiality, or availability of information or an information system; or (B) constitutes a violation or imminent threat of violation of law, security policies, security procedures, or acceptable use policies."`},
		{"vulnerability-response", "site", "definitions.md", "vulnerability-response", "The systematic process of tracking, evaluating, mitigating, monitoring, remediating, assessing exploitation, reporting, and otherwise managing detected vulnerabilities."},
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
	step := func(id, owner, action, completion string) readiness.Step {
		return readiness.Step{ID: id, OwnerRole: owner, Trigger: "one_time", Action: recommend(action, "This makes the named training population and its effectiveness review assessable without prescribing a vendor.", "training"), CompletionCriteria: []readiness.Statement{recommend(completion, "Reviewers need attributable records to distinguish actual review from a planned program.", "training")}}
	}
	setup := readiness.Steps{Items: []readiness.Step{
		step("map-training-populations", "Training owner", "Identify all employees, high-risk roles, development/engineering staff, and incident-response/disaster-recovery staff covered by the service's training.", "A dated population map identifies role overlap and any uncovered groups."),
		step("map-learning-objectives", "Training owner and role leads", "Link each population to relevant training content and learning objectives; distinguish general awareness from secure-delivery and response/recovery training.", "Each named population maps to relevant training versions and objectives."),
		step("choose-effectiveness-methods", "Training owner and role leads", "Identify existing review methods and records, or document how effectiveness will be evaluated for each population.", "The method identifies the security skill or behavior evaluated and the responsible reviewer; a learning-system product is not required."),
	}}
	p.Capabilities = []readiness.Capability{{ID: "workforce-training-review", Name: "Workforce training effectiveness review", ScopeKind: "service", Setup: setup, Alternatives: []readiness.Alternative{}}}
	timing := official(persistentDefinition, "persistent")
	book := readiness.Playbook{Kind: "actionable", ID: "ksi-ced-rat-required", Revision: 1, RequirementID: "KSI-CED-RAT", VariantIDs: []string{"2e959240c5101e26990ad59de38559b7df6f4e39c4d4d5ee88f1a7a4058b129b", "712c24e80c7d2f030e4a78c9a59113c301bd34e8159cc226b368e12f05f57271"}, OfficialStatement: trainingStatement,
		Purpose:       recommend("Check whether security training helps the relevant people perform their duties, and retain the repeated reviews. Attendance by itself does not show effectiveness was reviewed.", "The indicator concerns persistent effectiveness review, not just delivery or attendance.", "training", "training-page"),
		Applicability: recommend("The selected catalog profile resolves this indicator as required. The Class A required-rule list explicitly includes it; the other required variant resolves from the selected class/profile metadata.", "These exact inventory IDs retain the resolver's distinction between inclusion by FRC-CLA-MFR and ordinary profile applicability.", "training", "class-a"),
		Prerequisites: []readiness.Dependency{{CapabilityID: "workforce-training-review", Reason: recommend("A relevant training/population map and review process are prerequisites to demonstrating repeated effectiveness review across the four named populations.", "This is an inferred dependency, not a separate FedRAMP-mandated tool or document format.", "training")}},
		Setup:         readiness.Steps{NoneReason: "Use the shared workforce-training-review setup. This indicator does not prescribe an additional integration or training platform."},
		Operate: readiness.Steps{Items: []readiness.Step{{ID: "review-training-effectiveness", OwnerRole: "Training owner and role leads", Trigger: "recurring", Timing: &timing, Action: official("Persistently review the effectiveness of relevant training, including all four named populations.", "training", "persistent"), CompletionCriteria: []readiness.Statement{
			recommend("Retain dated review methods, findings and conclusions for each population, with current activity status and intentional interruptions documented.", "Repeated reviews and known status demonstrate persistence; no numeric cycle or pass score is inferred.", "training", "persistent"),
		}}}},
		Collect:     readiness.Steps{Items: []readiness.Step{step("collect-training-review-records", "Training owner", "Collect the current population-to-training map and dated effectiveness reviews for general employees, high-risk roles, engineers, and response/recovery staff.", "Exact artifact revisions cover the training objectives, review period, methods, reviewer provenance, findings and limitations for each named population.")}},
		ReviewSteps: readiness.Steps{Items: []readiness.Step{step("assess-training-review-coverage", "Service assessment owner", "Assess each population and persistence assertion separately; distinguish attendance, effectiveness-review activity and real-world security outcomes.", "The assessment cites exact revisions and explains missing populations or periods. A completed checklist cannot replace human evidence review.")}},
	}
	for _, a := range []struct{ id, clause, obligation, fact, scope, records, insufficient string }{
		{"persistent-effectiveness-review", "The effectiveness of relevant cybersecurity education and training is persistently reviewed, including at least", "Relevant cybersecurity training effectiveness is persistently reviewed.", "Review activity repeats over time and its current status is known.", "All relevant cybersecurity education and training in the service boundary.", "Dated review history, chosen cycle/status record, and intentional interruption explanations.", "A policy promising future reviews or one undated dashboard."},
		{"general-employee-training", "general training for all employees", "Effectiveness review includes general training for all employees.", "General training effectiveness review covers the employee population.", "All employees relevant to the service, reconciled to the training population rather than a cloud-resource count.", "Employee/population reconciliation, general-training objectives, evaluation results and review conclusions.", "A completion percentage without its employee denominator or effectiveness conclusions."},
		{"high-risk-role-training", "role-specific training for employees in high risk roles", "Effectiveness review includes role-specific training for employees in high-risk roles.", "Identified high-risk roles have relevant training and effectiveness review.", "The service's high-risk employee roles, with the role-identification rationale retained.", "High-risk role identification, role-specific content mapping and effectiveness-review records.", "Generic awareness assigned to everyone without role-specific effectiveness review."},
		{"secure-delivery-training", "training for development and engineering staff on secure software delivery", "Effectiveness review includes development and engineering staff training on secure software delivery.", "Secure-delivery training effectiveness is reviewed for development and engineering staff.", "Development and engineering populations supporting the service, including distinct environment responsibilities where relevant.", "Engineering population/content mapping, secure-delivery evaluation results and review conclusions.", "A secure pipeline configuration, repository protections or a course catalog without review records."},
		{"response-recovery-training", "and training for staff involved with incident response or disaster recovery", "Effectiveness review includes training for incident-response or disaster-recovery staff.", "Relevant response and recovery training effectiveness is reviewed.", "Staff involved with the service's incident response or disaster recovery; establish the actual role coverage.", "Responder/recovery role roster, training material and dated effectiveness conclusions from relevant evaluations.", "Incident or restore records that never evaluate training effectiveness."},
	} {
		criteria := readiness.Acceptance{Fact: a.fact, Scope: recommend(a.scope, "The named population must be identifiable before evidence can establish coverage.", "training"), TimePeriod: recommend("Retain the actual review dates and period, chosen cycle, current activity status and documented interruptions. No annual, quarterly or monthly deadline is supplied by this indicator.", "FRD-PER permits intentional irregular cycles; the evidence must explain persistence rather than invent a fixed deadline.", "persistent"), Provenance: recommend("Identify the training version, population, reviewer, method, findings and conclusions in attributable retained records.", "Provenance connects the effectiveness conclusion to the actual training and population evaluated.", "training"), Insufficient: []string{a.insufficient, "A declaration or successful data collection without a cited sufficiency assessment."}}
		for _, route := range []struct{ id, text, proves, limits string }{
			{"learning-system-and-reviews", "Learning-system exports plus role/population mapping and the corresponding effectiveness-review reports.", "Recorded participation or evaluation results together with the reviewers' interpretation for the named population.", "Participation alone does not demonstrate review, real-world behavior, or coverage of unlisted populations."},
			{"documented-evaluation", "Retained training material plus dated exercise, interview or practical-evaluation records, the population map and effectiveness-review conclusions.", "The same training-review facts without requiring a learning-system integration.", "A single exercise must identify the populations covered; it cannot silently establish all-population or long-term coverage."},
		} {
			criteria.Alternatives = append(criteria.Alternatives, readiness.EvidenceRoute{ID: route.id, Artifacts: []readiness.Statement{recommend(route.text, "This is an example evidence combination, not a mandated vendor or testing method.", "training"), recommend(a.records, "These records address this assertion's specific fact and population.", "training")}, Proves: route.proves, Limitations: route.limits})
		}
		refs := []string{"training"}
		if a.id == "persistent-effectiveness-review" {
			refs = append(refs, "persistent")
		}
		if a.id == "response-recovery-training" {
			refs = append(refs, "incident", "vulnerability-response")
		}
		book.Assertions = append(book.Assertions, readiness.Assertion{ID: a.id, Clauses: []string{a.clause}, Obligation: official(a.obligation, refs...), Acceptance: criteria})
	}
	p.Playbooks = []readiness.Playbook{book, {Kind: "needs_decision", ID: "ksi-ced-rat-path-decision", Revision: 1, RequirementID: "KSI-CED-RAT", VariantIDs: []string{"0d0ce7e029d4c66a490ff6192725e74b04c48c32885e3f6c0995a82de8a3c231"}, OfficialStatement: trainingStatement, Purpose: book.Purpose, Applicability: recommend("The saved path does not resolve this KSI's applicability. Retain its official context without imposing an implementation checklist.", "The catalog resolver explicitly leaves this variant unresolved; an actionable variant cannot stand in for that decision.", "training", "class-a"), DecisionOwnerRole: "Service assessment owner", DecisionQuestion: "Which certification path and its applicable KSI rule set are being assessed? The saved path does not resolve this indicator's applicability."}}
	if err = readiness.ValidateDraft(p); err != nil {
		return p, err
	}
	return p, readiness.VerifySources(p, rules, markdown)
}
