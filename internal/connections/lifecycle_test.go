package connections

import (
	"context"
	"errors"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
)

func TestDeletionRetainsSourceHistoryAndSuppressesReconciliation(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	store := assessment.Store{DB: s.DB}
	scope := assessment.Scope{Description: "Test", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}
	if _, err := store.SaveScope(ctx, a, "svc", 0, scope); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	var sourceID string
	var oldJSON []byte
	if err := s.DB.QueryRow(ctx, `SELECT id,data FROM records WHERE service_id='svc' AND kind='evidence_source' AND revision=1`).Scan(&sourceID, &oldJSON); err != nil {
		t.Fatal(err)
	}
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Production", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "prod"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO connection_source_bindings(service_id,source_id,connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision,actor_id) VALUES('svc',$1,$2,'aws','123456789012','account_region','us-east-1','enabled',1,$3)`, sourceID, d.Connection.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "delete"}
	deleted, err := s.ChangeLifecycle(ctx, a, in, "delete")
	if err != nil || deleted.Connection.Lifecycle != Deleted {
		t.Fatalf("delete: %v", err)
	}
	if _, err = s.ChangeLifecycle(ctx, a, in, "delete"); err != nil {
		t.Fatal("delete replay failed")
	}
	if _, err = store.SaveScope(ctx, a, "svc", 1, scope); err != nil {
		t.Fatal(err)
	}
	if err = store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	var original []byte
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT data FROM records WHERE service_id='svc' AND kind='evidence_source' AND id=$1 AND revision=1`, sourceID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if string(original) != string(oldJSON) {
		t.Fatal("source history rewritten")
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id='svc' AND kind='evidence_source'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("retired source resurrected: %d %v", count, err)
	}
	in.ExpectedRevision, in.Generation, in.IdempotencyKey = 2, 2, "restore"
	if _, err = s.ChangeLifecycle(ctx, a, in, "restore"); err != nil {
		t.Fatal(err)
	}
	var lifecycle string
	if err = s.DB.QueryRow(ctx, `SELECT lifecycle FROM connection_source_bindings WHERE source_id=$1`, sourceID).Scan(&lifecycle); err != nil || lifecycle != "retired" {
		t.Fatal("restoring connection silently restored source")
	}
	if _, err = s.Test(ctx, a, "svc", d.Connection.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("restored connection accepted old generation")
	}
}
