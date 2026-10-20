package assessment

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInventoryRetirementRequiresCompleteCompatibleGeneration(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO evidence_connections(service_id,id,provider,revision,generation,lifecycle,data,actor_id) VALUES($1,'cluster','kubernetes',1,1,'active','{}',$2)`, []any{service, a.ID}},
		{`INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES($1,'evidence_source','cluster-source',1,$2,'{}')`, []any{service, a.ID}},
		{`INSERT INTO connection_source_bindings(service_id,source_id,connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision,actor_id) VALUES($1,'cluster-source','cluster','kubernetes','uid','cluster','uid','enabled',1,$2)`, []any{service, a.ID}},
	}
	for _, stmt := range statements {
		if _, err := s.DB.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	addInventory := func(id string, at time.Time, complete bool, generation, scope, profile int, keys []string) {
		t.Helper()
		if _, err := s.DB.Exec(ctx, `INSERT INTO jobs(id,service_id,actor_id,requirement_id,check_id,input,state,deadline) VALUES($1,$2,$3,'KSI-CNA-RNT','kubernetes.network-isolation','{}','succeeded',now()+interval '1 hour')`, id, service, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.Exec(ctx, `INSERT INTO resource_inventories(service_id,job_id,partition,scope_revision,profile_revision,connection_id,generation,source_id,source_revision,binding_revision,complete,target_keys,observed_at) VALUES($1,$2,'cluster/uid',$3,$4,'cluster',$5,'cluster-source',1,1,$6,$7,$8)`, service, id, scope, profile, generation, complete, keys, at); err != nil {
			t.Fatal(err)
		}
	}
	addInventory("origin", now.Add(-time.Hour), true, 1, 1, 1, []string{"old-uid"})
	if _, err := s.DB.Exec(ctx, `INSERT INTO observations(service_id,id,scope_revision,profile_revision,actor_id,source_kind,artifact_hash,observed_at,target_key,data) VALUES($1,'old',1,1,$2,'collector_observation',$3,$4,'old-uid','{"target":{"kind":"kubernetes","uid":"old-uid","resource_id":"same-name"}}')`, service, a.ID, strings.Repeat("a", 64), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO observation_inventory_bindings VALUES($1,'old','origin','cluster/uid')`, service); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'measure','inventory-measure',1,'KSI-CNA-RNT',$2,'{"freshness_hours":1,"type":"automated"}')`, service, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO observation_links(service_id,observation_id,requirement_id,measure_id,measure_revision,evidence_id,data) VALUES($1,'old','KSI-CNA-RNT','inventory-measure',1,'unused','{}')`, service); err != nil {
		t.Fatal(err)
	}
	check := func(retained bool) {
		t.Helper()
		var exists bool
		if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM current_resource_observations WHERE service_id=$1 AND id='old')`, service).Scan(&exists); err != nil || exists != retained {
			t.Fatalf("retention=%v want %v: %v", exists, retained, err)
		}
		uncached, err := s.observationStatus(ctx, service, "KSI-CNA-RNT", now.Add(90*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		view := *s
		view.presentationCache, err = s.loadPresentationCache(ctx, service, a.ID, now.Add(90*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		cached, err := view.observationStatus(ctx, service, "KSI-CNA-RNT", now.Add(90*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if uncached.Inventory == nil || !reflect.DeepEqual(uncached.Inventory, cached.Inventory) {
			t.Fatalf("inventory projection divergence: %+v %+v", uncached.Inventory, cached.Inventory)
		}
		if retained && uncached.Inventory.CurrentResources != 1 || !retained && uncached.Inventory.RetiredResources != 1 {
			t.Fatalf("resource counts: %+v", uncached.Inventory)
		}
		if retained && uncached.Freshness != "stale" {
			t.Fatalf("stale evidence lost lifecycle identity: %+v", uncached)
		}
	}
	addInventory("partial", now.Add(-50*time.Minute), false, 1, 1, 1, []string{"new-uid"})
	check(true)
	addInventory("wrong-scope", now.Add(-40*time.Minute), true, 1, 2, 1, []string{})
	check(true)
	addInventory("wrong-profile", now.Add(-30*time.Minute), true, 1, 1, 2, []string{})
	check(true)
	addInventory("obsolete-generation", now.Add(-20*time.Minute), true, 2, 1, 1, []string{})
	check(true)
	addInventory("complete", now.Add(-10*time.Minute), true, 1, 1, 1, []string{"new-uid"})
	check(false)
	var count int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM observations WHERE service_id=$1`, service).Scan(&count); err != nil || count != 1 {
		t.Fatalf("immutable evidence deleted: %d %v", count, err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE evidence_connections SET generation=2 WHERE service_id=$1`, service); err != nil {
		t.Fatal(err)
	}
	check(true)
}
