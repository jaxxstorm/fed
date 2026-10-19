package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func comparisonFixture() SourceComparison {
	f := ReviewFinding{Finding: "Synthetic fixture comparison against the bound policy statement.", Citations: []string{"policy"}}
	return SourceComparison{Applicability: f, Obligations: f, Prerequisites: f, Evidence: f, Timing: f, Limitations: []string{"Synthetic test guidance, not a review of production policy."}, Uncertainties: []ReviewFinding{}}
}

func sourceFixture(t *testing.T) (Package, catalog.Snapshot, fstest.MapFS, fstest.MapFS) {
	t.Helper()
	p := draftFixture()
	commit, site := strings.Repeat("a", 40), strings.Repeat("b", 40)
	payload, _ := json.Marshal(map[string]any{"KSI": map[string]any{"TEST": map[string]any{"status": "stable", "indicators": map[string]any{"RULE": map[string]any{"name": "Test rule", "statement": p.Playbooks[0].OfficialStatement}}}}})
	schema := []byte(`{"type":"object"}`)
	snapshot, err := catalog.Parse(payload, schema, commit)
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]any{"rules": map[string]string{"repo": "fedramp/rules", "sha": commit}, "site": map[string]string{"repo": "fedramp/2026", "sha": site}})
	rules := fstest.MapFS{"rules.json": {Data: payload}}
	markdown := fstest.MapFS{"_sources.json": {Data: metadata}, "definitions.md": {Data: []byte("# Definition\nSynthetic definition.\n")}}
	p.CatalogID = snapshot.ID
	p.Sources = map[string]Source{"rules": {Repository: "fedramp/rules", Commit: commit}, "site": {Repository: "fedramp/2026", Commit: site}}
	p.Citations = []Citation{{ID: "policy", Source: "rules", Path: "rules.json", Anchor: "/KSI/TEST/indicators/RULE/statement", Excerpt: p.Playbooks[0].OfficialStatement, ExcerptHash: storage.Hash([]byte(p.Playbooks[0].OfficialStatement)), ContentHash: storage.Hash(payload)}}
	i, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	base := p.Playbooks[0]
	p.Playbooks = nil
	for _, v := range i.Variants {
		b := base
		b.ID = v.ID
		b.VariantIDs = []string{v.ID}
		b.OfficialStatement = v.Statement
		if v.Applicability == "not_applicable" {
			b.Kind = "excluded"
		} else if v.Applicability == "unresolved" {
			b.Kind = "needs_decision"
			b.DecisionQuestion = "Does the agency path include this indicator?"
			b.DecisionOwnerRole = "Compliance owner"
		}
		if b.Kind != "actionable" {
			b.Assertions = nil
			b.Prerequisites = nil
			b.Setup = Steps{}
			b.Operate = Steps{}
			b.Collect = Steps{}
			b.ReviewSteps = Steps{}
		}
		p.Playbooks = append(p.Playbooks, b)
	}
	return p, snapshot, rules, markdown
}

func TestApplicabilityPlansAndCoverage(t *testing.T) {
	p, snapshot, rules, markdown := sourceFixture(t)
	i, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateDraft(p); err != nil {
		t.Fatal(err)
	}
	if err = ValidateInventory(p, i); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for index := range p.Playbooks {
		hash, err := ReviewHash(p, p.Playbooks[index].ID)
		if err != nil {
			t.Fatal(err)
		}
		p.Playbooks[index].Review = &Review{ReviewerID: "test-reviewer", ReviewerType: "agent", Comparison: comparisonFixture(), ReviewedAt: now, ContentHash: hash}
	}
	report, err := Coverage(p, i, rules, markdown, now)
	if err != nil || !report.Complete {
		t.Fatalf("coverage: %+v %v", report, err)
	}
	for _, counts := range report.Profiles {
		if counts["reviewed"] != 1 {
			t.Fatal("profile lost its requirement")
		}
	}
	b := p.Playbooks[0]
	p.Playbooks = p.Playbooks[1:]
	report, err = Coverage(p, i, rules, markdown, now)
	if err != nil || report.Complete || report.Counts["missing"] != 1 {
		t.Fatal("missing variant accepted", report, err)
	}
	b.Kind = "actionable"
	p.Playbooks = append(p.Playbooks, b)
	// At least one explanation variant must not be repurposed as an action plan.
	for index := range p.Playbooks {
		for _, v := range i.Variants {
			if v.ID == p.Playbooks[index].VariantIDs[0] && v.Applicability == "not_applicable" {
				p.Playbooks[index].Kind = "actionable"
			}
		}
	}
	if err = ValidateInventory(p, i); err == nil {
		t.Fatal("wrong applicability kind accepted")
	}
}

func TestEmptyStatementApplicabilityPlanDoesNotInventObligations(t *testing.T) {
	p := draftFixture()
	b := &p.Playbooks[0]
	b.Kind = "needs_decision"
	b.OfficialStatement = ""
	b.Assertions = nil
	b.Prerequisites = nil
	b.Setup = Steps{}
	b.Operate = Steps{}
	b.Collect = Steps{}
	b.ReviewSteps = Steps{}
	b.DecisionQuestion = "Which class-specific obligation applies to this scope?"
	b.DecisionOwnerRole = "Compliance owner"
	if err := ValidateDraft(p); err != nil {
		t.Fatal(err)
	}
	i := catalog.GuidanceInventory{CatalogID: p.CatalogID, Variants: []catalog.GuidanceVariant{{ID: "variant", RequirementID: "RULE", Applicability: "unresolved"}}}
	if err := ValidateInventory(p, i); err != nil {
		t.Fatal(err)
	}
	b.Assertions = draftFixture().Playbooks[0].Assertions
	if err := ValidateDraft(p); err == nil {
		t.Fatal("applicability plan invented obligations")
	}
	b.Assertions = nil
	b.DecisionQuestion = ""
	if err := ValidateDraft(p); err == nil {
		t.Fatal("unresolved applicability without question accepted")
	}
	b.Kind = "excluded"
	b.DecisionOwnerRole = ""
	i.Variants[0].Applicability = "not_applicable"
	if err := ValidateDraft(p); err != nil {
		t.Fatal(err)
	}
	if err := ValidateInventory(p, i); err != nil {
		t.Fatal(err)
	}
}

func TestContentPublicationRequiresAuthenticatedReviewAndIsImmutable(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	p, snapshot, rules, markdown := sourceFixture(t)
	if _, err = catalog.Import(ctx, db, snapshot.Payload, snapshot.Schema, snapshot.Commit); err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db, Rules: rules, Markdown: markdown}
	service := "guidance-test"
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES($1,$2,'Guidance test'),('other-guidance-test',$2,'Other service')`, service, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	admin := a
	token, err := auth.IssueToken(ctx, db, admin, service, []string{"read", "edit"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	a, err = auth.TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, a, "other-guidance-test", p); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("agent escaped service grant", err)
	}
	// Fabricated JSON review metadata is not an authenticated review record.
	for index := range p.Playbooks {
		hash, _ := ReviewHash(p, p.Playbooks[index].ID)
		p.Playbooks[index].Review = &Review{ReviewerID: a.ID, ContentHash: hash, ReviewedAt: time.Now().UTC()}
	}
	if _, err = s.Publish(ctx, a, service, p); !errors.Is(err, ErrReviewRequired) {
		t.Fatal("forged reviews accepted", err)
	}
	for _, b := range p.Playbooks {
		if _, err = s.RecordContentReview(ctx, a, service, p, b.ID, comparisonFixture()); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := s.Publish(ctx, a, service, p)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, err := s.Publish(ctx, a, service, p); err != nil || repeated != hash {
		t.Fatal("publication not idempotent", err)
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.Resolve(snapshot.Requirements, inventory.Profiles[0].Profile)
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.Context(ctx, db, a, service, p.CatalogID, resolved[0], time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if view.State != "reviewed" || view.Label != AgentReviewLabel || view.Caveat != AgentReviewCaveat || view.ReleaseHash != hash || view.Guidance.Playbooks[0].Review.ReviewerType != "agent" || len(view.Guidance.Citations) == 0 {
		t.Fatalf("lost review attribution: %+v", view)
	}
	exported, err := s.ExportContexts(ctx, db, admin, service, p.CatalogID, resolved, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	viewJSON, _ := json.Marshal(view)
	exportJSON, _ := json.Marshal(exported[0])
	if string(viewJSON) != string(exportJSON) {
		t.Fatal("read/export guidance differs")
	}
	if _, err = s.Context(ctx, db, a, "other-guidance-test", p.CatalogID, resolved[0], time.Now().UTC()); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("guidance leaked across service scope", err)
	}
	missing, err := s.Context(ctx, db, admin, "other-guidance-test", p.CatalogID, resolved[0], time.Now().UTC())
	if err != nil || missing.State != "missing" || missing.Label != "" {
		t.Fatal("missing guidance labeled reviewed", missing, err)
	}
	changed := comparisonFixture()
	changed.Timing.Finding = "A different comparison of the same immutable guidance."
	if _, err = s.RecordContentReview(ctx, a, service, p, p.Playbooks[0].ID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("review findings overwritten", err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='readiness.guidance.published'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate publication audit", count, err)
	}
	if _, err = db.Exec(ctx, `UPDATE readiness_service_releases SET content_hash='rewritten'`); err == nil {
		t.Fatal("published history mutable")
	}
	if _, err = db.Exec(ctx, `DELETE FROM readiness_agent_reviews`); err == nil {
		t.Fatal("review history mutable")
	}
	p.Playbooks[0].Purpose.Text += " Updated interpretation."
	if _, err = s.Publish(ctx, a, service, p); !errors.Is(err, ErrReviewRequired) {
		t.Fatal("stale review accepted", err)
	}
	if _, err = s.RecordContentReview(ctx, a, service, p, p.Playbooks[0].ID, comparisonFixture()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, a, service, p); !errors.Is(err, ErrConflict) {
		t.Fatal("existing release overwritten", err)
	}
	if _, err = s.Publish(ctx, admin, "other-guidance-test", p); !errors.Is(err, ErrReviewRequired) {
		t.Fatal("reviews leaked across services", err)
	}
	if err = auth.RevokeToken(ctx, db, admin, service, a.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordContentReview(ctx, a, service, p, p.Playbooks[0].ID, comparisonFixture()); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("revoked agent recorded review", err)
	}
}

func TestSourceComparisonRequiresBoundCitationsAndExplicitLimitations(t *testing.T) {
	p := draftFixture()
	for name, change := range map[string]func(*SourceComparison){
		"missing dimension":    func(c *SourceComparison) { c.Timing = ReviewFinding{} },
		"foreign citation":     func(c *SourceComparison) { c.Evidence.Citations = []string{"unbound"} },
		"missing limitations":  func(c *SourceComparison) { c.Limitations = nil },
		"implicit uncertainty": func(c *SourceComparison) { c.Uncertainties = nil },
		"uncited uncertainty":  func(c *SourceComparison) { c.Uncertainties = []ReviewFinding{{Finding: "Timing is ambiguous."}} },
	} {
		t.Run(name, func(t *testing.T) {
			c := comparisonFixture()
			change(&c)
			if err := ValidateComparison(p, p.Playbooks[0].ID, c); err == nil {
				t.Fatal("incomplete comparison accepted")
			}
		})
	}
	c := comparisonFixture()
	c.Uncertainties = []ReviewFinding{{Finding: "The policy does not specify a numeric deadline; this remains qualified.", Citations: []string{"policy"}}}
	if err := ValidateComparison(p, p.Playbooks[0].ID, c); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageInvalidatesOnlyBooksUsingChangedDefinition(t *testing.T) {
	p, snapshot, rules, markdown := sourceFixture(t)
	i, err := catalog.Inventory(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	text := "Synthetic definition."
	p.Citations = append(p.Citations, Citation{ID: "definition", Source: "site", Path: "definitions.md", Anchor: "definition", Excerpt: text, ExcerptHash: storage.Hash([]byte(text)), ContentHash: storage.Hash(markdown["definitions.md"].Data)})
	p.Playbooks[0].Purpose.Citations = []string{"policy", "definition"}
	now := time.Now().UTC()
	for index := range p.Playbooks {
		hash, err := ReviewHash(p, p.Playbooks[index].ID)
		if err != nil {
			t.Fatal(err)
		}
		comparison := comparisonFixture()
		if index == 0 {
			comparison.Applicability.Citations = []string{"policy", "definition"}
		}
		p.Playbooks[index].Review = &Review{ReviewerID: "fixture", ReviewerType: "agent", Comparison: comparison, ReviewedAt: now, ContentHash: hash}
	}
	markdown["definitions.md"] = &fstest.MapFile{Data: []byte("# Definition\nNew definition.\n")}
	report, err := Coverage(p, i, rules, markdown, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Complete || report.Counts["needs_revalidation"] != 1 || report.Counts["reviewed"] != len(i.Variants)-1 {
		t.Fatalf("incorrect affected guidance: %+v", report)
	}
}
