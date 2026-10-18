package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
)

type uiContext struct{}

func isUI(r *http.Request) bool { return r.Context().Value(uiContext{}) == true }
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type UIError struct {
	Code        string            `json:"code"`
	Title       string            `json:"title"`
	Explanation string            `json:"explanation"`
	Retryable   bool              `json:"retryable"`
	Reference   string            `json:"reference"`
	Action      string            `json:"action"`
	FieldErrors map[string]string `json:"field_errors,omitempty"`
}

func uiFailure(w http.ResponseWriter, err error, contexts ...context.Context) {
	status := 500
	e := UIError{Code: "operation_failed", Title: "We couldn’t complete this action", Explanation: "Your changes have not been confirmed. Use the diagnostic reference when contacting the service administrator.", Reference: uuid.NewString()}
	switch {
	case errors.Is(err, auth.ErrMembershipPending):
		status = 403
		e.Code = "membership_pending"
		e.Title = "Waiting for approval"
		e.Explanation = "An administrator must approve your access."
		e.Action = "/approval"
	case errors.Is(err, auth.ErrLastAdmin):
		status = 409
		e.Code = "last_administrator"
		e.Title = "Keep an eligible administrator"
		e.Explanation = "Approve another administrator first and ensure your account remains eligible for the organization policy."
	case errors.Is(err, auth.ErrMembershipInvalid):
		status = 400
		e.Code = "invalid_input"
		e.Title = "Check organization inputs"
		e.Explanation = "Provide a valid role, email domain, name and current revision."
	case errors.Is(err, auth.ErrMembershipConflict):
		status = 409
		e.Code = "revision_conflict"
		e.Title = "Membership changed"
		e.Explanation = "Refresh the users list and review its current status before retrying."
	case errors.Is(err, assessment.ErrInvalid):
		status = 400
		e.Code = "invalid_input"
		e.Title = "Check these inputs"
		e.Explanation = "Check required fields, saved scope and evidence revisions before saving again."
	case errors.Is(err, assessment.ErrConflict):
		status = 409
		e.Code = "revision_conflict"
		e.Title = "Changes were saved elsewhere"
		e.Explanation = "Your entries are preserved. Load the latest revision and compare it with your changes before saving."
		e.Action = "Reload current revision"
	case errors.Is(err, assessment.ErrNotFound):
		status = 404
		e.Code = "not_found"
		e.Title = "This item is not available"
		e.Explanation = "It may not have been configured yet. Check service setup or return to the previous page."
	case errors.Is(err, auth.ErrForbidden):
		status = 403
		e.Code = "forbidden"
		e.Title = "Additional access is needed"
		e.Explanation = "Ask a service administrator for permission to perform this action."
	}
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	diagnostic(ctx, e, status)
	jsonResponse(w, status, e)
}

func (app *App) uiBootstrap(w http.ResponseWriter, r *http.Request) {
	a := auth.FromContext(r.Context())
	services, err := app.Store.Services(r.Context(), a)
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	capabilities := map[string][]string{}
	for _, s := range services {
		for _, action := range []string{"read", "edit", "check", "review", "admin", "export", "connect"} {
			if auth.Authorize(r.Context(), app.Store.DB, a, s.ID, action) == nil {
				capabilities[s.ID] = append(capabilities[s.ID], action)
			}
		}
	}
	profiles := map[string]string{}
	ids := []string{}
	for _, service := range services {
		ids = append(ids, service.ID)
	}
	rows, err := app.Store.DB.Query(r.Context(), `SELECT DISTINCT ON(service_id) service_id,data->>'class',data->>'path' FROM records WHERE service_id=ANY($1) AND kind='profile' AND id='profile' ORDER BY service_id,revision DESC`, ids)
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	for rows.Next() {
		var service, class, path string
		if err = rows.Scan(&service, &class, &path); err != nil {
			rows.Close()
			uiFailure(w, err, r.Context())
			return
		}
		profiles[service] = "Class " + class + " · " + path
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	member, err := auth.ActiveMember(r.Context(), app.Store.DB, a)
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	admin := member.Role == "admin"
	jsonResponse(w, 200, UIBootstrap{Services: services, CSRF: auth.CSRF(r.Context()), Capabilities: capabilities, CanCreate: admin, Profiles: profiles, Role: member.Role, CanManageOrganization: admin})
}

// Browser pages reuse the same authorized domain adapters as the original UI.
// This transport returns typed page data; rendering and interactive form state
// belong to the frontend. Download routes remain independent.
func (app *App) uiPage(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(context.WithValue(r.Context(), uiContext{}, true))
	switch r.PathValue("page") {
	case "settings", "sources", "collection", "history":
		q := r.URL.Query()
		q.Set("view", r.PathValue("page"))
		r.URL.RawQuery = q.Encode()
		app.service(w, r)
	case "requirement":
		app.detail(w, r)
	case "run":
		r.SetPathValue("id", r.PathValue("requirement"))
		app.run(w, r)
	default:
		uiFailure(w, assessment.ErrNotFound, r.Context())
	}
}
func (app *App) uiAction(w http.ResponseWriter, r *http.Request) {
	fields := map[string]string{}
	if r.PathValue("action") == "evidence-source" {
		var source assessment.EvidenceSource
		if json.Unmarshal([]byte(r.FormValue("payload")), &source) != nil {
			fields["payload"] = "The source configuration could not be read. Reload the form."
		} else {
			if strings.TrimSpace(source.Name) == "" {
				fields["name"] = "Give this source a recognizable name."
			}
			if source.FreshnessHours < 1 || source.FreshnessHours > 24*366 {
				fields["freshness_hours"] = "Choose between 1 and 8784 hours."
			}
			if source.Locator == "" {
				fields["locator"] = "Provide the source location."
			}
			if source.Type == "aws" || source.Type == "kubernetes" {
				if source.Region != "us-east-1" {
					fields["region"] = "Select us-east-1, the supported assessment region."
				}
				if source.Locator != source.AccountID+"/"+source.Region {
					fields["locator"] = "Use the account ID followed by /us-east-1."
				}
			}
			if source.Type == "document" || source.Type == "operational_records" {
				u, e := url.Parse(source.Locator)
				if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
					fields["locator"] = "Use an HTTPS document URL without credentials, query parameters or fragments."
				}
			}
			for _, p := range source.Paths {
				if p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
					fields["paths"] = "Use relative repository paths within the saved assessment scope."
				}
			}
			if strings.ContainsAny(source.AccessHint, "\r\n=:@?&") {
				fields["access_hint"] = "Use a profile or context name, without credentials or connection parameters."
			}
		}
	}
	if len(fields) > 0 {
		jsonResponse(w, 400, UIError{Code: "invalid_input", Title: "Check these inputs", Explanation: "Correct the fields below and save again. Your other entries are preserved.", FieldErrors: fields})
		return
	}
	app.action(w, r.WithContext(context.WithValue(r.Context(), uiContext{}, true)))
}
func (app *App) uiCreate(w http.ResponseWriter, r *http.Request) {
	app.createService(w, r.WithContext(context.WithValue(r.Context(), uiContext{}, true)))
}
func redirectAction(w http.ResponseWriter, r *http.Request, target string) {
	if isUI(r) {
		jsonResponse(w, 200, map[string]string{"redirect": target})
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

type UIRule struct {
	OpenGap       bool                               `json:"open_gap"`
	ID            string                             `json:"id"`
	Name          string                             `json:"name"`
	Category      string                             `json:"category"`
	Kind          string                             `json:"kind"`
	Applicability string                             `json:"applicability"`
	Statement     string                             `json:"statement"`
	State         assessment.RequirementPresentation `json:"state"`
}
type UIBootstrap struct {
	Role                  string               `json:"role,omitempty"`
	CanManageOrganization bool                 `json:"can_manage_organization,omitempty"`
	Services              []assessment.Service `json:"services"`
	CSRF                  string               `json:"csrf"`
	Capabilities          map[string][]string  `json:"capabilities"`
	CanCreate             bool                 `json:"can_create"`
	Profiles              map[string]string    `json:"profiles,omitempty"`
}
type UIOverview struct {
	EvidenceRecords int                                   `json:"evidence_records"`
	AsOf            time.Time                             `json:"as_of"`
	Rules           []UIRule                              `json:"rules"`
	Counts          assessment.ProgressCounts             `json:"counts"`
	Categories      map[string]*assessment.ProgressCounts `json:"categories"`
	SetupRequired   bool                                  `json:"setup_required"`
}

func (app *App) uiOverview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("service")
	a := auth.FromContext(r.Context())
	ctx := r.Context()
	snapshot, err := app.Store.Presentation(ctx, a, id)
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	now := snapshot.AsOf
	all := snapshot.Requirements
	items := []UIRule{}
	var counts assessment.ProgressCounts
	categories := map[string]*assessment.ProgressCounts{}
	for _, req := range all {
		state := snapshot.States[req.Requirement.ID]
		parts := strings.Split(req.Requirement.ID, "-")
		category := req.Requirement.Kind
		if len(parts) > 1 {
			category = parts[1]
		}
		items = append(items, UIRule{snapshot.OpenGaps[req.Requirement.ID], req.Requirement.ID, req.Requirement.Name, category, req.Requirement.Kind, req.Applicability, req.Statement, state})
		counts.Add(req.Applicability, state)
		if categories[category] == nil {
			categories[category] = &assessment.ProgressCounts{}
		}
		categories[category].Add(req.Applicability, state)
	}
	jsonResponse(w, 200, UIOverview{snapshot.EvidenceRecords, now, items, counts, categories, len(all) == 0})
}

func (app *App) uiRecords(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	switch kind {
	case "evidence", "rule_result", "evidence_source", "snapshot", "publication":
	default:
		uiFailure(w, assessment.ErrInvalid, r.Context())
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		uiFailure(w, assessment.ErrInvalid, r.Context())
		return
	}
	if kind == "evidence" {
		q := r.URL.Query()
		items, err := app.Store.EvidencePage(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), assessment.EvidenceFilter{Search: q.Get("q"), Source: q.Get("source"), Freshness: q.Get("freshness"), Requirement: q.Get("requirement"), Type: q.Get("type"), Offset: offset}, time.Now().UTC())
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		more := len(items) > 25
		if more {
			items = items[:25]
		}
		jsonResponse(w, 200, map[string]any{"items": items, "has_more": more, "offset": offset})
		return
	}
	items, err := app.Store.List(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), kind, r.URL.Query().Get("requirement"), 26, offset)
	if kind == "rule_result" {
		items, err = app.Store.ActivityPage(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.URL.Query().Get("q"), offset)
	}
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	more := len(items) > 25
	if more {
		items = items[:25]
	}
	jsonResponse(w, 200, map[string]any{"items": items, "has_more": more, "offset": offset})
}

func (app *App) uiArtifact(w http.ResponseWriter, r *http.Request) {
	revision, _ := strconv.Atoi(r.URL.Query().Get("revision"))
	if revision < 1 {
		uiFailure(w, assessment.ErrInvalid, r.Context())
		return
	}
	b, err := app.Store.ArtifactRevision(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), assessment.Ref{ID: r.PathValue("id"), Revision: revision})
	if err != nil {
		uiFailure(w, err, r.Context())
		return
	}
	jsonResponse(w, 200, map[string]string{"content": string(b)})
}
