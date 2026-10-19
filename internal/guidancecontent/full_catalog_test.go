package guidancecontent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

// Exercise the actual complete authored corpus and comparison inputs against an
// isolated test service. These receipts are test identities, not production
// publication or a service compliance assessment.
func TestFullAuthoredCatalogPublicationGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	db := testutil.Database(t)
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	batches, err := DraftBatches(rules, markdown)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Assemble(batches, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Playbooks) != 1338 {
		t.Fatalf("authored playbook denominator changed: %d", len(p.Playbooks))
	}
	raw, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../testdata/fedramp/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := draftInventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := Coverage(batches, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.Total != 1340 || coverage.Encoded != 1340 || coverage.Remaining != 0 {
		t.Fatal("incomplete original catalog denominator", coverage)
	}
	files, err := filepath.Glob("../../guidance/reviews/*.json")
	if err != nil {
		t.Fatal(err)
	}
	bundles := []ComparisonBundle{}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var b ComparisonBundle
		if err = json.Unmarshal(raw, &b); err != nil {
			t.Fatal(err)
		}
		bundles = append(bundles, b)
	}
	comparisons, err := RebindBundles(batches, p, bundles)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "full-catalog-owner")
	if err != nil {
		t.Fatal(err)
	}
	const service = "full-catalog-test"
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES($1,$2,'Full catalog fixture')`, service, admin.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Import(ctx, db, raw, schema, catalog.PinnedCommit); err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, admin, service, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	store := readiness.Store{DB: db, Rules: rules, Markdown: markdown}
	if _, err = store.Publish(ctx, agent, service, p); !errors.Is(err, readiness.ErrReviewRequired) {
		t.Fatal("complete authored text bypassed authenticated review", err)
	}
	started := time.Now()
	for i, book := range p.Playbooks {
		subset, err := readiness.RelevantPackage(p, book.ID)
		if err != nil {
			t.Fatal(err)
		}
		review, err := store.RecordContentReview(ctx, agent, service, subset, book.ID, comparisons.Comparisons[book.ID].Comparison)
		if err != nil {
			t.Fatal(book.ID, err)
		}
		if review.ReviewerType != "agent" || review.ReviewerID != agent.ID || review.ReviewedAt.IsZero() || review.ContentHash != comparisons.Comparisons[book.ID].ContentHash {
			t.Fatal("review attribution/hash differs", book.ID)
		}
		// JSON transports canonicalize UTC; pgx can use time.Local when TZ=UTC.
		review.ReviewedAt = review.ReviewedAt.UTC()
		p.Playbooks[i].Review = &review
	}
	hash, err := store.Publish(ctx, agent, service, p)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("missing immutable release identity")
	}
	profile := catalog.Profile{CatalogID: p.CatalogID, Type: "20x", Path: "Program", Class: "C", Context: "initial", Date: "2026-10-19"}
	profileRaw, _ := json.Marshal(profile)
	if _, err = db.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,actor_id,data) VALUES($1,'profile','profile',1,$2,$3),($1,'scope','scope',1,$2,'{}')`, service, admin.ID, profileRaw); err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.Resolve(snapshot.Requirements, profile)
	if err != nil {
		t.Fatal(err)
	}
	contexts, err := store.Contexts(ctx, db, agent, service, p.CatalogID, resolved, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(contexts) != len(resolved) {
		t.Fatal("resolved requirements lost")
	}
	for _, g := range contexts {
		if g.State != "reviewed" || g.ReleaseHash != hash || g.Label != readiness.AgentReviewLabel {
			t.Fatal("complete corpus context not current", g.VariantID, g.State)
		}
	}
	partial := p
	partial.Playbooks = append([]readiness.Playbook(nil), p.Playbooks[1:]...)
	report, err := readiness.Coverage(partial, inventory, rules, markdown, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if report.Complete || report.Counts["missing"] == 0 {
		t.Fatal("removing a published variant shrank the denominator", report.Counts)
	}
	t.Logf("Published %d playbooks covering %d variants; projected %d requirement contexts in %s", len(p.Playbooks), inventory.VariantCount, len(contexts), time.Since(started))
	t.Run("AuthoredCEDINRRPLTransportParity", func(t *testing.T) {
		testAuthoredTransportParity(t, ctx, &store, admin, agent, service, token, p, hash)
	})
}

type fullCatalogBearerTransport struct{ token string }

func (b fullCatalogBearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

// Uses the real full-catalog publication above, not a reduced catalog or invented
// source comparison. Only the service evidence below is synthetic and incomplete.
func testAuthoredTransportParity(t *testing.T, ctx context.Context, guidance *readiness.Store, admin, agent auth.Actor, service, token string, p readiness.Package, hash string) {
	t.Helper()
	store := &assessment.Store{DB: guidance.DB, Artifacts: storage.Files{Directory: t.TempDir()}, Guidance: guidance}
	if _, err := store.SaveEvidenceSource(ctx, admin, service, "journey-records", 0, assessment.EvidenceSource{Type: "document", Name: "Synthetic incomplete journey records", Locator: "https://docs.example/journeys", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	runner := &checks.Runner{DB: guidance.DB, Artifacts: store.Artifacts}
	server := httptest.NewServer(adapter.Handler(store, runner))
	defer server.Close()
	client := sdk.NewClient(&sdk.Implementation{Name: "full-authored-catalog-journey", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: fullCatalogBearerTransport{token}, Timeout: 30 * time.Second}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(t *testing.T, name string, input, output any) {
		t.Helper()
		result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: input})
		if err != nil {
			t.Fatal(name, err)
		}
		if result.IsError {
			t.Fatalf("%s failed: %+v", name, result.Content)
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		envelope := struct {
			Data any `json:"data"`
		}{Data: output}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(name, err)
		}
	}
	app := &web.App{Store: store, Runner: runner}
	browserAuth := &auth.Browser{DB: guidance.DB, Dev: &admin}
	browser := browserAuth.Protect(app.Handler())
	get := func(t *testing.T, path string, value any) {
		t.Helper()
		response := httptest.NewRecorder()
		browser.ServeHTTP(response, httptest.NewRequest("GET", "/api/ui/v1/services/"+service+path, nil).WithContext(ctx))
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), value) != nil {
			t.Fatal("authenticated browser read failed", response.Code, response.Body.String(), "context:", ctx.Err())
		}
	}
	var plan, browserPlan readiness.ServicePlan
	call(t, "get_service_readiness_plan", adapter.ListInput{ServiceID: service, Limit: 1}, &plan)
	get(t, "/readiness/plan?limit=1", &browserPlan)
	if !reflect.DeepEqual(plan, browserPlan) {
		t.Fatal("browser/MCP saved service plan differs")
	}
	for _, tc := range []struct {
		id           string
		assertions   []string
		owner        string
		triggers     []string
		content, gap string
	}{
		{"KSI-CED-RAT", []string{"persistent-effectiveness-review", "general-employee-training", "high-risk-role-training", "secure-delivery-training", "response-recovery-training"}, "Training owner and role leads", []string{"recurring"}, "Synthetic general employee attendance only.\n", "Attendance does not demonstrate repeated effectiveness review or coverage of all named populations."},
		{"KSI-INR-AAR", []string{"after-action-reports", "incorporated-lessons"}, "Incident response owner", []string{"event", "recurring"}, "Synthetic incident 42 report exists; lessons remain open.\n", "A report does not demonstrate repeated incorporation of lessons into operating practices."},
		{"KSI-RPL-TRC", []string{"tested-recovery-capability"}, "Recovery owner", []string{"recurring"}, "Synthetic development restore executed; production untested.\n", "Production execution records and comparisons with defined recovery objectives are absent."},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var before assessment.RuleInstructions
			call(t, "get_rule_instructions", adapter.RequirementInput{ServiceID: service, RequirementID: tc.id}, &before)
			var page web.Page
			get(t, "/pages/requirement/"+tc.id, &page)
			g := before.Guidance
			if g == nil || g.Guidance == nil || len(g.Guidance.Playbooks) != 1 || g.State != "reviewed" || g.ReleaseHash != hash || g.CatalogID != p.CatalogID || g.ReleaseRevision != 1 || g.Label != readiness.AgentReviewLabel || g.Caveat != readiness.AgentReviewCaveat || !reflect.DeepEqual(g, page.Guidance) || !reflect.DeepEqual(before.EvidenceCoverage, page.EvidenceCoverage) {
				t.Fatal("published authored guidance/evidence parity or release attribution lost")
			}
			book := g.Guidance.Playbooks[0]
			expected, err := readiness.RelevantPackage(p, book.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, original := range p.Playbooks {
				if original.ID == book.ID {
					expected.Playbooks[0].Review = original.Review
				}
			}
			if !reflect.DeepEqual(*g.Guidance, expected) {
				t.Fatal("authored obligations, proof alternatives/limitations, prerequisites, owners, timing or exact sources changed in transport")
			}
			reviewHash, err := readiness.ReviewHash(p, book.ID)
			if err != nil || book.Review == nil || book.Review.ContentHash != reviewHash || book.Review.ReviewerID != agent.ID || book.Review.ReviewerType != "agent" {
				t.Fatal("authenticated source-review binding lost", err)
			}
			var ids, instructionIDs []string
			for _, a := range book.Assertions {
				ids = append(ids, a.ID)
				if a.Obligation.Basis != readiness.Official || len(a.Obligation.Citations) == 0 || len(a.Clauses) == 0 || a.Acceptance.Fact == "" || len(a.Acceptance.Alternatives) < 2 || len(a.Acceptance.Insufficient) == 0 {
					t.Fatal("authored proof or official attribution missing", a.ID)
				}
			}
			for _, a := range before.Assertions {
				instructionIDs = append(instructionIDs, a.ID)
			}
			if !reflect.DeepEqual(ids, tc.assertions) || !reflect.DeepEqual(instructionIDs, tc.assertions) || len(book.Operate.Items) != len(tc.triggers) {
				t.Fatal("real assertion decomposition/operating triggers lost", ids, instructionIDs)
			}
			for i, step := range book.Operate.Items {
				if step.OwnerRole != tc.owner || step.Trigger != tc.triggers[i] || step.Timing == nil || len(step.CompletionCriteria) == 0 {
					t.Fatal("authored owner, trigger or timing lost", step)
				}
			}
			if tc.id == "KSI-RPL-TRC" && (!strings.Contains(book.Assertions[0].Acceptance.Alternatives[0].Artifacts[1].Text, "objective versions and comparison conclusions") || !strings.Contains(book.Assertions[0].Acceptance.Insufficient[0], "unexecuted test script")) {
				t.Fatal("recovery proof no longer distinguishes actual execution/objective comparison from plans")
			}
			assertionRaw, err := json.Marshal(book.Assertions)
			if err != nil {
				t.Fatal(err)
			}
			binding := before.Binding
			if binding.CatalogID != p.CatalogID || binding.ProfileRevision != 1 || binding.ScopeRevision != 1 || binding.GuidanceHash != hash || binding.GuidanceRevision != 1 || binding.GuidanceVariant != g.VariantID || binding.AssertionHash != storage.Hash(assertionRaw) || g.Progress == nil || g.Progress.Binding != plan.Binding {
				t.Fatal("exact assertion/profile/scope/release bindings differ", binding)
			}
			var evidence assessment.Record
			call(t, "submit_evidence_artifact", assessment.ArtifactSubmission{ServiceID: service, IdempotencyKey: tc.id, Source: assessment.Ref{ID: "journey-records", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/journeys/" + tc.id, SourceVersion: "synthetic-v1", MediaType: "text/plain", Content: tc.content, Description: "Synthetic incomplete proof", Method: "Read retained synthetic fixture", ObservedAt: time.Now().UTC(), RetrievedAt: time.Now().UTC()}, &evidence)
			gap := assessment.AssertionGap{Kind: "missing", Scope: "Saved service boundary", Period: "Current review period", Reason: tc.gap, NextAction: "Ask the accountable owner for attributable records covering the missing facts and scope."}
			citation := assessment.Citation{Evidence: assessment.Ref{ID: evidence.ID, Revision: evidence.Revision}, StartLine: 1, EndLine: 1}
			results := make([]assessment.AssertionResult, 0, len(ids))
			for _, id := range ids {
				results = append(results, assessment.AssertionResult{AssertionID: id, Outcome: "unknown", Rationale: tc.gap, Gaps: []assessment.AssertionGap{gap}, Citations: []assessment.Citation{citation}})
			}
			var receipt assessment.RuleReceipt
			call(t, "submit_rule_result", assessment.RuleResultSubmission{ServiceID: service, RequirementID: tc.id, IdempotencyKey: "assess-" + tc.id, Binding: binding, RequestedState: "partial", Explanation: tc.gap, Assertions: results}, &receipt)
			if receipt.Review != "pending" || receipt.Work.State != "partial" {
				t.Fatal("incomplete evidence bypassed human review", receipt)
			}
			var after assessment.RuleInstructions
			call(t, "get_rule_instructions", adapter.RequirementInput{ServiceID: service, RequirementID: tc.id}, &after)
			get(t, "/pages/requirement/"+tc.id, &page)
			if !reflect.DeepEqual(after.Guidance, page.Guidance) || !reflect.DeepEqual(after.EvidenceCoverage, page.EvidenceCoverage) || !reflect.DeepEqual(binding, after.Binding) || after.EvidenceCoverage == nil || len(after.EvidenceCoverage.Assertions) != len(ids) {
				t.Fatal("post-submission transport parity or fixed-revision binding lost")
			}
			for _, row := range after.EvidenceCoverage.Assertions {
				if !reflect.DeepEqual(row.Gaps, []assessment.AssertionGap{gap}) || !reflect.DeepEqual(row.Citations, []assessment.Citation{citation}) {
					t.Fatal("exact evidence citation or unresolved gap lost", row)
				}
			}
		})
	}
	var reviews int
	if err := guidance.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE service_id=$1 AND kind='review'`, service).Scan(&reviews); err != nil || reviews != 0 {
		t.Fatal("guidance review or synthetic evidence created human approval", reviews, err)
	}
}
