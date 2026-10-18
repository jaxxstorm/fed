// Package logging defines Fed's bounded, payload-free operational event schema.
package logging

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type key struct{}
type requestKey struct{}
type operationKey struct{}

// Fields deliberately has no arbitrary payload, error or header field. Callers
// supply only known operation names and resolved internal identifiers.
type Fields struct {
	RecordID                                        string
	RequestID, OperationID, ActorID, ServiceID      string
	JobID, BatchID, ConnectionID, SetupID           string
	Event, Component, Outcome, ErrorCode, Reference string
	Route, Method, Tool, Provider, Stage            string
	Status, Attempt, Revision, Count                int
	Generation                                      int64
	Duration                                        time.Duration
	LifetimeSeconds                                 int64
	Actions                                         []string
	ProtocolCode                                    int
}

type scope struct {
	logger zerolog.Logger
	fields Fields
}
type Request struct {
	mu               sync.Mutex
	fields           Fields
	multipleServices bool
}

type safeWriter struct {
	mu       sync.Mutex
	out      io.Writer
	fallback io.Writer
	once     sync.Once
}

func (w *safeWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.out.Write(p)
	if err != nil || n != len(p) {
		w.once.Do(func() {
			if w.fallback != nil {
				_, _ = io.WriteString(w.fallback, "fed: operational log output failed\n")
			}
		})
	}
	// A broken logging sink must not alter the result of a committed operation.
	return len(p), nil
}

func New(out, fallback io.Writer, level, format, mode string) (zerolog.Logger, error) {
	level = strings.ToLower(level)
	if level == "" {
		level = "info"
	}
	var l zerolog.Level
	switch level {
	case "debug":
		l = zerolog.DebugLevel
	case "info":
		l = zerolog.InfoLevel
	case "warn":
		l = zerolog.WarnLevel
	case "error":
		l = zerolog.ErrorLevel
	default:
		return zerolog.Nop(), errors.New("invalid KSI_LOG_LEVEL")
	}
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "console" {
		return zerolog.Nop(), errors.New("invalid KSI_LOG_FORMAT")
	}
	if out == nil {
		out = io.Discard
	}
	writer := io.Writer(&safeWriter{out: out, fallback: fallback})
	if format == "console" {
		writer = zerolog.ConsoleWriter{Out: writer, NoColor: true, TimeFormat: time.RFC3339}
	}
	return zerolog.New(writer).Level(l).With().Str("mode", mode).Logger(), nil
}

func FromEnv(get func(string) string, mode string) (zerolog.Logger, error) {
	return New(os.Stdout, os.Stderr, get("KSI_LOG_LEVEL"), get("KSI_LOG_FORMAT"), mode)
}
func WithLogger(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, key{}, scope{logger: l})
}

// Inherit copies only logging metadata; the SDK retains its own cancellation.
func Inherit(ctx, parent context.Context) context.Context {
	ctx = context.WithValue(ctx, key{}, get(parent))
	if r := parent.Value(requestKey{}); r != nil {
		ctx = context.WithValue(ctx, requestKey{}, r)
	}
	return ctx
}
func get(ctx context.Context) scope {
	if s, ok := ctx.Value(key{}).(scope); ok {
		return s
	}
	return scope{logger: zerolog.Nop()}
}
func Operation(ctx context.Context) context.Context {
	s := get(ctx)
	s.fields.OperationID = uuid.NewString()
	ctx = context.WithValue(ctx, key{}, s)
	return context.WithValue(ctx, operationKey{}, &Request{})
}

// Authorized is called only after service membership and action checks pass.
func Authorized(ctx context.Context, actor, service string) {
	if r, ok := ctx.Value(operationKey{}).(*Request); ok {
		r.mu.Lock()
		r.fields.ActorID = actor
		if service != "" {
			if r.fields.ServiceID != "" && r.fields.ServiceID != service {
				r.multipleServices = true
			}
			if r.multipleServices {
				r.fields.ServiceID = ""
			} else {
				r.fields.ServiceID = service
			}
		}
		r.mu.Unlock()
	}
}
func Identity(ctx context.Context, actor, service string) context.Context {
	s := get(ctx)
	s.fields.ActorID = actor
	s.fields.ServiceID = service
	return context.WithValue(ctx, key{}, s)
}
func NewRequest(ctx context.Context) (context.Context, string) {
	s := get(ctx)
	id := uuid.NewString()
	s.fields.RequestID = id
	ctx = context.WithValue(ctx, key{}, s)
	return context.WithValue(ctx, requestKey{}, &Request{}), id
}
func Route(ctx context.Context, route string) {
	if r, ok := ctx.Value(requestKey{}).(*Request); ok {
		r.mu.Lock()
		r.fields.Route = route
		r.mu.Unlock()
	}
}
func Actor(ctx context.Context, id string) {
	if r, ok := ctx.Value(requestKey{}).(*Request); ok {
		r.mu.Lock()
		r.fields.ActorID = id
		r.mu.Unlock()
	}
}
func bound(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func Emit(ctx context.Context, level zerolog.Level, f Fields) {
	f.ErrorCode = SafeCode(f.ErrorCode)
	s := get(ctx)
	if r, ok := ctx.Value(operationKey{}).(*Request); ok {
		r.mu.Lock()
		if f.ActorID == "" {
			f.ActorID = r.fields.ActorID
		}
		if f.ServiceID == "" {
			f.ServiceID = r.fields.ServiceID
		}
		r.mu.Unlock()
	}
	if f.RequestID == "" {
		f.RequestID = s.fields.RequestID
	}
	if f.OperationID == "" {
		f.OperationID = s.fields.OperationID
	}
	if f.ActorID == "" {
		f.ActorID = s.fields.ActorID
	}
	if f.ServiceID == "" {
		f.ServiceID = s.fields.ServiceID
	}
	if r, ok := ctx.Value(requestKey{}).(*Request); ok {
		r.mu.Lock()
		if f.Route == "" {
			f.Route = r.fields.Route
		}
		if f.ActorID == "" {
			f.ActorID = r.fields.ActorID
		}
		r.mu.Unlock()
	}
	e := s.logger.WithLevel(level)
	if e == nil {
		return
	}
	e.Time("time", time.Now().UTC()).Str("event", bound(f.Event)).Str("component", bound(f.Component))
	for _, v := range [][2]string{{"request_id", f.RequestID}, {"operation_id", f.OperationID}, {"actor_id", f.ActorID}, {"service_id", f.ServiceID}, {"job_id", f.JobID}, {"batch_id", f.BatchID}, {"connection_id", f.ConnectionID}, {"setup_id", f.SetupID}, {"outcome", f.Outcome}, {"error_code", f.ErrorCode}, {"reference", f.Reference}, {"route", f.Route}, {"method", f.Method}, {"tool", f.Tool}, {"provider", f.Provider}, {"stage", f.Stage}} {
		if v[1] != "" {
			e.Str(v[0], bound(v[1]))
		}
	}
	if f.Status != 0 {
		e.Int("status", f.Status)
	}
	if f.ProtocolCode != 0 {
		e.Int("protocol_code", f.ProtocolCode)
	}
	if f.RecordID != "" {
		e.Str("record_id", bound(f.RecordID))
	}
	if f.LifetimeSeconds != 0 {
		e.Int64("lifetime_seconds", f.LifetimeSeconds)
	}
	if len(f.Actions) > 0 {
		e.Strs("actions", f.Actions)
	}
	if f.Attempt != 0 {
		e.Int("attempt", f.Attempt)
	}
	if f.Revision != 0 {
		e.Int("revision", f.Revision)
	}
	if f.Generation != 0 {
		e.Int64("generation", f.Generation)
	}
	if f.Count != 0 {
		e.Int("count", f.Count)
	}
	if f.Duration != 0 {
		e.Float64("duration_ms", float64(f.Duration)/float64(time.Millisecond))
	}
	e.Msg(strings.ReplaceAll(f.Event, ".", " "))
}

// SafeCode never treats arbitrary provider/driver error text as log metadata.
func SafeCode(code string) string {
	switch code {
	case "membership_pending", "membership_conflict", "invalid_membership", "last_administrator", "membership_disabled":
		return code
	case "", "internal_error", "cancelled", "deadline_exceeded", "invalid_logging_configuration", "handler_panic", "transport_rejected", "protocol_error", "tool_error", "forbidden", "invalid_input", "not_found", "revision_conflict", "view_changed", "out_of_scope", "artifact_too_large", "invalid_observation", "connection_inactive", "connection_credentials_unavailable", "integration_unavailable", "connection_operation_failed", "connection_changed", "authorization_revoked", "collection_error", "scope_or_permission_changed", "wrong_tenant", "insufficient_permissions", "rate_limited", "grant_expired", "resource_unavailable", "role_trust_denied", "provider_error", "profile_sign_in_required", "fed_runtime_role_mismatch", "fed_role_verification_denied", "workspace_policy_restricted", "installation_removed", "installation_suspended", "authorization_cancelled", "authorization_exchange_failed", "administrator_approval_pending", "offline_consent_required", "customer_identity_permission_required", "missing_application_configuration", "setup_account_locked", "aws_setup_material_required", "delegated_role_configuration_required":
		return code
	default:
		return "unclassified_error"
	}
}

// Commit emits only after the database acknowledges a transaction. Replay is
// explicit so retries cannot be mistaken for new product changes.
func Commit(ctx context.Context, tx interface{ Commit(context.Context) error }, f Fields) error {
	err := tx.Commit(ctx)
	if err != nil {
		f.Outcome = "failed"
		f.ErrorCode = Code(err)
		Emit(ctx, zerolog.ErrorLevel, f)
	} else {
		if f.Outcome == "" {
			f.Outcome = "success"
		}
		level := zerolog.InfoLevel
		switch f.Outcome {
		case "failed", "partial", "denied", "expired", "error", "authorization_required":
			level = zerolog.WarnLevel
		}
		Emit(ctx, level, f)
	}
	return err
}
func Level(status int) zerolog.Level {
	if status >= 500 {
		return zerolog.ErrorLevel
	}
	if status >= 400 {
		return zerolog.WarnLevel
	}
	return zerolog.InfoLevel
}
func Code(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "internal_error"
}
