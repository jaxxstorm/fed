package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestTrustBrowserAdmissionAndReleaseIsolation(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db, Artifacts: storage.Files{Directory: t.TempDir()}}
	service, err := store.CreateService(ctx, admin, "Trust service")
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
	if _, err = store.SaveProfile(ctx, admin, service.ID, 0, catalog.Profile{CatalogID: cid, Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: "2026-10-16"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveScope(ctx, admin, service.ID, 0, assessment.Scope{Description: "Restricted boundary"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveServiceContext(ctx, admin, service.ID, 0, assessment.ServiceContext{Description: "PRIVATE IMPLEMENTATION", Boundary: "Private", Responsibilities: "Provider", Contacts: []string{"private@example.com"}, Deliverables: []assessment.ReleaseDeliverable{}}); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(-time.Minute)
	release, err := store.PrepareAssessmentRelease(ctx, admin, assessment.PrepareReleaseInput{ServiceID: service.ID, IdempotencyKey: "browser", Period: assessment.AssessmentPeriod{Start: end.Add(-time.Hour), End: end}, Public: assessment.PublicRelease{ServiceName: "Public service", Summary: "Incomplete report", Contacts: []string{}, Requirements: []assessment.PublicRequirement{}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ConfigureTrustCentre(ctx, admin, service.ID, "browser-service", true); err != nil {
		t.Fatal(err)
	}
	member, err := auth.Admit(ctx, db, auth.AdmissionConfig{}, auth.VerifiedIdentity{Issuer: "test", Subject: "external", Email: "external@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	reader := auth.Actor{ID: member.IdentityID, OrganizationID: member.OrganizationID, Kind: "human"}
	app := &App{Store: store}
	browser := &auth.Browser{DB: db, Dev: &reader}
	handler := app.TrustHandler(browser)
	call := func(method, path, body, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "ksi_dev_csrf", Value: "csrf"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/trust/browser-service", "/trust/browser-service?format=json"} {
		w := call("GET", path, "", "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), release.ID) {
			t.Fatalf("unpublished public leak: %d %s", w.Code, w.Body.String())
		}
	}
	if w := call("GET", "/trust/browser-service/access", "", ""); w.Code != 200 {
		t.Fatalf("pending reader admission: %d", w.Code)
	}
	if w := call("POST", "/trust/browser-service/access", "", ""); w.Code != 403 {
		t.Fatalf("missing CSRF: %d", w.Code)
	}
	if w := call("POST", "/trust/browser-service/access", "", "csrf"); w.Code != 200 {
		t.Fatalf("request access: %d %s", w.Code, w.Body.String())
	}
	requests, err := store.TrustRequests(ctx, admin, service.ID, 10, 0)
	if err != nil || len(requests) != 1 {
		t.Fatalf("requests: %v", err)
	}
	if err = store.DecideTrustAccess(ctx, admin, service.ID, requests[0].ID, "reader", time.Now().Add(time.Hour), true); err != nil {
		t.Fatal(err)
	}
	url := "/trust/browser-service/releases/" + release.ID + "/"
	if w := call("GET", url+"?format=json", "", ""); w.Code != 403 {
		t.Fatalf("unpublished restricted release: %d", w.Code)
	}
	transition := assessment.ReleaseTransition{ServiceID: service.ID, ID: release.ID, ExpectedRevision: 1, ManifestHash: release.ManifestHash, AudienceHash: release.AudienceHash, Action: "approve", Reason: "Reviewed incomplete report", IdempotencyKey: "approve"}
	if _, err = store.TransitionRelease(ctx, admin, transition); err != nil {
		t.Fatal(err)
	}
	transition.Action = "publish"
	transition.ExpectedRevision = 2
	transition.IdempotencyKey = "publish"
	if _, err = store.TransitionRelease(ctx, admin, transition); err != nil {
		t.Fatal(err)
	}
	w := call("GET", url+"?format=json", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "PRIVATE IMPLEMENTATION") || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("restricted manifest: %d", w.Code)
	}
	w = call("GET", "/trust/browser-service?format=json", "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), "private@example.com") {
		t.Fatal("public projection leaked private content")
	}
	if w = call("GET", url+"evidence/"+strings.Repeat("a", 64), "", ""); w.Code != 404 {
		t.Fatalf("unlisted artifact: %d", w.Code)
	}
	if _, err = db.Exec(ctx, `UPDATE trust_access_grants SET expires_at=now()-interval '1 second',granted_at=now()-interval '1 hour' WHERE service_id=$1`, service.ID); err != nil {
		t.Fatal(err)
	}
	if w = call("GET", url+"?format=zip", "", ""); w.Code != 403 {
		t.Fatalf("expired archive grant: %d", w.Code)
	}
	other, err := store.CreateService(ctx, admin, "Other trust service")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ConfigureTrustCentre(ctx, admin, other.ID, "other-service", true); err != nil {
		t.Fatal(err)
	}
	if w = call("GET", "/trust/other-service/releases/"+release.ID+"/?format=json", "", ""); w.Code != 403 {
		t.Fatalf("cross-service release access: %d", w.Code)
	}
	if _, err = db.Exec(ctx, `UPDATE trust_access_grants SET expires_at=now()+interval '1 hour' WHERE service_id=$1`, service.ID); err != nil {
		t.Fatal(err)
	}
	session := "trust-reader-session"
	if _, err = db.Exec(ctx, `INSERT INTO sessions(hash,identity_id,csrf_token,expires_at,policy_fingerprint) VALUES($1,$2,'csrf',now()+interval '1 hour',$3)`, storage.Hash([]byte(session)), reader.ID, (auth.AdmissionConfig{}).Fingerprint()); err != nil {
		t.Fatal(err)
	}
	actualHandler := app.TrustHandler(&auth.Browser{DB: db})
	callSession := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", url+"?format=json", nil)
		req.AddCookie(&http.Cookie{Name: "ksi_session", Value: session})
		rec := httptest.NewRecorder()
		actualHandler.ServeHTTP(rec, req)
		return rec
	}
	if w = callSession(); w.Code != 200 {
		t.Fatalf("verified reader session: %d", w.Code)
	}
	if _, err = db.Exec(ctx, `DELETE FROM sessions WHERE hash=$1`, storage.Hash([]byte(session))); err != nil {
		t.Fatal(err)
	}
	if w = callSession(); w.Code == 200 {
		t.Fatal("revoked session retained restricted access")
	}
	transition.Action = "withdraw"
	transition.ExpectedRevision = 3
	transition.IdempotencyKey = "withdraw"
	if _, err = store.TransitionRelease(ctx, admin, transition); err != nil {
		t.Fatal(err)
	}
	if w = call("GET", url+"?format=json", "", ""); w.Code != 403 {
		t.Fatalf("withdrawn release delivered: %d", w.Code)
	}
	if w = call("GET", "/trust/browser-service?format=json", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "withdrawn") || strings.Contains(w.Body.String(), "Incomplete report") {
		t.Fatalf("withdrawal public projection: %d %s", w.Code, w.Body.String())
	}
}
