package assessment

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/observation"
)

func TestNewAccessObservationSupersedesErrorWithoutDeletingHistory(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	const rid = "KSI-CNA-RNT"
	if _, err := s.SaveMeasure(ctx, a, service, "network", rid, 0, Measure{Name: "Network", Rationale: "Inspect network", FreshnessHours: 24, ScopeRevision: 1}); err != nil {
		t.Fatal(err)
	}
	base := observation.Submission{ServiceID: service, ScopeRevision: 1, ProfileRevision: 1, Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:eks:us-east-1:123456789012:cluster/partition"}, Method: "read-only fixture", ToolVersion: "1", Artifact: observation.Artifact{Schema: "aws.eks.endpoint/v1", Configuration: map[string]json.RawMessage{"public_access": json.RawMessage(`true`)}}, Links: []observation.Link{{RequirementID: rid, Measure: observation.Ref{ID: "network", Revision: 1}, MappingVersion: "v1", Rationale: "Network inspection"}}}
	old := base
	old.IdempotencyKey = "error"
	old.ObservedAt = time.Now().UTC().Add(-time.Minute)
	old.Coverage = observation.Coverage{State: "error", Error: "credentials_or_account_validation_failed", Expected: 1}
	if _, err := s.IngestObservation(ctx, a, old); err != nil {
		t.Fatal(err)
	}
	finding := base
	finding.IdempotencyKey = "finding"
	finding.Target.ResourceID = "arn:aws:eks:us-east-1:123456789012:cluster/resource"
	finding.ObservedAt = old.ObservedAt.Add(time.Second)
	finding.Coverage = observation.Coverage{State: "observed", Expected: 1, Observed: 1, Complete: true}
	finding.Evaluations = []observation.Evaluation{{Condition: "network", Outcome: "fail", Explanation: "Retained network finding"}}
	if _, err := s.IngestObservation(ctx, a, finding); err != nil {
		t.Fatal(err)
	}
	fresh := base
	fresh.IdempotencyKey = "access"
	fresh.ObservedAt = old.ObservedAt.Add(2 * time.Second)
	fresh.Coverage = observation.Coverage{State: "observed", Complete: true}
	fresh.Evaluations = []observation.Evaluation{{Condition: "collection_access", Outcome: "unknown", Explanation: "Partition enumerated"}}
	if _, err := s.IngestObservation(ctx, a, fresh); err != nil {
		t.Fatal(err)
	}
	status, err := s.observationStatus(ctx, service, rid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Errors) != 0 || status.Technical != "fail" || len(status.Findings) != 1 || status.Expected != 1 || status.Observed != 1 {
		t.Fatalf("access recovery erased finding or inflated coverage: %+v", status)
	}
	var retained int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM observations WHERE service_id=$1`, service).Scan(&retained); err != nil || retained != 3 {
		t.Fatalf("immutable history lost: %d %v", retained, err)
	}
}
