package readiness

import (
	"fmt"
	"sort"
)

type Truth string

const (
	True    Truth = "true"
	False   Truth = "false"
	Unknown Truth = "unknown"
)

// Evaluate preserves the distinction between an absent fact and a known empty
// value. Facts must be supplied from the selected profile/scope and explicit
// service decisions, never inferred from missing evidence.
func Evaluate(condition *Condition, facts map[string]string) Truth {
	if condition == nil {
		return True
	}
	value, ok := facts[condition.Fact]
	if !ok {
		return Unknown
	}
	if value == condition.Equals {
		return True
	}
	return False
}

type DependencyDecision struct {
	CapabilityID string    `json:"capability_id"`
	State        Truth     `json:"state"`
	Question     string    `json:"question,omitempty"`
	Reason       Statement `json:"reason"`
}

type CapabilityPlan struct {
	Capability    Capability           `json:"capability"`
	Dependencies  []DependencyDecision `json:"dependencies"`
	AlternativeID string               `json:"alternative_id,omitempty"`
	Steps         Steps                `json:"steps"`
	Blocked       bool                 `json:"blocked"`
}

type DependencyPlan struct {
	Roots        []DependencyDecision `json:"roots"`
	Capabilities []CapabilityPlan     `json:"capabilities"`
}

// ResolveDependencies orders known prerequisites before dependents. Unknown
// edges remain questions, never silently mandated capabilities or exclusions.
// This pure projection operates on one scope instance; callers must not combine
// facts/choices from different environments before invoking it.
func ResolveDependencies(p Package, playbookID string, facts, alternatives map[string]string) (DependencyPlan, error) {
	out := DependencyPlan{Roots: []DependencyDecision{}, Capabilities: []CapabilityPlan{}}
	var roots []Dependency
	found := false
	for _, b := range p.Playbooks {
		if b.ID == playbookID {
			roots = b.Prerequisites
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("%w: unknown playbook %s", ErrInvalid, playbookID)
	}
	caps := map[string]Capability{}
	for _, c := range p.Capabilities {
		if _, ok := caps[c.ID]; ok {
			return out, fmt.Errorf("%w: duplicate capability %s", ErrInvalid, c.ID)
		}
		caps[c.ID] = c
	}
	state := map[string]int{}
	blocked := map[string]bool{}
	decision := func(d Dependency) DependencyDecision {
		x := DependencyDecision{CapabilityID: d.CapabilityID, State: Evaluate(d.Condition, facts), Reason: d.Reason}
		if x.State == Unknown {
			x.Question = d.Condition.Question
		}
		return x
	}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("%w: dependency cycle at %s", ErrInvalid, id)
		}
		if state[id] == 2 {
			return nil
		}
		c, ok := caps[id]
		if !ok {
			return fmt.Errorf("%w: unknown capability %s", ErrInvalid, id)
		}
		state[id] = 1
		plan := CapabilityPlan{Capability: c, Dependencies: []DependencyDecision{}, Steps: c.Setup}
		if choice := alternatives[id]; choice != "" {
			matched := false
			for _, a := range c.Alternatives {
				if a.ID == choice {
					matched = true
					plan.Steps = a.Steps
					plan.AlternativeID = choice
					break
				}
			}
			if !matched {
				return fmt.Errorf("%w: unknown alternative %s for %s", ErrInvalid, choice, id)
			}
		}
		deps := append([]Dependency(nil), c.Dependencies...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].CapabilityID < deps[j].CapabilityID })
		for _, d := range deps {
			if _, ok := caps[d.CapabilityID]; !ok {
				return fmt.Errorf("%w: unknown prerequisite %s", ErrInvalid, d.CapabilityID)
			}
			x := decision(d)
			plan.Dependencies = append(plan.Dependencies, x)
			if x.State == Unknown {
				plan.Blocked = true
			}
			if x.State == True {
				if err := visit(d.CapabilityID); err != nil {
					return err
				}
				plan.Blocked = plan.Blocked || blocked[d.CapabilityID]
			}
		}
		blocked[id] = plan.Blocked
		state[id] = 2
		out.Capabilities = append(out.Capabilities, plan)
		return nil
	}
	ordered := append([]Dependency(nil), roots...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CapabilityID < ordered[j].CapabilityID })
	for _, d := range ordered {
		if _, ok := caps[d.CapabilityID]; !ok {
			return out, fmt.Errorf("%w: unknown prerequisite %s", ErrInvalid, d.CapabilityID)
		}
		x := decision(d)
		out.Roots = append(out.Roots, x)
		if x.State == True {
			if err := visit(d.CapabilityID); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
