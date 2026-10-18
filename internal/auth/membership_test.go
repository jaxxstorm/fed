package auth

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestAdmissionApprovalAndLastAdmin(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	cfg := AdmissionConfig{AdminEmail: "admin@example.com"}
	if err := CheckBootstrap(ctx, db, AdmissionConfig{}); err == nil {
		t.Fatal("fresh installation without bootstrap accepted")
	}
	identity := VerifiedIdentity{Issuer: "https://issuer.example", Subject: "admin", Email: "admin@example.com", EmailVerified: true}
	admin, err := Admit(ctx, db, cfg, identity)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Role != "admin" || admin.State != "active" {
		t.Fatal(admin)
	}
	if err := CheckBootstrap(ctx, db, AdmissionConfig{}); err != nil {
		t.Fatal("established installation requires bootstrap config", err)
	}
	a := Actor{ID: admin.IdentityID, OrganizationID: admin.OrganizationID, Kind: "human"}
	identity.Subject, identity.Email = "user", "user@example.com"
	pending, err := Admit(ctx, db, cfg, identity)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Role != "" || pending.State != "pending" {
		t.Fatal("not jailed", pending)
	}
	again, err := Admit(ctx, db, cfg, identity)
	if err != nil || again.IdentityID != pending.IdentityID {
		t.Fatal("duplicate pending", err)
	}
	user := Actor{ID: pending.IdentityID, OrganizationID: pending.OrganizationID, Kind: "human"}
	if _, err = ActiveMember(ctx, db, user); !errors.Is(err, ErrMembershipPending) {
		t.Fatal("pending allowed", err)
	}
	approved, err := ChangeMembership(ctx, db, a, MembershipChange{IdentityID: user.ID, ExpectedRevision: 1, Operation: "approve", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ActiveMember(ctx, db, user); err != nil {
		t.Fatal(err)
	}
	if _, err = ChangeMembership(ctx, db, user, MembershipChange{IdentityID: a.ID, ExpectedRevision: 1, Operation: "disable"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer managed users", err)
	}
	if _, err = ChangeMembership(ctx, db, a, MembershipChange{IdentityID: a.ID, ExpectedRevision: 1, Operation: "disable"}); !errors.Is(err, ErrLastAdmin) {
		t.Fatal("last admin removed", err)
	}
	disabled, err := ChangeMembership(ctx, db, a, MembershipChange{IdentityID: user.ID, ExpectedRevision: approved.Revision, Operation: "disable"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Admit(ctx, db, cfg, identity); !errors.Is(err, ErrForbidden) {
		t.Fatal("disabled reentered", err)
	}
	enabled, err := ChangeMembership(ctx, db, a, MembershipChange{IdentityID: user.ID, ExpectedRevision: disabled.Revision, Operation: "enable"})
	if err != nil || enabled.State != "pending" || enabled.Role != "" {
		t.Fatal("enabled without approval", enabled, err)
	}
	identity.Subject = "different"
	if _, err = Admit(ctx, db, cfg, identity); !errors.Is(err, ErrMembershipConflict) {
		t.Fatal("email rebound", err)
	}
	identity.Subject = "user"
	identity.Email = "renamed@example.com"
	renamed, err := Admit(ctx, db, cfg, identity)
	if err != nil || renamed.IdentityID != user.ID || renamed.State != "pending" {
		t.Fatal("email change altered identity or approval", renamed, err)
	}
	identity.Email = "admin@example.com"
	if _, err = Admit(ctx, db, cfg, identity); !errors.Is(err, ErrMembershipConflict) {
		t.Fatal("email collision", err)
	}
}

func TestConcurrentEmailBootstrap(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := Admit(ctx, db, AdmissionConfig{AdminEmail: "admin@example.com"}, VerifiedIdentity{Issuer: "issuer", Subject: "admin", Email: "admin@example.com", EmailVerified: true})
			if err != nil || m.State != "active" {
				t.Error(m, err)
			}
		}()
	}
	wg.Wait()
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM organization_memberships WHERE role='admin'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	m, err := Admit(ctx, db, AdmissionConfig{AdminEmail: "other@example.com"}, VerifiedIdentity{Issuer: "issuer", Subject: "other", Email: "other@example.com", EmailVerified: true})
	if err != nil || m.State != "pending" {
		t.Fatal("configuration granted another admin", m, err)
	}
}
