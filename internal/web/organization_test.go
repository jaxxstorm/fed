package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestOrganizationAPIApprovalAndCSRF(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	m, err := auth.Admit(ctx, db, auth.AdmissionConfig{}, auth.VerifiedIdentity{Issuer: "test", Subject: "member", Email: "member@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	member := auth.Actor{ID: m.IdentityID, OrganizationID: m.OrganizationID, Kind: "human"}
	app := App{Store: &assessment.Store{DB: db}}
	call := func(a auth.Actor, method, path, body, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "ksi_dev_csrf", Value: "csrf"})
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		b := auth.Browser{DB: db, Dev: &a}
		b.Protect(app.Handler()).ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/api/ui/v1/bootstrap", "/api/ui/v1/organization", "/api/ui/v1/members"} {
		w := call(member, "GET", path, "", "")
		if w.Code != 403 || !strings.Contains(w.Body.String(), "membership_pending") {
			t.Fatal("pending leak", path, w.Code)
		}
	}
	body, _ := json.Marshal(auth.MembershipChange{IdentityID: member.ID, ExpectedRevision: 1, Operation: "approve", Role: "viewer"})
	if w := call(admin, "POST", "/api/ui/v1/members", string(body), ""); w.Code != 403 {
		t.Fatal("missing CSRF", w.Code)
	}
	if w := call(admin, "POST", "/api/ui/v1/members", string(body), "csrf"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(admin, "POST", "/api/ui/v1/members", string(body), "csrf"); w.Code != 409 {
		t.Fatal("stale approval", w.Code)
	}
	if w := call(member, "GET", "/api/ui/v1/organization", "", ""); w.Code != 403 {
		t.Fatal("viewer policy access", w.Code)
	}
	w := call(member, "GET", "/api/ui/v1/bootstrap", "", "")
	var bootstrap UIBootstrap
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &bootstrap) != nil || bootstrap.Role != "viewer" || bootstrap.CanManageOrganization || bootstrap.CanCreate {
		t.Fatal("viewer bootstrap", w.Code, w.Body.String())
	}
	if w = call(admin, "GET", "/api/ui/v1/members", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "member@example.com") {
		t.Fatal("member list", w.Code)
	}
	if w = call(admin, "POST", "/api/ui/v1/organization", `{"name":"Bad","allowed_domains":[],"expected_revision":1,"unexpected":true}`, "csrf"); w.Code != 400 {
		t.Fatal("unknown input accepted", w.Code)
	}
}
