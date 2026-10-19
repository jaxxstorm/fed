package assessment

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Count actual PostgreSQL calls, including calls that bypass recordReader.
type journeyQueryTrace struct{ count atomic.Int64 }

func (q *journeyQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}
func (*journeyQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestReadinessJourneyQueueBatchBudgetAndCompleteTraversal(t *testing.T) {
	s, actor, service := agentFixture(t)
	ctx := context.Background()
	trace := &journeyQueryTrace{}
	cfg := s.DB.Config()
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	view := *s
	view.DB = pool
	var baseline int64
	for _, limit := range []int{1, 25, 100} {
		trace.count.Store(0)
		page, err := view.ReadinessQueue(ctx, actor, ReadinessQueueInput{ServiceID: service, Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		queries := trace.count.Load()
		t.Logf("limit=%d returned=%d total=%d PostgreSQL queries=%d", limit, len(page.Items), page.Total, queries)
		if len(page.Items) != min(limit, page.Total) || page.Total <= 100 {
			t.Fatalf("fixture did not exercise page bounds: %+v", page)
		}
		if queries > 40 {
			t.Fatalf("readiness queue exceeded fixed 40-query budget: %d", queries)
		}
		if baseline == 0 {
			baseline = queries
		} else if queries != baseline {
			t.Fatalf("page size changed query count: %d -> %d", baseline, queries)
		}
	}
	seen := map[string]bool{}
	input := ReadinessQueueInput{ServiceID: service, Limit: 100}
	var hash string
	for pages := 0; ; pages++ {
		if pages > 100 {
			t.Fatal("cursor traversal failed to terminate")
		}
		page, err := view.ReadinessQueue(ctx, actor, input)
		if err != nil {
			t.Fatal(err)
		}
		if hash == "" {
			hash = page.ViewHash
		}
		if page.ViewHash != hash || page.Binding.ScopeRevision != 1 || page.Binding.ProfileRevision != 1 {
			t.Fatal("revision drift during traversal")
		}
		for _, item := range page.Items {
			if seen[item.RequirementID] {
				t.Fatalf("duplicate requirement %s", item.RequirementID)
			}
			seen[item.RequirementID] = true
		}
		if page.NextCursor == "" {
			if len(seen) != page.Total {
				t.Fatalf("traversed %d of %d requirements", len(seen), page.Total)
			}
			break
		}
		input.Cursor = page.NextCursor
	}
}
