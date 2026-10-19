package assessment

import (
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func TestGuidanceActionPreservesComplianceAndFailurePriority(t *testing.T) {
	g := readiness.GuidanceContext{State: "reviewed", Guidance: &readiness.Package{Playbooks: []readiness.Playbook{{Kind: "actionable", Collect: readiness.Steps{Items: []readiness.Step{{OwnerRole: "Training owner", Action: readiness.Statement{Text: "Collect dated training-effectiveness reviews."}}}}}}}}
	open := PresentRequirement("required", Status{}, false)
	updated := WithGuidanceAction(open, g)
	if updated.Status != open.Status || updated.EvidenceComplete || updated.Action != "Training owner: Collect dated training-effectiveness reviews." {
		t.Fatal("guidance action missing or changed compliance", updated)
	}
	for _, state := range []Status{{Technical: "fail"}, {Technical: "error"}, {Work: EvidenceWork{State: "needs_refresh"}}, {Work: EvidenceWork{State: "evidence_complete", Assessment: "supported"}, Evidence: "current", Review: "reviewed"}} {
		before := PresentRequirement("required", state, true)
		after := WithGuidanceAction(before, g)
		if after.Status != before.Status || after.Action != before.Action || after.HumanReviewed != before.HumanReviewed {
			t.Fatal("collection suggestion overrode finding, freshness or review state", before, after)
		}
	}
	g.State = "needs_revalidation"
	if stale := WithGuidanceAction(open, g); stale.Action != open.Action || stale.GuidanceState != "needs_revalidation" {
		t.Fatal("stale guidance recommended as current", stale)
	}
}

func TestScopedGuidanceActionsKeepComplianceAndAccessGaps(t *testing.T) {
	g := readiness.GuidanceContext{State: "reviewed", Guidance: &readiness.Package{Playbooks: []readiness.Playbook{{Kind: "actionable"}}}, Progress: &readiness.RequirementProgress{
		Questions:    []readiness.PlanQuestion{{Question: "Has a production incident occurred?", Reason: "Only a triggering event makes this duty due."}},
		Capabilities: []readiness.CapabilityReadiness{{Name: "Incident response", Implementation: "unknown", EvidenceState: "missing", Steps: readiness.Steps{Items: []readiness.Step{{OwnerRole: "Incident owner", Action: readiness.Statement{Text: "Identify the on-call escalation route."}}}}}},
	}}
	before := PresentRequirement("required", Status{}, false)
	after := WithGuidanceAction(before, g)
	if after.Action != g.Progress.Questions[0].Question || after.Status != before.Status || after.Reason != before.Reason || after.EvidenceComplete {
		t.Fatal("unresolved condition became compliance", after)
	}
	g.Progress.Questions = nil
	after = WithGuidanceAction(before, g)
	if !strings.Contains(after.Action, "Incident owner") || !strings.Contains(after.Action, "Confirm whether Incident response exists and request its records") || strings.Contains(after.Action, "Identify the on-call") || after.Reason != before.Reason {
		t.Fatal("specific prerequisite/owner missing", after)
	}
	blocked := PresentRequirement("required", Status{Technical: "fail", Collection: ObservationStatus{Recovery: []observation.Recovery{{Title: "Production access denied", Explanation: "Cannot read records.", Action: "Ask source owner for access.", Target: "production"}}}}, true)
	after = WithGuidanceAction(blocked, g)
	if after.Status != "failing" || after.Action != blocked.Action || len(after.ContextGaps) != 1 || after.ContextGaps[0].Targets[0] != "production" {
		t.Fatal("guidance obscured contradiction or access gap", after)
	}
	for _, state := range []string{"missing", "draft", "needs_revalidation"} {
		g.State = state
		g.Reason = "Source comparison unavailable."
		after = WithGuidanceAction(before, g)
		if after.Action != before.Action || after.GuidanceReason != g.Reason {
			t.Fatal("unreviewed text became action", after)
		}
	}
}
