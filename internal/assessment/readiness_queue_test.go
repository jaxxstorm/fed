package assessment

import (
	"context"
	"errors"
	"testing"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestReadinessQueuePaginationAndScopeFence(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	in := ReadinessQueueInput{ServiceID: service, Group: "evidence", Limit: 2}
	first, err := s.ReadinessQueue(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total <= 2 || len(first.Items) != 2 || first.NextCursor == "" || first.CountUnit != "requirements" || first.Binding.ScopeRevision != 1 {
		t.Fatal("queue missing counts, bounds or binding", first)
	}
	in.Cursor = first.NextCursor
	second, err := s.ReadinessQueue(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if second.ViewHash != first.ViewHash || second.Items[0].RequirementID == first.Items[0].RequirementID {
		t.Fatal("unstable or repeated page", second)
	}
	in.Group = "review"
	if _, err = s.ReadinessQueue(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("cursor crossed group", err)
	}
	in.Group = "evidence"
	if _, err = s.SaveScope(ctx, a, service, 1, Scope{Description: "Changed saved boundary", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadinessQueue(ctx, a, in); !errors.Is(err, ErrViewChanged) {
		t.Fatal("cursor reused across changed scope", err)
	}
	if _, err = s.ReadinessQueue(ctx, auth.Actor{}, ReadinessQueueInput{ServiceID: service}); err == nil {
		t.Fatal("unauthorized queue disclosed")
	}
	in.Cursor = ""
	in.Group = "review"
	review, err := s.ReadinessQueue(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if review.Total != 0 || review.Counts["review"] != 0 {
		t.Fatal("untouched requirements inferred ready for review", review)
	}
	in.Limit = 101
	if _, err = s.ReadinessQueue(ctx, a, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded queue accepted", err)
	}
}
