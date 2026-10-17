package assessment

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestCollectionReconciliation(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	s := Store{DB: db}
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := s.CreateService(ctx, a, "Test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	id, err := catalog.Import(ctx, db, p, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if id != MappedCatalog {
		t.Fatal("template catalog does not match fixture")
	}
	profile := catalog.Profile{CatalogID: id, Type: "20x", Path: "Program", Class: "C", Date: "2026-10-16", Context: "initial"}
	if _, err = s.SaveProfile(ctx, a, svc.ID, 0, profile); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Description: "East only", Accounts: []Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}
	if _, err = s.SaveScope(ctx, a, svc.ID, 0, scope); err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReconcileCollection(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var rds CollectionEntry
	manual := false
	for _, e := range plan.Entries {
		if e.CheckID == "aws.rds.configuration" && e.RequirementID == "KSI-RPL-ABO" {
			rds = e
		}
		if e.RequirementID == "AFC-CSO-INB" && e.State == "manual_evidence" {
			manual = true
		}
	}
	if rds.Measure.Revision != 1 || rds.State != "policy_required" || !manual {
		t.Fatalf("missing generated mapping or eligibility: %+v manual=%v", rds, manual)
	}
	again, err := s.ReconcileCollection(ctx, svc.ID)
	if err != nil || again.Revision != plan.Revision {
		t.Fatal("non-idempotent reconciliation", err)
	}
	mr, err := s.Get(ctx, a, svc.ID, "measure", rds.Measure.ID)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Decode[Measure](mr)
	if err != nil {
		t.Fatal(err)
	}
	m.FreshnessHours = 72
	m.Rationale = "Customer override"
	if _, err = s.SaveMeasure(ctx, a, svc.ID, mr.ID, mr.RequirementID, mr.Revision, m); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveScope(ctx, a, svc.ID, 1, scope); err != nil {
		t.Fatal(err)
	}
	plan, err = s.ReconcileCollection(ctx, svc.ID)
	if err != nil || plan.Revision != 2 {
		t.Fatal("new scope not reconciled", err)
	}
	mr, err = s.Get(ctx, a, svc.ID, "measure", mr.ID)
	if err != nil {
		t.Fatal(err)
	}
	m, err = Decode[Measure](mr)
	if err != nil || m.FreshnessHours != 72 || m.Rationale != "Customer override" {
		t.Fatal("override lost", err)
	}
	before := plan
	c, err := s.CollectionConfiguration(ctx, a, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.FreshnessHours = 96
	c.Overrides = map[string]json.RawMessage{"minimum_retention_days": json.RawMessage(`7`)}
	updated, err := s.ConfigureCollection(ctx, a, svc.ID, c.Revision, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfigureCollection(ctx, a, svc.ID, c.Revision, c); err != ErrConflict {
		t.Fatalf("stale configuration accepted: %v", err)
	}
	plan, err = s.ReconcileCollection(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ConfigurationRevision != updated.Revision || plan.Revision == before.Revision {
		t.Fatal("configuration reused old plan")
	}
	var oldPlan []byte
	if err = db.QueryRow(ctx, `SELECT data FROM collection_plans WHERE service_id=$1 AND revision=$2`, svc.ID, before.Revision).Scan(&oldPlan); err != nil {
		t.Fatal(err)
	}
	old, err := Decode[CollectionPlan](Record{Data: oldPlan})
	if err != nil || old.ConfigurationRevision != before.ConfigurationRevision {
		t.Fatal("old plan mutated", err)
	}
	for _, entry := range plan.Entries {
		if entry.CheckID == "" {
			continue
		}
		rec, e := s.Get(ctx, a, svc.ID, "measure", entry.Measure.ID)
		if e != nil {
			t.Fatal(e)
		}
		measure, e := Decode[Measure](rec)
		if e != nil {
			t.Fatal(e)
		}
		if entry.Measure.ID == rds.Measure.ID {
			if measure.FreshnessHours != 72 {
				t.Fatal("custom freshness lost")
			}
		} else if measure.FreshnessHours != 96 || measure.ConfigurationRevision != updated.Revision {
			t.Fatal("defaults did not follow configuration")
		}
	}
	var history int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM collection_configurations WHERE service_id=$1`, svc.ID).Scan(&history); err != nil || history < 3 {
		t.Fatal("configuration history missing", err)
	}
	profile.CatalogID = "unmapped"
	if _, err = db.Exec(ctx, `INSERT INTO catalogs(id,commit_sha,source_url,payload_hash,schema_hash,payload,schema_payload) SELECT 'unmapped',commit_sha,source_url,payload_hash,schema_hash,payload,schema_payload FROM catalogs LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO requirements(catalog_id,id,kind,data) SELECT 'unmapped',id,kind,data FROM requirements WHERE catalog_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, a, svc.ID, 1, profile); err != nil {
		t.Fatal(err)
	}
	plan, err = s.ReconcileCollection(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range plan.Entries {
		if e.CheckID != "" {
			t.Fatal("unrecognized catalog guessed a mapping")
		}
	}
}
