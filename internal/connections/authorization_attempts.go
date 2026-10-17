package connections

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type AuthorizationHandoff struct {
	OperationID string    `json:"operation_id"`
	URL         string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (s *Service) FailAuthorization(ctx context.Context, a auth.Actor, claim AuthorizationClaim, code string) error {
	if !slices.Contains([]string{"authorization_cancelled", "authorization_invalid_code", "authorization_unavailable", "missing_application_configuration", "authorization_exchange_failed", "authorization_binding_failed"}, code) {
		return ErrInvalid
	}
	state := "failed"
	if code == "authorization_cancelled" {
		state = "cancelled"
	}
	result, _ := json.Marshal(Operation{ErrorCode: code})
	tag, err := s.DB.Exec(ctx, `UPDATE connection_operations o SET state=$3,result=$4,updated_at=now() WHERE o.service_id=$1 AND o.id=$2 AND o.state='running' AND EXISTS(SELECT 1 FROM connection_authorizations a WHERE a.service_id=o.service_id AND a.operation_id=o.id AND a.browser_actor_id=$5 AND a.consumed_at IS NOT NULL)`, claim.ServiceID, claim.OperationID, state, result, a.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// AuthorizationChallenge is an internal browser redirect value, not an MCP DTO.
type AuthorizationChallenge struct {
	State         string `json:"-"`
	PKCEChallenge string `json:"-"`
	RedirectURI   string `json:"-"`
}

type AuthorizationClaim struct {
	ServiceID    string
	ConnectionID string
	OperationID  string
	Provider     Provider
	Generation   int64
	RedirectURI  string
	Verifier     Credentials
}

func publicOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", ErrInvalid
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return "", ErrInvalid
	}
	return u.Scheme + "://" + u.Host, nil
}

func (s *Service) BeginAuthorization(ctx context.Context, a auth.Actor, in OperationInput, origin string) (AuthorizationHandoff, error) {
	origin, err := publicOrigin(origin)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 {
		return AuthorizationHandoff{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return AuthorizationHandoff{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return AuthorizationHandoff{}, err
	}
	request, _ := json.Marshal(struct {
		Input  OperationInput
		Origin string
	}{in, origin})
	digest := storage.Hash(request)
	var oldHash string
	var old []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='connection.authorize' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&oldHash, &old)
	if err == nil {
		if oldHash != digest {
			return AuthorizationHandoff{}, ErrConflict
		}
		var handoff AuthorizationHandoff
		if err = json.Unmarshal(old, &handoff); err != nil {
			return handoff, err
		}
		return handoff, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AuthorizationHandoff{}, err
	}
	provider, err := lockCredentialConnection(ctx, tx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	if d.Connection.Revision != in.ExpectedRevision {
		return AuthorizationHandoff{}, ErrConflict
	}
	definition, _ := s.Registry.Provider(Provider(provider))
	if Provider(provider) == Kubernetes {
		definition.Guide.CallbackPath = "/connections/upload/kubernetes"
	}
	if Provider(provider) == AWS {
		definition.Guide.CallbackPath = "/connections/upload/aws"
	}
	if definition.Guide.CallbackPath == "" || d.Connection.Mode != ServerManaged {
		return AuthorizationHandoff{}, ErrInvalid
	}
	if s.Credentials.Keys == nil {
		return AuthorizationHandoff{}, ErrCredentialUnavailable
	}
	handoff := AuthorizationHandoff{OperationID: uuid.NewString(), ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
	handoff.URL = origin + "/connections/authorize/" + url.PathEscape(in.ServiceID) + "/" + handoff.OperationID
	var tokenHash any
	if a.TokenHash != "" {
		tokenHash = a.TokenHash
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_operations(service_id,connection_id,id,generation,actor_id,token_hash,operation,state) VALUES($1,$2,$3,$4,$5,$6,'authorize','pending')`, in.ServiceID, in.ConnectionID, handoff.OperationID, in.Generation, a.ID, tokenHash)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_authorizations(service_id,connection_id,operation_id,provider,generation,state_hash,initiating_actor_id,redirect_uri,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, in.ServiceID, in.ConnectionID, handoff.OperationID, provider, in.Generation, storage.Hash([]byte(auth.Secret())), a.ID, origin+definition.Guide.CallbackPath, handoff.ExpiresAt)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	response, _ := json.Marshal(handoff)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'connection.authorize',$3,$4,$5)`, a.ID, in.ServiceID, in.IdempotencyKey, digest, response)
	if err != nil {
		return AuthorizationHandoff{}, err
	}
	return handoff, tx.Commit(ctx)
}

func authorizeBrowser(ctx context.Context, tx pgx.Tx, a auth.Actor, service, sessionHash string) error {
	if a.Kind != "human" || a.TokenHash != "" || sessionHash == "" {
		return auth.ErrForbidden
	}
	if err := auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE hash=$1 AND identity_id=$2 AND expires_at>now())`, sessionHash, a.ID).Scan(&valid)
	if err != nil || !valid {
		return auth.ErrForbidden
	}
	return nil
}

func authorizeInitiator(ctx context.Context, tx pgx.Tx, service, operation string) error {
	var a auth.Actor
	err := tx.QueryRow(ctx, `SELECT i.id,i.organization_id,i.kind,COALESCE(o.token_hash,'') FROM connection_operations o JOIN identities i ON i.id=o.actor_id WHERE o.service_id=$1 AND o.id=$2`, service, operation).Scan(&a.ID, &a.OrganizationID, &a.Kind, &a.TokenHash)
	if err != nil {
		return auth.ErrForbidden
	}
	return auth.Authorize(ctx, tx, a, service, "connect")
}

func authorizationAAD(a auth.Actor, claim AuthorizationClaim) []byte {
	return append(credentialAAD(a.OrganizationID, claim.ServiceID, claim.ConnectionID, string(claim.Provider), claim.Generation), []byte("/authorization/"+claim.OperationID)...)
}

func (s *Service) BindAuthorizationBrowser(ctx context.Context, a auth.Actor, service, operation, sessionHash string) (AuthorizationChallenge, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeBrowser(ctx, tx, a, service, sessionHash); err != nil {
		return AuthorizationChallenge{}, err
	}
	if err = authorizeInitiator(ctx, tx, service, operation); err != nil {
		return AuthorizationChallenge{}, err
	}
	var claim AuthorizationClaim
	claim.ServiceID, claim.OperationID = service, operation
	err = tx.QueryRow(ctx, `SELECT connection_id,provider,generation,redirect_uri FROM connection_authorizations WHERE service_id=$1 AND operation_id=$2 AND expires_at>now() AND consumed_at IS NULL AND browser_actor_id IS NULL FOR UPDATE`, service, operation).Scan(&claim.ConnectionID, &claim.Provider, &claim.Generation, &claim.RedirectURI)
	if err != nil {
		return AuthorizationChallenge{}, ErrConflict
	}
	if _, err = lockCredentialConnection(ctx, tx, a, service, claim.ConnectionID, "connect", claim.Generation); err != nil {
		return AuthorizationChallenge{}, err
	}
	state, verifier := auth.Secret(), auth.Secret()
	key, nonce, ciphertext, err := s.Credentials.Keys.seal([]byte(verifier), authorizationAAD(a, claim))
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE connection_authorizations SET browser_actor_id=$3,browser_session_hash=$4,state_hash=$5,verifier_key_id=$6,verifier_nonce=$7,verifier_ciphertext=$8 WHERE service_id=$1 AND operation_id=$2`, service, operation, a.ID, sessionHash, storage.Hash([]byte(state)), key, nonce, ciphertext)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	hash := sha256.Sum256([]byte(verifier))
	out := AuthorizationChallenge{State: state, PKCEChallenge: base64.RawURLEncoding.EncodeToString(hash[:]), RedirectURI: claim.RedirectURI}
	return out, tx.Commit(ctx)
}

// ClaimAuthorization consumes state before exchanging the provider code. A
// failed exchange requires a new attempt rather than permitting code replay.
func (s *Service) ClaimAuthorization(ctx context.Context, a auth.Actor, provider Provider, state, sessionHash, redirectURI string) (AuthorizationClaim, error) {
	if len(state) != 64 || strings.TrimSpace(redirectURI) == "" {
		return AuthorizationClaim{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AuthorizationClaim{}, err
	}
	defer tx.Rollback(ctx)
	var claim AuthorizationClaim
	var key string
	var nonce, ciphertext []byte
	err = tx.QueryRow(ctx, `SELECT service_id,connection_id,operation_id,provider,generation,redirect_uri,verifier_key_id,verifier_nonce,verifier_ciphertext FROM connection_authorizations WHERE state_hash=$1 AND provider=$2 AND browser_actor_id=$3 AND browser_session_hash=$4 AND redirect_uri=$5 AND expires_at>now() AND consumed_at IS NULL FOR UPDATE`, storage.Hash([]byte(state)), provider, a.ID, sessionHash, redirectURI).Scan(&claim.ServiceID, &claim.ConnectionID, &claim.OperationID, &claim.Provider, &claim.Generation, &claim.RedirectURI, &key, &nonce, &ciphertext)
	if err != nil {
		return AuthorizationClaim{}, ErrConflict
	}
	if err = authorizeBrowser(ctx, tx, a, claim.ServiceID, sessionHash); err != nil {
		return AuthorizationClaim{}, err
	}
	if err = authorizeInitiator(ctx, tx, claim.ServiceID, claim.OperationID); err != nil {
		return AuthorizationClaim{}, err
	}
	if _, err = lockCredentialConnection(ctx, tx, a, claim.ServiceID, claim.ConnectionID, "connect", claim.Generation); err != nil {
		return AuthorizationClaim{}, err
	}
	plain, err := s.Credentials.Keys.open(key, nonce, ciphertext, authorizationAAD(a, claim))
	if err != nil {
		return AuthorizationClaim{}, err
	}
	defer clear(plain)
	claim.Verifier = NewCredentials(map[string]string{"verifier": string(plain)})
	_, err = tx.Exec(ctx, `UPDATE connection_authorizations SET consumed_at=now(),verifier_key_id=NULL,verifier_nonce=NULL,verifier_ciphertext=NULL WHERE service_id=$1 AND operation_id=$2`, claim.ServiceID, claim.OperationID)
	if err != nil {
		return AuthorizationClaim{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE connection_operations SET state='running',updated_at=now() WHERE service_id=$1 AND id=$2 AND state='pending'`, claim.ServiceID, claim.OperationID)
	if err != nil {
		return AuthorizationClaim{}, err
	}
	return claim, tx.Commit(ctx)
}

// CompleteAuthorization accepts only a previously claimed browser attempt.
// Provider-specific identity/installation ownership verification happens before
// this call; no caller-controlled provenance is accepted by transport handlers.
func (s *Service) CompleteAuthorization(ctx context.Context, a auth.Actor, claim AuthorizationClaim, sessionHash, verifiedAccount string, credentials Credentials, verifiedHints ...string) (Operation, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeBrowser(ctx, tx, a, claim.ServiceID, sessionHash); err != nil {
		return Operation{}, err
	}
	if err = authorizeInitiator(ctx, tx, claim.ServiceID, claim.OperationID); err != nil {
		return Operation{}, err
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_authorizations a JOIN connection_operations o ON o.service_id=a.service_id AND o.id=a.operation_id WHERE a.service_id=$1 AND a.operation_id=$2 AND a.connection_id=$3 AND a.provider=$4 AND a.generation=$5 AND a.browser_actor_id=$6 AND a.browser_session_hash=$7 AND a.consumed_at IS NOT NULL AND a.expires_at>now() AND o.state='running')`, claim.ServiceID, claim.OperationID, claim.ConnectionID, claim.Provider, claim.Generation, a.ID, sessionHash).Scan(&valid)
	if err != nil {
		return Operation{}, err
	}
	if !valid {
		return Operation{}, ErrConflict
	}
	if _, err = lockCredentialConnection(ctx, tx, a, claim.ServiceID, claim.ConnectionID, "connect", claim.Generation); err != nil {
		return Operation{}, err
	}
	d, err := readConnection(ctx, tx, claim.ServiceID, claim.ConnectionID)
	if err != nil {
		return Operation{}, err
	}
	declaredMatches := d.Connection.Configuration.AccountID == verifiedAccount || slices.Contains(verifiedHints, d.Connection.Configuration.AccountID)
	if verifiedAccount == "" || !declaredMatches || (d.Connection.VerifiedAccountID != "" && d.Connection.VerifiedAccountID != verifiedAccount) {
		return Operation{}, ErrInvalid
	}
	next := claim.Generation + 1
	now, until := time.Now().UTC(), time.Now().UTC().Add(15*time.Minute)
	health := AccessHealth{State: Verified, VerifiedAccountID: verifiedAccount, Generation: next, ActorID: a.ID, CheckedAt: &now, ValidUntil: &until, Limitations: []string{}}
	b, _ := json.Marshal(health)
	_, err = tx.Exec(ctx, `UPDATE evidence_connections SET generation=$3,access_health=$4,verified_account_id=$5,updated_at=now() WHERE service_id=$1 AND id=$2`, claim.ServiceID, claim.ConnectionID, next, b, verifiedAccount)
	if err != nil {
		return Operation{}, err
	}
	if err = s.Credentials.PutTx(ctx, tx, a, claim.ServiceID, claim.ConnectionID, next, credentials); err != nil {
		return Operation{}, err
	}
	out := Operation{ID: claim.OperationID, ConnectionID: claim.ConnectionID, State: "succeeded", Access: &health}
	b, _ = json.Marshal(out)
	_, err = tx.Exec(ctx, `UPDATE connection_operations SET state='succeeded',result=$3,updated_at=now() WHERE service_id=$1 AND id=$2`, claim.ServiceID, claim.OperationID, b)
	if err != nil {
		return Operation{}, err
	}
	if err = audit(ctx, tx, a, d.Connection, "authorized"); err != nil {
		return Operation{}, err
	}
	return out, tx.Commit(ctx)
}
