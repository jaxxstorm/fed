package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestOrganizationRolesPolicyAndRevocation(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	first, err := Admit(ctx, db, AdmissionConfig{AdminEmail: "admin@example.com"}, VerifiedIdentity{Issuer: "issuer", Subject: "admin", Email: "admin@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	a := Actor{ID: first.IdentityID, OrganizationID: first.OrganizationID, Kind: "human"}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('svc',$1,'Service')`, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	org, err := GetOrganization(ctx, db, a)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := Admit(ctx, db, AdmissionConfig{}, VerifiedIdentity{Issuer: "issuer", Subject: "stale", Email: "stale@outside.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	org, err = UpdateOrganization(ctx, db, a, OrganizationUpdate{Name: "Example", AllowedDomains: []string{"EXAMPLE.com"}, ExpectedRevision: org.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: stale.IdentityID, ExpectedRevision: stale.Revision, Operation: "approve", Role: "viewer"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("stale policy approval", err)
	}
	if _, err = UpdateOrganization(ctx, db, a, OrganizationUpdate{Name: "Example", AllowedDomains: []string{"outside.com"}, ExpectedRevision: org.Revision}); !errors.Is(err, ErrLastAdmin) {
		t.Fatal("policy lockout accepted", err)
	}
	if _, err = Admit(ctx, db, AdmissionConfig{}, VerifiedIdentity{Issuer: "issuer", Subject: "outside", Email: "person@outside.com", EmailVerified: true}); !errors.Is(err, ErrForbidden) {
		t.Fatal("outside admitted", err)
	}
	second, err := Admit(ctx, db, AdmissionConfig{}, VerifiedIdentity{Issuer: "issuer", Subject: "second", Email: "second@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	b := Actor{ID: second.IdentityID, OrganizationID: second.OrganizationID, Kind: "human"}
	for _, role := range []string{"viewer", "user", "admin"} {
		op := "role"
		if second.State == "pending" {
			op = "approve"
		}
		second, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: b.ID, ExpectedRevision: second.Revision, Operation: op, Role: role})
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"read", "edit", "check", "connect", "review", "export", "admin"} {
			allowed := role == "admin" || action == "read" || (role == "user" && action != "admin")
			if got := Authorize(ctx, db, b, "svc", action); (got == nil) != allowed {
				t.Fatalf("%s %s: %v", role, action, got)
			}
		}
	}
	token, err := IssueToken(ctx, db, b, "svc", []string{"read", "check"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := TokenActor(ctx, db, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ListMembers(ctx, db, agent, 50, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("agent managed organization", err)
	}
	// Even a stale legacy wildcard grant cannot restore demoted human privileges.
	if _, err = db.Exec(ctx, `INSERT INTO permissions VALUES($1,'*','admin')`, b.ID); err != nil {
		t.Fatal(err)
	}
	second, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: b.ID, ExpectedRevision: second.Revision, Operation: "role", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, db, b, "svc", "edit"); !errors.Is(err, ErrForbidden) {
		t.Fatal("legacy grant bypass", err)
	}
	if err = Authorize(ctx, db, agent, "svc", "check"); !errors.Is(err, ErrForbidden) {
		t.Fatal("issued token survived demotion", err)
	}
	if _, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: b.ID, ExpectedRevision: second.Revision - 1, Operation: "disable"}); !errors.Is(err, ErrMembershipConflict) {
		t.Fatal("stale revision", err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO organizations VALUES('other','Other'); INSERT INTO services(id,organization_id,name) VALUES('other-svc','other','Other')`); err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, db, a, "other-svc", "read"); !errors.Is(err, ErrForbidden) {
		t.Fatal("cross-organization", err)
	}
	resolved, err := ResolveAdministrator(ctx, db, "issuer", "", "ADMIN@example.com")
	if err != nil || resolved.ID != a.ID {
		t.Fatal("email maintenance resolution", err)
	}
}

func TestConcurrentAdministratorsAndRecovery(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := BootstrapAdmin(ctx, db, "issuer", "first")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Admit(ctx, db, AdmissionConfig{}, VerifiedIdentity{Issuer: "issuer", Subject: "second", Email: "second@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	m, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: m.IdentityID, ExpectedRevision: m.Revision, Operation: "approve", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	b := Actor{ID: m.IdentityID, OrganizationID: m.OrganizationID, Kind: "human"}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, in := range []struct {
		actor    Actor
		target   string
		revision int
	}{{a, b.ID, m.Revision}, {b, a.ID, 1}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ChangeMembership(ctx, db, in.actor, MembershipChange{IdentityID: in.target, ExpectedRevision: in.revision, Operation: "disable"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent admin removal", successes)
	}
	// Explicit owner-authorized recovery, never a bootstrap re-grant.
	if err = RecoverAdministrator(ctx, db, a.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = ActiveMember(ctx, db, a); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE organization_memberships SET role='viewer' WHERE identity_id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = BootstrapAdmin(ctx, db, "issuer", "first"); err != nil {
		t.Fatal(err)
	}
	after, err := Member(ctx, db, a.ID)
	if err != nil || after.Role != "viewer" {
		t.Fatal("restart restored admin", after, err)
	}
	if _, err = BootstrapAdmin(ctx, db, "issuer", "new-subject"); !errors.Is(err, ErrForbidden) {
		t.Fatal("bootstrap added admin", err)
	}
	var audits int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM organization_audit_events WHERE action='membership.operator_recovered'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("missing recovery audit", audits, err)
	}
}
