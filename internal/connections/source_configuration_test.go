package connections

import (
	"context"
	"errors"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
)

func TestSourceConfigurationAtomicAndLifecycleAware(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	store := assessment.Store{DB: s.DB}
	if _, err := store.SaveScope(ctx, a, "svc", 0, assessment.Scope{Description: "Documents"}); err != nil {
		t.Fatal(err)
	}
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "connection", Provider: Documents, Name: "Policies", Mode: Manual, Configuration: Configuration{AccountID: "docs.example"}})
	if err != nil {
		t.Fatal(err)
	}
	in := SourceConfigurationInput{ServiceID: "svc", SourceID: "policy", ConnectionID: d.Connection.ID, Generation: 1, IdempotencyKey: "source", Resource: ResourceKey{Provider: Documents, Tenant: "docs.example", Kind: "document", ID: "https://docs.example/policy"}, Source: assessment.EvidenceSource{Type: "document", Name: "Policy", Locator: "https://docs.example/policy", Enabled: true, FreshnessHours: 48, ScopeRevision: 2}}
	if _, err = s.ConfigureSource(ctx, a, in); !errors.Is(err, assessment.ErrConflict) {
		t.Fatalf("stale scope: %v", err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_source_bindings`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed creation retained binding")
	}
	in.Source.ScopeRevision = 1
	created, err := s.ConfigureSource(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.ConfigureSource(ctx, a, in)
	if err != nil || replay.Revision != created.Revision {
		t.Fatal("source retry created revision")
	}
	v, err := assessment.Decode[assessment.EvidenceSource](created)
	if err != nil {
		t.Fatal(err)
	}
	v.Locator = "https://docs.example/other"
	if _, err = store.SaveEvidenceSource(ctx, a, "svc", "policy", 1, v); !errors.Is(err, assessment.ErrConflict) {
		t.Fatal("legacy edit changed bound identity")
	}
	if _, err = s.ChangeSource(ctx, a, SourceOperationInput{ServiceID: "svc", SourceID: "policy", ExpectedRevision: 1, ExpectedBindingRevision: 1, IdempotencyKey: "retire"}, "retire"); err != nil {
		t.Fatal(err)
	}
	v.Locator = "https://docs.example/policy"
	if _, err = store.SaveEvidenceSource(ctx, a, "svc", "policy", 2, v); !errors.Is(err, assessment.ErrConflict) {
		t.Fatal("legacy edit revived retired source")
	}
}
