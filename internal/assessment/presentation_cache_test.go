package assessment

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/observation"
	"reflect"
	"testing"
	"time"
)

type countedReader struct {
	recordReader
	count int
}

func assertPresentationParity(t *testing.T, s *Store, a auth.Actor, service, rid string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	view := *s
	var err error
	view.presentationCache, err = view.loadPresentationCache(ctx, service, a.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	got, err := view.Status(ctx, a, service, rid, at)
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.Status(ctx, a, service, rid, at)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cached status differs for %s\ngot %s\nwant %s", rid, data(got), data(want))
	}
}
func (c *countedReader) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.count++
	return c.recordReader.Query(ctx, sql, args...)
}
func (c *countedReader) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.count++
	return c.recordReader.QueryRow(ctx, sql, args...)
}
func TestPresentationBatchParityAndQueryBound(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.SaveMeasure(ctx, a, service, "encryption", "KSI-SVC-SIN", 0, Measure{Name: "Encryption", Rationale: "At rest", FreshnessHours: 48, ScopeRevision: 1}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, s.DB, a, service, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	in := observation.Submission{ServiceID: service, ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "presentation-test", Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:rds:us-east-1:123456789012:db:test"}, ObservedAt: now, Method: "DescribeDBInstances", ToolVersion: "aws-cli/2", Coverage: observation.Coverage{Expected: 1, Observed: 1, Complete: true, State: "observed"}, Artifact: observation.Artifact{Schema: "aws.rds/v1", Configuration: map[string]json.RawMessage{"storage_encrypted": json.RawMessage(`false`)}}, Links: []observation.Link{{RequirementID: "KSI-SVC-SIN", Measure: observation.Ref{ID: "encryption", Revision: 1}, MappingVersion: "v1", Rationale: "At rest"}}, Evaluations: []observation.Evaluation{{Condition: "encryption", Outcome: "fail", Explanation: "Storage encryption disabled"}}}
	if _, err = s.IngestObservation(ctx, agent, in); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{now, now.Add(72 * time.Hour)} {
		counter := &countedReader{recordReader: s.DB}
		view := *s
		view.reader = counter
		view.presentationCache, err = view.loadPresentationCache(ctx, service, a.ID, at)
		if err != nil {
			t.Fatal(err)
		}
		if counter.count > 8 {
			t.Fatalf("cache load used %d queries", counter.count)
		}
		before := counter.count
		requirements, err := s.Requirements(ctx, a, service)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range requirements {
			got, err := view.Status(ctx, a, service, r.Requirement.ID, at)
			if err != nil {
				t.Fatal(err)
			}
			if r.Requirement.ID == "KSI-SVC-SIN" || r.Requirement.ID == "KSI-SVC-ACM" {
				want, err := s.Status(ctx, a, service, r.Requirement.ID, at)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("batch parity %s\ngot %#v\nwant %#v", r.Requirement.ID, got, want)
				}
			}
		}
		if counter.count != before {
			t.Fatalf("%d rules caused %d additional queries", len(requirements), counter.count-before)
		}
	}
}
