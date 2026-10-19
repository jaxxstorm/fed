package readiness

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestCandidatesSeparateFreshnessDeclarationsAndServiceScope(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('service',$1,'Test'),('other',$1,'Other')`, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, item := range []struct {
		service, kind, id string
		data              any
	}{
		{"service", "evidence_source", "source", map[string]any{}},
		{"service", "evidence", "old", map[string]any{"scope_revision": 1, "observed_at": now.Add(-48 * time.Hour), "document": map[string]any{"source": EvidenceRef{ID: "source", Revision: 1}, "freshness_hours": 24}}},
		{"service", "evidence", "current", map[string]any{"scope_revision": 1, "observed_at": now, "document": map[string]any{"source": EvidenceRef{ID: "source", Revision: 1}, "freshness_hours": 24}}},
		{"other", "evidence", "foreign", map[string]any{"scope_revision": 1, "observed_at": now}},
	} {
		raw, _ := json.Marshal(item.data)
		if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES($1,$2,$3,1,$4,$5)`, item.service, item.kind, item.id, a.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	binding := WorkBinding{ScopeRevision: 1}
	plan := ServicePlan{ServiceID: "service", Binding: binding, Capabilities: []CapabilityReadiness{{Implementation: "declared_present", Review: "not_assessed", Declaration: &WorkRecord{Binding: binding, Data: WorkData{Evidence: []EvidenceRef{{ID: "old", Revision: 1}, {ID: "current", Revision: 1}, {ID: "foreign", Revision: 1}}}}}}}
	if err = projectCandidates(ctx, db, &plan, now); err != nil {
		t.Fatal(err)
	}
	n := plan.Capabilities[0]
	if n.Implementation != "declared_present" || n.Review != "not_assessed" || n.EvidenceState != "partly_stale" || len(n.Evidence) != 3 {
		t.Fatal("freshness changed implementation/review or lost references", n)
	}
	if n.Evidence[0].State != "stale" || n.Evidence[1].State != "available_for_assessment" || n.Evidence[2].State != "unavailable" {
		t.Fatal("candidate freshness or service isolation wrong", n.Evidence)
	}
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('service','evidence_source','source',2,$1,'{}')`, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = projectCandidates(ctx, db, &plan, now); err != nil {
		t.Fatal(err)
	}
	if plan.Capabilities[0].EvidenceState != "requires_reassessment" {
		t.Fatal("changed source policy reused as current", plan)
	}
}
