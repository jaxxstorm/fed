package assessment

import (
	"github.com/jaxxstorm/fed/internal/observation"
	"strings"
	"testing"
)

func TestBlockedGuidanceGroupsTargetsAndPreservesMissingEvidence(t *testing.T) {
	s := Status{Technical: "error", Evidence: "current", Work: EvidenceWork{State: "partial", Blockers: []string{"Dated drift-review records are missing."}}, Collection: ObservationStatus{Recovery: []observation.Recovery{observation.ExplainError("github_credentials_missing", "repo-a", "collector_observation"), observation.ExplainError("github_credentials_missing", "repo-b", "collector_observation")}}}
	p := PresentRequirement("required", s, true)
	if p.Status != "blocked" || len(p.NextSteps) < 3 || len(p.NextSteps[0].Targets) != 2 || p.NextSteps[0].Provider != "github" || !strings.Contains(p.Reason, "GitHub access") || !strings.Contains(p.NextSteps[1].Explanation, "drift-review") {
		t.Fatalf("missing actionable guidance: %+v", p)
	}
	if p.EvidenceComplete || p.ReviewRemaining {
		t.Fatal("guidance changed assessment completion")
	}
}

func TestPresentationPrecedence(t *testing.T) {
	ready := Status{Evidence: "current", Review: "reviewed", Work: EvidenceWork{State: "evidence_complete", Assessment: "supported"}}
	for _, tc := range []struct {
		name, app, want string
		status          Status
		activity        bool
	}{
		{"documentary passing", "required", "passing", ready, false},
		{"not applicable failure", "not_applicable", "not_applicable", Status{Technical: "fail"}, false},
		{"unresolved failure", "unresolved", "needs_scoping", Status{Technical: "fail"}, false},
		{"failure and blocker", "required", "failing", Status{Technical: "fail", Work: EvidenceWork{State: "blocked"}}, false},
		{"complete contradictory investigation", "required", "failing", Status{Work: EvidenceWork{State: "evidence_complete", Assessment: "contradicted"}}, false},
		{"collection error", "required", "blocked", Status{Technical: "error"}, false},
		{"expiry", "required", "needs_refresh", Status{Evidence: "stale", Review: "reviewed", Work: ready.Work}, false},
		{"partial", "required", "needs_evidence", Status{Work: EvidenceWork{State: "partial"}}, false},
		{"review needed", "required", "in_review", Status{Evidence: "current", Review: "review-needed", Work: ready.Work}, false},
		{"passing check alone", "required", "in_progress", Status{Technical: "pass", Collection: ObservationStatus{Count: 1}}, false},
		{"active collection", "required", "in_progress", Status{}, true},
		{"untouched missing assertions", "required", "not_started", Status{Work: EvidenceWork{State: "open", MissingAssertions: []string{"whole-statement"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PresentRequirement(tc.app, tc.status, tc.activity)
			if got.Status != tc.want {
				t.Fatalf("got %s want %s", got.Status, tc.want)
			}
			if got.Reason == "" || got.Action == "" {
				t.Fatal("status has no explanation or next action")
			}
			if len(got.NextSteps) == 0 {
				t.Fatalf("%s has no actionable guidance", got.Status)
			}
			for _, step := range got.NextSteps {
				if step.Explanation == "" || step.Action == "" {
					t.Fatalf("incomplete guidance for %s: %+v", got.Status, step)
				}
			}
		})
	}
}

func TestFailureHeadlineExplainsFindingInsteadOfAccessGap(t *testing.T) {
	s := Status{Technical: "error", Collection: ObservationStatus{
		Recovery: []observation.Recovery{observation.ExplainError("kubernetes_context_binding_missing", "cluster", "collector_observation")},
		Findings: []observation.Finding{{ID: "finding", Condition: "public_endpoint", Outcome: "fail", Explanation: "Unrestricted public endpoint", ArtifactSchema: "aws.eks.endpoint/v1"}},
	}}
	p := PresentRequirement("required", s, false)
	if p.Status != "failing" || strings.Contains(p.Reason, "Choose a Kubernetes context") || p.Action != "View findings" || len(p.ContextGaps) != 1 {
		t.Fatalf("finding and access gap confused: %+v", p)
	}
	s.Collection.Findings = nil
	p = PresentRequirement("required", s, false)
	if p.Status != "blocked" || p.Label != "Evidence blocked" {
		t.Fatalf("access gap presented as control failure: %+v", p)
	}
}

func TestProgressDenominators(t *testing.T) {
	var p ProgressCounts
	for _, app := range []string{"required", "required", "recommended", "optional", "not_applicable", "unresolved"} {
		p.Add(app, RequirementPresentation{Status: "passing", EvidenceComplete: true})
	}
	if p.Required != 2 || p.Passing != 2 || p.ScopeDecisions != 1 || p.Optional != 1 || p.Recommended != 1 || p.NotApplicable != 1 || p.EvidenceComplete != 2 {
		t.Fatalf("invalid denominators: %+v", p)
	}
	var empty ProgressCounts
	if empty.Required != 0 || empty.Passing != 0 {
		t.Fatal("empty service claimed progress")
	}
}
