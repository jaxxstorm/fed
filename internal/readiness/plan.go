package readiness

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/jaxxstorm/fed/internal/catalog"
)

type PlanQuestion struct {
	Fact          string      `json:"fact,omitempty"`
	ExpectedValue string      `json:"expected_value,omitempty"`
	Declaration   *WorkRecord `json:"declaration,omitempty"`
	RequirementID string      `json:"requirement_id"`
	CapabilityID  string      `json:"capability_id,omitempty"`
	Scope         *ScopeKey   `json:"scope,omitempty"`
	Question      string      `json:"question"`
	Reason        string      `json:"reason"`
}

type CapabilityReadiness struct {
	Evidence          []CandidateEvidence `json:"evidence"`
	ID                string              `json:"id"`
	CapabilityID      string              `json:"capability_id"`
	Name              string              `json:"name"`
	Scope             ScopeKey            `json:"scope"`
	RequirementIDs    []string            `json:"requirement_ids"`
	PrerequisiteIDs   []string            `json:"prerequisite_ids"`
	Implementation    string              `json:"implementation"`
	EvidenceState     string              `json:"evidence_state"`
	Trigger           string              `json:"trigger"`
	Review            string              `json:"review"`
	Blocked           bool                `json:"blocked"`
	NeedsRevalidation bool                `json:"needs_revalidation"`
	Steps             Steps               `json:"steps"`
	AlternativeID     string              `json:"alternative_id,omitempty"`
	Alternatives      []Alternative       `json:"alternatives"`
	Declaration       *WorkRecord         `json:"declaration,omitempty"`
	NextAction        string              `json:"next_action"`
	Rationale         string              `json:"rationale"`
}

type ServicePlan struct {
	Duties        []DutyReadiness       `json:"duties"`
	DutyCount     int                   `json:"duty_count"`
	Label         string                `json:"label,omitempty"`
	Caveat        string                `json:"caveat,omitempty"`
	Sources       map[string]Source     `json:"sources,omitempty"`
	QuestionCount int                   `json:"question_count"`
	ServiceID     string                `json:"service_id"`
	Binding       WorkBinding           `json:"binding"`
	State         string                `json:"state"`
	Capabilities  []CapabilityReadiness `json:"capabilities"`
	Questions     []PlanQuestion        `json:"questions"`
	CountUnit     string                `json:"count_unit"`
	Total         int                   `json:"total"`
	Offset        int                   `json:"offset"`
	NextOffset    *int                  `json:"next_offset,omitempty"`
}

type DutyReadiness struct {
	Evidence          []CandidateEvidence `json:"evidence"`
	ID                string              `json:"id"`
	RequirementID     string              `json:"requirement_id"`
	Duty              DutyRef             `json:"duty"`
	Step              Step                `json:"step"`
	Implementation    string              `json:"implementation"`
	Trigger           string              `json:"trigger"`
	EvidenceState     string              `json:"evidence_state"`
	Review            string              `json:"review"`
	NeedsRevalidation bool                `json:"needs_revalidation"`
	Declaration       *WorkRecord         `json:"declaration,omitempty"`
	NextAction        string              `json:"next_action"`
}

func ScopeKeys(raw []byte) ([]ScopeKey, error) {
	var scope struct {
		Accounts []struct {
			ID       string
			Regions  []string
			Clusters []string
		}
		Kubernetes []struct {
			ClusterID   string `json:"cluster_id"`
			ClusterWide bool   `json:"cluster_wide"`
			Namespaces  []struct{ UID string }
		}
		Repositories []struct{ Name string }
	}
	if err := json.Unmarshal(raw, &scope); err != nil {
		return nil, err
	}
	keys := []ScopeKey{{Kind: "service"}}
	for _, a := range scope.Accounts {
		for _, r := range a.Regions {
			keys = append(keys, ScopeKey{Kind: "account", AccountID: a.ID, Region: r})
			for _, c := range a.Clusters {
				keys = append(keys, ScopeKey{Kind: "cluster", AccountID: a.ID, Region: r, ClusterID: c})
			}
		}
	}
	for _, c := range scope.Kubernetes {
		if c.ClusterWide {
			keys = append(keys, ScopeKey{Kind: "cluster", ClusterID: c.ClusterID})
		}
		for _, n := range c.Namespaces {
			if n.UID != "" {
				keys = append(keys, ScopeKey{Kind: "namespace", ClusterID: c.ClusterID, NamespaceUID: n.UID})
			}
		}
	}
	for _, r := range scope.Repositories {
		keys = append(keys, ScopeKey{Kind: "repository", Repository: r.Name})
	}
	seen := map[ScopeKey]bool{}
	out := []ScopeKey{}
	for _, key := range keys {
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Slice(out, func(i, j int) bool { return InstanceID("", "", out[i]) < InstanceID("", "", out[j]) })
	return out, nil
}

// RelatedScope never guesses that an unassociated standalone cluster belongs
// to an AWS account, even if their display names happen to match.
func RelatedScope(a, b ScopeKey) bool {
	if a.Kind == "service" || b.Kind == "service" {
		return true
	}
	if a.Kind == b.Kind {
		return a == b
	}
	shared := false
	for _, pair := range [][2]string{{a.AccountID, b.AccountID}, {a.Region, b.Region}, {a.ClusterID, b.ClusterID}, {a.NamespaceUID, b.NamespaceUID}, {a.Repository, b.Repository}} {
		if pair[0] != "" && pair[1] != "" {
			if pair[0] != pair[1] {
				return false
			}
			shared = true
		}
	}
	return shared
}

func profileFacts(p catalog.Profile) map[string]string {
	f := map[string]string{"profile.type": p.Type, "profile.path": p.Path, "profile.class": p.Class, "profile.context": p.Context, "profile.date": p.Date}
	if p.NextAssessmentStarted != nil {
		f["profile.next_assessment_started"] = fmt.Sprint(*p.NextAssessmentStarted)
	}
	return f
}

func BuildPlan(service string, binding WorkBinding, p Package, profile catalog.Profile, resolved []catalog.Resolved, scopes []ScopeKey, records []WorkRecord) (ServicePlan, error) {
	out := ServicePlan{ServiceID: service, Binding: binding, State: "reviewed", Capabilities: []CapabilityReadiness{}, Questions: []PlanQuestion{}, CountUnit: "scoped capability instances"}
	out.Label = AgentReviewLabel
	out.Caveat = AgentReviewCaveat
	out.Sources = p.Sources
	out.Duties = []DutyReadiness{}
	selected := map[string]bool{}
	for _, r := range resolved {
		v, err := catalog.Variant(r)
		if err != nil {
			return out, err
		}
		selected[v.ID] = true
	}
	caps := map[string]Capability{}
	for _, c := range p.Capabilities {
		caps[c.ID] = c
	}
	work := map[string]WorkRecord{}
	for _, r := range records {
		if r.ServiceID == service && r.ID == WorkInstanceID(service, r.Data) {
			if previous, ok := work[r.ID]; !ok || previous.Revision < r.Revision {
				work[r.ID] = r
			}
		}
	}
	nodes := map[string]*CapabilityReadiness{}
	active := map[string]bool{}
	factsFor := func(cap string, key ScopeKey) map[string]string {
		f := profileFacts(profile)
		f["scope.kind"] = key.Kind
		for k, v := range map[string]string{"scope.account_id": key.AccountID, "scope.region": key.Region, "scope.cluster_id": key.ClusterID, "scope.namespace_uid": key.NamespaceUID, "scope.repository": key.Repository} {
			if v != "" {
				f[k] = v
			}
		}
		if w, ok := work[InstanceID(service, cap, key)]; ok && w.Binding == binding {
			for k, v := range w.Data.Decisions {
				if len(k) > 9 && k[:9] == "decision." {
					f[k] = v
				}
			}
		}
		return f
	}
	var visit func(string, ScopeKey, string) (string, error)
	visit = func(cap string, key ScopeKey, rid string) (string, error) {
		id := InstanceID(service, cap, key)
		if active[id] {
			return "", fmt.Errorf("%w: scoped dependency cycle", ErrInvalid)
		}
		c, ok := caps[cap]
		if !ok {
			return "", fmt.Errorf("%w: unknown capability %s", ErrInvalid, cap)
		}
		if c.ScopeKind != key.Kind {
			return "", ErrInvalid
		}
		n, exists := nodes[id]
		if !exists {
			n = &CapabilityReadiness{ID: id, CapabilityID: cap, Name: c.Name, Scope: key, RequirementIDs: []string{}, PrerequisiteIDs: []string{}, Implementation: "unknown", EvidenceState: "missing", Trigger: "unknown", Review: "not_assessed", Steps: c.Setup, Alternatives: c.Alternatives, NextAction: "Confirm whether this capability exists and identify its operating records.", Rationale: "Missing proof does not establish that implementation is absent."}
			if w, ok := work[id]; ok {
				n.Declaration = &w
				if w.Binding != binding {
					n.NeedsRevalidation = true
					n.NextAction = "Revalidate the saved declaration against the current guidance and service boundary."
				} else {
					n.Implementation = w.Data.Implementation
					n.Trigger = w.Data.Trigger
					if len(w.Data.Evidence) > 0 {
						n.EvidenceState = "available_for_assessment"
						n.NextAction = "Assess the referenced records against each dependent obligation."
						n.Rationale = "Attached records are candidate evidence, not a sufficiency assessment."
					} else {
						n.NextAction = "Collect records showing this capability's scope and operation."
					}
					if w.Data.AlternativeID != "" {
						matched := false
						for _, alt := range c.Alternatives {
							if alt.ID == w.Data.AlternativeID {
								n.AlternativeID = alt.ID
								n.Steps = alt.Steps
								matched = true
							}
						}
						if !matched {
							return "", ErrInvalid
						}
					}
				}
			}
			nodes[id] = n
		}
		if slices.Contains(n.RequirementIDs, rid) {
			return id, nil
		}
		n.RequirementIDs = append(n.RequirementIDs, rid)
		active[id] = true
		for _, dep := range c.Dependencies {
			state := Evaluate(dep.Condition, factsFor(cap, key))
			if state == False {
				continue
			}
			if state == Unknown {
				n.Blocked = true
				copyKey := key
				q := PlanQuestion{RequirementID: rid, CapabilityID: cap, Scope: &copyKey, Question: dep.Condition.Question, Reason: dep.Reason.Text, Fact: dep.Condition.Fact, ExpectedValue: dep.Condition.Equals}
				if w, ok := work[id]; ok {
					q.Declaration = &w
				}
				out.Questions = append(out.Questions, q)
				continue
			}
			target, ok := caps[dep.CapabilityID]
			if !ok {
				return "", ErrInvalid
			}
			matched := false
			for _, candidate := range scopes {
				if candidate.Kind != target.ScopeKind || !RelatedScope(key, candidate) {
					continue
				}
				matched = true
				child, err := visit(target.ID, candidate, rid)
				if err != nil {
					return "", err
				}
				if !slices.Contains(n.PrerequisiteIDs, child) {
					n.PrerequisiteIDs = append(n.PrerequisiteIDs, child)
				}
				n.Blocked = n.Blocked || nodes[child].Blocked
			}
			if !matched {
				n.Blocked = true
				copyKey := key
				out.Questions = append(out.Questions, PlanQuestion{RequirementID: rid, CapabilityID: target.ID, Scope: &copyKey, Question: "Which saved " + target.ScopeKind + " instance supplies this prerequisite?", Reason: "No related instance can be established from the current service boundary."})
			}
		}
		active[id] = false
		return id, nil
	}
	for _, book := range p.Playbooks {
		matches := false
		for _, id := range book.VariantIDs {
			matches = matches || selected[id]
		}
		if !matches {
			continue
		}
		if book.Kind == "needs_decision" {
			out.Questions = append(out.Questions, PlanQuestion{RequirementID: book.RequirementID, Question: book.DecisionQuestion, Reason: book.Applicability.Text})
			continue
		}
		if book.Kind != "actionable" {
			continue
		}
		for _, step := range book.Operate.Items {
			ref := DutyRef{PlaybookID: book.ID, StepID: step.ID}
			id := WorkInstanceID(service, WorkData{Duty: &ref, Scope: ScopeKey{Kind: "service"}})
			d := DutyReadiness{ID: id, RequirementID: book.RequirementID, Duty: ref, Step: step, Implementation: "unknown", Trigger: "unknown", EvidenceState: "missing", Review: "not_assessed", NextAction: "Ask " + step.OwnerRole + " for the current activity status and operating records. Use the cited timing; no deadline has been inferred."}
			if w, ok := work[id]; ok {
				d.Declaration = &w
				if w.Binding != binding {
					d.NeedsRevalidation = true
					d.NextAction = "Revalidate the saved operating declaration against the current guidance and service boundary."
				} else {
					d.Implementation = w.Data.Implementation
					d.Trigger = w.Data.Trigger
					if len(w.Data.Evidence) > 0 {
						d.EvidenceState = "available_for_assessment"
					}
					switch d.Trigger {
					case "not_triggered":
						d.NextAction = "Retain the event applicability rationale and readiness evidence. No triggering event does not mean the duty was completed."
					case "due":
						d.NextAction = "Ask " + step.OwnerRole + " for records covering the due activity and its cited timing; preserve any missed-duty evidence."
					case "observed":
						d.NextAction = "Collect the operating records for the reported event or cycle and assess their sufficiency. A reported occurrence is not proof of completion."
					}
				}
			}
			out.Duties = append(out.Duties, d)
		}
		for _, dep := range book.Prerequisites {
			if Evaluate(dep.Condition, profileFacts(profile)) == False {
				continue
			}
			c, ok := caps[dep.CapabilityID]
			if !ok {
				return out, ErrInvalid
			}
			matched := false
			for _, key := range scopes {
				if key.Kind != c.ScopeKind {
					continue
				}
				matched = true
				state := Evaluate(dep.Condition, factsFor(c.ID, key))
				if state == False {
					continue
				}
				if state == Unknown {
					copyKey := key
					q := PlanQuestion{RequirementID: book.RequirementID, CapabilityID: c.ID, Scope: &copyKey, Question: dep.Condition.Question, Reason: dep.Reason.Text, Fact: dep.Condition.Fact, ExpectedValue: dep.Condition.Equals}
					if w, ok := work[InstanceID(service, c.ID, key)]; ok {
						q.Declaration = &w
					}
					out.Questions = append(out.Questions, q)
					continue
				}
				if _, err := visit(c.ID, key, book.RequirementID); err != nil {
					return out, err
				}
			}
			if !matched {
				out.Questions = append(out.Questions, PlanQuestion{RequirementID: book.RequirementID, CapabilityID: c.ID, Question: "Which saved " + c.ScopeKind + " instance supplies this prerequisite?", Reason: "The saved service boundary has no matching instance; absence from scope is not proof of absent implementation."})
			}
		}
	}
	// Deterministic topological order, with exact scoped identifiers as tie-breakers.
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	seen := map[string]bool{}
	var appendNode func(string)
	appendNode = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		n := nodes[id]
		sort.Strings(n.PrerequisiteIDs)
		for _, child := range n.PrerequisiteIDs {
			appendNode(child)
		}
		sort.Strings(n.RequirementIDs)
		out.Capabilities = append(out.Capabilities, *n)
	}
	for _, id := range ids {
		appendNode(id)
	}
	out.Total = len(out.Capabilities)
	rank := func(d DutyReadiness) int {
		if d.NeedsRevalidation {
			return 0
		}
		switch d.Trigger {
		case "due":
			return 1
		case "unknown":
			return 2
		case "observed":
			return 3
		default:
			return 4
		}
	}
	sort.Slice(out.Duties, func(i, j int) bool {
		a, b := out.Duties[i], out.Duties[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		return a.ID < b.ID
	})
	out.DutyCount = len(out.Duties)
	questionKeys := map[string]PlanQuestion{}
	for _, q := range out.Questions {
		raw, _ := json.Marshal(q)
		questionKeys[string(raw)] = q
	}
	orderedQuestions := make([]string, 0, len(questionKeys))
	for key := range questionKeys {
		orderedQuestions = append(orderedQuestions, key)
	}
	sort.Strings(orderedQuestions)
	out.Questions = []PlanQuestion{}
	for _, key := range orderedQuestions {
		out.Questions = append(out.Questions, questionKeys[key])
	}
	out.QuestionCount = len(out.Questions)
	return out, nil
}
