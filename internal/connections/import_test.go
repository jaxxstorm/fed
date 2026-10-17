package connections

import (
	"context"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestImportPreservesHistoricalRecordsAndReplays(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	store := assessment.Store{DB: s.DB}
	scope := assessment.Scope{Description: "Test", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}, Clusters: []string{"prod"}}}, Repositories: []assessment.Repository{{Name: "example/one", Commit: "1111111111111111111111111111111111111111", Paths: []string{"."}}, {Name: "example/two", Commit: "2222222222222222222222222222222222222222", Paths: []string{"."}}}}
	if _, err := store.SaveScope(ctx, a, "svc", 0, scope); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	hash := func() string {
		t.Helper()
		var hash string
		if err := s.DB.QueryRow(ctx, `SELECT encode(sha256(convert_to(string_agg(row_to_json(r)::text,E'\n' ORDER BY kind,id,revision),'UTF8')),'hex') FROM records r WHERE service_id='svc'`).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		return hash
	}
	before := hash()
	bindings := EnvironmentBindings{AWSProfiles: map[string]string{"123456789012": "prod"}, KubernetesContexts: map[string]string{"123456789012/us-east-1/prod": "prod-context"}}
	count, err := s.ImportSources(ctx, a, "svc", bindings)
	if err != nil || count != 4 {
		t.Fatalf("import: %d %v", count, err)
	}
	if before != hash() {
		t.Fatal("import rewrote versioned records")
	}
	if count, err = s.ImportSources(ctx, a, "svc", bindings); err != nil || count != 0 {
		t.Fatalf("repeat: %d %v", count, err)
	}
	connections, err := s.List(ctx, a, "svc", false)
	if err != nil || len(connections) != 3 {
		t.Fatalf("grouping: %d %v", len(connections), err)
	}
	for _, d := range connections {
		if !d.Connection.Imported || d.Access.State != NotConfigured {
			t.Fatal("import inferred access")
		}
	}
	var sourceID string
	if err = s.DB.QueryRow(ctx, `SELECT source_id FROM connection_source_bindings WHERE provider='aws'`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeSource(ctx, a, SourceOperationInput{ServiceID: "svc", SourceID: sourceID, ExpectedRevision: 1, ExpectedBindingRevision: 1, IdempotencyKey: "retire"}, "retire"); err != nil {
		t.Fatal(err)
	}
	if count, err = s.ImportSources(ctx, a, "svc", bindings); err != nil || count != 0 {
		t.Fatal("import recreated retired binding")
	}
	if err = store.ReconcileEvidenceSources(ctx, a, "svc"); err != nil {
		t.Fatal(err)
	}
	var lifecycle string
	if err = s.DB.QueryRow(ctx, `SELECT lifecycle FROM connection_source_bindings WHERE source_id=$1`, sourceID).Scan(&lifecycle); err != nil || lifecycle != "retired" {
		t.Fatal("retired source restored")
	}
}

func TestImportPreservesArtifactsFrozenExportsAndExactReferences(t *testing.T) {
	ctx:=context.Background()
	s,a:=setupService(t)
	artifacts:=storage.Files{Directory:t.TempDir()}
	store:=assessment.Store{DB:s.DB,Artifacts:artifacts}
	if _,err:=store.SaveScope(ctx,a,"svc",0,assessment.Scope{Description:"Documents"}); err!=nil { t.Fatal(err) }
	source,err:=store.SaveEvidenceSource(ctx,a,"svc","policy",0,assessment.EvidenceSource{Type:"document",Name:"Policy",Locator:"https://docs.example/policy",ScopeRevision:1,Enabled:true,FreshnessHours:48})
	if err!=nil { t.Fatal(err) }
	content:=[]byte("Approved policy\nEvery deployment requires review.\n")
	key,err:=artifacts.Put(ctx,content)
	if err!=nil { t.Fatal(err) }
	evidence,_:=json.Marshal(assessment.Evidence{Source:"agent",Description:"Policy",ArtifactKey:key,ScopeRevision:1,Verification:"agent_observation",Document:&assessment.DocumentaryProvenance{Source:assessment.Ref{ID:source.ID,Revision:source.Revision},Locator:"https://docs.example/policy",SourceVersion:"approved-v1"}})
	snapshot,_:=json.Marshal(map[string]any{"format":"assessment/v1","service_id":"svc","records":[]assessment.Record{source,{ServiceID:"svc",Kind:"evidence",ID:"historical",Revision:1,Data:evidence}},"citations":[]map[string]any{{"evidence_id":"historical","revision":1,"start_line":2,"end_line":2}}})
	for kind,raw:=range map[string][]byte{"evidence":evidence,"snapshot":snapshot} {
		if _,err=s.DB.Exec(ctx,`INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES('svc',$1,'historical',1,$2,$3)`,kind,a.ID,raw); err!=nil { t.Fatal(err) }
	}
	before:=map[string][]byte{}
	for _,format:=range []string{"json","html"} { data,_,err:=store.Export(ctx,a,"svc","historical",format); if err!=nil { t.Fatal(err) }; before[format]=data }
	var originalDigest string
	const digest=`SELECT encode(sha256(convert_to(string_agg(row_to_json(r)::text,E'\n' ORDER BY kind,id,revision),'UTF8')),'hex') FROM records r WHERE service_id='svc'`
	if err=s.DB.QueryRow(ctx,digest).Scan(&originalDigest); err!=nil { t.Fatal(err) }
	if _,err=s.ImportSources(ctx,a,"svc",EnvironmentBindings{}); err!=nil { t.Fatal(err) }
	var importedDigest string
	if err=s.DB.QueryRow(ctx,digest).Scan(&importedDigest); err!=nil || importedDigest!=originalDigest { t.Fatal("import rewrote historical payloads",err) }
	for _,format:=range []string{"json","html"} { data,_,err:=store.Export(ctx,a,"svc","historical",format); if err!=nil || !bytes.Equal(data,before[format]) { t.Fatal("frozen export changed",format,err) } }
	retained,err:=artifacts.Get(ctx,key)
	if err!=nil || !bytes.Equal(retained,content) { t.Fatal("artifact bytes changed",err) }
}
