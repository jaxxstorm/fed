package web

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
)

func connectionSessionHash(r *http.Request) string {
	return auth.SessionHash(r.Context())
}

func (app *App) connectionAuthorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	a := auth.FromContext(r.Context())
	service, operation := r.PathValue("service"), r.PathValue("operation")
	op, err := app.Connections.GetOperation(r.Context(), a, service, operation)
	if err != nil {
		failure(w, r, err)
		return
	}
	d, err := app.Connections.Get(r.Context(), a, service, op.ConnectionID)
	if err != nil {
		failure(w, r, err)
		return
	}
	adapter, err := app.Connections.Registry.Adapter(d.Connection.Provider)
	if err != nil {
		http.Error(w, "Configure this provider application before authorizing.", http.StatusConflict)
		return
	}
	authorizer, ok := adapter.(connections.Authorizer)
	if d.Connection.Provider == connections.Kubernetes || d.Connection.Provider == connections.AWS {
		challenge, err := app.Connections.BindAuthorizationBrowser(r.Context(), a, service, operation, connectionSessionHash(r))
		if err != nil {
			http.Error(w, "Authorization expired. Start a new upload attempt.", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := kubeconfigUploadPage
		if d.Connection.Provider == connections.AWS {
			page = awsBindingUploadPage
		}
		_ = page.Execute(w, struct{ State, CSRF string }{challenge.State, auth.CSRF(r.Context())})
		return
	}
	if !ok {
		http.Error(w, "This access mode does not support browser authorization.", http.StatusConflict)
		return
	}
	challenge, err := app.Connections.BindAuthorizationBrowser(r.Context(), a, service, operation, connectionSessionHash(r))
	if err != nil {
		http.Error(w, "Authorization expired or browser session unavailable. Start a new attempt.", http.StatusConflict)
		return
	}
	target, err := authorizer.AuthorizationURL(d.Connection, challenge)
	if err != nil || !connections.AllowedAuthorizationURL(d.Connection.Provider, target) {
		http.Error(w, "Provider authorization configuration is invalid.", http.StatusConflict)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

var kubeconfigUploadPage = template.Must(template.New("upload").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect Kubernetes</title><main><h1>Upload Kubernetes access</h1><p>Use a single-context kubeconfig with embedded read-only credentials and TLS verification. Executable helpers and local file references are rejected. The endpoint must be allowed by the server operator.</p><form method="post" action="/connections/upload/kubernetes"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><input type="hidden" name="state" value="{{.State}}"><label for="kubeconfig">Kubeconfig</label><textarea id="kubeconfig" name="kubeconfig" rows="20" cols="60" required autocomplete="off" spellcheck="false"></textarea><p>Credentials are encrypted and cannot be read back through the API.</p><button type="submit">Verify and save access</button></form></main></html>`))

var awsBindingUploadPage = template.Must(template.New("aws-binding").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Bind AWS role access</title><main><h1>Verify AWS role trust</h1><p>The server uses the configured profile and assume-role ARN. Enter the external ID required by the target role trust policy. No AWS access keys are uploaded.</p><form method="post" action="/connections/upload/aws"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><input type="hidden" name="state" value="{{.State}}"><label for="external-id">Role external ID</label><input id="external-id" name="external_id" type="password" maxlength="1224" autocomplete="off"><button type="submit">Verify role and save binding</button></form></main></html>`))

func (app *App) connectionUpload(w http.ResponseWriter, r *http.Request) {
	provider := connections.Provider(r.PathValue("provider"))
	if provider != connections.Kubernetes && provider != connections.AWS {
		http.NotFound(w, r)
		return
	}
	field := "kubeconfig"
	if provider == connections.AWS {
		field = "external_id"
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if r.ParseForm() != nil || len(r.PostForm.Get(field)) > connections.MaxResponseBytes {
		http.Error(w, "Invalid kubeconfig upload.", 400)
		return
	}
	a := auth.FromContext(r.Context())
	claim, err := app.Connections.ClaimAuthorization(r.Context(), a, provider, r.PostForm.Get("state"), connectionSessionHash(r), strings.TrimRight(app.ConnectionOrigin, "/")+"/connections/upload/"+string(provider))
	if err != nil {
		http.Error(w, "Upload attempt expired. Start a new attempt.", 409)
		return
	}
	target := base(claim.ServiceID) + "?page=Connections&connection=" + url.QueryEscape(claim.ConnectionID) + "&operation=" + url.QueryEscape(claim.OperationID)
	complete := func() error {
		d, err := app.Connections.Get(r.Context(), a, claim.ServiceID, claim.ConnectionID)
		if err != nil {
			return err
		}
		adapter, err := app.Connections.Registry.Adapter(provider)
		if err != nil {
			return err
		}
		credentials := connections.NewCredentials(map[string]string{field: r.PostForm.Get(field)})
		r.PostForm.Del(field)
		r.Form.Del(field)
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		health, err := adapter.Test(ctx, d.Connection, credentials)
		if err != nil {
			return err
		}
		if health.State != connections.Verified {
			return connections.ErrInvalid
		}
		_, err = app.Connections.CompleteAuthorization(ctx, a, claim, connectionSessionHash(r), health.VerifiedAccountID, credentials)
		return err
	}
	if err = complete(); err != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		_ = app.Connections.FailAuthorization(ctx, a, claim, "authorization_binding_failed")
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (app *App) connectionCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	provider := connections.Provider(r.PathValue("provider"))
	definition, ok := app.Connections.Registry.Provider(provider)
	if !ok || definition.Guide.CallbackPath == "" {
		http.NotFound(w, r)
		return
	}
	redirectURI := strings.TrimRight(app.ConnectionOrigin, "/") + definition.Guide.CallbackPath
	a := auth.FromContext(r.Context())
	claim, err := app.Connections.ClaimAuthorization(r.Context(), a, provider, r.URL.Query().Get("state"), connectionSessionHash(r), redirectURI)
	if err != nil {
		http.Redirect(w, r, "/?connection_error=authorization_expired", http.StatusSeeOther)
		return
	}
	target := base(claim.ServiceID) + "?page=Connections&connection=" + url.QueryEscape(claim.ConnectionID) + "&operation=" + url.QueryEscape(claim.OperationID)
	fail := func(code string) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		_ = app.Connections.FailAuthorization(ctx, a, claim, code)
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
	if r.URL.Query().Get("error") != "" {
		fail("authorization_cancelled")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 4096 {
		fail("authorization_invalid_code")
		return
	}
	d, err := app.Connections.Get(r.Context(), a, claim.ServiceID, claim.ConnectionID)
	if err != nil {
		fail("authorization_unavailable")
		return
	}
	adapter, err := app.Connections.Registry.Adapter(provider)
	if err != nil {
		fail("missing_application_configuration")
		return
	}
	authorizer, ok := adapter.(connections.Authorizer)
	if !ok {
		fail("missing_application_configuration")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	grant, err := authorizer.ExchangeAuthorization(ctx, d.Connection, claim, connections.AuthorizationResponse{Code: code, InstallationID: r.URL.Query().Get("installation_id")})
	if err != nil {
		fail("authorization_exchange_failed")
		return
	}
	if _, err = app.Connections.CompleteAuthorization(ctx, a, claim, connectionSessionHash(r), grant.AccountID, grant.Credentials, grant.AccountHint); err != nil {
		fail("authorization_binding_failed")
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
