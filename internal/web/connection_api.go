package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
)

func connectionFailure(w http.ResponseWriter, err error) {
	code := "connection_operation_failed"
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, auth.ErrForbidden):
		uiFailure(w, err)
		return
	case errors.Is(err, connections.ErrInvalid):
		code = "invalid_input"
		status = 400
	case errors.Is(err, assessment.ErrInvalid):
		code = "invalid_input"
		status = 400
	case errors.Is(err, connections.ErrConflict), errors.Is(err, assessment.ErrConflict):
		code = "revision_conflict"
		status = 409
	case errors.Is(err, connections.ErrInactive):
		code = "connection_inactive"
		status = 409
	case errors.Is(err, connections.ErrCredentialUnavailable):
		code = "connection_credentials_unavailable"
		status = 409
	default:
		var recovery interface{ RecoveryCode() string }
		if errors.As(err, &recovery) {
			code = recovery.RecoveryCode()
		}
	}
	e := UIError{Code: code, Title: "Connection action needs attention", Explanation: "Review the provider setup and current revisions, then retry. Your form entries are preserved.", Retryable: status >= 500, Action: "Review connection setup"}
	if explanation, ok := map[string]string{
		"setup_account_locked":         "Generated trust is already bound to this AWS account. Cancel this setup and start a new one for a different account.",
		"aws_setup_material_required":  "Generate the AWS role setup first, apply its trust and read policies in the customer account, then verify the role ARN.",
		"role_trust_denied":            "Check the Fed principal, generated external ID and customer role trust policy. Allow IAM changes to propagate, then retry verification.",
		"fed_role_verification_denied": "The Fed task role needs iam:GetRole on its configured IAM role to verify its identity. Ask the operator to update the task-role permissions.",
		"fed_runtime_role_mismatch":    "The running Fed identity does not match KSI_AWS_FED_ROLE_ARN. Ask the operator to configure the application task role and its full IAM path.",
	}[code]; ok {
		e.Explanation = explanation
		e.Retryable = false
		status = http.StatusConflict
	}
	if status == 400 {
		e.FieldErrors = map[string]string{"configuration": "Check the account, provider-specific fields, source scope and required revisions."}
	}
	jsonResponse(w, status, e)
}

func connectionAction[I any, O any](mux *http.ServeMux, path string, call func(context.Context, auth.Actor, I) (O, error)) {
	mux.HandleFunc("POST /api/ui/v1/services/{service}/connections/"+path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			connectionFailure(w, connections.ErrInvalid)
			return
		}
		var envelope struct {
			ServiceID string `json:"service_id"`
		}
		if json.Unmarshal(body, &envelope) != nil || envelope.ServiceID != r.PathValue("service") {
			connectionFailure(w, connections.ErrInvalid)
			return
		}
		var in I
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil {
			connectionFailure(w, connections.ErrInvalid)
			return
		}
		out, err := call(r.Context(), auth.FromContext(r.Context()), in)
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
}

func (app *App) registerConnectionAPI(mux *http.ServeMux) {
	app.registerSetupAPI(mux)
	s := app.Connections
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/scope", func(w http.ResponseWriter, r *http.Request) {
		out, err := app.Store.Get(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), "scope", "scope")
		if errors.Is(err, assessment.ErrNotFound) {
			jsonResponse(w, 200, assessment.Record{Revision: 0})
			return
		}
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.List(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.URL.Query().Get("include_deleted") == "true")
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/providers", func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Authorize(r.Context(), s.DB, auth.FromContext(r.Context()), r.PathValue("service"), "read"); err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, s.Registry.Providers())
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/{connection}", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Get(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("connection"))
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/{connection}/inventory", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		limit := 0
		if query.Get("limit") != "" {
			var err error
			limit, err = strconv.Atoi(query.Get("limit"))
			if err != nil {
				connectionFailure(w, connections.ErrInvalid)
				return
			}
		}
		out, err := s.ListInventory(r.Context(), auth.FromContext(r.Context()), connections.InventoryQuery{ServiceID: r.PathValue("service"), ConnectionID: r.PathValue("connection"), Limit: limit, Cursor: query.Get("cursor"), Search: query.Get("q"), Selection: query.Get("selection"), Kind: query.Get("kind")})
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/operations/{operation}/status", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.GetOperation(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("operation"))
		if err != nil {
			connectionFailure(w, err)
			return
		}
		jsonResponse(w, 200, out)
	})
	connectionAction(mux, "configure", s.Configure)
	connectionAction(mux, "resolve-commit", s.ResolveRepositoryCommit)
	connectionAction(mux, "test", s.TestWithReceipt)
	connectionAction(mux, "renew", s.Renew)
	connectionAction(mux, "authorize", func(ctx context.Context, a auth.Actor, in connections.OperationInput) (connections.AuthorizationHandoff, error) {
		return s.BeginAuthorization(ctx, a, in, app.ConnectionOrigin)
	})
	connectionAction(mux, "discover", s.StartServerDiscovery)
	connectionAction(mux, "discover-page", s.DiscoverServerPage)
	connectionAction(mux, "select", s.SelectSources)
	connectionAction(mux, "configure-source", s.ConfigureSource)
	connectionAction(mux, "refresh-source", s.RefreshSource)
	for _, operation := range []string{"disconnect", "delete", "restore"} {
		connectionAction(mux, operation, func(ctx context.Context, a auth.Actor, in connections.OperationInput) (connections.Detail, error) {
			return s.ChangeLifecycle(ctx, a, in, operation)
		})
	}
	for _, operation := range []string{"retire", "disable", "restore", "rebind"} {
		connectionAction(mux, operation+"-source", func(ctx context.Context, a auth.Actor, in connections.SourceOperationInput) (assessment.Record, error) {
			return s.ChangeSource(ctx, a, in, operation)
		})
	}
}
