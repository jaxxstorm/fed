package auth

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/logging"
)

type Organization struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	AllowedDomains []string `json:"allowed_domains"`
	Revision       int      `json:"revision"`
}
type OrganizationUpdate struct {
	Name             string   `json:"name"`
	AllowedDomains   []string `json:"allowed_domains"`
	ExpectedRevision int      `json:"expected_revision"`
}
type MemberPage struct {
	Members    []Membership `json:"members"`
	NextOffset *int         `json:"next_offset,omitempty"`
}

func GetOrganization(ctx context.Context, db *pgxpool.Pool, a Actor) (Organization, error) {
	if err := requireAdmin(ctx, db, a); err != nil {
		return Organization{}, err
	}
	var out Organization
	err := db.QueryRow(ctx, `SELECT o.id,o.name,s.allowed_domains,s.revision FROM organization_settings s JOIN organizations o ON o.id=s.organization_id WHERE o.id=$1`, a.OrganizationID).Scan(&out.ID, &out.Name, &out.AllowedDomains, &out.Revision)
	return out, err
}

func ListMembers(ctx context.Context, db *pgxpool.Pool, a Actor, limit, offset int) (MemberPage, error) {
	out := MemberPage{Members: []Membership{}}
	if limit <= 0 || limit > 100 || offset < 0 {
		return out, ErrMembershipInvalid
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = requireAdmin(ctx, tx, a); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT `+memberColumns+` FROM organization_memberships m JOIN identities i ON i.id=m.identity_id WHERE m.organization_id=$1 ORDER BY (m.state='pending') DESC,m.created_at,m.identity_id LIMIT $2 OFFSET $3`, a.OrganizationID, limit+1, offset)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Members = append(out.Members, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Members) > limit {
		next := offset + limit
		out.NextOffset = &next
		out.Members = out.Members[:limit]
	}
	return out, tx.Commit(ctx)
}

func UpdateOrganization(ctx context.Context, db *pgxpool.Pool, a Actor, in OrganizationUpdate) (Organization, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 200 || len(in.AllowedDomains) > 100 {
		return Organization{}, ErrMembershipInvalid
	}
	domains := []string{}
	for _, raw := range in.AllowedDomains {
		d, err := NormalizeDomain(raw)
		if err != nil {
			return Organization{}, err
		}
		if !slices.Contains(domains, d) {
			domains = append(domains, d)
		}
	}
	slices.Sort(domains)
	tx, err := db.Begin(ctx)
	if err != nil {
		return Organization{}, err
	}
	defer tx.Rollback(ctx)
	var revision int
	if err = tx.QueryRow(ctx, `SELECT revision FROM organization_settings WHERE organization_id=$1 FOR UPDATE`, a.OrganizationID).Scan(&revision); err != nil {
		return Organization{}, ErrForbidden
	}
	if err = requireAdmin(ctx, tx, a); err != nil {
		return Organization{}, err
	}
	if revision != in.ExpectedRevision {
		return Organization{}, ErrMembershipConflict
	}
	m, err := Member(ctx, tx, a.ID)
	if err != nil {
		return Organization{}, err
	}
	if !memberEligible(m, domains) {
		return Organization{}, ErrLastAdmin
	}
	if err = remainingAdmin(ctx, tx, a.OrganizationID, domains); err != nil {
		return Organization{}, err
	}
	out := Organization{ID: a.OrganizationID, Name: in.Name, AllowedDomains: domains, Revision: revision + 1}
	var before []byte
	if err = tx.QueryRow(ctx, `SELECT jsonb_build_object('name',o.name,'allowed_domains',s.allowed_domains) FROM organizations o JOIN organization_settings s ON s.organization_id=o.id WHERE o.id=$1`, a.OrganizationID).Scan(&before); err != nil {
		return Organization{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE organizations SET name=$2 WHERE id=$1`, a.OrganizationID, out.Name); err != nil {
		return Organization{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE organization_settings SET allowed_domains=$2,revision=$3,updated_at=now() WHERE organization_id=$1`, a.OrganizationID, domains, out.Revision); err != nil {
		return Organization{}, err
	}
	after, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,revision,before_state,after_state) VALUES($1,$2,'organization.updated',$3,$4,$5)`, a.OrganizationID, a.ID, out.Revision, before, after); err != nil {
		return Organization{}, err
	}
	return out, logging.Commit(ctx, tx, logging.Fields{Event: "organization.updated", Component: "auth", ActorID: a.ID, Revision: out.Revision})
}

func ResolveAdministrator(ctx context.Context, db *pgxpool.Pool, issuer, subject, email string) (Actor, error) {
	if email != "" {
		var err error
		email, err = NormalizeEmail(email)
		if err != nil {
			return Actor{}, err
		}
	}
	var a Actor
	err := db.QueryRow(ctx, `SELECT i.id,i.organization_id,i.kind FROM identities i JOIN organization_memberships m ON m.identity_id=i.id WHERE i.issuer=$1 AND (($2<>'' AND i.subject=$2) OR ($2='' AND m.email=$3 AND m.email_verified))`, issuer, subject, email).Scan(&a.ID, &a.OrganizationID, &a.Kind)
	if err != nil {
		return Actor{}, ErrForbidden
	}
	return a, requireAdmin(ctx, db, a)
}

// RecoverAdministrator is an explicit operator operation. Runtime DML grants
// are insufficient: the caller must own the identity table (migration login).
func RecoverAdministrator(ctx context.Context, db *pgxpool.Pool, id string, clearDomains bool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner bool
	if err = tx.QueryRow(ctx, `SELECT pg_get_userbyid(relowner)=current_user FROM pg_class WHERE oid='identities'::regclass`).Scan(&owner); err != nil || !owner {
		return ErrForbidden
	}
	var org string
	var domains []string
	if err = tx.QueryRow(ctx, `SELECT organization_id,allowed_domains FROM organization_settings WHERE singleton FOR UPDATE`).Scan(&org, &domains); err != nil {
		return err
	}
	m, err := Member(ctx, tx, id)
	if err != nil || m.OrganizationID != org {
		return ErrForbidden
	}
	before := m
	if clearDomains {
		domains = []string{}
	}
	if !memberEligible(m, domains) {
		return errors.New("recovery identity is ineligible; verify its email or explicitly clear domain restrictions")
	}
	if clearDomains {
		if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,target_id,action,revision,before_state,after_state) SELECT organization_id,$1,'organization.domains.recovered',revision+1,jsonb_build_object('allowed_domains',allowed_domains),'{"allowed_domains":[]}'::jsonb FROM organization_settings WHERE singleton`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE organization_settings SET allowed_domains='{}',revision=revision+1,updated_at=now() WHERE singleton`); err != nil {
			return err
		}
	}
	m.Role, m.State, m.Revision = "admin", "active", m.Revision+1
	if _, err = tx.Exec(ctx, `UPDATE organization_memberships SET role='admin',state='active',revision=$2,updated_at=now() WHERE identity_id=$1`, id, m.Revision); err != nil {
		return err
	}
	if err = membershipAudit(ctx, tx, "", m, "membership.operator_recovered", before); err != nil {
		return err
	}
	return logging.Commit(ctx, tx, logging.Fields{Event: "membership.operator_recovered", Component: "auth", RecordID: id, Revision: m.Revision})
}
