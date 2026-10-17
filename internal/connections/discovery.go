package connections

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type DiscoverySubmission struct {
	ServiceID      string               `json:"service_id"`
	SessionID      string               `json:"session_id"`
	IdempotencyKey string               `json:"idempotency_key"`
	ObservedAt     time.Time            `json:"observed_at"`
	Method         string               `json:"method"`
	Limitations    []string             `json:"limitations"`
	Resources      []DiscoveredResource `json:"resources"`
	Coverage       []KindCoverage       `json:"coverage"`
	Complete       bool                 `json:"complete"`
	NextCursor     string               `json:"next_cursor,omitempty"`
}

type DiscoveryReceipt struct {
	SessionID   string         `json:"session_id"`
	State       DiscoveryState `json:"state"`
	Accepted    int            `json:"accepted"`
	ActorID     string         `json:"actor_id"`
	Mode        AccessMode     `json:"access_mode"`
	Method      string         `json:"method"`
	ObservedAt  time.Time      `json:"observed_at"`
	Limitations []string       `json:"limitations"`
	Coverage    []KindCoverage `json:"coverage"`
	NextCursor  string         `json:"next_cursor,omitempty"`
}

func (s *Service) StartAgentDiscovery(ctx context.Context, a auth.Actor, in OperationInput) (DiscoverySession, error) {
	return s.startDiscovery(ctx, a, in, AgentManaged)
}

func (s *Service) startDiscovery(ctx context.Context, a auth.Actor, in OperationInput, mode AccessMode) (DiscoverySession, error) {
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 {
		return DiscoverySession{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return DiscoverySession{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return DiscoverySession{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return DiscoverySession{}, err
	}
	if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation); err != nil {
		return DiscoverySession{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return DiscoverySession{}, err
	}
	if d.Connection.Revision != in.ExpectedRevision {
		return DiscoverySession{}, ErrConflict
	}
	request, _ := json.Marshal(struct {
		Input OperationInput
		Mode  AccessMode
	}{in, mode})
	digest := storage.Hash(request)
	var oldHash string
	var old []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='discovery.start' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&oldHash, &old)
	if err == nil {
		if oldHash != digest {
			return DiscoverySession{}, ErrConflict
		}
		var out DiscoverySession
		if err = json.Unmarshal(old, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DiscoverySession{}, err
	}
	out := DiscoverySession{ID: uuid.NewString(), ServiceID: in.ServiceID, ConnectionID: in.ConnectionID, Generation: in.Generation, ActorID: a.ID, Mode: mode, State: Running, StartedAt: time.Now().UTC(), Coverage: []KindCoverage{}}
	_, err = tx.Exec(ctx, `INSERT INTO connection_discovery_sessions(service_id,connection_id,id,generation,actor_id,mode,state,started_at) VALUES($1,$2,$3,$4,$5,$6,'running',$7)`, out.ServiceID, out.ConnectionID, out.ID, out.Generation, out.ActorID, out.Mode, out.StartedAt)
	if err != nil {
		return out, err
	}
	b, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'discovery.start',$3,$4,$5)`, a.ID, in.ServiceID, in.IdempotencyKey, digest, b)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) SubmitAgentDiscovery(ctx context.Context, a auth.Actor, in DiscoverySubmission) (DiscoveryReceipt, error) {
	return s.submitDiscovery(ctx, a, in, true)
}

func validateResource(c Connection, r DiscoveredResource) bool {
	if r.Resource.Provider != c.Provider || r.Resource.Tenant != c.Tenant() || r.Resource.ID == "" || len(r.Resource.ID) > 512 || strings.TrimSpace(r.Name) == "" || len(r.Name) > 256 || len(r.Locator) > 2048 {
		return false
	}
	u, err := url.Parse(r.Locator)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch c.Provider {
	case GitHub:
		return r.Resource.Kind == "repository" && regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(r.Resource.ID) && u.Scheme == "https" && u.Host == "github.com" && regexp.MustCompile(`^/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(u.Path)
	case AWS:
		if c.Configuration.Region != "us-east-1" {
			return false
		}
		if r.Resource.Kind == "account_region" {
			return r.Resource.ID == "us-east-1" && r.Locator == c.Configuration.AccountID+"/us-east-1"
		}
		return r.Resource.Kind == "eks_cluster" && r.Resource.ID == "arn:aws:eks:us-east-1:"+c.Configuration.AccountID+":cluster/"+r.Name && r.Locator == c.Configuration.AccountID+"/us-east-1"
	case Workspace:
		return (r.Resource.Kind == "org_unit" || r.Resource.Kind == "group") && r.Locator == "google-workspace://"+url.PathEscape(c.Tenant())+"/"+r.Resource.Kind+"/"+url.PathEscape(r.Resource.ID)
	case Kubernetes:
		if r.Resource.Kind == "namespace" {
			if _, err := uuid.Parse(r.Resource.ID); err != nil {
				return false
			}
			if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`).MatchString(r.Name) {
				return false
			}
		} else if r.Resource.Kind != "cluster" || r.Resource.ID != c.Configuration.ClusterID {
			return false
		}
		return r.Locator == "kubernetes://cluster/"+url.PathEscape(c.Configuration.AccountID)+"/"+r.Resource.Kind+"/"+url.PathEscape(r.Resource.ID)
	case Documents:
		return (r.Resource.Kind == "document" || r.Resource.Kind == "operational_records") && u.Scheme == "https" && u.Host == c.Configuration.AccountID && r.Resource.ID == r.Locator
	}
	return false
}

func (s *Service) submitDiscovery(ctx context.Context, a auth.Actor, in DiscoverySubmission, agent bool, serverRequest ...[]byte) (DiscoveryReceipt, error) {
	if len(in.Resources) > MaxPageSize || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || strings.TrimSpace(in.Method) == "" || len(in.Method) > 256 || len(in.NextCursor) > 4096 || len(in.Coverage) > 10 || len(in.Limitations) > 20 || in.ObservedAt.IsZero() || in.ObservedAt.After(time.Now().Add(5*time.Minute)) {
		return DiscoveryReceipt{}, ErrInvalid
	}
	request, err := json.Marshal(in)
	if err != nil || len(request) > MaxResponseBytes {
		return DiscoveryReceipt{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "connect"); err != nil {
		return DiscoveryReceipt{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return DiscoveryReceipt{}, err
	}
	var session DiscoverySession
	var oldCoverage []byte
	var oldMetadata []byte
	err = tx.QueryRow(ctx, `SELECT connection_id,generation,actor_id,mode,state,started_at,coverage,metadata,next_cursor FROM connection_discovery_sessions WHERE service_id=$1 AND id=$2 FOR UPDATE`, in.ServiceID, in.SessionID).Scan(&session.ConnectionID, &session.Generation, &session.ActorID, &session.Mode, &session.State, &session.StartedAt, &oldCoverage, &oldMetadata, &session.NextCursor)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	if session.ActorID != a.ID || agent != (session.Mode == AgentManaged) {
		return DiscoveryReceipt{}, auth.ErrForbidden
	}
	if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, session.ConnectionID, "connect", session.Generation); err != nil {
		return DiscoveryReceipt{}, err
	}
	var priorHash string
	var prior []byte
	digest := storage.Hash(request)
	if !agent {
		if len(serverRequest) != 1 {
			return DiscoveryReceipt{}, ErrInvalid
		}
		digest = storage.Hash(serverRequest[0])
	}
	err = tx.QueryRow(ctx, `SELECT request_hash,receipt FROM connection_discovery_pages WHERE service_id=$1 AND session_id=$2 AND idempotency_key=$3`, in.ServiceID, in.SessionID, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return DiscoveryReceipt{}, ErrConflict
		}
		var out DiscoveryReceipt
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DiscoveryReceipt{}, err
	}
	if session.State != Running {
		return DiscoveryReceipt{}, ErrConflict
	}
	if !agent {
		var serverInput ServerDiscoveryInput
		if json.Unmarshal(serverRequest[0], &serverInput) != nil || serverInput.Cursor != session.NextCursor {
			return DiscoveryReceipt{}, ErrConflict
		}
	}
	d, err := readConnection(ctx, tx, in.ServiceID, session.ConnectionID)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	definition, _ := s.Registry.Provider(d.Connection.Provider)
	var previousMetadata struct {
		Limitations []string `json:"limitations"`
	}
	if err = json.Unmarshal(oldMetadata, &previousMetadata); err != nil {
		return DiscoveryReceipt{}, err
	}
	limitations := slices.Clone(previousMetadata.Limitations)
	for _, limitation := range in.Limitations {
		if !slices.Contains(limitations, limitation) {
			limitations = append(limitations, limitation)
		}
	}
	if len(limitations) > 100 {
		return DiscoveryReceipt{}, ErrInvalid
	}
	coverage := map[string]KindCoverage{}
	if err = json.Unmarshal(oldCoverage, &session.Coverage); err != nil {
		return DiscoveryReceipt{}, err
	}
	for _, entry := range session.Coverage {
		coverage[entry.Kind] = entry
	}
	for _, entry := range in.Coverage {
		if !slices.Contains(definition.ResourceKinds, entry.Kind) {
			return DiscoveryReceipt{}, ErrInvalid
		}
		coverage[entry.Kind] = entry
	}
	seen := map[ResourceKey]bool{}
	for _, resource := range in.Resources {
		if seen[resource.Resource] || !validateResource(d.Connection, resource) {
			return DiscoveryReceipt{}, ErrInvalid
		}
		seen[resource.Resource] = true
		if err = matchImportedResource(ctx, tx, a, in.ServiceID, resource, d.Connection.Configuration.AccountID); err != nil {
			return DiscoveryReceipt{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO connection_discovery_seen(service_id,session_id,tenant,resource_kind,resource_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, in.ServiceID, in.SessionID, resource.Resource.Tenant, resource.Resource.Kind, resource.Resource.ID)
		if err != nil {
			return DiscoveryReceipt{}, err
		}
		resource.SourceID = ""
		resource.LastSeenAt = in.ObservedAt
		resource.Available = true
		b, _ := json.Marshal(resource)
		_, err = tx.Exec(ctx, `INSERT INTO connection_inventory(service_id,connection_id,tenant,resource_kind,resource_id,data,session_id,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(service_id,connection_id,tenant,resource_kind,resource_id) DO UPDATE SET data=excluded.data,session_id=excluded.session_id,observed_at=excluded.observed_at,available=true WHERE connection_inventory.observed_at<=excluded.observed_at`, in.ServiceID, session.ConnectionID, resource.Resource.Tenant, resource.Resource.Kind, resource.Resource.ID, b, in.SessionID, in.ObservedAt)
		if err != nil {
			return DiscoveryReceipt{}, err
		}
	}
	out := DiscoveryReceipt{SessionID: in.SessionID, State: Running, Accepted: len(in.Resources), ActorID: a.ID, Mode: session.Mode, Method: in.Method, ObservedAt: in.ObservedAt, Limitations: limitations}
	if in.Complete {
		out.State = Complete
		for _, kind := range definition.ResourceKinds {
			entry, ok := coverage[kind]
			if !ok || !entry.CanMarkAbsent() || len(limitations) > 0 {
				out.State = PartialDiscovery
				continue
			}
			_, err = tx.Exec(ctx, `UPDATE connection_inventory i SET available=false FROM connection_discovery_sessions previous WHERE i.service_id=$1 AND i.connection_id=$2 AND i.resource_kind=$3 AND NOT EXISTS(SELECT 1 FROM connection_discovery_seen seen WHERE seen.service_id=i.service_id AND seen.session_id=$4 AND seen.tenant=i.tenant AND seen.resource_kind=i.resource_kind AND seen.resource_id=i.resource_id) AND previous.service_id=i.service_id AND previous.id=i.session_id AND previous.started_at<=$5`, in.ServiceID, session.ConnectionID, kind, in.SessionID, session.StartedAt)
			if err != nil {
				return DiscoveryReceipt{}, err
			}
		}
	}
	combined := []KindCoverage{}
	for _, kind := range definition.ResourceKinds {
		if c, ok := coverage[kind]; ok {
			combined = append(combined, c)
		}
	}
	out.Coverage = combined
	out.NextCursor = in.NextCursor
	b, _ := json.Marshal(combined)
	metadata, _ := json.Marshal(struct {
		Method      string    `json:"method"`
		ObservedAt  time.Time `json:"observed_at"`
		Limitations []string  `json:"limitations"`
	}{in.Method, in.ObservedAt, limitations})
	_, err = tx.Exec(ctx, `UPDATE connection_discovery_sessions SET state=$3,coverage=$4,next_cursor=$5,metadata=$6,finished_at=CASE WHEN $7 THEN now() ELSE NULL END WHERE service_id=$1 AND id=$2`, in.ServiceID, in.SessionID, out.State, b, in.NextCursor, metadata, in.Complete)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	b, _ = json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO connection_discovery_pages(service_id,session_id,idempotency_key,request_hash,receipt) VALUES($1,$2,$3,$4,$5)`, in.ServiceID, in.SessionID, in.IdempotencyKey, digest, b)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	if err = audit(ctx, tx, a, d.Connection, "discovery.page"); err != nil {
		return DiscoveryReceipt{}, err
	}
	return out, tx.Commit(ctx)
}
