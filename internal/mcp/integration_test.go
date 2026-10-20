package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/checks"
	"github.com/jaxxstorm/fed/internal/integrations"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

type collector struct{}

func (collector) Collect(context.Context, assessment.Scope, assessment.Measure) (checks.Result, error) {
	return checks.Result{Observations: []checks.Observation{{Scope: "fixture/account/region", Outcome: "pass", Data: map[string]any{"enabled": true}}}, Limitations: []string{"Fixture configuration observation"}}, nil
}
func unpack[T any](t *testing.T, result *sdk.CallToolResult) T {
	t.Helper()
	if result.IsError {
		for _, content := range result.Content {
			if text, ok := content.(*sdk.TextContent); ok {
				t.Log(text.Text)
			}
		}
		t.Fatalf("tool failed: %+v", result)
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data T `json:"data"`
	}
	if err = json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func TestAgentToHumanAssessmentFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db := testutil.Database(t)
	store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.CreateService(ctx, admin, "Ignis fixture")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateService(ctx, admin, "Other service")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	catalogID, err := catalog.Import(ctx, db, payload, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	profile := catalog.Profile{CatalogID: catalogID, Type: "20x", Path: "Program", Class: "C", Date: "2026-10-16", Context: "initial"}
	if _, err = store.SaveProfile(ctx, admin, service.ID, 0, profile); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, admin, service.ID, 0, assessment.Scope{Description: "Fixture scope", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveMeasure(ctx, admin, service.ID, "m", "KSI-SVC-ACM", 0, assessment.Measure{Name: "Configuration", Rationale: "Fixture collector", FreshnessHours: 24, ScopeRevision: 1, CheckID: "fixture"}); err != nil {
		t.Fatal(err)
	}
	runner := &checks.Runner{DB: db, Artifacts: store.Artifacts, Registry: map[string]checks.Collector{"fixture": collector{}}}
	token, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read", "edit", "check", "export"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(adapter.Handler(store, runner))
	defer server.Close()
	client := sdk.NewClient(&sdk.Implementation{Name: "test-agent", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil || len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "gather_evidence" {
		t.Fatalf("evidence prompt discovery: %v %v", prompts, err)
	}
	prompt, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: "gather_evidence", Arguments: map[string]string{"service_id": service.ID}})
	if err != nil || len(prompt.Messages) != 1 {
		t.Fatalf("evidence prompt: %v %v", prompt, err)
	}
	text, ok := prompt.Messages[0].Content.(*sdk.TextContent)
	if !ok || !strings.Contains(text.Text, service.ID) || !strings.Contains(text.Text, "ingest_observation") || !strings.Contains(text.Text, "manual_evidence_required") {
		t.Fatal("prompt omitted evidence execution workflow")
	}
	if _, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: "gather_evidence", Arguments: map[string]string{"service_id": "outside-token-scope"}}); err == nil {
		t.Fatal("cross-service evidence prompt allowed")
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 35 {
		t.Fatalf("tools: %v %v", tools, err)
	}
	call := func(name string, args any) *sdk.CallToolResult {
		t.Helper()
		v, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	page := unpack[adapter.Page](t, call("list_requirements", adapter.ListInput{ServiceID: service.ID, Kind: "KSI", Limit: 1}))
	services := unpack[[]assessment.Service](t, call("list_services", struct{}{}))
	if len(services) != 1 || services[0].ID != service.ID {
		t.Fatal("service discovery leaked another service")
	}
	savedScope := unpack[assessment.Record](t, call("get_assessment_scope", adapter.ServiceInput{ServiceID: service.ID}))
	if savedScope.Revision != 1 {
		t.Fatal("missing scope revision")
	}
	if !call("save_assessment_scope", adapter.ScopeInput{ServiceID: other.ID, Scope: assessment.Scope{Description: "unauthorized"}}).IsError {
		t.Fatal("cross-service boundary edit allowed")
	}
	if !call("save_assessment_scope", adapter.ScopeInput{ServiceID: service.ID, ExpectedRevision: 0, Scope: assessment.Scope{Description: "stale"}}).IsError {
		t.Fatal("stale scope edit allowed")
	}
	createdScope := unpack[assessment.Record](t, call("save_assessment_scope", adapter.ScopeInput{ServiceID: service.ID, ExpectedRevision: 1, Scope: assessment.Scope{Description: "Agent-populated boundary"}}))
	if createdScope.Revision != 2 || createdScope.ActorID == admin.ID {
		t.Fatal("scope attribution missing")
	}
	// Later fixture measures reference the initial scope, so restore the fixture's
	// current revision after independently exercising the agent mutation contract.
	if _, err = db.Exec(ctx, `DELETE FROM records WHERE service_id=$1 AND kind='scope' AND revision=2`, service.ID); err != nil {
		t.Fatal(err)
	}
	submission := observation.Submission{ServiceID: service.ID, ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "sdk-observation", Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:rds:us-east-1:123456789012:db:test"}, ObservedAt: time.Now().UTC(), Method: "DescribeDBInstances", ToolVersion: "aws-cli/2", Coverage: observation.Coverage{Expected: 1, Observed: 1, Complete: true, State: "observed"}, Artifact: observation.Artifact{Schema: "aws.rds/v1", Configuration: map[string]json.RawMessage{"storage_encrypted": json.RawMessage(`true`)}}, Links: []observation.Link{{RequirementID: "KSI-SVC-ACM", Measure: observation.Ref{ID: "m", Revision: 1}, MappingVersion: "agent/v1", Rationale: "Supplemental configuration observation; not drift evidence"}}, Evaluations: []observation.Evaluation{{Condition: "storage_encryption", Outcome: "pass", Explanation: "Agent reports enabled encryption"}}}
	observationReceipt := unpack[observation.Receipt](t, call("ingest_observation", submission))
	duplicate := unpack[observation.Receipt](t, call("ingest_observation", submission))
	if duplicate.ID != observationReceipt.ID || observationReceipt.SourceKind != "agent_observation" {
		t.Fatal("ingestion provenance/idempotency")
	}
	artifact := unpack[map[string]any](t, call("get_evidence_artifact", adapter.GetInput{ServiceID: service.ID, ID: observationReceipt.Evidence[0].ID}))
	if artifact["content"] == "" {
		t.Fatal("missing artifact")
	}
	outside := submission
	outside.IdempotencyKey = "outside"
	outside.Target.Region = "eu-west-2"
	if !call("ingest_observation", outside).IsError {
		t.Fatal("out-of-scope ingestion accepted")
	}
	changedSubmission := submission
	changedSubmission.Method = "different"
	if !call("ingest_observation", changedSubmission).IsError {
		t.Fatal("conflicting retry accepted")
	}
	preDraft := unpack[assessment.Status](t, call("get_requirement_status", adapter.RequirementInput{ServiceID: service.ID, RequirementID: "KSI-SVC-ACM"}))
	if preDraft.Draft != nil || preDraft.Evidence != "current" || preDraft.Review != "pending" {
		t.Fatal("agent evidence requires draft")
	}
	spoofRaw, _ := json.Marshal(submission)
	var spoof map[string]any
	_ = json.Unmarshal(spoofRaw, &spoof)
	spoof["source_kind"] = "collector_observation"
	if !call("ingest_observation", spoof).IsError {
		t.Fatal("caller-controlled provenance accepted")
	}
	preApp := &web.App{Store: store, Runner: runner}
	preBrowser := auth.Browser{DB: db, Dev: &admin}
	preView := httptest.NewRecorder()
	preBrowser.Protect(preApp.Handler()).ServeHTTP(preView, httptest.NewRequest("GET", "http://localhost/api/ui/v1/services/"+service.ID+"/pages/requirement/KSI-SVC-ACM", nil))
	if preView.Code != 200 || !strings.Contains(preView.Body.String(), "agent_observation") {
		t.Fatal("ingested observation not visible before drafting")
	}
	if page.NextOffset == nil {
		t.Fatal("pagination missing")
	}
	if !call("list_requirements", adapter.ListInput{ServiceID: other.ID}).IsError {
		t.Fatal("cross-service read allowed")
	}
	resource, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "ksi://services/" + service.ID + "/requirements/KSI-SVC-ACM"})
	if err != nil || len(resource.Contents) != 1 {
		t.Fatalf("resource: %v", err)
	}
	if _, err = session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "ksi://catalog/" + url.QueryEscape(catalogID) + "/requirements/KSI-SVC-ACM"}); err != nil {
		t.Fatal(err)
	}
	input := adapter.RunInput{ServiceID: service.ID, RequirementID: "KSI-SVC-ACM", CheckID: "fixture", IdempotencyKey: "run", Input: checks.Input{ScopeRevision: 1, Measure: assessment.Ref{ID: "m", Revision: 1}}}
	job := unpack[checks.Job](t, call("run_check", input))
	replay := unpack[checks.Job](t, call("run_check", input))
	if job.ID != replay.ID {
		t.Fatal("duplicate job")
	}
	if _, err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	job = unpack[checks.Job](t, call("get_check_run", adapter.GetInput{ServiceID: service.ID, ID: job.ID}))
	if job.Result == nil || len(job.Result.EvidenceIDs) != 1 {
		t.Fatalf("job: %+v", job)
	}
	evidence := unpack[assessment.Record](t, call("get_evidence", adapter.GetInput{ServiceID: service.ID, ID: job.Result.EvidenceIDs[0]}))
	draft := assessment.Draft{Explanation: "Agent explanation based on fixture evidence", ProfileRevision: 1, ScopeRevision: 1, Evidence: []assessment.Ref{{ID: evidence.ID, Revision: 1}}, Measures: []assessment.Ref{{ID: "m", Revision: 1}}}
	draftInput := adapter.DraftInput{ServiceID: service.ID, RequirementID: "KSI-SVC-ACM", ExpectedRevision: 0, Draft: draft}
	unpack[assessment.Record](t, call("save_assessment_draft", draftInput))
	if !call("save_assessment_draft", draftInput).IsError {
		t.Fatal("stale revision accepted")
	}
	status := unpack[assessment.Status](t, call("get_requirement_status", adapter.RequirementInput{ServiceID: service.ID, RequirementID: "KSI-SVC-ACM"}))
	if status.Review != "pending" || status.Technical != "pass" {
		t.Fatalf("status: %+v", status)
	}
	app := &web.App{Store: store, Runner: runner}
	browser := auth.Browser{DB: db, Dev: &admin}
	handler := browser.Protect(app.Handler())
	view := httptest.NewRecorder()
	handler.ServeHTTP(view, httptest.NewRequest("GET", "http://localhost/api/ui/v1/services/"+service.ID+"/pages/requirement/KSI-SVC-ACM", nil))
	if view.Code != 200 || !strings.Contains(view.Body.String(), draft.Explanation) || !strings.Contains(view.Body.String(), "I reviewed this implementation") {
		t.Fatalf("view: %d %s", view.Code, view.Body.String())
	}
	csrfCookie := view.Result().Cookies()[0]
	post := func(action string, values url.Values, csrf bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "http://localhost/api/ui/v1/services/"+service.ID+"/actions/"+action, strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(csrfCookie)
		if csrf {
			req.Header.Set("X-CSRF-Token", csrfCookie.Value)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	values := url.Values{"requirement_id": {"KSI-SVC-ACM"}, "revision": {"1"}, "reviewed": {"yes"}, "notes": {"Reviewed fixture"}}
	if w := post("review", values, false); w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if w := post("review", values, true); w.Code != 200 {
		t.Fatalf("review: %d %s", w.Code, w.Body.String())
	}
	status = unpack[assessment.Status](t, call("get_requirement_status", adapter.RequirementInput{ServiceID: service.ID, RequirementID: "KSI-SVC-ACM"}))
	if status.Review != "reviewed" {
		t.Fatalf("review not visible to agent: %+v", status)
	}
	snapshot := unpack[map[string]any](t, call("prepare_publication", adapter.PrepareInput{ServiceID: service.ID, IdempotencyKey: "snapshot"}))
	snapshotID := snapshot["id"].(string)
	jsonExport, _, err := store.Export(ctx, admin, service.ID, snapshotID, "json")
	if err != nil {
		t.Fatal(err)
	}
	htmlExport, _, err := store.Export(ctx, admin, service.ID, snapshotID, "html")
	if err != nil {
		t.Fatal(err)
	}
	_, pre, ok := strings.Cut(string(htmlExport), "<pre>")
	if !ok {
		t.Fatal("missing report content")
	}
	pre, _, ok = strings.Cut(pre, "</pre>")
	if !ok || html.UnescapeString(pre) != string(jsonExport) {
		t.Fatal("exports differ")
	}
	draft.Explanation = "Changed after publication"
	if _, err = store.SaveDraft(ctx, admin, service.ID, "KSI-SVC-ACM", 1, draft); err != nil {
		t.Fatal(err)
	}
	after, _, err := store.Export(ctx, admin, service.ID, snapshotID, "json")
	if err != nil || string(after) != string(jsonExport) {
		t.Fatal("snapshot changed")
	}
	if w := post("review", values, true); w.Code != 409 {
		t.Fatalf("stale browser review: %d", w.Code)
	}
	if _, err = store.SaveDestination(ctx, admin, service.ID, 0, assessment.Destination{Name: "First", AdapterType: "vendor-a"}); err != nil {
		t.Fatal(err)
	}
	if err = store.Publish(ctx, admin, service.ID, snapshotID); !errors.Is(err, integrations.ErrUnavailable) {
		t.Fatalf("unsupported publication: %v", err)
	}
	if _, err = store.SaveDestination(ctx, admin, service.ID, 1, assessment.Destination{Name: "Second", AdapterType: "vendor-b"}); err != nil {
		t.Fatal(err)
	}
	otherToken, err := auth.IssueToken(ctx, db, admin, other.ID, []string{"read", "export"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := auth.TokenActor(ctx, db, otherToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Export(ctx, otherActor, service.ID, snapshotID, "json"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("cross-service export: %v", err)
	}
	publications, err := store.List(ctx, admin, service.ID, "publication", "", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range publications {
		p, err := assessment.Decode[assessment.Publication](item)
		if err != nil {
			t.Fatal(err)
		}
		if p.State == "published" {
			t.Fatal("false publication")
		}
		if p.State == "failed" && (p.Destination == nil || p.Destination.Revision != 1) {
			t.Fatal("destination history changed")
		}
	}
	preview := post("preview-profile", url.Values{"catalog_id": {catalogID}, "class": {"B"}, "path": {"Program"}, "context": {"initial"}, "date": {"2026-10-16"}, "assessment_started": {"unknown"}, "expected_revision": {"1"}}, true)
	if preview.Code != 200 || !strings.Contains(preview.Body.String(), "Changed obligations") {
		t.Fatalf("profile preview: %d", preview.Code)
	}
	profile.Class = "B"
	profileJSON, _ := json.Marshal(profile)
	if w := post("profile", url.Values{"payload": {string(profileJSON)}, "expected_revision": {"1"}}, true); w.Code != 200 {
		t.Fatalf("profile save: %d", w.Code)
	}
	if _, err = store.ReconcileCollection(ctx, service.ID); err != nil {
		t.Fatal(err)
	}
	planPage := unpack[map[string]any](t, call("get_collection_plan", adapter.ListInput{ServiceID: service.ID, Limit: 1}))
	if planPage["next_offset"] == nil {
		t.Fatal("collection plan pagination missing")
	}
	batch := unpack[checks.Batch](t, call("collect_now", adapter.PrepareInput{ServiceID: service.ID, IdempotencyKey: "sdk-batch"}))
	retryBatch := unpack[checks.Batch](t, call("collect_now", adapter.PrepareInput{ServiceID: service.ID, IdempotencyKey: "sdk-batch"}))
	if batch.ID != retryBatch.ID {
		t.Fatal("collection batch retry duplicated")
	}
	unpack[checks.Batch](t, call("get_collection_batch", adapter.GetInput{ServiceID: service.ID, ID: batch.ID}))
	configuration, err := store.CollectionConfiguration(ctx, admin, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	configuration.Paused = true
	unpack[assessment.CollectionConfiguration](t, call("configure_collection", adapter.CollectionConfigInput{ServiceID: service.ID, ExpectedRevision: configuration.Revision, Configuration: configuration}))
	if !call("configure_collection", adapter.CollectionConfigInput{ServiceID: service.ID, ExpectedRevision: configuration.Revision, Configuration: configuration}).IsError {
		t.Fatal("stale configuration overwritten")
	}
	readToken, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	readSession, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{readToken}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer readSession.Close()
	for name, args := range map[string]any{"collect_now": adapter.PrepareInput{ServiceID: service.ID, IdempotencyKey: "denied"}, "ingest_observation": submission, "configure_collection": adapter.CollectionConfigInput{ServiceID: service.ID, ExpectedRevision: configuration.Revision, Configuration: configuration}} {
		result, e := readSession.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if e != nil || !result.IsError {
			t.Fatalf("read-only %s allowed: %v", name, e)
		}
	}
	if err = auth.RevokeToken(ctx, db, admin, service.ID, storage.Hash([]byte(token))); err != nil {
		t.Fatal(err)
	}
	if _, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "list_requirements", Arguments: adapter.ListInput{ServiceID: service.ID}}); err == nil {
		t.Fatal("revoked token accepted")
	}
}
