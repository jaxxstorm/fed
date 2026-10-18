package connections

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type ServerDiscoveryInput struct {
	ServiceID      string `json:"service_id"`
	SessionID      string `json:"session_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Cursor         string `json:"cursor"`
	Limit          int    `json:"limit"`
}

func (s *Service) StartServerDiscovery(ctx context.Context, a auth.Actor, in OperationInput) (DiscoverySession, error) {
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return DiscoverySession{}, err
	}
	d, err := readConnection(ctx, s.DB, in.ServiceID, in.ConnectionID)
	if err != nil {
		return DiscoverySession{}, err
	}
	if d.Connection.Mode != ServerManaged && d.Connection.Mode != ServerEnvironment {
		return DiscoverySession{}, ErrInvalid
	}
	if _, err = s.Registry.Adapter(d.Connection.Provider); err != nil {
		return DiscoverySession{}, err
	}
	return s.startDiscovery(ctx, a, in, d.Connection.Mode)
}

func (s *Service) DiscoverServerPage(ctx context.Context, a auth.Actor, in ServerDiscoveryInput) (DiscoveryReceipt, error) {
	limit, err := PageSize(in.Limit)
	if err != nil || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || len(in.Cursor) > 4096 {
		return DiscoveryReceipt{}, ErrInvalid
	}
	in.Limit = limit
	if err = auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return DiscoveryReceipt{}, err
	}
	var session DiscoverySession
	err = s.DB.QueryRow(ctx, `SELECT connection_id,generation,actor_id,mode,state,next_cursor FROM connection_discovery_sessions WHERE service_id=$1 AND id=$2`, in.ServiceID, in.SessionID).Scan(&session.ConnectionID, &session.Generation, &session.ActorID, &session.Mode, &session.State, &session.NextCursor)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	if session.ActorID != a.ID || (session.Mode != ServerManaged && session.Mode != ServerEnvironment) {
		return DiscoveryReceipt{}, auth.ErrForbidden
	}
	d, err := readConnection(ctx, s.DB, in.ServiceID, session.ConnectionID)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	if err = d.Connection.CheckGeneration(session.Generation); err != nil {
		return DiscoveryReceipt{}, err
	}
	request, _ := json.Marshal(in)
	var digest string
	var prior []byte
	err = s.DB.QueryRow(ctx, `SELECT request_hash,receipt FROM connection_discovery_pages WHERE service_id=$1 AND session_id=$2 AND idempotency_key=$3`, in.ServiceID, in.SessionID, in.IdempotencyKey).Scan(&digest, &prior)
	if err == nil {
		if digest != storage.Hash(request) {
			return DiscoveryReceipt{}, ErrConflict
		}
		var out DiscoveryReceipt
		err = json.Unmarshal(prior, &out)
		if err == nil {
			logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "connection.discovery.page", Component: "connections", ActorID: a.ID, ServiceID: in.ServiceID, ConnectionID: session.ConnectionID, Outcome: "replayed", Count: out.Accepted})
		}
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DiscoveryReceipt{}, err
	}
	if session.State != Running || session.NextCursor != in.Cursor {
		return DiscoveryReceipt{}, ErrConflict
	}
	adapter, err := s.Registry.Adapter(d.Connection.Provider)
	if err != nil {
		return DiscoveryReceipt{}, err
	}
	var credentials Credentials
	if d.Connection.Mode == ServerManaged {
		credentials, err = s.Credentials.Get(ctx, a, in.ServiceID, session.ConnectionID, "connect", session.Generation)
		if err != nil {
			return DiscoveryReceipt{}, err
		}
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	page, callErr := adapter.Discover(call, d.Connection, credentials, in.Cursor, limit)
	partial := false
	for _, coverage := range page.Coverage {
		if coverage.ErrorCode != "" {
			partial = true
			logging.Emit(ctx, zerolog.WarnLevel, logging.Fields{Event: "provider.discovery.coverage", Component: "provider", Provider: string(d.Connection.Provider), ServiceID: in.ServiceID, ConnectionID: session.ConnectionID, Outcome: "partial", ErrorCode: coverage.ErrorCode})
		}
	}
	providerResult(ctx, d.Connection, "discover", start, callErr, partial)
	complete := page.NextCursor == ""
	if callErr != nil {
		complete = false
		page.NextCursor = in.Cursor
		definition, _ := s.Registry.Provider(d.Connection.Provider)
		page.Coverage = nil
		for _, kind := range definition.ResourceKinds {
			page.Coverage = append(page.Coverage, KindCoverage{Kind: kind, ErrorCode: "provider_error"})
		}
	}
	return s.submitDiscovery(ctx, a, DiscoverySubmission{ServiceID: in.ServiceID, SessionID: in.SessionID, IdempotencyKey: in.IdempotencyKey, ObservedAt: time.Now().UTC(), Method: "provider." + string(d.Connection.Provider), Resources: page.Resources, Coverage: page.Coverage, NextCursor: page.NextCursor, Complete: complete}, false, request)
}
