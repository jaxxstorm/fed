package readiness

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/testutil"
)

type countingContextReader struct {
	*pgxpool.Pool
	workReads, evidenceReads int
}

func (q *countingContextReader) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM readiness_work") {
		q.workReads++
	}
	if strings.Contains(sql, "unnest($2::text[]") {
		q.evidenceReads++
	}
	return q.Pool.Query(ctx, sql, args...)
}

func TestRequirementProgressBatchedScopedAndSourceFenced(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "context-owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('service',$1,'Context'),('other',$1,'Other')`, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	p, snapshot, rules, markdown := sourceFixture(t)
	if _, err = catalog.Import(ctx, db, snapshot.Payload, snapshot.Schema, snapshot.Commit); err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db, Rules: rules, Markdown: markdown}
	for _, book := range p.Playbooks {
		if _, err = s.RecordContentReview(ctx, a, "service", p, book.ID, comparisonFixture()); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := s.Publish(ctx, a, "service", p)
	if err != nil {
		t.Fatal(err)
	}
	profile := catalog.Profile{CatalogID: p.CatalogID, Type: "20x", Path: "Program", Class: "C", Context: "initial", Date: "2026-10-18"}
	raw, _ := json.Marshal(profile)
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('service','profile','profile',1,$1,$2),('service','scope','scope',1,$1,'{}'),('service','evidence_source','source',1,$1,'{}'),('service','evidence','proof',1,$1,jsonb_build_object('scope_revision',1,'observed_at',$3::text,'document',jsonb_build_object('source',jsonb_build_object('id','source','revision',1),'freshness_hours',24)))`, a.ID, raw, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	binding := WorkBinding{CatalogID: p.CatalogID, ReleaseRevision: 1, ReleaseHash: hash, ProfileRevision: 1, ScopeRevision: 1}
	in := WorkChange{ServiceID: "service", IdempotencyKey: "context-work", Binding: binding, Data: WorkData{CapabilityID: "monitoring", Scope: ScopeKey{Kind: "service"}, OwnerRole: "Security operations", Implementation: "declared_present", Trigger: "unknown", Evidence: []EvidenceRef{{ID: "proof", Revision: 1}}}}
	if _, err = s.SaveWork(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.Resolve(snapshot.Requirements, profile)
	if err != nil {
		t.Fatal(err)
	}
	q := &countingContextReader{Pool: db}
	batch, err := s.Contexts(ctx, q, a, "service", p.CatalogID, resolved, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if q.workReads != 1 || q.evidenceReads != 1 {
		t.Fatalf("unbatched reads: %+v", q)
	}
	var selected int
	for i, g := range batch {
		if g.Progress != nil && len(g.Progress.Capabilities) > 0 {
			selected = i
			break
		}
	}
	g := batch[selected]
	if g.Progress == nil || len(g.Progress.Capabilities) == 0 {
		t.Fatal("scoped progress missing", batch)
	}
	c := g.Progress.Capabilities[0]
	if g.Progress.Binding != binding || c.Implementation != "declared_present" || c.EvidenceState != "available_for_assessment" || c.Review != "not_assessed" || len(c.Evidence) != 1 || c.Evidence[0].Reference.Revision != 1 {
		t.Fatal("declaration became proof or lost exact binding", g.Progress)
	}
	one, err := s.Context(ctx, db, a, "service", p.CatalogID, resolved[selected], time.Now().UTC())
	if err != nil || !reflect.DeepEqual(g.Progress, one.Progress) {
		t.Fatal("single/batch context diverged", err, g.Progress, one.Progress)
	}
	many := append(append([]catalog.Resolved{}, resolved...), resolved...)
	q.workReads, q.evidenceReads = 0, 0
	if _, err = s.Contexts(ctx, q, a, "service", p.CatalogID, many, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if q.workReads != 1 || q.evidenceReads != 1 {
		t.Fatal("per-card database fanout", q.workReads, q.evidenceReads)
	}
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('service','evidence_source','source',2,$1,'{}')`, a.ID); err != nil {
		t.Fatal(err)
	}
	stale, err := s.Context(ctx, db, a, "service", p.CatalogID, resolved[selected], time.Now().UTC())
	if err != nil || stale.Progress.Capabilities[0].Evidence[0].State != "stale" {
		t.Fatal("source revision fence lost", err, stale)
	}
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('service','scope','scope',2,$1,'{}')`, a.ID); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Context(ctx, db, a, "service", p.CatalogID, resolved[selected], time.Now().UTC())
	if err != nil || !changed.Progress.Capabilities[0].NeedsRevalidation || changed.Progress.Capabilities[0].Implementation != "unknown" {
		t.Fatal("old declaration reused across scope", err, changed)
	}
	other, err := s.Context(ctx, db, a, "other", p.CatalogID, resolved[selected], time.Now().UTC())
	if err != nil || other.Progress != nil {
		t.Fatal("work leaked between services", err, other)
	}
}
