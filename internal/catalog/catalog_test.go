package catalog

import (
	"context"
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/testutil"
)

func fixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	p, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return p, s
}
func TestOfficialApplicability(t *testing.T) {
	payload, schema := fixture(t)
	s, err := Parse(payload, schema, PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ class, id, want string }{
		{"A", "KSI-SVC-SIN", "required"}, {"A", "CDS-CSO-UTC", "required"},
		{"A", "KSI-SVC-ACM", "not_applicable"}, {"B", "KSI-SVC-PRR", "optional"},
		{"C", "KSI-SVC-PRR", "required"}, {"D", "KSI-SVC-PRR", "unresolved"},
		{"B", "FRC-CSX-VVK", "recommended"}, {"C", "FRC-CSX-VVK", "required"},
		{"D", "SDR-CSX-KMT", "unresolved"},
	} {
		t.Run(tt.class+tt.id, func(t *testing.T) {
			out, err := Resolve(s.Requirements, Profile{CatalogID: s.ID, Type: "20x", Path: "Program", Class: tt.class, Date: "2026-10-16", Context: "initial"})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range out {
				if r.Requirement.ID == tt.id {
					if r.Applicability != tt.want {
						t.Fatalf("want %s, got %s (%s)", tt.want, r.Applicability, r.Reason)
					}
					return
				}
			}
			t.Fatal("missing rule")
		})
	}
	p := Profile{CatalogID: s.ID, Type: "20x", Path: "Program", Class: "C", Date: "2026-10-16", Context: "ongoing"}
	out, err := Resolve(s.Requirements, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out {
		if r.Requirement.ID == "FRC-CSX-VVK" && r.Applicability != "optional" {
			t.Fatalf("ongoing date: %+v", r)
		}
	}
	p.Date = "2027-02-01"
	out, err = Resolve(s.Requirements, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out {
		if r.Requirement.ID == "FRC-CSX-VVK" && r.Applicability != "unresolved" {
			t.Fatal("assessment-dependent grace ignored")
		}
	}
}

func TestAtomicCatalogImport(t *testing.T) {
	db := testutil.Database(t)
	p, s := fixture(t)
	ctx := context.Background()
	id, err := Import(ctx, db, p, s, PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Import(ctx, db, []byte(`{"info":{}}`), s, PinnedCommit); err == nil {
		t.Fatal("invalid import accepted")
	}
	var active string
	if err = db.QueryRow(ctx, `SELECT catalog_id FROM catalog_activation`).Scan(&active); err != nil || active != id {
		t.Fatalf("activation changed: %s %v", active, err)
	}
	loaded, err := Load(ctx, db, id)
	if err != nil || len(loaded) < 100 {
		t.Fatalf("catalog incomplete: %d %v", len(loaded), err)
	}
	if _, err = Import(ctx, db, p, s, PinnedCommit); err != nil {
		t.Fatal(err)
	}
}
