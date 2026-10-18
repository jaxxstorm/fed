package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/jaxxstorm/fed/internal/storage"
)

type Membership struct {
	IdentityID     string `json:"identity_id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	State          string `json:"state"`
	Revision       int    `json:"revision"`
	EmailVerified  bool   `json:"-"`
	LegacySubject  bool   `json:"-"`
}

const memberColumns = `m.identity_id,m.organization_id,coalesce(m.email,''),i.name,coalesce(m.role,''),m.state,m.revision,m.email_verified,m.legacy_subject`

func scanMember(row pgx.Row) (Membership, error) {
	var m Membership
	err := row.Scan(&m.IdentityID, &m.OrganizationID, &m.Email, &m.Name, &m.Role, &m.State, &m.Revision, &m.EmailVerified, &m.LegacySubject)
	return m, err
}

func Member(ctx context.Context, q Querier, id string) (Membership, error) {
	return scanMember(q.QueryRow(ctx, `SELECT `+memberColumns+` FROM organization_memberships m JOIN identities i ON i.id=m.identity_id WHERE m.identity_id=$1 AND i.kind='human' AND i.organization_id=m.organization_id AND m.category='member'`, id))
}

func memberEligible(m Membership, domains []string) bool {
	return EmailAllowed(m.Email, m.EmailVerified, domains) || (m.LegacySubject && m.Email == "" && len(domains) == 0)
}

// ActiveMember is also used at service boundaries, not just HTTP middleware.
func ActiveMember(ctx context.Context, q Querier, a Actor) (Membership, error) {
	// Transactions retain this shared lock through their durable commit, fencing
	// concurrent policy/membership changes which take the organization write lock.
	var domains []string
	lock, err := authorizationLock(ctx, q)
	if err != nil {
		return Membership{}, ErrForbidden
	}
	if err := q.QueryRow(ctx, `SELECT allowed_domains FROM organization_settings WHERE organization_id=$1`+lock, a.OrganizationID).Scan(&domains); err != nil {
		return Membership{}, ErrForbidden
	}
	m, err := Member(ctx, q, a.ID)
	if err != nil || a.Kind != "human" || m.OrganizationID != a.OrganizationID {
		return Membership{}, ErrForbidden
	}
	if !memberEligible(m, domains) {
		return Membership{}, ErrForbidden
	}
	if m.State == "pending" {
		return m, ErrMembershipPending
	}
	if m.State != "active" {
		return Membership{}, ErrForbidden
	}
	return m, nil
}

type VerifiedIdentity struct {
	Issuer, Subject, Email, Name string
	EmailVerified                bool
	Claims                       map[string]any
}

func (c AdmissionConfig) Fingerprint() string {
	b, _ := json.Marshal([]string{c.OrganizationClaim, c.OrganizationValue})
	return storage.Hash(b)
}

// Admit is called only after OIDC token/state/nonce verification. A new identity
// proves who it is here; administrative approval separately grants access.
func Admit(ctx context.Context, db *pgxpool.Pool, c AdmissionConfig, v VerifiedIdentity) (Membership, error) {
	if err := c.Validate(); err != nil {
		return Membership{}, err
	}
	if v.Issuer == "" || v.Subject == "" || !c.ClaimsAllowed(v.Claims) {
		return Membership{}, ErrForbidden
	}
	email, emailErr := NormalizeEmail(v.Email)
	tx, err := db.Begin(ctx)
	if err != nil {
		return Membership{}, err
	}
	defer tx.Rollback(ctx)
	var org string
	var complete bool
	var domains []string
	if err = tx.QueryRow(ctx, `SELECT organization_id,bootstrap_complete,allowed_domains FROM organization_settings WHERE singleton FOR UPDATE`).Scan(&org, &complete, &domains); err != nil {
		return Membership{}, err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM identities WHERE issuer=$1 AND subject=$2 AND kind='human' AND organization_id=$3`, v.Issuer, v.Subject, org).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, err
	}
	if id != "" {
		m, err := Member(ctx, tx, id)
		if err != nil {
			return Membership{}, ErrForbidden
		}
		if m.State == "disabled" {
			return Membership{}, ErrForbidden
		}
		if emailErr != nil || !v.EmailVerified {
			if !m.LegacySubject || m.Email != "" || len(domains) > 0 {
				return Membership{}, ErrForbidden
			}
		} else {
			if !EmailAllowed(email, true, domains) {
				return Membership{}, ErrForbidden
			}
			var collision bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships WHERE organization_id=$1 AND email=$2 AND identity_id<>$3)`, org, email, id).Scan(&collision); err != nil {
				return Membership{}, err
			}
			if collision {
				return Membership{}, ErrMembershipConflict
			}
			if _, err = tx.Exec(ctx, `UPDATE organization_memberships SET email=$2,email_verified=true,legacy_subject=false,updated_at=now(),revision=revision+CASE WHEN email IS DISTINCT FROM $2 THEN 1 ELSE 0 END WHERE identity_id=$1`, id, email); err != nil {
				return Membership{}, err
			}
		}
		m, err = Member(ctx, tx, id)
		if err != nil {
			return Membership{}, err
		}
		return m, tx.Commit(ctx)
	}
	if emailErr != nil || !v.EmailVerified || !EmailAllowed(email, true, domains) {
		return Membership{}, ErrForbidden
	}
	role, state := "", "pending"
	if !complete {
		bootstrap, _ := NormalizeEmail(c.AdminEmail)
		if !(c.AdminEmail != "" && bootstrap == email) && !(c.AdminSubject != "" && c.AdminSubject == v.Subject) {
			return Membership{}, ErrForbidden
		}
		role, state = "admin", "active"
	}
	var collision bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships WHERE organization_id=$1 AND email=$2)`, org, email).Scan(&collision); err != nil {
		return Membership{}, err
	}
	if collision {
		return Membership{}, ErrMembershipConflict
	}
	id = uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO identities(id,organization_id,kind,issuer,subject,name) VALUES($1,$2,'human',$3,$4,$5)`, id, org, v.Issuer, v.Subject, v.Name); err != nil {
		return Membership{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(identity_id,organization_id,email,email_verified,role,state) VALUES($1,$2,$3,true,NULLIF($4,''),$5)`, id, org, email, role, state); err != nil {
		return Membership{}, err
	}
	if !complete {
		if _, err = tx.Exec(ctx, `UPDATE organization_settings SET bootstrap_complete=true WHERE singleton`); err != nil {
			return Membership{}, err
		}
	}
	m, err := Member(ctx, tx, id)
	if err != nil {
		return Membership{}, err
	}
	event := "membership.requested"
	if !complete {
		event = "membership.bootstrapped"
	}
	if err = membershipAudit(ctx, tx, id, m, event, Membership{}); err != nil {
		return Membership{}, err
	}
	return m, logging.Commit(ctx, tx, logging.Fields{Event: event, Component: "auth", ActorID: id, RecordID: id, Revision: m.Revision})
}

func membershipAudit(ctx context.Context, tx pgx.Tx, actor string, m Membership, event string, before Membership) error {
	// Audit role/state transitions without copying identity claims or emails.
	old, _ := json.Marshal(map[string]string{"role": before.Role, "state": before.State})
	next, _ := json.Marshal(map[string]string{"role": m.Role, "state": m.State})
	_, err := tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,target_id,action,revision,before_state,after_state) VALUES($1,NULLIF($2,''),$3,$4,$5,$6,$7)`, m.OrganizationID, actor, m.IdentityID, event, m.Revision, old, next)
	return err
}

func requireAdmin(ctx context.Context, q Querier, a Actor) error {
	m, err := ActiveMember(ctx, q, a)
	if err != nil || m.Role != "admin" {
		return ErrForbidden
	}
	return nil
}

type MembershipChange struct {
	IdentityID       string `json:"identity_id"`
	ExpectedRevision int    `json:"expected_revision"`
	Operation        string `json:"operation"`
	Role             string `json:"role"`
}

func ChangeMembership(ctx context.Context, db *pgxpool.Pool, a Actor, in MembershipChange) (Membership, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Membership{}, err
	}
	defer tx.Rollback(ctx)
	var domains []string
	if err = tx.QueryRow(ctx, `SELECT allowed_domains FROM organization_settings WHERE organization_id=$1 FOR UPDATE`, a.OrganizationID).Scan(&domains); err != nil {
		return Membership{}, ErrForbidden
	}
	if err = requireAdmin(ctx, tx, a); err != nil {
		return Membership{}, err
	}
	m, err := Member(ctx, tx, in.IdentityID)
	if err != nil || m.OrganizationID != a.OrganizationID {
		return Membership{}, ErrForbidden
	}
	if m.Revision != in.ExpectedRevision {
		return Membership{}, ErrMembershipConflict
	}
	before := m
	switch in.Operation {
	case "approve", "role":
		if in.Role != "admin" && in.Role != "user" && in.Role != "viewer" {
			return Membership{}, ErrMembershipInvalid
		}
		if in.Operation == "approve" && m.State != "pending" || in.Operation == "role" && m.State != "active" {
			return Membership{}, ErrMembershipConflict
		}
		if !memberEligible(m, domains) {
			return Membership{}, ErrForbidden
		}
		m.Role, m.State = in.Role, "active"
	case "disable":
		m.State = "disabled"
	case "enable":
		if m.State != "disabled" {
			return Membership{}, ErrMembershipConflict
		}
		m.State, m.Role = "pending", ""
	default:
		return Membership{}, ErrMembershipInvalid
	}
	m.Revision++
	if _, err = tx.Exec(ctx, `UPDATE organization_memberships SET role=NULLIF($2,''),state=$3,revision=$4,updated_at=$5 WHERE identity_id=$1`, m.IdentityID, m.Role, m.State, m.Revision, time.Now()); err != nil {
		return Membership{}, err
	}
	if before.Role == "admin" && before.State == "active" && (m.Role != "admin" || m.State != "active") {
		if err = remainingAdmin(ctx, tx, m.OrganizationID, domains); err != nil {
			return Membership{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE issuer_identity_id=$1 AND revoked_at IS NULL`, m.IdentityID); err != nil {
			return Membership{}, err
		}
	}
	if m.State == "disabled" {
		if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE identity_id=$1`, m.IdentityID); err != nil {
			return Membership{}, err
		}
	}
	if err = membershipAudit(ctx, tx, a.ID, m, "membership."+in.Operation, before); err != nil {
		return Membership{}, err
	}
	return m, logging.Commit(ctx, tx, logging.Fields{Event: "membership." + in.Operation, Component: "auth", ActorID: a.ID, RecordID: m.IdentityID, Revision: m.Revision})
}

func remainingAdmin(ctx context.Context, tx pgx.Tx, org string, domains []string) error {
	rows, err := tx.Query(ctx, `SELECT `+memberColumns+` FROM organization_memberships m JOIN identities i ON i.id=m.identity_id WHERE m.organization_id=$1 AND m.role='admin' AND m.state='active' AND m.category='member' AND i.kind='human'`, org)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return err
		}
		if memberEligible(m, domains) {
			return nil
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return ErrLastAdmin
}
