package assessment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestEvidenceRequestFulfillmentRequiresHumanAndCurrentBinding(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := s.SaveEvidenceSource(ctx, a, service, "records", 0, EvidenceSource{Type: "operational_records", Name: "Records", Locator: "https://docs.example/records", ScopeRevision: 1, Enabled: true, FreshnessHours: 48})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "record", Source: Ref{"records", 1}, ScopeRevision: 1, Locator: "https://docs.example/records/review", SourceVersion: "v1", MediaType: "text/plain", Content: "Dated review record", ObservedAt: now, RetrievedAt: now, Method: "Read record", Description: "Review"})
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := s.RuleInstructions(ctx, a, service, "KSI-CNA-RNT")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, assertion := range instructions.Assertions {
		ids = append(ids, assertion.ID)
	}
	in := EvidenceRequestChange{ServiceID: service, RequirementID: "KSI-CNA-RNT", ID: "review-records", Request: EvidenceRequest{Binding: instructions.Binding, AssertionIDs: ids, Title: "Dated reviews", AcceptanceCriteria: "Review the decisions and dates", Period: AssessmentPeriod{Start: now.Add(-24 * time.Hour), End: now}, State: "open"}}
	if _, err = s.DB.Exec(ctx, `INSERT INTO readiness_work(service_id,id,revision,actor_id,binding,data) VALUES($1,'work',1,$2,'{}','{}')`, service, a.ID); err != nil {
		t.Fatal(err)
	}
	in.Request.ReadinessWork = &Ref{ID: "work", Revision: 1}
	r, err := s.SaveEvidenceRequest(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := in
	duplicate.ID = "duplicate-work"
	if _, err = s.SaveEvidenceRequest(ctx, a, duplicate); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate readiness work: %v", err)
	}
	page, err := s.EvidenceRequests(ctx, a, EvidenceRequestQuery{ServiceID: service, State: "open"})
	if err != nil || len(page) != 1 || page[0].RevalidationNeeded {
		t.Fatalf("request queue: %+v %v", page, err)
	}
	in.ExpectedRevision = r.Revision
	in.Request.State = "submitted"
	in.Request.Fulfillment = []Citation{{Evidence: Ref{ev.ID, ev.Revision}, StartLine: 1, EndLine: 1}}
	token, err := auth.IssueToken(ctx, s.DB, a, service, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateService(ctx, a, "Other service")
	if err != nil {
		t.Fatal(err)
	}
	cross := in
	cross.ServiceID = other.ID
	if _, err = s.SaveEvidenceRequest(ctx, agent, cross); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("cross-service write: %v", err)
	}
	r, err = s.SaveEvidenceRequest(ctx, agent, in)
	if err != nil {
		t.Fatal(err)
	}
	in.ExpectedRevision = r.Revision
	in.Request.State = "accepted"
	in.Request.DecisionReason = "Records meet this request"
	if _, err = s.SaveEvidenceRequest(ctx, agent, in); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("agent accepted own fulfillment: %v", err)
	}
	changed := in
	changed.Request.AcceptanceCriteria = "Different criteria"
	if _, err = s.SaveEvidenceRequest(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("criteria changed during acceptance: %v", err)
	}
	_, err = s.SaveScope(ctx, a, service, 1, Scope{Description: "Changed scope", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveEvidenceRequest(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("obsolete fulfillment accepted: %v", err)
	}
	page, err = s.EvidenceRequests(ctx, a, EvidenceRequestQuery{ServiceID: service, State: "submitted"})
	if err != nil || len(page) != 1 || !page[0].RevalidationNeeded {
		t.Fatalf("stale request not flagged: %+v %v", page, err)
	}
	var revisions int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind='evidence_request'`, service).Scan(&revisions); err != nil || revisions != 2 {
		t.Fatalf("failed transitions mutated history: %d %v", revisions, err)
	}
}
