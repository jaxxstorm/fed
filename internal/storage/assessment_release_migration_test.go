package storage

import (
	"context"
	"testing"
)

func TestAssessmentReleaseMigrationPreservesLegacyRecords(t *testing.T) {
	db := databaseBeforeMigration(t, "migrations/018_")
	ctx := context.Background()
	_, err := db.Exec(ctx, `
	INSERT INTO identities(id,organization_id,kind,issuer,subject,name)
	SELECT 'release-admin',organization_id,'human','issuer','release-subject','Admin' FROM organization_settings;
	INSERT INTO services(id,organization_id,name)
	SELECT 'release-service',organization_id,'Existing' FROM organization_settings;
	INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES
	('release-service','assessment','KSI-CNA-RNT',1,'release-admin','{"explanation":"Legacy narrative"}'),
	('release-service','review','review-1',1,'release-admin','{"assessment":{"id":"KSI-CNA-RNT","revision":1},"notes":"Legacy review"}'),
	('release-service','snapshot','snapshot-1',1,'release-admin','{"format":"ksi-assessment-bundle/v1","records":[]}');`)
	if err != nil {
		t.Fatal(err)
	}
	var before, after string
	const fingerprint = `SELECT md5(string_agg(row_to_json(r)::text,'' ORDER BY kind,id,revision)) FROM records r`
	if err = db.QueryRow(ctx, fingerprint).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, fingerprint).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("migration changed legacy assessment/review/export bytes")
	}
	for _, table := range []string{"assessment_releases", "trust_access_grants", "assessment_review_dependencies", "assessment_reporting_runs"} {
		var count int
		if err = db.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s backfilled authority or reporting state: %d %v", table, count, err)
		}
	}
	_, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES ('release-service','assessment_decision','decision',1,'release-admin','{}')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO assessment_review_dependencies(service_id,decision_id,decision_revision,dependency_kind,dependency_id,dependency_revision) VALUES ('release-service','decision',1,'assessment','KSI-CNA-RNT',2)`)
	if err == nil {
		t.Fatal("accepted nonexistent evidence/assessment revision")
	}
	_, err = db.Exec(ctx, `INSERT INTO assessment_review_dependencies(service_id,decision_id,decision_revision,dependency_kind,dependency_id,dependency_revision) VALUES ('release-service','decision',1,'assessment','KSI-CNA-RNT',1)`)
	if err != nil {
		t.Fatal(err)
	}
}
