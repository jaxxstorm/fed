package readiness

import "testing"

func TestDependencyConditionsOrderAndAlternatives(t *testing.T) {
	p := draftFixture()
	reason := p.Playbooks[0].Purpose
	base := p.Capabilities[0]
	base.ID = "inventory"
	base.Dependencies = nil
	p.Capabilities = append(p.Capabilities, base)
	p.Capabilities[0].Dependencies = []Dependency{{CapabilityID: "inventory", Reason: reason, Condition: &Condition{Fact: "decision.external_resources", Equals: "yes", Question: "Does this scope include externally managed resources?"}}}
	p.Capabilities[0].Alternatives = []Alternative{{ID: "provider", Explanation: reason, Steps: Steps{NoneReason: "Use the provider-managed setup already described in the selected route."}}}
	unknown, err := ResolveDependencies(p, "rule", nil, nil)
	if err != nil || len(unknown.Capabilities) != 1 || !unknown.Capabilities[0].Blocked || unknown.Capabilities[0].Dependencies[0].State != Unknown || unknown.Capabilities[0].Dependencies[0].Question == "" {
		t.Fatal("unknown condition was excluded or mandated", unknown, err)
	}
	yes, err := ResolveDependencies(p, "rule", map[string]string{"decision.external_resources": "yes"}, map[string]string{"monitoring": "provider"})
	if err != nil || len(yes.Capabilities) != 2 || yes.Capabilities[0].Capability.ID != "inventory" || yes.Capabilities[1].Blocked || yes.Capabilities[1].AlternativeID != "provider" || yes.Capabilities[1].Steps.NoneReason == "" {
		t.Fatal("dependency order or alternative lost", yes, err)
	}
	no, err := ResolveDependencies(p, "rule", map[string]string{"decision.external_resources": "no"}, nil)
	if err != nil || len(no.Capabilities) != 1 || no.Capabilities[0].Blocked || no.Capabilities[0].Dependencies[0].State != False {
		t.Fatal("known exclusion lost", no, err)
	}
	if _, err = ResolveDependencies(p, "rule", nil, map[string]string{"monitoring": "invented"}); err == nil {
		t.Fatal("invalid alternative accepted")
	}
	p.Capabilities[1].Dependencies = []Dependency{{CapabilityID: "monitoring", Reason: reason}}
	if _, err = ResolveDependencies(p, "rule", map[string]string{"decision.external_resources": "yes"}, nil); err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestSharedDependenciesAppearOnce(t *testing.T) {
	p := draftFixture()
	reason := p.Playbooks[0].Purpose
	c := p.Capabilities[0]
	c.ID = "reviewing"
	c.Dependencies = []Dependency{{CapabilityID: "monitoring", Reason: reason}}
	p.Capabilities = append(p.Capabilities, c)
	p.Playbooks[0].Prerequisites = append(p.Playbooks[0].Prerequisites, Dependency{CapabilityID: "reviewing", Reason: reason})
	plan, err := ResolveDependencies(p, "rule", nil, nil)
	if err != nil || len(plan.Capabilities) != 2 || plan.Capabilities[0].Capability.ID != "monitoring" {
		t.Fatal("shared dependency duplicated", plan, err)
	}
}
