package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
)

func (app *App) registerSetupAPI(mux *http.ServeMux) {
	s := app.Connections
	connectionAction(mux, "setups/begin", s.BeginSetup)
	connectionAction(mux, "setups/update", func(ctx context.Context, a auth.Actor, in connections.UpdateSetupInput) (connections.Setup, error) {
		return s.UpdateSetup(ctx, a, in.SetupOperationInput, in.Input)
	})
	connectionAction(mux, "setups/cancel", s.CancelSetup)
	connectionAction(mux, "setups/complete", s.CompleteSetup)
	connectionAction(mux, "setups/aws-material", s.AWSSetupMaterial)
	connectionAction(mux, "setups/aws-verify", s.VerifyAWSSetup)
	connectionAction(mux, "setups/aws-rotate", s.RotateAWSSetupExternalID)
	connectionAction(mux, "setups/authorize", func(ctx context.Context, a auth.Actor, in connections.SetupOperationInput) (connections.AuthorizationHandoff, error) {
		return s.SetupHandoff(ctx, a, in, app.ConnectionOrigin)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/setups/{setup}/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		out, err := s.GetSetup(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("setup"))
		if err != nil {
			connectionFailure(w, err, r)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /api/ui/v1/services/{service}/connections/readiness/{provider}/status", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.ProviderSetupReadiness(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), connections.Provider(r.PathValue("provider")))
		if err != nil {
			connectionFailure(w, err, r)
			return
		}
		jsonResponse(w, 200, out)
	})
	mux.HandleFunc("GET /connections/setup/authorize", app.setupAuthorize)
	mux.HandleFunc("GET /connections/setup/github", app.setupGitHubReturn)
}

func setupTarget(setup connections.Setup) string {
	return base(setup.ServiceID) + "?page=Connections&provider=" + url.QueryEscape(string(setup.Provider)) + "&setup=" + url.QueryEscape(setup.ID)
}

func (app *App) setupAuthorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	a := auth.FromContext(r.Context())
	setup, err := app.Connections.ClaimSetupHandoff(r.Context(), a, r.URL.Query().Get("handoff"), connectionSessionHash(r))
	if err != nil {
		connectionFailure(w, err, r)
		return
	}
	stage := "oauth"
	if setup.Provider == connections.GitHub {
		stage = "installation"
	}
	target, err := app.Connections.SetupAuthorizationURL(r.Context(), a, connections.SetupOperationInput{ServiceID: setup.ServiceID, SetupID: setup.ID, ExpectedRevision: setup.Revision}, connectionSessionHash(r), app.ConnectionOrigin, stage)
	if err != nil {
		connectionFailure(w, err, r)
		return
	}
	if !connections.AllowedAuthorizationURL(setup.Provider, target) {
		connectionFailure(w, connections.ErrInvalid, r)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (app *App) setupGitHubReturn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	a := auth.FromContext(r.Context())
	claim, err := app.Connections.ClaimSetupAuthorization(r.Context(), a, connections.GitHub, r.URL.Query().Get("state"), connectionSessionHash(r), strings.TrimRight(app.ConnectionOrigin, "/")+"/connections/setup/github")
	if err != nil {
		if setup, recoveryErr := app.Connections.SetupAuthorizationRecovery(r.Context(), a, r.URL.Query().Get("state"), connectionSessionHash(r)); recoveryErr == nil {
			http.Redirect(w, r, setupTarget(setup)+"&setup_error=authorization_expired", http.StatusSeeOther)
			return
		}
		connectionFailure(w, err, r)
		return
	}
	if r.URL.Query().Get("setup_action") == "request" || r.URL.Query().Get("installation_id") == "" {
		_ = app.Connections.FailSetupConsent(r.Context(), a, claim, connectionSessionHash(r), "administrator_approval_pending")
		http.Redirect(w, r, setupTarget(claim.Setup)+"&setup_error=administrator_approval_pending", http.StatusSeeOther)
		return
	}
	if err = app.Connections.ReturnSetupInstallation(r.Context(), a, claim, connectionSessionHash(r), r.URL.Query().Get("installation_id")); err != nil {
		connectionFailure(w, err, r)
		return
	}
	target, err := app.Connections.SetupAuthorizationURL(r.Context(), a, connections.SetupOperationInput{ServiceID: claim.Setup.ServiceID, SetupID: claim.Setup.ID, ExpectedRevision: claim.Setup.Revision}, connectionSessionHash(r), app.ConnectionOrigin, "oauth")
	if err != nil {
		connectionFailure(w, err, r)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (app *App) setupCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	a := auth.FromContext(r.Context())
	provider := connections.Provider(r.PathValue("provider"))
	claim, err := app.Connections.ClaimSetupAuthorization(r.Context(), a, provider, r.URL.Query().Get("state"), connectionSessionHash(r), strings.TrimRight(app.ConnectionOrigin, "/")+"/connections/callback/"+string(provider))
	if err != nil {
		if setup, recoveryErr := app.Connections.SetupAuthorizationRecovery(r.Context(), a, r.URL.Query().Get("state"), connectionSessionHash(r)); recoveryErr == nil {
			http.Redirect(w, r, setupTarget(setup)+"&setup_error=authorization_expired", http.StatusSeeOther)
			return
		}
		connectionFailure(w, err, r)
		return
	}
	code := ""
	if r.URL.Query().Get("error") != "" {
		code = "authorization_cancelled"
		if r.URL.Query().Get("error") == "admin_policy_enforced" || r.URL.Query().Get("error") == "org_internal" {
			code = "workspace_policy_restricted"
		}
	} else if len(r.URL.Query().Get("code")) > 4096 {
		code = "authorization_invalid_code"
	} else {
		_, err = app.Connections.ExchangeSetupConsent(r.Context(), a, claim, connectionSessionHash(r), r.URL.Query().Get("code"))
		if err != nil {
			code = "authorization_exchange_failed"
			var recovery interface{ RecoveryCode() string }
			if errors.As(err, &recovery) {
				code = recovery.RecoveryCode()
			}
		}
	}
	target := setupTarget(claim.Setup)
	if code != "" {
		_ = app.Connections.FailSetupConsent(r.Context(), a, claim, connectionSessionHash(r), code)
		target += "&setup_error=" + url.QueryEscape(code)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
