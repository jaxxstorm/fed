package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/testutil"
)

// Populate every Statement field by reflection, including nested steps,
// alternatives and dependencies. New model fields must not silently disappear
// from review hashes when the production typed walker is updated.
func TestStatementReferencesIncludeEveryAuthoredField(t *testing.T) {
	var book Playbook
	capabilities := []Capability{{}}
	want := map[string]bool{}
	var populate func(reflect.Value)
	populate = func(v reflect.Value) {
		if v.Type() == reflect.TypeOf(Statement{}) {
			id := fmt.Sprintf("citation-%d", len(want))
			want[id] = true
			v.Set(reflect.ValueOf(Statement{Citations: []string{id}}))
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).Name != "Review" {
					populate(v.Field(i))
				}
			}
		case reflect.Pointer:
			v.Set(reflect.New(v.Type().Elem()))
			populate(v.Elem())
		case reflect.Slice:
			v.Set(reflect.MakeSlice(v.Type(), 1, 1))
			populate(v.Index(0))
		}
	}
	populate(reflect.ValueOf(&book).Elem())
	populate(reflect.ValueOf(&capabilities).Elem())
	book.Review = &Review{Comparison: SourceComparison{Evidence: ReviewFinding{Citations: []string{"review-only"}}}}
	if got := statementReferences(book, capabilities); !reflect.DeepEqual(got, want) {
		t.Fatalf("authored citation set changed: got %v want %v", got, want)
	}
}

type countingSourceFS struct {
	fs.FS
	reads map[string]int
}

func (f countingSourceFS) ReadFile(name string) ([]byte, error) {
	f.reads[name]++
	return fs.ReadFile(f.FS, name)
}

func TestSourceVerificationBatchesReadsButRechecksNextOperation(t *testing.T) {
	p, _, rules, markdown := sourceFixture(t)
	r := countingSourceFS{rules, map[string]int{}}
	m := countingSourceFS{markdown, map[string]int{}}
	v := newSourceVerifier(r, m)
	for range 3 {
		if err := v.verify(p); err != nil {
			t.Fatal(err)
		}
	}
	if r.reads["rules.json"] != 1 || m.reads["_sources.json"] != 1 {
		t.Fatal("source files reread per citation/playbook", r.reads, m.reads)
	}
	// Same citation ID with different content cannot reuse a previous success.
	changed := p
	changed.Citations = append([]Citation(nil), p.Citations...)
	changed.Citations[0].ContentHash = "changed"
	if err := v.verify(changed); err == nil {
		t.Fatal("citation identity bypassed content validation")
	}
	rules["rules.json"] = &fstest.MapFile{Data: []byte(`{"changed":true}`)}
	if err := VerifySources(p, r, m); err == nil {
		t.Fatal("next operation reused stale source validation")
	}
	delete(markdown, "_sources.json")
	if err := VerifySources(p, r, m); err == nil {
		t.Fatal("missing provenance accepted")
	}
}

func TestContextSourceBatchIsRequestLocal(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "source-batch-owner")
	if err != nil {
		t.Fatal(err)
	}
	const service = "source-batch"
	if _, err := db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES($1,$2,'Source batch')`, service, admin.OrganizationID); err != nil {
		t.Fatal(err)
	}
	p, snapshot, rules, markdown := sourceFixture(t)
	if _, err := catalog.Import(ctx, db, snapshot.Payload, snapshot.Schema, snapshot.Commit); err != nil {
		t.Fatal(err)
	}
	r := countingSourceFS{rules, map[string]int{}}
	m := countingSourceFS{markdown, map[string]int{}}
	s := Store{DB: db, Rules: r, Markdown: m}
	for _, book := range p.Playbooks {
		if _, err := s.RecordContentReview(ctx, admin, service, p, book.ID, comparisonFixture()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Publish(ctx, admin, service, p); err != nil {
		t.Fatal(err)
	}
	profile := catalog.Profile{CatalogID: p.CatalogID, Type: "20x", Path: "Program", Class: "C", Context: "initial", Date: "2026-10-19"}
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES($1,'profile','profile',1,$2,$3),($1,'scope','scope',1,$2,'{}')`, service, admin.ID, raw); err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.Resolve(snapshot.Requirements, profile)
	if err != nil {
		t.Fatal(err)
	}
	clear(r.reads)
	clear(m.reads)
	batch, err := s.Contexts(ctx, db, admin, service, p.CatalogID, append(resolved, resolved...), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if r.reads["rules.json"] != 1 || m.reads["_sources.json"] != 1 {
		t.Fatal("context projection rereads sources per card", r.reads, m.reads)
	}
	for _, g := range batch {
		if g.State != "reviewed" || g.Progress == nil {
			t.Fatal("reviewed progress missing", g)
		}
	}
	// The next request must invalidate the previously reviewed projection even
	// though its release and authenticated review receipts have not changed.
	rules["rules.json"] = &fstest.MapFile{Data: []byte(`{"changed":true}`)}
	stale, err := s.Context(ctx, db, admin, service, p.CatalogID, resolved[0], time.Now().UTC())
	if err != nil || stale.State != "needs_revalidation" || stale.Progress != nil || stale.Guidance.Playbooks[0].Review == nil {
		t.Fatal("stale sources reused or review attribution lost", stale, err)
	}
}
