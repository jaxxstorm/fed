package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func (app *App) TrustHandler(browser *auth.Browser) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /trust/{slug}", app.publicTrustCentre)
	m.Handle("GET /trust/{slug}/access", browser.Protect(http.HandlerFunc(app.trustAccess)))
	m.Handle("POST /trust/{slug}/access", browser.Protect(http.HandlerFunc(app.trustAccess)))
	m.Handle("GET /trust/{slug}/releases/{release}/", browser.Protect(http.HandlerFunc(app.restrictedTrustRelease)))
	m.Handle("GET /trust/{slug}/releases/{release}/evidence/{hash}", browser.Protect(http.HandlerFunc(app.trustArtifact)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		m.ServeHTTP(w, r)
	})
}

var publicTrustTemplate = template.Must(template.New("trust").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Trust centre</title><style>body{font:16px/1.6 system-ui;max-width:960px;margin:2rem auto;padding:1rem}section{border-bottom:1px solid #ccc;padding:1rem 0}pre{white-space:pre-wrap}</style><main><h1>Trust centre</h1><p>Published assessment reports. Publication is not a FedRAMP authorization claim.</p><p><a href="/auth/login?return_to=/trust/{{.Slug}}/access">Sign in to request evidence access</a></p><p>External readers must be supported by this service's configured identity provider and admission policy.</p>{{range .Releases}}<section>{{if eq .State "withdrawn"}}<h2>Withdrawn release {{.ID}}</h2><p>{{.Label}}</p>{{else}}<h2>{{.Public.ServiceName}}</h2><p>{{.Public.Summary}}</p><p>{{.Label}}</p><p>Period {{.Period.Start}} – {{.Period.End}}. Prepared {{.PreparedAt}}.</p>{{if .Incomplete}}<strong>Incomplete report: unassessed or unsupported requirements remain.</strong>{{end}}{{range .Public.Contacts}}<p>{{.}}</p>{{end}}{{range .Public.Requirements}}<details><summary>{{.RequirementID}}</summary><p>{{.Summary}}</p></details>{{end}}<p><a href="/trust/{{$.Slug}}/releases/{{.ID}}/">Open restricted assessment and evidence</a></p>{{end}}</section>{{else}}<p>No published releases.</p>{{end}}{{if .Next}}<a href="?offset={{.Next}}">Next releases</a>{{end}}</main></html>`))

func (app *App) publicTrustCentre(w http.ResponseWriter, r *http.Request) {
	offset := 0
	var err error
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			http.Error(w, "invalid offset", 400)
			return
		}
	}
	items, err := app.Store.PublicReleaseIndex(r.Context(), r.PathValue("slug"), 25, offset)
	if err != nil {
		http.Error(w, "trust centre unavailable", 404)
		return
	}
	if r.URL.Query().Get("format") == "json" {
		jsonResponse(w, 200, items)
		return
	}
	next := 0
	if len(items) == 25 {
		next = offset + 25
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = publicTrustTemplate.Execute(w, struct {
		Slug     string
		Releases []assessment.PublicReleaseView
		Next     int
	}{r.PathValue("slug"), items, next})
}

var trustAccessTemplate = template.Must(template.New("access").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><title>Evidence access</title><main><h1>Request evidence access</h1><p>{{.Notice}}</p><p>A trust-centre grant provides only the approved service or release access. It does not grant access to the organization's internal workspace.</p><form method="post"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><label>Release ID (leave empty to request this service's published releases)<input name="release_id"></label><button>Request access</button></form><h2>Your requests</h2>{{range .Requests}}<p>{{.ID}}: {{.State}} · release {{.ReleaseID}}</p>{{else}}<p>No requests yet.</p>{{end}}<a href="/trust/{{.Slug}}">Return to published releases</a></main></html>`))

func (app *App) trustAccess(w http.ResponseWriter, r *http.Request) {
	a := auth.FromContext(r.Context())
	slug := r.PathValue("slug")
	service, err := app.Store.TrustService(r.Context(), slug)
	if err != nil {
		http.Error(w, "trust centre unavailable", 404)
		return
	}
	notice := "An administrator will review your request."
	if r.Method == http.MethodPost {
		_, err := app.Store.RequestTrustAccess(r.Context(), a, slug, r.FormValue("release_id"))
		if err != nil {
			http.Error(w, "request unavailable", 403)
			return
		}
		notice = "Access request recorded. No organization membership was approved."
	}
	rows, err := app.Store.DB.Query(r.Context(), `SELECT id,state,coalesce(release_id,'') FROM trust_access_requests WHERE service_id=$1 AND identity_id=$2 ORDER BY requested_at DESC LIMIT 100`, service, a.ID)
	if err != nil {
		http.Error(w, "requests unavailable", 503)
		return
	}
	defer rows.Close()
	items := []assessment.TrustAccessRequest{}
	for rows.Next() {
		var item assessment.TrustAccessRequest
		if err = rows.Scan(&item.ID, &item.State, &item.ReleaseID); err != nil {
			http.Error(w, "requests unavailable", 503)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "requests unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = trustAccessTemplate.Execute(w, struct {
		Slug, CSRF, Notice string
		Requests           []assessment.TrustAccessRequest
	}{slug, auth.CSRF(r.Context()), notice, items})
}

func (app *App) restrictedTrustRelease(w http.ResponseWriter, r *http.Request) {
	release, err := app.Store.RestrictedRelease(r.Context(), auth.FromContext(r.Context()), r.PathValue("slug"), r.PathValue("release"))
	if err != nil {
		http.Error(w, "Approved, unexpired access to this published release is required.", 403)
		return
	}
	app.deliverRelease(w, r, release)
}
func (app *App) deliverRelease(w http.ResponseWriter, r *http.Request, release assessment.AssessmentRelease) {
	w.Header().Set("Cache-Control", "private, no-store")
	switch r.URL.Query().Get("format") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		content, err := json.Marshal(release.Manifest)
		if err != nil {
			http.Error(w, "manifest unavailable", 500)
			return
		}
		_, _ = w.Write(content)
	case "zip":
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="assessment.zip"`)
		if err := app.Store.WriteReleaseArchive(r.Context(), w, release.Manifest); err != nil {
			panic(http.ErrAbortHandler)
		}
	default:
		content, err := assessment.ReleaseHTML(release.Manifest)
		if err != nil {
			http.Error(w, "report unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}
func (app *App) trustArtifact(w http.ResponseWriter, r *http.Request) {
	release, err := app.Store.RestrictedRelease(r.Context(), auth.FromContext(r.Context()), r.PathValue("slug"), r.PathValue("release"))
	if err != nil {
		http.Error(w, "access denied", 403)
		return
	}
	key := r.PathValue("hash")
	found := false
	for _, e := range release.Manifest.Evidence {
		if e.Hash == key {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "artifact not in this release", 404)
		return
	}
	content, err := app.Store.Artifacts.Get(r.Context(), key)
	if err != nil || storage.Hash(content) != key {
		http.Error(w, "artifact unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="evidence-`+key+`"`)
	_, _ = w.Write(content)
}
