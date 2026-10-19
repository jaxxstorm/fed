package readiness

import (
	"testing"

	"github.com/jaxxstorm/fed/internal/catalog"
)

func TestScopedPlanKeepsDeclarationsSeparateFromProof(t *testing.T) {
	p := draftFixture()
	rules := []catalog.Resolved{{Requirement: catalog.Requirement{ID: "RULE"}, Statement: p.Playbooks[0].OfficialStatement, Applicability: "required"}}
	v, err := catalog.Variant(rules[0])
	if err != nil {
		t.Fatal(err)
	}
	p.Playbooks[0].VariantIDs = []string{v.ID}
	p.Capabilities[0].ScopeKind = "cluster"
	binding := WorkBinding{CatalogID: p.CatalogID, ReleaseRevision: 1, ReleaseHash: "release", ProfileRevision: 1, ScopeRevision: 1}
	prod := ScopeKey{Kind: "cluster", ClusterID: "production"}
	dev := ScopeKey{Kind: "cluster", ClusterID: "development"}
	declaration := WorkRecord{ID: InstanceID("s", "monitoring", dev), ServiceID: "s", Revision: 1, Binding: binding, Data: WorkData{CapabilityID: "monitoring", Scope: dev, Implementation: "declared_present", Trigger: "not_triggered", Evidence: []EvidenceRef{{ID: "config", Revision: 1}}}}
	plan, err := BuildPlan("s", binding, p, catalog.Profile{}, rules, []ScopeKey{prod, dev}, []WorkRecord{declaration})
	if err != nil || plan.Total != 2 {
		t.Fatal("lost scoped instances", plan, err)
	}
	for _, n := range plan.Capabilities {
		if n.Scope == dev {
			if n.Implementation != "declared_present" || n.EvidenceState != "available_for_assessment" || n.Review != "not_assessed" || n.Trigger != "not_triggered" {
				t.Fatal("declaration became compliance", n)
			}
		} else if n.Implementation != "unknown" || n.EvidenceState != "missing" {
			t.Fatal("development declaration leaked to production", n)
		}
	}
	changed := binding
	changed.ScopeRevision = 2
	plan, err = BuildPlan("s", changed, p, catalog.Profile{}, rules, []ScopeKey{dev}, []WorkRecord{declaration})
	if err != nil || plan.Capabilities[0].Implementation != "unknown" || !plan.Capabilities[0].NeedsRevalidation {
		t.Fatal("old scope declaration reused", plan, err)
	}
	plan, err = BuildPlan("other", binding, p, catalog.Profile{}, rules, []ScopeKey{dev}, []WorkRecord{declaration})
	if err != nil || plan.Capabilities[0].Implementation != "unknown" {
		t.Fatal("cross-service declaration reused", plan, err)
	}
}

func TestScopedPlanSharedPrerequisitesAndUnknownConditions(t *testing.T) {
	p := draftFixture()
	rules := []catalog.Resolved{{Requirement: catalog.Requirement{ID: "RULE"}, Statement: p.Playbooks[0].OfficialStatement, Applicability: "required"}, {Requirement: catalog.Requirement{ID: "OTHER"}, Statement: p.Playbooks[0].OfficialStatement, Applicability: "required"}}
	v, _ := catalog.Variant(rules[0])
	p.Playbooks[0].VariantIDs = []string{v.ID}
	other := p.Playbooks[0]
	other.ID = "other"
	other.RequirementID = "OTHER"
	v, _ = catalog.Variant(rules[1])
	other.VariantIDs = []string{v.ID}
	p.Playbooks = append(p.Playbooks, other)
	scope := []ScopeKey{{Kind: "service"}}
	plan, err := BuildPlan("s", WorkBinding{}, p, catalog.Profile{}, rules, scope, nil)
	if err != nil || plan.Total != 1 || len(plan.Capabilities[0].RequirementIDs) != 2 {
		t.Fatal("shared prerequisite duplicated", plan, err)
	}
	p.Playbooks[0].Prerequisites = append([]Dependency(nil), p.Playbooks[0].Prerequisites...)
	p.Playbooks[0].Prerequisites[0].Condition = &Condition{Fact: "decision.external", Equals: "yes", Question: "Are externally managed resources used?"}
	plan, err = BuildPlan("s", WorkBinding{}, p, catalog.Profile{}, rules, scope, nil)
	if err != nil || len(plan.Questions) != 1 || len(plan.Capabilities[0].RequirementIDs) != 1 {
		t.Fatal("unknown condition became requirement", plan, err)
	}
	if RelatedScope(ScopeKey{Kind: "cluster", ClusterID: "standalone"}, ScopeKey{Kind: "account", AccountID: "aws", Region: "us-east-1"}) {
		t.Fatal("invented account association")
	}
}
