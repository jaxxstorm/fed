package assessment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestIssueClosurePreservesGapAndRequiresHumanDisposition(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	gap, err := s.SaveGap(ctx, a, service, "missing-records", "KSI-CNA-RNT", 0, Gap{Explanation: "Operating records absent", Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	in := AssessmentIssueChange{ServiceID: service, RequirementID: "KSI-CNA-RNT", ID: "issue", Issue: AssessmentIssue{Binding: instructions.Binding, Explanation: "Investigate operating evidence gap", State: "open", Gaps: []Ref{{gap.ID, gap.Revision}}, Milestones: []IssueMilestone{{ID: "investigate", Description: "Find archived records"}}}}
	r, err := s.SaveAssessmentIssue(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	in.ExpectedRevision = r.Revision
	in.Issue.State = "closed"
	in.Issue.Disposition = "Duplicate investigation; linked gap remains open and does not establish compliance."
	token, err := auth.IssueToken(ctx, s.DB, a, service, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveAssessmentIssue(ctx, agent, in); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("agent closed issue: %v", err)
	}
	closed, err := s.SaveAssessmentIssue(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if closed.ActorID != a.ID || closed.Revision != 2 {
		t.Fatalf("closure provenance lost: %+v", closed)
	}
	old, err := s.Get(ctx, a, service, "gap", gap.ID)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Decode[Gap](old)
	if err != nil || v.Status != "open" || old.Revision != gap.Revision {
		t.Fatalf("issue closure erased gap: %+v %v", v, err)
	}
}
