package mcp_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/checks"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReviewedReleaseBrowserMCPJourneys(t *testing.T) {
	for _, rid := range []string{"AFC-CSO-INB", "KSI-CNA-RNT"} {
		t.Run(rid, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			db := testutil.Database(t)
			store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
			admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
			if err != nil {
				t.Fatal(err)
			}
			service, err := store.CreateService(ctx, admin, "Reviewed journey")
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
			cid, err := catalog.Import(ctx, db, rules, schema, catalog.PinnedCommit)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.SaveProfile(ctx, admin, service.ID, 0, catalog.Profile{CatalogID: cid, Type: "20x", Path: "Program", Class: "A", Context: "initial", Date: "2026-10-16"}); err != nil {
				t.Fatal(err)
			}
			if _, err = store.SaveScope(ctx, admin, service.ID, 0, assessment.Scope{Description: "Fixture boundary", Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}); err != nil {
				t.Fatal(err)
			}
			if _, err = store.SaveEvidenceSource(ctx, admin, service.ID, "records", 0, assessment.EvidenceSource{Type: "operational_records", Name: "Dated records", Locator: "https://records.example/", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
				t.Fatal(err)
			}
			token, err := auth.IssueToken(ctx, db, admin, service.ID, []string{"read", "edit", "export"}, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(adapter.Handler(store, nil))
			defer server.Close()
			client := sdk.NewClient(&sdk.Implementation{Name: "journey", Version: "1"}, nil)
			session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			call := func(name string, in any) *sdk.CallToolResult {
				t.Helper()
				r, e := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: in})
				if e != nil {
					t.Fatal(e)
				}
				return r
			}
			instructions := unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: rid}))
			end := time.Now().UTC().Add(-time.Minute)
			period := assessment.AssessmentPeriod{Start: end.Add(-24 * time.Hour), End: end}
			content := "Security inbox receives messages at security@example.com.\nDated test message received and acknowledged by the on-call security reviewer."
			if rid == "KSI-CNA-RNT" {
				content = "Policy says network isolation is required.\nObserved configuration contradicts the policy; inaccessible partitions remain unknown."
			}
			ev := unpack[assessment.Record](t, call("submit_evidence_artifact", assessment.ArtifactSubmission{ServiceID: service.ID, IdempotencyKey: "record", Source: assessment.Ref{ID: "records", Revision: 1}, ScopeRevision: 1, Locator: "https://records.example/event", SourceVersion: "v1", MediaType: "text/plain", Content: content, ObservedAt: end.Add(-time.Hour), RetrievedAt: end, Method: "Read dated operating record", Description: "Dated investigation evidence"}))
			citation := assessment.Citation{Evidence: assessment.Ref{ID: ev.ID, Revision: ev.Revision}, StartLine: 1, EndLine: 2}
			known := func(text string) assessment.NarrativeField {
				return assessment.NarrativeField{State: "known", Text: text}
			}
			narrative := assessment.ImplementationNarrative{Description: known("The provider operates the declared process within this saved boundary."), ResponsibleRoles: known("Security on-call team"), Mechanisms: known("Documented configuration and dated operating record"), Procedures: known("Review incoming events, assign an owner, and record acknowledgement."), Frequency: known("Evidence covers one dated event; broader frequency remains unverified."), Boundary: known("The saved service boundary; no inference for other environments.")}
			v := assessment.ImplementationAssessment{Version: assessment.ImplementationAssessmentVersion, Period: period, Narrative: narrative}
			for _, assertion := range instructions.Assertions {
				support := assessment.EvidenceSupport{Outcome: "unknown", Rationale: "One dated sample does not establish whole-period performance", Citations: []assessment.Citation{citation}, Limitations: []string{"Full operating population is unknown"}}
				implementation := support
				if rid == "KSI-CNA-RNT" {
					implementation.Outcome = "contradicted"
					implementation.Rationale = "Observed configuration conflicts with the documented isolation intent"
				}
				v.Assertions = append(v.Assertions, assessment.ImplementationAssertion{AssertionID: assertion.ID, Claim: "Service-specific investigation with explicit limits", Implementation: implementation, Operating: support, Sample: assessment.OperationalSample{Method: "Read one dated record", SampleSize: 1, PopulationUnknownReason: "Full population not established", Coverage: "sample", CoverageReason: "Only one event inspected", Events: []assessment.OperatingEvent{{Citation: citation, OccurredAt: end.Add(-time.Hour), Description: "Operating event inspected"}}}})
			}
			unpack[assessment.ImplementationReceipt](t, call("submit_implementation_assessment", assessment.ImplementationSubmission{ServiceID: service.ID, RequirementID: rid, IdempotencyKey: "assessment", Binding: instructions.Binding, Assessment: v}))
			if rid == "KSI-CNA-RNT" {
				if _, err = store.SaveMeasure(ctx, admin, service.ID, "network", rid, 0, assessment.Measure{Name: "Network inventory", Rationale: "Incomplete discovery", FreshnessHours: 24, ScopeRevision: 1, CheckID: "fixture"}); err != nil {
					t.Fatal(err)
				}
				partial := observation.Submission{ServiceID: service.ID, ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "partial-inventory", Target: observation.Target{Kind: "aws", AccountID: "123456789012", Region: "us-east-1", ResourceID: "arn:aws:eks:us-east-1:123456789012:cluster/fixture"}, ObservedAt: end, Method: "DescribeCluster", ToolVersion: "fixture/v1", Coverage: observation.Coverage{Expected: 1, Observed: 0, Complete: false, State: "error", Error: "access_denied"}, Artifact: observation.Artifact{Schema: "aws.eks/v1", Configuration: map[string]json.RawMessage{"public_access": json.RawMessage(`null`)}}, Links: []observation.Link{{RequirementID: rid, Measure: observation.Ref{ID: "network", Revision: 1}, MappingVersion: "agent/v1", Rationale: "Access gap, not passing coverage"}}}
				partial.Artifact.Schema = "aws.eks.endpoint/v1"
				unpack[observation.Receipt](t, call("ingest_observation", partial))
				instructions = unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: rid}))
				unpack[assessment.Record](t, call("save_assessment_issue", assessment.AssessmentIssueChange{ServiceID: service.ID, RequirementID: rid, ID: "investigation", Issue: assessment.AssessmentIssue{Binding: instructions.Binding, Explanation: "Obtain missing partition evidence and investigate contradictory configuration", State: "open"}}))
			}
			instructions = unpack[assessment.RuleInstructions](t, call("get_rule_instructions", adapter.RequirementInput{ServiceID: service.ID, RequirementID: rid}))
			target := unpack[assessment.AssessmentReviewTarget](t, call("get_assessment_review_target", adapter.RequirementInput{ServiceID: service.ID, RequirementID: rid}))
			app := &web.App{Store: store, Runner: &checks.Runner{DB: db, Artifacts: store.Artifacts}}
			browser := &auth.Browser{DB: db, Dev: &admin}
			handler := browser.Protect(app.Handler())
			browserView := httptest.NewRecorder()
			handler.ServeHTTP(browserView, httptest.NewRequest("GET", "/api/ui/v1/services/"+service.ID+"/pages/requirement/"+rid, nil))
			if browserView.Code != 200 || !strings.Contains(browserView.Body.String(), "Security on-call team") {
				t.Fatalf("browser omitted agent narrative: %d %s", browserView.Code, browserView.Body.String())
			}
			encode := func(v any) string {
				b, e := json.Marshal(v)
				if e != nil {
					t.Fatal(e)
				}
				return string(b)
			}
			post := func(action string, values url.Values) {
				t.Helper()
				r := httptest.NewRequest("POST", "/services/"+service.ID+"/actions/"+action, strings.NewReader(values.Encode()))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				r.Header.Set("X-CSRF-Token", "csrf")
				r.AddCookie(&http.Cookie{Name: "ksi_dev_csrf", Value: "csrf"})
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code < 300 || w.Code >= 400 {
					t.Fatalf("browser action %s: %d %s", action, w.Code, w.Body.String())
				}
			}
			for _, capacity := range []string{"organizational_adoption", "internal_reviewer"} {
				post("assessment-decision", url.Values{"requirement_id": {rid}, "assessment": {encode(target.Assessment)}, "binding": {encode(instructions.Binding)}, "dependency_hash": {target.DependencyHash}, "capacity": {capacity}, "conclusion": {"accepted"}, "procedures": {"Reviewed the implementation and explicit evidence limits"}, "sampling": {"One dated record"}, "notes": {"Accurate incomplete assessment, not a compliance conclusion"}})
			}
			post("service-context", url.Values{"expected_revision": {"0"}, "description": {"Private service description"}, "boundary": {"Saved boundary"}, "responsibilities": {"Provider operations; customer configuration responsibilities remain explicit"}, "contacts": {"owner@example.com"}, "deliverables": {encode([]assessment.ReleaseDeliverable{{RequirementID: rid, Description: "Dated operating record", Evidence: citation.Evidence}})}})
			post("prepare-release", url.Values{"key": {"release"}, "period_start": {period.Start.Format(time.RFC3339Nano)}, "period_end": {period.End.Format(time.RFC3339Nano)}, "public": {encode(assessment.PublicRelease{ServiceName: "Reviewed service", Summary: "Explicitly incomplete report", Contacts: []string{}, Requirements: []assessment.PublicRequirement{{RequirementID: rid, Summary: "Evidence limitations and ongoing investigation remain."}}})}})
			releases, err := store.ListReleases(ctx, admin, service.ID, 10, 0)
			if err != nil || len(releases) != 1 {
				t.Fatalf("prepared releases: %v", err)
			}
			release := releases[0]
			if !release.Manifest.Incomplete || len(release.Manifest.Blockers) != 0 {
				t.Fatalf("incomplete reviewed report readiness: %v", release.Manifest.Blockers)
			}
			found := false
			for _, requirement := range release.Manifest.Requirements {
				if requirement.Requirement.Requirement.ID == rid {
					found = true
					if requirement.Implementation == nil || requirement.Reviews == nil || requirement.Reviews.Adoption != "accepted" || requirement.Reviews.Review != "accepted" {
						t.Fatal("reviewed narrative omitted from release")
					}
					if rid == "KSI-CNA-RNT" && (len(requirement.Issues) != 1 || len(requirement.Collection.Errors) == 0 || requirement.Implementation.Assessment.Assertions[0].Implementation.Outcome != "contradicted") {
						t.Fatal("technical release erased access gap, open issue or contradiction")
					}
				}
			}
			if !found {
				t.Fatal("requirement missing from release")
			}
			var archive bytes.Buffer
			if err = store.WriteReleaseArchive(ctx, &archive, release.Manifest); err != nil {
				t.Fatal(err)
			}
			if _, err = zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len())); err != nil {
				t.Fatal(err)
			}
			post("trust-config", url.Values{"slug": {"reviewed-service"}, "enabled": {"yes"}})
			for index, action := range []string{"approve", "publish"} {
				revision := "1"
				if index == 1 {
					revision = "2"
				}
				post("release-transition", url.Values{"id": {release.ID}, "expected_revision": {revision}, "manifest_hash": {release.ManifestHash}, "audience_hash": {release.AudienceHash}, "transition": {action}, "key": {action}, "reason": {"Reviewed exact incomplete report and disclosure"}, "confirmed": {"yes"}})
			}
			public, err := store.PublicReleaseIndex(ctx, "reviewed-service", 10, 0)
			if err != nil || len(public) != 1 || !public[0].Incomplete || strings.Contains(encode(public), "Private service description") {
				t.Fatalf("public projection: %v", err)
			}
			// Optional fixture-only artifacts for the genuine human comprehension
			// walkthrough. Passing this test is not human acceptance.
			if output := os.Getenv("KSI_ACCEPTANCE_OUTPUT"); output != "" {
				dir := filepath.Join(output, rid)
				if err = os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "assessment.zip"), archive.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				publicPage := httptest.NewRecorder()
				app.TrustHandler(browser).ServeHTTP(publicPage, httptest.NewRequest("GET", "/trust/reviewed-service", nil))
				if publicPage.Code != 200 {
					t.Fatal("public acceptance preview unavailable")
				}
				if err = os.WriteFile(filepath.Join(dir, "public.html"), publicPage.Body.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
			}
			member, err := auth.Admit(ctx, db, auth.AdmissionConfig{}, auth.VerifiedIdentity{Issuer: "test", Subject: "reader", Email: "reader@example.com", EmailVerified: true})
			if err != nil {
				t.Fatal(err)
			}
			reader := auth.Actor{ID: member.IdentityID, OrganizationID: member.OrganizationID, Kind: "human"}
			request, err := store.RequestTrustAccess(ctx, reader, "reviewed-service", release.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.DecideTrustAccess(ctx, admin, service.ID, request.ID, "reader", time.Now().Add(time.Hour), true); err != nil {
				t.Fatal(err)
			}
			restricted, err := store.RestrictedRelease(ctx, reader, "reviewed-service", release.ID)
			if err != nil || restricted.ManifestHash != release.ManifestHash {
				t.Fatalf("restricted delivery: %v", err)
			}
			page := unpack[adapter.Page](t, call("list_assessment_releases", adapter.ListInput{ServiceID: service.ID, Limit: 10}))
			if !strings.Contains(encode(page.Items), release.ManifestHash) {
				t.Fatal("MCP omitted frozen browser release")
			}
		})
	}
}
