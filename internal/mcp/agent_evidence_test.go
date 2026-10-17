package mcp_test

import (
	"context"
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
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExternalDocumentWorkflowWithoutWorker(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.CreateService(ctx, admin, "External agent")
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
	id, err := catalog.Import(ctx, db, payload, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveProfile(ctx, admin, service.ID, 0, catalog.Profile{CatalogID: id, Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: "2026-10-16"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, admin, service.ID, 0, assessment.Scope{Description: "Documentary boundary"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveEvidenceSource(ctx, admin, service.ID, "policies", 0, assessment.EvidenceSource{Type: "document", Name: "Approved policies", Locator: "https://docs.example/security", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	runner := &checks.Runner{DB: db, Artifacts: store.Artifacts}
	server := httptest.NewServer(adapter.Handler(store, runner))
	defer server.Close()
	token, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "external-evidence-agent", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any) *sdk.CallToolResult {
		t.Helper()
		result, e := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			t.Fatal(e)
		}
		return result
	}
	page := unpack[assessment.OpenRulesPage](t, call("list_open_rules", assessment.OpenRulesInput{ServiceID: service.ID, Kind: "KSI", Limit: 1}))
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal("missing open-rule page")
	}
	sources := unpack[adapter.Page](t, call("list_evidence_sources", adapter.ListInput{ServiceID: service.ID}))
	if sources.Items == nil {
		t.Fatal("configured sources unavailable")
	}
	i := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: "KSI-SVC-SIN"}))
	if len(i.Sources) != 1 || len(i.Assertions) == 0 || i.ExpectedDraftRevision != 0 {
		t.Fatal("instructions depend on collector setup")
	}
	artifact := assessment.ArtifactSubmission{ServiceID: service.ID, IdempotencyKey: "document", Source: assessment.Ref{ID: "policies", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/security/encryption", SourceVersion: "v1", MediaType: "text/plain", Content: "Fixture source: encryption controls are absent.\n", Description: "Observed failure, not compliant policy", Method: "Read fixture source", ObservedAt: time.Now().Add(-time.Minute).UTC(), RetrievedAt: time.Now().UTC()}
	ev := unpack[assessment.Record](t, call("submit_evidence_artifact", artifact))
	content := unpack[map[string]any](t, call("get_evidence_artifact", adapter.GetInput{ServiceID: service.ID, ID: ev.ID}))
	if content["media_type"] != "text/plain" || content["content"] != artifact.Content {
		t.Fatal("document round trip")
	}
	sub := assessment.RuleResultSubmission{ServiceID: service.ID, RequirementID: i.Rule.Requirement.ID, IdempotencyKey: "result", Binding: i.Binding, Explanation: "Fixture investigation found contradicting evidence", RequestedState: "evidence_complete", Assertions: []assessment.AssertionResult{{AssertionID: i.Assertions[0].ID, Outcome: "contradicted", Rationale: "Fixture records missing controls", Citations: []assessment.Citation{{Evidence: assessment.Ref{ID: ev.ID, Revision: ev.Revision}, StartLine: 1, EndLine: 1}}}}}
	receipt := unpack[assessment.RuleReceipt](t, call("submit_rule_result", sub))
	if receipt.Work.State != "evidence_complete" || receipt.Work.Assessment != "contradicted" || receipt.Review != "pending" {
		t.Fatalf("failure confused with completion: %+v", receipt)
	}
	replay := unpack[assessment.RuleReceipt](t, call("submit_rule_result", sub))
	if replay.Result != receipt.Result {
		t.Fatal("receipt retry duplicated")
	}
	if !call("list_open_rules", assessment.OpenRulesInput{ServiceID: service.ID, Kind: "KSI", Limit: 1, Cursor: page.NextCursor}).IsError {
		t.Fatal("stale cursor accepted")
	}
	for _, tc := range []struct{ rid, state string }{{"KSI-SVC-ACM", "partial"}, {"KSI-RPL-ABO", "blocked"}} {
		instructions := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: tc.rid}))
		request := assessment.RuleResultSubmission{ServiceID: service.ID, RequirementID: tc.rid, IdempotencyKey: tc.rid, Binding: instructions.Binding, Explanation: "Investigation requires additional operational records", RequestedState: tc.state, Blockers: []string{"Configured source does not contain required operational records"}}
		result := unpack[assessment.RuleReceipt](t, call("submit_rule_result", request))
		if result.Work.State != tc.state {
			t.Fatal("missing partial/blocked result")
		}
	}
	app := &web.App{Store: store, Runner: runner}
	browser := auth.Browser{DB: db, Dev: &admin}
	view := httptest.NewRecorder()
	browser.Protect(app.Handler()).ServeHTTP(view, httptest.NewRequest("GET", "http://localhost/api/ui/v1/services/"+service.ID+"/pages/requirement/KSI-SVC-SIN", nil))
	if view.Code != 200 || !strings.Contains(view.Body.String(), "evidence_complete") || !strings.Contains(view.Body.String(), ev.ID) || !strings.Contains(view.Body.String(), "contradicted") {
		t.Fatal("browser differs from receipt", view.Code)
	}
	sourceView := httptest.NewRecorder()
	browser.Protect(app.Handler()).ServeHTTP(sourceView, httptest.NewRequest("GET", "http://localhost/api/ui/v1/services/"+service.ID+"/pages/sources", nil))
	if sourceView.Code != 200 || !strings.Contains(sourceView.Body.String(), "Approved policies") {
		t.Fatal("source UI differs from MCP", sourceView.Code)
	}
	forged := httptest.NewRecorder()
	browser.Protect(app.Handler()).ServeHTTP(forged, httptest.NewRequest("POST", "http://localhost/services/"+service.ID+"/actions/evidence-source", strings.NewReader("id=policies")))
	if forged.Code != http.StatusForbidden {
		t.Fatal("source edit without CSRF allowed", forged.Code)
	}
	cookies := sourceView.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing CSRF cookie")
	}
	form := url.Values{"id": {"policies"}, "expected_revision": {"1"}, "csrf_token": {cookies[0].Value}, "payload": {`{"type":"document","name":"Updated policy directory","locator":"https://docs.example/security","scope_revision":1,"enabled":true,"freshness_hours":48}`}}
	edit := httptest.NewRequest("POST", "http://localhost/api/ui/v1/services/"+service.ID+"/actions/evidence-source", strings.NewReader(form.Encode()))
	edit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edit.AddCookie(cookies[0])
	saved := httptest.NewRecorder()
	browser.Protect(app.Handler()).ServeHTTP(saved, edit)
	if saved.Code != http.StatusOK {
		t.Fatal("source UI save failed", saved.Code)
	}
	updated := unpack[adapter.Page](t, call("list_evidence_sources", adapter.ListInput{ServiceID: service.ID}))
	if updated.Items == nil {
		t.Fatal("saved UI source not discoverable")
	}
	var jobs, measures, plans int
	if err = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM jobs),(SELECT count(*) FROM records WHERE kind='measure'),(SELECT count(*) FROM collection_plans)`).Scan(&jobs, &measures, &plans); err != nil || jobs+measures+plans != 0 {
		t.Fatal("workflow required collector work", err)
	}
	other, err := store.CreateService(ctx, admin, "Other service")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, admin, other.ID, 0, assessment.Scope{Description: "Other boundary"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveEvidenceSource(ctx, admin, other.ID, "policies", 0, assessment.EvidenceSource{Type: "document", Name: "Other policy", Locator: "https://docs.example/security", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	foreignInput := artifact
	foreignInput.ServiceID = other.ID
	foreign, err := store.SubmitEvidenceArtifact(ctx, admin, foreignInput)
	if err != nil {
		t.Fatal(err)
	}
	current := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: sub.RequirementID}))
	cross := sub
	cross.IdempotencyKey = "cross-service"
	cross.ExpectedResultRevision = current.ExpectedResultRevision
	cross.ExpectedDraftRevision = current.ExpectedDraftRevision
	cross.Binding = current.Binding
	cross.Assertions = []assessment.AssertionResult{{AssertionID: "whole-statement", Outcome: "supported", Rationale: "Unauthorized citation", Citations: []assessment.Citation{{Evidence: assessment.Ref{ID: foreign.ID, Revision: 1}, StartLine: 1, EndLine: 1}}}}
	if !call("submit_rule_result", cross).IsError {
		t.Fatal("cross-service citation accepted")
	}
	readonly, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	readSession, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{readonly}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer readSession.Close()
	for name, args := range map[string]any{"submit_evidence_artifact": artifact, "submit_rule_result": sub} {
		result, e := readSession.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if e != nil || !result.IsError {
			t.Fatal("read-only mutation allowed", name, e)
		}
	}
	if !call("get_rule_instructions", adapter.RequirementInput{ServiceID: "other-service", RequirementID: "KSI-SVC-SIN"}).IsError {
		t.Fatal("cross-service instructions allowed")
	}
	if err = auth.RevokeToken(ctx, db, admin, service.ID, storage.Hash([]byte(token))); err != nil {
		t.Fatal(err)
	}
	result, e := session.CallTool(ctx, &sdk.CallToolParams{Name: "list_open_rules", Arguments: assessment.OpenRulesInput{ServiceID: service.ID}})
	if e == nil && !result.IsError {
		t.Fatal("revoked token allowed")
	}
}
