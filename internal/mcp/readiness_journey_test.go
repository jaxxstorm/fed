package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/checks"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Complete only for this synthetic one-rule catalog. No real FedRAMP content
// or semantic source-review claim is manufactured by this fixture.
func publishedJourneyFixture(t *testing.T) (readiness.Package, catalog.Snapshot, fstest.MapFS, fstest.MapFS) {
	t.Helper()
	const obligation = "Retain the monitored-system inventory and dated alert review records."
	commit, site := strings.Repeat("a", 40), strings.Repeat("b", 40)
	payload, err := json.Marshal(map[string]any{"KSI": map[string]any{"TEST": map[string]any{"status": "stable", "indicators": map[string]any{"RULE": map[string]any{"name": "Synthetic journey only", "statement": obligation}}}}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := catalog.Parse(payload, []byte(`{"type":"object"}`), commit)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]any{"rules": map[string]string{"repo": "fedramp/rules", "sha": commit}, "site": map[string]string{"repo": "fedramp/2026", "sha": site}})
	if err != nil {
		t.Fatal(err)
	}
	rules := fstest.MapFS{"rules.json": {Data: payload}}
	markdown := fstest.MapFS{"_sources.json": {Data: metadata}, "definitions.md": {Data: []byte("# Synthetic\nTest-only source corpus.\n")}}
	official := readiness.Statement{Text: obligation, Basis: readiness.Official, Citations: []string{"policy"}}
	recommend := func(text string) readiness.Statement {
		return readiness.Statement{Text: text, Basis: readiness.Recommendation, Citations: []string{"policy"}, Rationale: "Synthetic evidence collection example; not an additional official obligation."}
	}
	step := func(id, action string) readiness.Steps {
		return readiness.Steps{Items: []readiness.Step{{ID: id, OwnerRole: "Monitoring owner", Trigger: "one_time", Action: recommend(action), CompletionCriteria: []readiness.Statement{recommend("Identify the record version, service boundary and recorded review period.")}}}}
	}
	none := readiness.Steps{NoneReason: "The synthetic obligation prescribes no separate operating cadence."}
	p := readiness.Package{Format: readiness.Format, CatalogID: snapshot.ID, Revision: 1,
		Sources:      map[string]readiness.Source{"rules": {Repository: "fedramp/rules", Commit: commit}, "site": {Repository: "fedramp/2026", Commit: site}},
		Citations:    []readiness.Citation{{ID: "policy", Source: "rules", Path: "rules.json", Anchor: "/KSI/TEST/indicators/RULE/statement", Excerpt: obligation, ExcerptHash: storage.Hash([]byte(obligation)), ContentHash: storage.Hash(payload)}},
		Capabilities: []readiness.Capability{{ID: "monitoring-records", Name: "Retained monitoring inventory", ScopeKind: "service", Setup: step("locate-inventory", "Ask the monitoring owner for the retained system inventory and its service scope.")}},
	}
	base := readiness.Playbook{Kind: "actionable", Revision: 1, RequirementID: "RULE", OfficialStatement: obligation, Purpose: official, Applicability: recommend("Use the synthetic catalog resolver's selected applicability; this is not a real FedRAMP determination."),
		Prerequisites: []readiness.Dependency{{CapabilityID: "monitoring-records", Reason: recommend("An identifiable inventory is needed to reconcile which systems the alert review records cover.")}}, Setup: none, Operate: none,
		Collect:     step("collect-reviews", "Ask the monitoring owner for dated alert review records mapped to the inventory."),
		ReviewSteps: step("human-review", "An authorized human must review cited records and uncovered systems; agent guidance review does not approve service evidence."),
		Assertions:  []readiness.Assertion{{ID: "inventory-and-review", Obligation: official, Clauses: []string{obligation}, Acceptance: readiness.Acceptance{Fact: "The inventory and dated alert reviews cover the identified systems.", Scope: recommend("The saved service boundary and identified systems."), TimePeriod: recommend("The actual dates of retained alert reviews; no numeric cadence is inferred."), Provenance: recommend("Retain source versions, review dates and accountable reviewers."), Alternatives: []readiness.EvidenceRoute{{ID: "retained-records", Artifacts: []readiness.Statement{recommend("System inventory and dated alert review records.")}, Proves: "Recorded inventory coverage and alert review activity.", Limitations: "Does not demonstrate unrecorded systems or every alert delivery."}}, Insufficient: []string{"An undated dashboard without inventory reconciliation."}}}},
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range inventory.Variants {
		book := base
		book.ID, book.VariantIDs, book.OfficialStatement = variant.ID, []string{variant.ID}, variant.Statement
		if variant.Applicability == "not_applicable" {
			book.Kind = "excluded"
		}
		if variant.Applicability == "unresolved" {
			book.Kind = "needs_decision"
			book.DecisionQuestion = "Does the synthetic agency path include this indicator?"
			book.DecisionOwnerRole = "Scope owner"
		}
		if book.Kind != "actionable" {
			book.Assertions = nil
			book.Prerequisites = nil
			book.Setup = readiness.Steps{}
			book.Operate = readiness.Steps{}
			book.Collect = readiness.Steps{}
			book.ReviewSteps = readiness.Steps{}
		}
		p.Playbooks = append(p.Playbooks, book)
	}
	return p, snapshot, rules, markdown
}

func TestReadinessJourneyPublishedSyntheticGuidanceParity(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	p, snapshot, rules, markdown := publishedJourneyFixture(t)
	guidance := &readiness.Store{DB: db, Rules: rules, Markdown: markdown}
	store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}, Guidance: guidance}
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "synthetic-journey-admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.CreateService(ctx, admin, "SYNTHETIC complete tiny catalog")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateService(ctx, admin, "Unpublished service")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Import(ctx, db, snapshot.Payload, snapshot.Schema, snapshot.Commit); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveProfile(ctx, admin, service.ID, 0, catalog.Profile{CatalogID: p.CatalogID, Type: "20x", Path: "Program", Class: "C", Context: "initial", Date: "2026-10-18"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, admin, service.ID, 0, assessment.Scope{Description: "Synthetic saved service boundary"}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = guidance.Publish(ctx, agent, service.ID, p); !errors.Is(err, readiness.ErrReviewRequired) {
		t.Fatal("unreviewed package accepted", err)
	}
	finding := func(text string) readiness.ReviewFinding {
		return readiness.ReviewFinding{Finding: text, Citations: []string{"policy"}}
	}
	comparison := readiness.SourceComparison{
		Applicability: finding("Synthetic catalog variants follow their resolver applicability; excluded and unresolved variants impose no assertions."),
		Obligations:   finding("The sole synthetic statement requires retained inventory and dated alert review records; the assertion preserves both clauses."),
		Prerequisites: finding("Inventory reconciliation is a labeled inferred dependency, not a mandated product."),
		Evidence:      finding("Inventory plus dated reviews establish recorded coverage; an undated dashboard does not."),
		Timing:        finding("The synthetic statement requires dated records but supplies no numeric cadence."),
		Limitations:   []string{"Synthetic test-only source comparison; not a review of any real FedRAMP catalog or service evidence."}, Uncertainties: []readiness.ReviewFinding{},
	}
	receipts := map[string]readiness.Review{}
	for _, book := range p.Playbooks {
		if _, err = guidance.RecordContentReview(ctx, agent, other.ID, p, book.ID, comparison); !errors.Is(err, auth.ErrForbidden) {
			t.Fatal("review escaped service scope", err)
		}
		receipt, err := guidance.RecordContentReview(ctx, agent, service.ID, p, book.ID, comparison)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.ReviewerID != agent.ID || receipt.ReviewerType != "agent" || receipt.ReviewedAt.IsZero() {
			t.Fatal("unauthenticated review receipt", receipt)
		}
		// PostgreSQL may return the UTC instant with time.Local as its location
		// when the host uses UTC. JSON decodes Z as time.UTC; compare the same
		// instant in the transport's location, not host-specific pointer identity.
		receipt.ReviewedAt = receipt.ReviewedAt.UTC()
		receipts[book.ID] = receipt
	}
	if _, err = guidance.Publish(ctx, agent, other.ID, p); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("publication escaped service grant", err)
	}
	hash, err := guidance.Publish(ctx, agent, service.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	quality, err := guidance.Quality(ctx, agent, service.ID, 100, 0)
	if err != nil || !quality.Complete || quality.VariantCount != len(p.Playbooks) || quality.Counts["reviewed"] != len(p.Playbooks) || quality.ReleaseHash != hash {
		t.Fatal("synthetic denominator incomplete", quality, err)
	}
	t.Logf("Published complete SYNTHETIC catalog: %d variants, release %s", quality.VariantCount, hash)
	runner := &checks.Runner{DB: db, Artifacts: store.Artifacts}
	server := httptest.NewServer(adapter.Handler(store, runner))
	defer server.Close()
	client := sdk.NewClient(&sdk.Implementation{Name: "synthetic-published-guidance-journey", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, input any) *sdk.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: input})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	instructions := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: "RULE"}))
	app := &web.App{Store: store, Runner: runner}
	browserAuth := &auth.Browser{DB: db, Dev: &admin}
	browser := browserAuth.Protect(app.Handler())
	get := func(path string, value any) {
		t.Helper()
		response := httptest.NewRecorder()
		browser.ServeHTTP(response, httptest.NewRequest("GET", "/api/ui/v1/services/"+service.ID+path, nil))
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), value) != nil {
			t.Fatal("browser read failed", response.Code, response.Body.String())
		}
	}
	var page web.Page
	get("/pages/requirement/RULE", &page)
	if instructions.Guidance == nil || instructions.Guidance.State != "reviewed" || !reflect.DeepEqual(instructions.Guidance, page.Guidance) || !reflect.DeepEqual(instructions.EvidenceCoverage, page.EvidenceCoverage) {
		t.Fatal("published browser/MCP context differs", instructions.Guidance, page.Guidance)
	}
	g := instructions.Guidance
	if g.ReleaseHash != hash || g.ReleaseRevision != 1 || g.CatalogID != p.CatalogID || g.Label != readiness.AgentReviewLabel || g.Caveat != readiness.AgentReviewCaveat {
		t.Fatal("release identity or caveat lost", g)
	}
	book := g.Guidance.Playbooks[0]
	var expectedBook readiness.Playbook
	for _, original := range p.Playbooks {
		if original.ID == book.ID {
			expectedBook = original
			receipt := receipts[book.ID]
			expectedBook.Review = &receipt
		}
	}
	if !reflect.DeepEqual(book, expectedBook) || !reflect.DeepEqual(g.Guidance.Capabilities, p.Capabilities) {
		t.Fatal("published obligations, acceptance criteria, prerequisites or actions differ from reviewed content")
	}
	expectedReviewHash, err := readiness.ReviewHash(p, book.ID)
	if err != nil || book.Review.ContentHash != expectedReviewHash {
		t.Fatal("review receipt does not bind exact playbook content", err)
	}
	if !reflect.DeepEqual(*book.Review, receipts[book.ID]) || !reflect.DeepEqual(g.Guidance.Sources, p.Sources) || !reflect.DeepEqual(g.Guidance.Citations, p.Citations) {
		t.Fatal("exact authenticated review or source binding changed")
	}
	if len(book.Assertions) != 1 || book.Assertions[0].ID != "inventory-and-review" || len(book.Prerequisites) != 1 || book.Prerequisites[0].CapabilityID != "monitoring-records" || len(g.Guidance.Capabilities) != 1 || g.Guidance.Capabilities[0].Setup.Items[0].OwnerRole != "Monitoring owner" {
		t.Fatal("obligation or prerequisite context omitted")
	}
	assertionRaw, err := json.Marshal(book.Assertions)
	if err != nil {
		t.Fatal(err)
	}
	binding := instructions.Binding
	if binding.CatalogID != p.CatalogID || binding.ProfileRevision != 1 || binding.ScopeRevision != 1 || binding.GuidanceHash != hash || binding.GuidanceRevision != 1 || binding.GuidanceVariant != g.VariantID || binding.AssertionHash != storage.Hash(assertionRaw) {
		t.Fatal("instruction binding differs from release", binding)
	}
	if g.Progress == nil || g.Progress.Binding.ProfileRevision != 1 || g.Progress.Binding.ScopeRevision != 1 || g.Progress.Binding.ReleaseHash != hash || len(g.Progress.Capabilities) != 1 || g.Progress.Capabilities[0].Review != "not_assessed" || g.Progress.Capabilities[0].EvidenceState != "missing" {
		t.Fatal("guidance review confused with service evidence review", g.Progress)
	}
	var browserPlan readiness.ServicePlan
	get("/readiness/plan?limit=1", &browserPlan)
	plan := unpack[readiness.ServicePlan](t, call("get_service_readiness_plan", adapter.ListInput{ServiceID: service.ID, Limit: 1}))
	if !reflect.DeepEqual(plan, browserPlan) {
		t.Fatal("published prerequisite plan differs between transports")
	}
	if plan.Binding != g.Progress.Binding {
		t.Fatal("service plan and requirement context use different saved revisions")
	}
	if !strings.Contains(book.ReviewSteps.Items[0].Action.Text, "authorized human") {
		t.Fatal("human-only review boundary absent")
	}
	if _, err = store.Review(ctx, agent, service.ID, "RULE", assessment.Review{Assessment: assessment.Ref{ID: "RULE", Revision: 1}}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("agent can grant human assessment review", err)
	}
	if !call("get_rule_instructions", adapter.RequirementInput{ServiceID: other.ID, RequirementID: "RULE"}).IsError {
		t.Fatal("MCP leaked published context to another service")
	}
	raw, err := json.Marshal(instructions)
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		ServiceID  string                 `json:"service_id"`
		Rule       catalog.Resolved       `json:"rule"`
		Steps      []string               `json:"steps"`
		Assertions []assessment.Assertion `json:"assertions"`
		Binding    struct {
			CatalogID       string           `json:"catalog_id"`
			ProfileRevision int              `json:"profile_revision"`
			ScopeRevision   int              `json:"scope_revision"`
			Version         string           `json:"version"`
			Sources         []assessment.Ref `json:"sources"`
		} `json:"binding"`
	}
	if json.Unmarshal(raw, &legacy) != nil || legacy.ServiceID != service.ID || legacy.Rule.Requirement.ID != "RULE" || legacy.Binding.CatalogID != p.CatalogID || legacy.Binding.ProfileRevision != 1 || legacy.Binding.ScopeRevision != 1 || legacy.Binding.Version == "" || len(legacy.Steps) == 0 || len(legacy.Assertions) != 1 || legacy.Assertions[0].ID != "inventory-and-review" {
		t.Fatal("old-field consumer cannot read published instructions")
	}
	var reviews int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind IN ('assessment','review')`, service.ID).Scan(&reviews); err != nil || reviews != 0 {
		t.Fatal("guidance publication created service assessment/review", reviews, err)
	}
}

func TestReadinessJourneyTransportRevisionAndEvidenceParity(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}, Guidance: &readiness.Store{DB: db}}
	actor, err := auth.BootstrapAdmin(ctx, db, "test", "journey-admin")
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.CreateService(ctx, actor, "Journey acceptance")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	id, err := catalog.Import(ctx, db, rules, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveProfile(ctx, actor, service.ID, 0, catalog.Profile{CatalogID: id, Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: "2026-10-16"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, actor, service.ID, 0, assessment.Scope{Description: "Development and production"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveEvidenceSource(ctx, actor, service.ID, "records", 0, assessment.EvidenceSource{Type: "document", Name: "Retained records", Locator: "https://docs.example/journeys", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	runner := &checks.Runner{DB: db, Artifacts: store.Artifacts}
	server := httptest.NewServer(adapter.Handler(store, runner))
	defer server.Close()
	token, err := auth.IssueToken(ctx, db, actor, service.ID, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "readiness-journey", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any) *sdk.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	app := &web.App{Store: store, Runner: runner}
	browserAuth := &auth.Browser{DB: db, Dev: &actor}
	browser := browserAuth.Protect(app.Handler())
	for _, tc := range []struct{ id, content, gap, action string }{
		{"KSI-RPL-TRC", "Development restore executed; production was not tested.\n", "Production recovery tests and objective comparisons are absent.", "Ask the recovery owner for production execution records and objective versions."},
		{"KSI-INR-AAR", "Incident 42 has an after-action report; lessons remain open.\n", "The report does not demonstrate incorporation of lessons.", "Ask the incident response owner for dated lesson dispositions and resulting practice records."},
		{"KSI-CED-RAT", "General employee training attendance was exported.\n", "Attendance does not demonstrate effectiveness review for engineers or high-risk roles.", "Ask the training owner for population mappings and dated effectiveness conclusions."},
	} {
		t.Run(tc.id, func(t *testing.T) {
			before := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: tc.id}))
			artifact := assessment.ArtifactSubmission{ServiceID: service.ID, IdempotencyKey: tc.id, Source: assessment.Ref{ID: "records", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/journeys/" + tc.id, SourceVersion: "v1", MediaType: "text/plain", Content: tc.content, Description: tc.id + " incomplete proof", Method: "Read retained fixture record", ObservedAt: time.Now().UTC(), RetrievedAt: time.Now().UTC()}
			evidence := unpack[assessment.Record](t, call("submit_evidence_artifact", artifact))
			gap := assessment.AssertionGap{Kind: "missing", Scope: "Service boundary", Period: "Current review period", Reason: tc.gap, NextAction: tc.action}
			citation := assessment.Citation{Evidence: assessment.Ref{ID: evidence.ID, Revision: evidence.Revision}, StartLine: 1, EndLine: 1}
			receipt := unpack[assessment.RuleReceipt](t, call("submit_rule_result", assessment.RuleResultSubmission{ServiceID: service.ID, RequirementID: tc.id, IdempotencyKey: "assess-" + tc.id, Binding: before.Binding, RequestedState: "partial", Explanation: tc.gap, Assertions: []assessment.AssertionResult{{AssertionID: before.Assertions[0].ID, Outcome: "unknown", Rationale: tc.gap, Gaps: []assessment.AssertionGap{gap}, Citations: []assessment.Citation{citation}}}}))
			if receipt.Review != "pending" || receipt.Work.State != "partial" {
				t.Fatal("incomplete proof bypassed review", receipt)
			}
			instructions := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: tc.id}))
			response := httptest.NewRecorder()
			browser.ServeHTTP(response, httptest.NewRequest("GET", "/api/ui/v1/services/"+service.ID+"/pages/requirement/"+tc.id, nil))
			var page web.Page
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
				t.Fatal("browser detail failed", response.Code, response.Body.String())
			}
			if !reflect.DeepEqual(instructions.Guidance, page.Guidance) || !reflect.DeepEqual(instructions.EvidenceCoverage, page.EvidenceCoverage) {
				t.Fatal("browser/MCP guidance or exact evidence context differs")
			}
			if !reflect.DeepEqual(before.Binding, instructions.Binding) || instructions.Binding.ScopeRevision != 1 || instructions.Binding.ProfileRevision != 1 {
				t.Fatal("binding changed during fixed-revision journey")
			}
			row := instructions.EvidenceCoverage.Assertions[0]
			if !reflect.DeepEqual(row.Gaps, []assessment.AssertionGap{gap}) || !reflect.DeepEqual(row.Citations, []assessment.Citation{citation}) {
				t.Fatal("gap or exact citation lost", row)
			}
			// An old additive-contract consumer still reads its original fields.
			raw, err := json.Marshal(instructions)
			if err != nil {
				t.Fatal(err)
			}
			var legacy struct {
				ServiceID  string                        `json:"service_id"`
				Rule       catalog.Resolved              `json:"rule"`
				Binding    assessment.InstructionBinding `json:"binding"`
				Steps      []string                      `json:"steps"`
				Assertions []assessment.Assertion        `json:"assertions"`
			}
			if json.Unmarshal(raw, &legacy) != nil || legacy.ServiceID != service.ID || legacy.Rule.Requirement.ID != tc.id || len(legacy.Assertions) == 0 || len(legacy.Steps) == 0 {
				t.Fatal("legacy instruction fields unavailable")
			}
			artifactView := unpack[map[string]any](t, call("get_evidence_artifact", adapter.GetInput{ServiceID: service.ID, ID: evidence.ID}))
			if artifactView["content"] != tc.content {
				t.Fatal("retained proof changed")
			}
		})
	}
}
