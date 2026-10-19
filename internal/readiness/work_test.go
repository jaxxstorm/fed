package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestScopedWorkHistoryAndAuthorization(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('service',$1,'Test'),('other',$1,'Other')`, admin.OrganizationID); err != nil {
		t.Fatal(err)
	}
	p, snapshot, rules, markdown := sourceFixture(t)
	for i := range p.Playbooks {
		book := &p.Playbooks[i]
		if book.Kind != "actionable" {
			continue
		}
		timing := book.Purpose
		book.Operate = Steps{Items: []Step{
			{ID: "event-duty", Action: book.Purpose, OwnerRole: "Operations", Trigger: "event", Timing: &timing, CompletionCriteria: []Statement{book.Purpose}},
			{ID: "recurring-duty", Action: book.Purpose, OwnerRole: "Operations", Trigger: "recurring", Timing: &timing, CompletionCriteria: []Statement{book.Purpose}},
		}}
	}
	if _, err = catalog.Import(ctx, db, snapshot.Payload, snapshot.Schema, snapshot.Commit); err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db, Rules: rules, Markdown: markdown}
	for _, b := range p.Playbooks {
		if _, err = s.RecordContentReview(ctx, admin, "service", p, b.ID, comparisonFixture()); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := s.Publish(ctx, admin, "service", p)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := json.Marshal(catalog.Profile{CatalogID: p.CatalogID, Type: "20x", Path: "Program", Class: "C", Context: "initial", Date: "2026-10-18"})
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('service','profile','profile',1,$1,$2),('service','scope','scope',1,$1,'{"accounts":[{"id":"production","regions":["us-east-1"]}],"repositories":[]}')`, admin.ID, profile); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, admin, "service", []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	a, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	in := WorkChange{ServiceID: "service", IdempotencyKey: "first", Binding: WorkBinding{CatalogID: p.CatalogID, ReleaseRevision: 1, ReleaseHash: hash, ProfileRevision: 1, ScopeRevision: 1}, Data: WorkData{CapabilityID: "monitoring", Scope: ScopeKey{Kind: "service"}, OwnerRole: "Security operations", Implementation: "declared_present", Trigger: "unknown", Decisions: map[string]string{}, Evidence: []EvidenceRef{}}}
	r, err := s.SaveWork(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Revision != 1 || r.ActorID != a.ID {
		t.Fatal("wrong work identity", r)
	}
	retry, err := s.SaveWork(ctx, a, in)
	if err != nil || retry.Revision != r.Revision || !retry.CreatedAt.Equal(r.CreatedAt) {
		t.Fatal("retry changed history", retry, err)
	}
	changed := in
	changed.Data.Notes = "Different retry"
	if _, err = s.SaveWork(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed idempotent request accepted", err)
	}
	changed = in
	changed.IdempotencyKey = "second"
	if _, err = s.SaveWork(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("stale revision accepted", err)
	}
	changed.ExpectedRevision = 1
	changed.Data.Implementation = "declared_missing"
	if _, err = s.SaveWork(ctx, a, changed); err != nil {
		t.Fatal(err)
	}
	items, err := s.Work(ctx, a, "service", 100, 0)
	if err != nil || len(items) != 1 || items[0].Revision != 2 {
		t.Fatal("latest work missing", items, err)
	}
	quality, err := s.Quality(ctx, a, "service", 1, 0)
	if err != nil || !quality.Complete || len(quality.Entries) != 1 || quality.ReleaseHash != hash || quality.Counts["reviewed"] != quality.VariantCount {
		t.Fatal("published coverage missing", quality, err)
	}
	if _, err = s.Quality(ctx, a, "other", 1, 0); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("coverage leaked across services", err)
	}
	if _, err = s.Quality(ctx, a, "service", 101, 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("coverage limit unbounded", err)
	}
	emptyQuality, err := s.Quality(ctx, a, "service", 100, int(^uint(0)>>1))
	if err != nil || len(emptyQuality.Entries) != 0 || emptyQuality.NextOffset != nil {
		t.Fatal("large offset not safely bounded", emptyQuality, err)
	}
	unavailable := s
	unavailable.Markdown = nil
	staleQuality, err := unavailable.Quality(ctx, a, "service", 100, 0)
	if err != nil || staleQuality.Complete || staleQuality.Counts["needs_revalidation"] != staleQuality.VariantCount {
		t.Fatal("missing official corpus accepted", staleQuality, err)
	}
	plan, err := s.Plan(ctx, a, "service", 1, 0)
	if err != nil || plan.Total != 1 || len(plan.Capabilities) != 1 || plan.Capabilities[0].Implementation != "declared_missing" {
		t.Fatal("stored plan projection differs", plan, err)
	}
	if plan.Capabilities[0].Review != "not_assessed" || plan.CountUnit != "scoped capability instances" {
		t.Fatal("readiness count conflated with compliance", plan)
	}
	duties, err := s.Plan(ctx, a, "service", 100, 0)
	if err != nil || duties.DutyCount != 2 || len(duties.Duties) != 2 {
		t.Fatal("operating duties omitted", duties, err)
	}
	for _, d := range duties.Duties {
		if d.Trigger != "unknown" || d.EvidenceState != "missing" || d.Review != "not_assessed" {
			t.Fatal("unrecorded duty inferred complete", d)
		}
		change := in
		change.IdempotencyKey = d.ID
		change.Data.CapabilityID = ""
		change.Data.Duty = &d.Duty
		change.Data.Implementation = "unknown"
		change.Data.Trigger = "not_triggered"
		if d.Step.Trigger == "recurring" {
			if _, err = s.SaveWork(ctx, a, change); !errors.Is(err, ErrInvalid) {
				t.Fatal("recurring duty accepted no-event exemption", err)
			}
			change.Data.Trigger = "due"
		}
		receipt, e := s.SaveWork(ctx, a, change)
		if e != nil || receipt.ID != d.ID {
			t.Fatal("duty write failed", receipt, e)
		}
	}
	duties, err = s.Plan(ctx, a, "service", 100, 0)
	if err != nil || duties.Duties[0].Trigger != "due" || duties.Duties[1].Trigger != "not_triggered" || duties.Duties[1].Review != "not_assessed" || duties.Duties[1].EvidenceState != "missing" {
		t.Fatal("event declaration became completion or due order lost", duties, err)
	}
	detail, err := s.Capability(ctx, a, "service", plan.Capabilities[0].ID, 1, 0)
	if err != nil || detail.Total != 1 || len(detail.Capabilities) != 1 || detail.Capabilities[0].ID != plan.Capabilities[0].ID || detail.Binding != plan.Binding {
		t.Fatal("capability detail differs from shared plan", detail, err)
	}
	if _, err = s.Capability(ctx, a, "service", "unknown", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown capability not rejected", err)
	}
	if _, err = s.Capability(ctx, a, "other", plan.Capabilities[0].ID, 1, 0); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("capability detail leaked across services", err)
	}
	empty, err := s.Plan(ctx, a, "service", 1, 1)
	if err != nil || len(empty.Capabilities) != 0 {
		t.Fatal("plan pagination failed", empty, err)
	}
	foreign := in
	foreign.ServiceID = "other"
	if _, err = s.SaveWork(ctx, a, foreign); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("agent wrote foreign service", err)
	}
	if _, err = s.Work(ctx, a, "other", 100, 0); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("agent read foreign service", err)
	}
	var assessments int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind IN ('assessment','review')`).Scan(&assessments); err != nil || assessments != 0 {
		t.Fatal("declaration changed compliance", assessments, err)
	}
	if _, err = db.Exec(ctx, `UPDATE readiness_work SET data='{}'`); err == nil {
		t.Fatal("historical declaration mutable")
	}
	changed.IdempotencyKey = "third"
	changed.ExpectedRevision = 2
	changed.Binding.ScopeRevision = 2
	if _, err = s.SaveWork(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("stale scope binding accepted", err)
	}
	changed.Binding.ScopeRevision = 1
	changed.Data.AssigneeID = "unknown-user"
	if _, err = s.SaveWork(ctx, a, changed); !errors.Is(err, ErrInvalid) {
		t.Fatal("ineligible owner accepted", err)
	}
	if err = auth.RevokeToken(ctx, db, admin, "service", a.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveWork(ctx, a, in); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("revoked token replayed write", err)
	}
}

func TestScopeKeysDoNotWidenBoundary(t *testing.T) {
	raw := []byte(`{"accounts":[{"id":"production","regions":["us-east-1"],"clusters":["prod"]}],"kubernetes":[{"cluster_id":"dev","cluster_wide":false,"namespaces":[{"uid":"ns-1"}]}],"repositories":[{"name":"org/repo"}]}`)
	for _, key := range []ScopeKey{{Kind: "service"}, {Kind: "account", AccountID: "production", Region: "us-east-1"}, {Kind: "cluster", AccountID: "production", Region: "us-east-1", ClusterID: "prod"}, {Kind: "namespace", ClusterID: "dev", NamespaceUID: "ns-1"}, {Kind: "repository", Repository: "org/repo"}} {
		if !MatchScope(key, raw) {
			t.Fatal("saved scope rejected", key)
		}
	}
	for _, key := range []ScopeKey{{Kind: "cluster", ClusterID: "dev"}, {Kind: "namespace", ClusterID: "prod", NamespaceUID: "ns-1"}, {Kind: "namespace", ClusterID: "dev", NamespaceUID: "recreated"}, {Kind: "account", AccountID: "production", Region: "us-west-2"}, {Kind: "service", AccountID: "production"}, {Kind: "repository", Repository: "other/repo"}} {
		if MatchScope(key, raw) {
			t.Fatal("scope widened", key)
		}
	}
	prod := ScopeKey{Kind: "cluster", ClusterID: "production"}
	dev := ScopeKey{Kind: "cluster", ClusterID: "development"}
	if InstanceID("s", "c", prod) == InstanceID("s", "c", dev) || InstanceID("s", "c", prod) == InstanceID("other", "c", prod) {
		t.Fatal("scope/service identity collapsed")
	}
}
