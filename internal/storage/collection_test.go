package storage_test

import (
	"context"
	"testing"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestCollectionMigration(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('one',$1,'One'),('two',$1,'Two')`, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('one','scope','scope',1,$1,'{"description":"existing"}')`, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var description string
	if err = db.QueryRow(ctx, `SELECT data->>'description' FROM records WHERE service_id='one'`).Scan(&description); err != nil || description != "existing" {
		t.Fatalf("existing record: %s %v", description, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO observations(service_id,id,scope_revision,profile_revision,actor_id,source_kind,artifact_hash,observed_at,target_key,data) VALUES('one','observation',1,1,$1,'agent_observation',$2,now(),'target','{}')`, a.ID, storage.Hash([]byte("artifact"))); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO observation_links(service_id,observation_id,requirement_id,measure_id,measure_revision,evidence_id,data) VALUES('two','observation','rule','measure',1,'evidence','{}')`); err == nil {
		t.Fatal("cross-service observation link accepted")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO collection_plans(service_id,revision,scope_revision,profile_revision,template_version,data) VALUES('one',1,1,1,'v1','{}')`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM collection_plans`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rollback: %d %v", n, err)
	}
}
