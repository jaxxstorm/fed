package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/jaxxstorm/fed/internal/storage"
)

var ErrForbidden = errors.New("forbidden")

type Actor struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Kind           string `json:"kind"`
	TokenHash      string `json:"-"`
}

type actorKey struct{}

func WithActor(ctx context.Context, a Actor) context.Context {
	logging.Actor(ctx, a.ID)
	ctx = logging.Identity(ctx, a.ID, "")
	return context.WithValue(ctx, actorKey{}, a)
}
func FromContext(ctx context.Context) Actor { a, _ := ctx.Value(actorKey{}).(Actor); return a }

type Querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Read-only snapshots cannot acquire row locks. Writable transactions retain
// locks until commit; pool queries have no enclosing mutation to fence.
func authorizationLock(ctx context.Context, q Querier) (string, error) {
	if _, ok := q.(pgx.Tx); !ok {
		return "", nil
	}
	var readOnly string
	if err := q.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly); err != nil {
		return "", err
	}
	if readOnly == "on" {
		return "", nil
	}
	return " FOR SHARE", nil
}

// Authorize rechecks grants and token revocation on every operation, including
// worker execution. The actor's service scope is never trusted from input JSON.
func Authorize(ctx context.Context, q Querier, a Actor, service, action string) error {
	if a.ID == "" || (action == "review" && a.Kind != "human") {
		return ErrForbidden
	}
	var organization, kind string
	if err := q.QueryRow(ctx, `SELECT organization_id,kind FROM identities WHERE id=$1`, a.ID).Scan(&organization, &kind); err != nil {
		return ErrForbidden
	}
	if organization != a.OrganizationID || kind != a.Kind {
		return ErrForbidden
	}
	if service != "" {
		var org string
		if err := q.QueryRow(ctx, `SELECT organization_id FROM services WHERE id=$1`, service).Scan(&org); err != nil || org != organization {
			return ErrForbidden
		}
	}
	if a.TokenHash != "" {
		var services, actions []string
		lock, err := authorizationLock(ctx, q)
		if err != nil {
			return ErrForbidden
		}
		if err := q.QueryRow(ctx, `SELECT service_ids,actions FROM api_tokens WHERE hash=$1 AND identity_id=$2 AND revoked_at IS NULL AND expires_at>now()`+lock, a.TokenHash, a.ID).Scan(&services, &actions); err != nil {
			return ErrForbidden
		}
		if !slices.Contains(services, service) || !slices.Contains(actions, action) {
			return ErrForbidden
		}
	}
	var allowed bool
	if kind == "human" {
		m, err := ActiveMember(ctx, q, a)
		if err != nil {
			return err
		}
		allowed = m.Role == "admin" || action == "read" || (m.Role == "user" && slices.Contains([]string{"edit", "check", "connect", "review", "export"}, action))
		if !allowed {
			return ErrForbidden
		}
		logging.Authorized(ctx, a.ID, service)
		return nil
	}
	if service == "" || action == "admin" {
		return ErrForbidden
	}
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM permissions WHERE identity_id=$1 AND (service_id=$2 OR service_id='*') AND (action=$3 OR action='admin'))`, a.ID, service, action).Scan(&allowed)
	if err != nil || !allowed {
		return ErrForbidden
	}
	logging.Authorized(ctx, a.ID, service)
	return nil
}

func Secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func TokenActor(ctx context.Context, db *pgxpool.Pool, token string) (Actor, error) {
	a := Actor{TokenHash: storage.Hash([]byte(token))}
	err := db.QueryRow(ctx, `SELECT i.id,i.organization_id,i.kind FROM identities i JOIN api_tokens t ON t.identity_id=i.id WHERE t.hash=$1 AND t.revoked_at IS NULL AND t.expires_at>now()`, a.TokenHash).Scan(&a.ID, &a.OrganizationID, &a.Kind)
	if err != nil {
		return Actor{}, ErrForbidden
	}
	return a, nil
}

func BootstrapAdmin(ctx context.Context, db *pgxpool.Pool, issuer, subject string) (Actor, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(817032)`); err != nil {
		return Actor{}, err
	}
	var org string
	var complete bool
	if err = tx.QueryRow(ctx, `SELECT organization_id,bootstrap_complete FROM organization_settings WHERE singleton FOR UPDATE`).Scan(&org, &complete); err != nil {
		return Actor{}, err
	}
	if complete {
		var a Actor
		if err = tx.QueryRow(ctx, `SELECT id,organization_id,kind FROM identities WHERE issuer=$1 AND subject=$2 AND kind='human'`, issuer, subject).Scan(&a.ID, &a.OrganizationID, &a.Kind); err != nil {
			return Actor{}, ErrForbidden
		}
		// A restart may resolve the configured identity, but never restore its role.
		return a, tx.Commit(ctx)
	}
	a := Actor{ID: uuid.NewString(), OrganizationID: org, Kind: "human"}
	err = tx.QueryRow(ctx, `INSERT INTO identities(id,organization_id,kind,issuer,subject,name) VALUES($1,$2,'human',$3,$4,'Administrator') ON CONFLICT(issuer,subject) DO UPDATE SET subject=excluded.subject RETURNING id,organization_id,kind`, a.ID, a.OrganizationID, issuer, subject).Scan(&a.ID, &a.OrganizationID, &a.Kind)
	if err != nil {
		return Actor{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO permissions(identity_id,service_id,action) VALUES($1,'*','admin') ON CONFLICT DO NOTHING`, a.ID); err != nil {
		return Actor{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(identity_id,organization_id,legacy_subject,role,state) VALUES($1,$2,true,'admin','active')`, a.ID, org); err != nil {
		return Actor{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE organization_settings SET bootstrap_complete=true WHERE singleton`); err != nil {
		return Actor{}, err
	}
	return a, tx.Commit(ctx)
}

func IssueToken(ctx context.Context, db *pgxpool.Pool, admin Actor, service string, actions []string, lifetime time.Duration) (string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = Authorize(ctx, tx, admin, service, "admin"); err != nil {
		return "", err
	}
	if lifetime <= 0 || lifetime > 90*24*time.Hour || len(actions) == 0 {
		return "", errors.New("invalid token lifetime or actions")
	}
	for _, action := range actions {
		if !slices.Contains([]string{"read", "edit", "check", "export", "connect"}, action) {
			return "", errors.New("invalid agent action")
		}
	}
	id := uuid.NewString()
	token := Secret()
	if _, err = tx.Exec(ctx, `INSERT INTO identities(id,organization_id,kind,subject,name) VALUES($1,$2,'agent',$1,'MCP agent')`, id, admin.OrganizationID); err != nil {
		return "", err
	}
	for _, action := range actions {
		if _, err = tx.Exec(ctx, `INSERT INTO permissions(identity_id,service_id,action) VALUES($1,$2,$3)`, id, service, action); err != nil {
			return "", err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO api_tokens(hash,identity_id,service_ids,actions,expires_at,issuer_identity_id) VALUES($1,$2,$3,$4,$5,$6)`, storage.Hash([]byte(token)), id, []string{service}, actions, time.Now().Add(lifetime), admin.ID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'token.issued',$4)`, admin.OrganizationID, service, admin.ID, id); err != nil {
		return "", err
	}
	return token, logging.Commit(ctx, tx, logging.Fields{Event: "auth.token.issued", Component: "auth", ActorID: admin.ID, ServiceID: service, LifetimeSeconds: int64(lifetime / time.Second), Actions: actions})
}

func RevokeToken(ctx context.Context, db *pgxpool.Pool, admin Actor, service, hash string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = Authorize(ctx, tx, admin, service, "admin"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE hash=$1 AND service_ids=ARRAY[$2]::text[]`, hash, service)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrForbidden
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'token.revoked',$4)`, admin.OrganizationID, service, admin.ID, hash); err != nil {
		return err
	}
	return logging.Commit(ctx, tx, logging.Fields{Event: "auth.token.revoked", Component: "auth", ActorID: admin.ID, ServiceID: service})
}
