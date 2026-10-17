package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/checks"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/integrations"
	"github.com/jaxxstorm/fed/internal/observation"
)

type App struct {
	Store            *assessment.Store
	Runner           *checks.Runner
	Connections      *connections.Service
	ConnectionOrigin string
}

func (app *App) Handler() http.Handler {
	m := http.NewServeMux()
	if app.Connections != nil {
		app.registerConnectionAPI(m)
		m.HandleFunc("POST /connections/upload/{provider}", app.connectionUpload)
		m.HandleFunc("GET /connections/authorize/{service}/{operation}", app.connectionAuthorize)
		m.HandleFunc("GET /connections/callback/{provider}", app.connectionCallback)
	}
	m.HandleFunc("GET /api/ui/v1/bootstrap", app.uiBootstrap)
	m.HandleFunc("GET /api/ui/v1/services/{service}/overview", app.uiOverview)
	m.HandleFunc("GET /api/ui/v1/services/{service}/pages/{page}", app.uiPage)
	m.HandleFunc("GET /api/ui/v1/services/{service}/pages/{page}/{requirement}", app.uiPage)
	m.HandleFunc("GET /api/ui/v1/services/{service}/records/{kind}", app.uiRecords)
	m.HandleFunc("GET /api/ui/v1/services/{service}/preview/{id}", app.uiArtifact)
	m.HandleFunc("POST /api/ui/v1/services/{service}/actions/{action}", app.uiAction)
	m.HandleFunc("POST /api/ui/v1/services", app.uiCreate)
	m.Handle("GET /assets/", frontendAssets())
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		frontendPage(w, r)
	})
	m.HandleFunc("POST /services", app.createService)
	m.HandleFunc("GET /services/{service}", func(w http.ResponseWriter, r *http.Request) {
		frontendPage(w, r)
	})
	m.HandleFunc("GET /services/{service}/requirements/{requirement}", func(w http.ResponseWriter, r *http.Request) {
		frontendPage(w, r)
	})
	m.HandleFunc("POST /services/{service}/actions/{action}", app.action)
	m.HandleFunc("GET /services/{service}/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		frontendPage(w, r)
	})
	m.HandleFunc("GET /services/{service}/exports/{id}/{format}", app.export)
	m.HandleFunc("GET /services/{service}/artifacts/{id}", app.artifact)
	return m
}
func pretty(v any) string                   { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func field(name, label, value string) Field { return Field{Name: name, Label: label, Value: value} }
func hidden(name, value string) Field       { return Field{Name: name, Value: value, Type: "hidden"} }
func area(name, label, value string) Field {
	f := field(name, label, value)
	f.Type = "textarea"
	return f
}
func choice(name, label, value string, options ...string) Field {
	f := field(name, label, value)
	f.Type = "select"
	f.Options = options
	return f
}
func form(r *http.Request, action, title, button string, fields ...Field) Form {
	return Form{Action: action, Title: title, Button: button, CSRF: auth.CSRF(r.Context()), Fields: fields}
}
func base(service string) string { return "/services/" + url.PathEscape(service) }
func render(w http.ResponseWriter, r *http.Request, p Page) {
	jsonResponse(w, http.StatusOK, p)
}
func failure(w http.ResponseWriter, r *http.Request, err error) {
	if isUI(r) {
		uiFailure(w, err)
		return
	}
	status := 500
	message := "Operation failed. Check service logs and configuration."
	switch {
	case errors.Is(err, assessment.ErrConflict):
		status = 409
		message = "This record changed. Reload and review the latest revision before saving."
	case errors.Is(err, assessment.ErrInvalid):
		status = 400
		message = "Invalid input. Check the fields, scope, and evidence references."
	case errors.Is(err, assessment.ErrNotFound):
		status = 404
		message = "Record not found. Configure the service profile and scope first."
	case errors.Is(err, auth.ErrForbidden):
		status = 403
		message = "You do not have permission for this action."
	case errors.Is(err, integrations.ErrUnavailable):
		status = 409
		message = "No supported live trust-centre adapter is configured. Download an assessment bundle instead."
	}
	http.Error(w, message, status)
}

func (app *App) index(w http.ResponseWriter, r *http.Request) {
	services, err := app.Store.Services(r.Context(), auth.FromContext(r.Context()))
	if err != nil {
		failure(w, r, err)
		return
	}
	p := Page{Title: "Services"}
	for _, s := range services {
		p.Links = append(p.Links, Link{s.Name, base(s.ID)})
	}
	p.Forms = []Form{form(r, "/services", "Create a service", "Create", field("name", "Service name", ""))}
	render(w, r, p)
}
func (app *App) createService(w http.ResponseWriter, r *http.Request) {
	s, err := app.Store.CreateService(r.Context(), auth.FromContext(r.Context()), r.FormValue("name"))
	if err != nil {
		failure(w, r, err)
		return
	}
	redirectAction(w, r, base(s.ID)+"?view=settings")
}
func (app *App) page(r *http.Request) (Page, error) {
	id := r.PathValue("service")
	if err := auth.Authorize(r.Context(), app.Store.DB, auth.FromContext(r.Context()), id, "read"); err != nil {
		return Page{}, err
	}
	var name string
	err := app.Store.DB.QueryRow(r.Context(), `SELECT name FROM services WHERE id=$1`, id).Scan(&name)
	if err != nil {
		return Page{}, err
	}
	return Page{Title: name, Links: []Link{{"KSIs", base(id)}, {"Evidence sources", base(id) + "?view=sources"}, {"Collection", base(id) + "?view=collection"}, {"Broader obligations", base(id) + "?kind=FRR"}, {"Unreviewed", base(id) + "?gaps=1"}, {"Settings", base(id) + "?view=settings"}, {"History & exports", base(id) + "?view=history"}}}, nil
}

func (app *App) service(w http.ResponseWriter, r *http.Request) {
	p, err := app.page(r)
	if err != nil {
		failure(w, r, err)
		return
	}
	id := r.PathValue("service")
	a := auth.FromContext(r.Context())
	switch r.URL.Query().Get("view") {
	case "sources":
		if err = app.Store.ReconcileEvidenceSources(r.Context(), a, id); err != nil {
			failure(w, r, err)
			return
		}
		scope, e := app.Store.Get(r.Context(), a, id, "scope", "scope")
		if e != nil {
			failure(w, r, e)
			return
		}
		p.Notice = "Agents use this shared source directory and their own read-only tools to gather evidence. Configure a source or update its access hint to unblock work."
		reads, e := app.sourceReads(r.Context(), id)
		if e != nil {
			failure(w, r, e)
			return
		}
		for offset := 0; ; offset += 100 {
			items, e := app.Store.List(r.Context(), a, id, "evidence_source", "", 100, offset)
			if isUI(r) {
				start, _ := strconv.Atoi(r.URL.Query().Get("source_offset"))
				items, e = app.Store.SourcePage(r.Context(), a, id, r.URL.Query().Get("q"), start)
				q := r.URL.Query()
				q.Set("page", "Sources")
				if start > 0 {
					q.Set("source_offset", strconv.Itoa(max(0, start-25)))
					p.Pagination = append(p.Pagination, Link{"Previous sources", base(id) + "?" + q.Encode()})
				}
				if len(items) > 25 {
					q.Set("source_offset", strconv.Itoa(start+25))
					p.Pagination = append(p.Pagination, Link{"Next sources", base(id) + "?" + q.Encode()})
					items = items[:25]
				}
			}
			if e != nil {
				failure(w, r, e)
				return
			}
			if isUI(r) && len(items) == 0 {
				if r.URL.Query().Get("q") != "" {
					p.Notice = "No sources match this search. Change or clear the search to see other sources."
				} else {
					p.Notice = "Add an evidence source below to tell agents where to investigate."
				}
			}
			for _, item := range items {
				source, decodeErr := assessment.Decode[assessment.EvidenceSource](item)
				if decodeErr != nil {
					failure(w, r, decodeErr)
					return
				}
				p.Forms = append(p.Forms, form(r, base(id)+"/actions/evidence-source", "Source · "+source.Name, "Save source", hidden("id", item.ID), hidden("expected_revision", strconv.Itoa(item.Revision)), area("payload", "Source configuration", string(item.Data))))
				p.Forms[len(p.Forms)-1].Hints = sourceHints(source, reads)
			}
			if isUI(r) || len(items) < 100 {
				break
			}
		}
		example := assessment.EvidenceSource{Type: "document", Name: "", Locator: "https://docs.example/security", ScopeRevision: scope.Revision, Enabled: true, FreshnessHours: 48, Paths: []string{}, RequirementIDs: []string{}}
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/evidence-source", "New evidence source", "Add source", hidden("id", uuid.NewString()), hidden("expected_revision", "0"), area("payload", "Source configuration", pretty(example))))
	case "collection":
		if isUI(r) {
			rows, e := app.Store.DB.Query(r.Context(), `SELECT id,check_id,state,created_at FROM jobs WHERE service_id=$1 ORDER BY created_at DESC LIMIT 25`, id)
			if e != nil {
				failure(w, r, e)
				return
			}
			for rows.Next() {
				var jid, check, state string
				var at time.Time
				if e = rows.Scan(&jid, &check, &state, &at); e != nil {
					rows.Close()
					failure(w, r, e)
					return
				}
				p.Activities = append(p.Activities, Link{check + " · " + state + " · " + at.Format("02 Jan 15:04"), base(id) + "/runs/" + jid})
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				failure(w, r, e)
				return
			}
		}
		plan, e := app.Store.GetCollectionPlan(r.Context(), a, id)
		if errors.Is(e, assessment.ErrNotFound) {
			p.Notice = "Collection plan is waiting for a configured scope/profile and a running worker."
			break
		}
		if e != nil {
			failure(w, r, e)
			return
		}
		configuration, e := app.Store.CollectionConfiguration(r.Context(), a, id)
		if e != nil {
			failure(w, r, e)
			return
		}
		p.Notice = "Read-only collection uses saved scope only. Initial collection and scheduled refresh run in the background; application defaults are 24-hour refresh and 48-hour freshness."
		p.Sections = append(p.Sections, Section{"Schedule", pretty(configuration)})
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/collect", "Collect supported evidence", "Collect now", hidden("key", uuid.NewString())))
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/collection-config", "Collection configuration", "Save configuration", hidden("expected_revision", strconv.Itoa(configuration.Revision)), area("payload", "Configuration JSON (paused, cadence_hours, freshness_hours, overrides)", pretty(configuration))))
		if batchID := r.URL.Query().Get("batch"); batchID != "" {
			batch, e := app.Runner.GetBatch(r.Context(), a, id, batchID)
			if e != nil {
				failure(w, r, e)
				return
			}
			p.Metrics = append(p.Metrics, Metric{"Run state", batch.State}, Metric{"Completed checks", strconv.Itoa(batch.CompletedJobs) + " / " + strconv.Itoa(len(batch.Jobs))})
			p.Diagnostics = append(p.Diagnostics, Section{"Exact collection batch", pretty(batch)})
			for _, job := range batch.Jobs {
				p.Activities = append(p.Activities, Link{job.CheckID + " · " + job.State, base(id) + "/runs/" + job.ID})
			}
			for _, entry := range batch.Blocked {
				p.Activities = append(p.Activities, Link{entry.RequirementID + " · needs configuration", base(id) + "/requirements/" + entry.RequirementID})
			}
		}
		rows, e := app.Store.DB.Query(r.Context(), `SELECT id,created_at FROM collection_batches WHERE service_id=$1 ORDER BY created_at DESC LIMIT 25`, id)
		if e != nil {
			failure(w, r, e)
			return
		}
		for rows.Next() {
			var batchID string
			var at time.Time
			if e = rows.Scan(&batchID, &at); e != nil {
				rows.Close()
				failure(w, r, e)
				return
			}
			p.Links = append(p.Links, Link{"Batch " + at.Format(time.RFC3339), base(id) + "?view=collection&batch=" + batchID})
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			failure(w, r, e)
			return
		}
		for _, entry := range plan.Entries {
			if entry.CheckID != "" {
				if isUI(r) {
					p.Diagnostics = append(p.Diagnostics, Section{entry.RequirementID + " · " + entry.CheckID, pretty(entry)})
				} else {
					p.Sections = append(p.Sections, Section{entry.RequirementID + " · " + entry.CheckID, pretty(entry)})
				}
			}
		}
	case "settings":
		profile := catalog.Profile{Type: "20x", Path: "Program", Class: "B", Context: "initial", Date: time.Now().Format(time.DateOnly)}
		revision := 0
		if v, e := app.Store.Get(r.Context(), a, id, "profile", "profile"); e == nil {
			profile, e = assessment.Decode[catalog.Profile](v)
			if e != nil {
				failure(w, r, e)
				return
			}
			revision = v.Revision
		} else if !errors.Is(e, assessment.ErrNotFound) {
			failure(w, r, e)
			return
		}
		if profile.CatalogID == "" {
			_ = app.Store.DB.QueryRow(r.Context(), `SELECT catalog_id FROM catalog_activation`).Scan(&profile.CatalogID)
		}
		if profile.CatalogID == "" {
			p.Notice = "Import the official catalog with the import-catalog command before selecting a profile."
		}
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/preview-profile", "Certification profile", "Preview changes", hidden("expected_revision", strconv.Itoa(revision)), field("catalog_id", "Catalog snapshot", profile.CatalogID), choice("class", "Class", profile.Class, "A", "B", "C", "D"), choice("path", "Path", profile.Path, "Program", "Agency"), choice("context", "Context", profile.Context, "initial", "ongoing"), field("date", "Evaluation date (YYYY-MM-DD)", profile.Date), choice("assessment_started", "Next assessment has started (transition grace)", "unknown", "unknown", "yes", "no")))
		scope := assessment.Scope{Description: "Describe the assessment boundary", Accounts: []assessment.Account{}, Repositories: []assessment.Repository{}}
		revision = 0
		if v, e := app.Store.Get(r.Context(), a, id, "scope", "scope"); e == nil {
			scope, e = assessment.Decode[assessment.Scope](v)
			if e != nil {
				failure(w, r, e)
				return
			}
			revision = v.Revision
		}
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/scope", "Assessment scope", "Save scope", hidden("expected_revision", strconv.Itoa(revision)), area("payload", "Scope JSON (accounts, regions, clusters, repositories at pinned commits)", pretty(scope))))
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/token", "Agent access", "Issue scoped token", field("hours", "Lifetime in hours", "24"), choice("connect", "Allow connection management", "no", "no", "yes")))
		if isUI(r) && auth.Authorize(r.Context(), app.Store.DB, a, id, "admin") == nil {
			rows, e := app.Store.DB.Query(r.Context(), `SELECT hash,created_at,expires_at FROM api_tokens WHERE $1=ANY(service_ids) AND revoked_at IS NULL AND expires_at>now() ORDER BY created_at DESC`, id)
			if e != nil {
				failure(w, r, e)
				return
			}
			for rows.Next() {
				var hash string
				var created, expires time.Time
				if e = rows.Scan(&hash, &created, &expires); e != nil {
					rows.Close()
					failure(w, r, e)
					return
				}
				f := form(r, base(id)+"/actions/revoke-token", "Agent access issued "+created.Format("02 Jan 2006 15:04"), "Revoke this token", hidden("hash", hash))
				f.Hints = []string{"Expires " + expires.UTC().Format(time.RFC3339)}
				p.Forms = append(p.Forms, f)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				failure(w, r, e)
				return
			}
		} else if !isUI(r) {
			p.Forms = append(p.Forms, form(r, base(id)+"/actions/revoke-token", "Revoke agent access", "Revoke", field("hash", "Token SHA-256 hash", "")))
		}
		destination := assessment.Destination{}
		revision = 0
		if v, e := app.Store.Get(r.Context(), a, id, "destination", "destination"); e == nil {
			destination, e = assessment.Decode[assessment.Destination](v)
			if e != nil {
				failure(w, r, e)
				return
			}
			revision = v.Revision
		}
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/destination", "Optional trust-centre destination", "Save destination", hidden("expected_revision", strconv.Itoa(revision)), field("name", "Name", destination.Name), field("adapter_type", "Adapter type (live connectors are not installed)", destination.AdapterType), field("configuration_ref", "Configuration reference, not credentials", destination.ConfigurationRef)))
	case "history":
		hasSnapshots := false
		for _, kind := range []string{"publication", "snapshot"} {
			items, e := app.Store.List(r.Context(), a, id, kind, "", 100, 0)
			if e != nil {
				failure(w, r, e)
				return
			}
			for _, item := range items {
				if kind == "snapshot" {
					hasSnapshots = true
					bundle, e := assessment.Decode[assessment.Bundle](item)
					if e != nil {
						failure(w, r, e)
						return
					}
					counts := assessment.ProgressCounts{}
					statuses := map[string]assessment.Status{}
					for _, s := range bundle.Statuses {
						statuses[s.RequirementID] = s
					}
					for _, req := range bundle.Requirements {
						counts.Add(req.Applicability, assessment.PresentRequirement(req.Applicability, statuses[req.Requirement.ID], false))
					}
					label := bundle.PreparedAt.Format("02 Jan 2006 · 15:04 UTC")
					p.Sections = append(p.Sections, Section{"Snapshot · " + label, "Class " + bundle.Profile.Class + " · " + strconv.Itoa(counts.Passing) + " / " + strconv.Itoa(counts.Required) + " required requirements passing at preparation. " + strconv.Itoa(counts.ScopeDecisions) + " scope decisions outstanding."})
					p.Links = append(p.Links, Link{"Download JSON · " + label, base(id) + "/exports/" + item.ID + "/json"}, Link{"Download HTML · " + label, base(id) + "/exports/" + item.ID + "/html"})
				} else {
					publication, e := assessment.Decode[assessment.Publication](item)
					if e != nil {
						failure(w, r, e)
						return
					}
					p.Sections = append(p.Sections, Section{"Publication · " + item.CreatedAt.Format("02 Jan 2006 15:04 UTC"), "Status: " + strings.ReplaceAll(publication.State, "_", " ")})
					p.Diagnostics = append(p.Diagnostics, Section{"Publication revision " + strconv.Itoa(item.Revision), pretty(item)})
				}
			}
		}
		p.Forms = append(p.Forms, form(r, base(id)+"/actions/prepare", "Prepare assessment bundle", "Prepare", hidden("key", uuid.NewString())))
		if !hasSnapshots {
			p.Sections = append(p.Sections, Section{"No snapshots prepared yet", "Prepare a bundle to capture this assessment at its current revisions. Later evidence updates will not change that snapshot."})
		}
		events, e := app.audit(r.Context(), a, id)
		if e != nil {
			failure(w, r, e)
			return
		}
		if isUI(r) {
			p.Diagnostics = append(p.Diagnostics, Section{"Recent audit events", pretty(events)})
		} else {
			p.Sections = append(p.Sections, Section{"Recent audit events", pretty(events)})
		}
	default:
		plan, _ := app.Store.GetCollectionPlan(r.Context(), a, id)
		eligibility := map[string]string{}
		for _, entry := range plan.Entries {
			eligibility[entry.RequirementID] = entry.State
		}
		requirements, e := app.Store.Requirements(r.Context(), a, id)
		if errors.Is(e, assessment.ErrNotFound) {
			p.Notice = "Configure a profile and assessment scope in Settings to begin."
			break
		}
		if e != nil {
			failure(w, r, e)
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "KSI"
		}
		for _, req := range requirements {
			if req.Requirement.Kind != kind {
				continue
			}
			status, e := app.Store.Status(r.Context(), a, id, req.Requirement.ID, time.Now())
			if e != nil {
				failure(w, r, e)
				return
			}
			if r.URL.Query().Get("gaps") == "1" && status.Review == "reviewed" {
				gaps, e := app.Store.List(r.Context(), a, id, "gap", req.Requirement.ID, 100, 0)
				if e != nil {
					failure(w, r, e)
					return
				}
				open := false
				for _, item := range gaps {
					g, e := assessment.Decode[assessment.Gap](item)
					if e != nil {
						failure(w, r, e)
						return
					}
					if g.Status == "open" {
						open = true
					}
				}
				if !open {
					continue
				}
			}
			if filter := r.URL.Query().Get("applicability"); filter != "" && req.Applicability != filter {
				continue
			}
			evidenceLabel := status.Evidence
			if status.Collection.Count > 0 {
				evidenceLabel += " · " + strconv.Itoa(status.Collection.Count) + " observations"
				if status.Collection.MissingMeasures > 0 {
					evidenceLabel += " · incomplete coverage"
				}
			}
			eligibility[req.Requirement.ID] = status.Automation
			switch eligibility[req.Requirement.ID] {
			case "manual_evidence":
				if status.Collection.Count == 0 {
					evidenceLabel = "Agent investigation needed"
				}
			case "unsupported_automation":
				if status.Collection.Count == 0 {
					evidenceLabel = "Agent investigation needed"
				}
			}
			switch status.Work.State {
			case "evidence_complete":
				evidenceLabel = "☑ Evidence complete"
			case "partial":
				evidenceLabel = "☐ Partial evidence"
			case "blocked":
				evidenceLabel = "☐ Blocked"
			case "needs_refresh":
				evidenceLabel = "☐ Needs refresh"
			default:
				if status.Collection.Count == 0 {
					evidenceLabel = "☐ Agent investigation needed"
				}
			}
			if req.Applicability == "not_applicable" {
				evidenceLabel = "not applicable"
			}
			p.Rows = append(p.Rows, Row{req.Requirement.ID, req.Requirement.Name, req.Applicability, status.Technical, evidenceLabel, status.Review, base(id) + "/requirements/" + req.Requirement.ID})
		}
	}
	render(w, r, p)
}

func (app *App) detail(w http.ResponseWriter, r *http.Request) {
	p, err := app.page(r)
	if err != nil {
		failure(w, r, err)
		return
	}
	id, reqID := r.PathValue("service"), r.PathValue("requirement")
	p.Detail = true
	a := auth.FromContext(r.Context())
	requirements, err := app.Store.Requirements(r.Context(), a, id)
	if err != nil {
		failure(w, r, err)
		return
	}
	found := false
	applicability := "unresolved"
	for _, req := range requirements {
		if req.Requirement.ID == reqID {
			applicability = req.Applicability
			found = true
			p.Title = reqID + " · " + req.Requirement.Name
			p.Sections = append(p.Sections, Section{"What this requirement asks for", req.Statement})
			p.Diagnostics = append(p.Diagnostics, Section{"Applicability & official source", req.Applicability + ": " + req.Reason + "\n" + req.Requirement.NarrativePath})
			break
		}
	}
	if !found {
		failure(w, r, assessment.ErrNotFound)
		return
	}
	status, err := app.Store.Status(r.Context(), a, id, reqID, time.Now())
	if err != nil {
		failure(w, r, err)
		return
	}
	label := func(value string) string { return strings.ReplaceAll(value, "_", " ") }
	primary := assessment.PresentRequirement(applicability, status, false)
	p.Primary = &primary
	p.Metrics = []Metric{{"Evidence work", label(status.Work.State)}, {"Technical checks", label(status.Technical)}, {"Evidence freshness", label(status.Evidence)}, {"Human review", label(status.Review)}}
	p.Diagnostics = append(p.Diagnostics, Section{"Evidence work · full details", pretty(status.Work)})
	if instructions, e := app.Store.RuleInstructions(r.Context(), a, id, reqID); e == nil {
		if applicability != "not_applicable" {
			for _, assertion := range instructions.Assertions {
				needed := status.Work.State == "open" || status.Work.State == "" || slices.Contains(status.Work.MissingAssertions, assertion.ID)
				if !needed && (primary.Status == "passing" || primary.Status == "in_review") {
					continue
				}
				title := "Verify assertion: " + assertion.ID
				if needed {
					title = "Gather evidence for: " + assertion.ID
				}
				primary.NextSteps = append(primary.NextSteps, assessment.RequirementNextStep{Title: title, Explanation: assertion.Statement, Targets: []string{}, Action: "Collect supporting " + strings.Join(assertion.EvidenceKinds, ", ") + " for the saved service scope. Cite exact lines or JSON fields and explain how they support this assertion; explicitly record gaps or contradictions."})
			}
			sourceNames := []string{}
			for _, record := range instructions.Sources {
				source, err := assessment.Decode[assessment.EvidenceSource](record)
				if err == nil {
					sourceNames = append(sourceNames, source.Name+" · "+source.Locator)
				}
			}
			sourceAction := "Use the configured sources below within their saved boundaries. Open Sources to inspect source revisions and scope. An external agent can use its own credentials; server collection requires a configured connection."
			if len(sourceNames) == 0 {
				sourceAction = "No matching evidence sources are configured. Open Connections to configure provider access and select resources, or Sources to register documentary evidence. Select scope explicitly before collecting."
			}
			primary.NextSteps = append(primary.NextSteps, assessment.RequirementNextStep{Title: "Where to gather evidence", Explanation: sourceAction, Targets: sourceNames, Action: "After gathering evidence, ask your agent to read get_rule_instructions and submit_rule_result for " + reqID + " using the current instruction bindings and exact evidence citations."})
			for index, step := range instructions.Steps {
				primary.NextSteps = append(primary.NextSteps, assessment.RequirementNextStep{Title: fmt.Sprintf("Investigation step %d · %s", index+1, strings.ReplaceAll(instructions.GuidanceKind, "_", " ")), Explanation: step, Targets: []string{}, Action: "Apply this step to " + reqID + " and the official statement shown below."})
			}
		}
		for _, assertion := range instructions.Assertions {
			state := "Investigation recorded"
			if status.Work.State == "open" || status.Work.State == "" || slices.Contains(status.Work.MissingAssertions, assertion.ID) {
				state = "Evidence needed"
			}
			if status.Work.State == "needs_refresh" {
				state = "Update evidence"
			}
			p.Sections = append(p.Sections, Section{"Evidence checklist · " + state, assertion.Statement + "\nSupporting records: " + strings.Join(assertion.EvidenceKinds, ", ")})
		}
	} else if !errors.Is(e, assessment.ErrNotFound) {
		failure(w, r, e)
		return
	}
	for _, recovery := range status.Collection.Recovery {
		p.Sections = append(p.Sections, Section{recovery.Title, recovery.Target + " · " + recovery.Environment + "\n" + recovery.Explanation})
		if app.Connections != nil {
			p.Links = append(p.Links, Link{"Review evidence connection", base(r.PathValue("service")) + "?page=Connections&provider=" + url.QueryEscape(recovery.ConnectionProvider)})
		}
	}
	if len(status.Work.Blockers) > 0 {
		p.Sections = append(p.Sections, Section{"What’s still needed", "• " + strings.Join(status.Work.Blockers, "\n• ")})
	}
	if len(status.Work.Reasons) > 0 {
		p.Sections = append(p.Sections, Section{"Why this needs attention", strings.Join(status.Work.Reasons, "\n")})
	}
	for _, finding := range status.Collection.Findings[:min(10, len(status.Collection.Findings))] {
		title := label(finding.Condition)
		if finding.Target.ResourceID != "" {
			title += " · " + finding.Target.ResourceID
		}
		p.Findings = append(p.Findings, Section{title, finding.Explanation})
	}
	if len(status.Collection.Findings) > 10 {
		p.Findings = append(p.Findings, Section{strconv.Itoa(len(status.Collection.Findings)-10) + " more findings", "Expand collection diagnostics below for the complete finding list."})
	}
	if result, e := app.Store.Get(r.Context(), a, id, "rule_result", reqID); e == nil {
		p.Diagnostics = append(p.Diagnostics, Section{"Agent result & exact citations", pretty(result)})
		var stored struct {
			Submission assessment.RuleResultSubmission `json:"submission"`
		}
		if json.Unmarshal(result.Data, &stored) == nil {
			for _, assertion := range stored.Submission.Assertions {
				p.Sections = append(p.Sections, Section{"Assertion · " + label(assertion.Outcome), assertion.Rationale})
				for _, citation := range assertion.Citations {
					record, e := app.Store.EvidenceRevision(r.Context(), a, id, citation.Evidence)
					if e != nil {
						failure(w, r, e)
						return
					}
					p.Citations = append(p.Citations, CitationCard{record, citation.StartLine, citation.EndLine, citation.JSONPointer, "Agent result"})
				}
			}
		}
	} else if !errors.Is(e, assessment.ErrNotFound) {
		failure(w, r, e)
		return
	}
	p.Diagnostics = append(p.Diagnostics, Section{"Collection diagnostics · " + strconv.Itoa(status.Collection.Count) + " observations · " + strconv.Itoa(len(status.Collection.Findings)) + " findings", pretty(status.Collection)})
	profile, err := app.Store.Get(r.Context(), a, id, "profile", "profile")
	if err != nil {
		failure(w, r, err)
		return
	}
	scope, err := app.Store.Get(r.Context(), a, id, "scope", "scope")
	if err != nil {
		failure(w, r, err)
		return
	}
	p.Diagnostics = append(p.Diagnostics, Section{"Pinned assessment profile", pretty(profile)})
	measures, err := app.Store.List(r.Context(), a, id, "measure", reqID, 100, 0)
	if err != nil {
		failure(w, r, err)
		return
	}
	offset := 0
	if raw := r.URL.Query().Get("evidence_offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 1000000 {
			failure(w, r, assessment.ErrInvalid)
			return
		}
	}
	const evidencePageSize = 20
	evidence, err := app.Store.List(r.Context(), a, id, "evidence", reqID, evidencePageSize+1, offset)
	if err != nil {
		failure(w, r, err)
		return
	}
	pageURL := base(id) + "/requirements/" + reqID + "?evidence_offset="
	if offset > 0 {
		p.Pagination = append(p.Pagination, Link{"← Previous 20", pageURL + strconv.Itoa(max(0, offset-evidencePageSize)) + "#evidence"})
	}
	if len(evidence) > evidencePageSize {
		p.Pagination = append(p.Pagination, Link{"Next 20 →", pageURL + strconv.Itoa(offset+evidencePageSize) + "#evidence"})
		evidence = evidence[:evidencePageSize]
	}
	for _, item := range evidence {
		e, err := assessment.Decode[assessment.Evidence](item)
		if err != nil {
			failure(w, r, err)
			return
		}
		source := label(e.Verification)
		switch e.Verification {
		case "collector_observation":
			source = "Server collector"
		case "agent_observation":
			source = "Agent observation"
		case "unverified_reference":
			source = "External reference · unverified"
		}
		card := EvidenceCard{Title: e.Description, Source: source, Observed: e.ObservedAt.UTC().Format("02 Jan 2006 · 15:04 UTC"), Metadata: pretty(item)}
		if card.Title == "" {
			card.Title = "Evidence record"
		}
		if e.Target != nil {
			card.Target = e.Target.ResourceID
		}
		if e.Document != nil {
			card.Target = e.Document.Locator + " · " + e.Document.SourceVersion
		}
		if e.ArtifactKey != "" {
			card.URL = base(id) + "/artifacts/" + item.ID
			card.Action = "Download evidence"
		} else if e.URL != "" {
			card.URL = e.URL
			card.Action = "Open source"
		}
		p.Evidence = append(p.Evidence, card)
	}
	jobs, err := app.Runner.List(r.Context(), a, id, reqID)
	if err != nil {
		failure(w, r, err)
		return
	}
	for _, job := range jobs {
		p.Activities = append(p.Activities, Link{job.CheckID + " · " + job.State, base(id) + "/runs/" + job.ID})
		if job.Result != nil {
			p.Diagnostics = append(p.Diagnostics, Section{"Run coverage · " + job.CheckID, pretty(map[string]any{"expected": job.Result.Expected, "observed": job.Result.Observed, "limitations": job.Result.Limitations, "outcome": job.Result.Outcome})})
		}
	}
	action := func(name string) string { return base(id) + "/actions/" + name }
	common := []Field{hidden("requirement_id", reqID), hidden("scope_revision", strconv.Itoa(scope.Revision))}
	measureFields := func(m assessment.Measure, mid string, rev int) []Field {
		return append(append([]Field{}, common...), hidden("id", mid), hidden("expected_revision", strconv.Itoa(rev)), field("name", "Measure name", m.Name), area("rationale", "How this supports the KSI", m.Rationale), choice("check_id", "Registered check", m.CheckID, "", "aws.guardduty.configuration", "aws.eks.control-plane-logging", "github.pulumi.workflow-evidence"), area("expected", "Expected values, one per line", strings.Join(m.Expected, "\n")), field("freshness_hours", "Freshness policy in hours", strconv.Itoa(m.FreshnessHours)))
	}
	p.Forms = append(p.Forms, form(r, action("measure"), "Add customer measure", "Save measure", measureFields(assessment.Measure{FreshnessHours: 24}, uuid.NewString(), 0)...))
	for _, item := range measures {
		m, err := assessment.Decode[assessment.Measure](item)
		if err != nil {
			failure(w, r, err)
			return
		}
		p.Forms = append(p.Forms, form(r, action("measure"), "Edit measure: "+m.Name, "Save new revision", measureFields(m, item.ID, item.Revision)...))
		if m.CheckID != "" {
			p.Forms = append(p.Forms, form(r, action("check"), "Run: "+m.Name, "Run check", hidden("requirement_id", reqID), hidden("id", item.ID), hidden("revision", strconv.Itoa(item.Revision)), hidden("scope_revision", strconv.Itoa(m.ScopeRevision)), hidden("check_id", m.CheckID), hidden("key", uuid.NewString())))
		}
	}
	p.Forms = append(p.Forms, form(r, action("evidence"), "Attach external evidence", "Attach reference", append(append([]Field{}, common...), hidden("key", uuid.NewString()), field("source", "Source / provider", "external"), field("url", "HTTPS document URL", ""), area("description", "Evidence description", ""), field("observed_at", "Observation time (RFC3339)", time.Now().UTC().Format(time.RFC3339)))...))
	draft := assessment.Draft{ProfileRevision: profile.Revision, ScopeRevision: scope.Revision, Evidence: []assessment.Ref{}, Measures: []assessment.Ref{}}
	revision := 0
	if status.Draft != nil {
		draft, err = assessment.Decode[assessment.Draft](*status.Draft)
		if err != nil {
			failure(w, r, err)
			return
		}
		revision = status.Draft.Revision
		p.Sections = append(p.Sections, Section{"Assessment summary", draft.Explanation})
		p.Sections = append(p.Sections, Section{"Human review target", "Assessment draft revision " + strconv.Itoa(revision) + " · scope revision " + strconv.Itoa(draft.ScopeRevision) + " · profile revision " + strconv.Itoa(draft.ProfileRevision)})
		for _, ref := range draft.Evidence {
			evidence, e := app.Store.EvidenceRevision(r.Context(), a, id, ref)
			if e != nil {
				failure(w, r, e)
				return
			}
			p.Citations = append(p.Citations, CitationCard{evidence, 0, 0, nil, "Draft revision " + strconv.Itoa(revision)})
		}
	}
	p.Forms = append(p.Forms, form(r, action("draft"), "Implementation explanation and evidence citations", "Save draft", hidden("requirement_id", reqID), hidden("expected_revision", strconv.Itoa(revision)), hidden("profile_revision", strconv.Itoa(profile.Revision)), hidden("scope_revision", strconv.Itoa(scope.Revision)), area("explanation", "How this requirement is addressed", draft.Explanation), area("evidence", "Evidence references: [{\"id\":\"…\",\"revision\":1}]", pretty(draft.Evidence)), area("measures", "Measure references", pretty(draft.Measures))))
	draftForm := &p.Forms[len(p.Forms)-1]
	for _, item := range evidence {
		ev, e := assessment.Decode[assessment.Evidence](item)
		if e != nil {
			failure(w, r, e)
			return
		}
		draftForm.References = append(draftForm.References, ReferenceOption{item.ID, item.Revision, ev.Description, "evidence"})
	}
	for _, c := range p.Citations {
		ev, e := assessment.Decode[assessment.Evidence](c.Evidence)
		if e != nil {
			failure(w, r, e)
			return
		}
		draftForm.References = append(draftForm.References, ReferenceOption{c.Evidence.ID, c.Evidence.Revision, ev.Description, "evidence"})
	}
	for _, item := range measures {
		m, e := assessment.Decode[assessment.Measure](item)
		if e != nil {
			failure(w, r, e)
			return
		}
		draftForm.References = append(draftForm.References, ReferenceOption{item.ID, item.Revision, m.Name, "measures"})
	}
	if revision > 0 {
		p.Forms = append(p.Forms, form(r, action("review"), "Human review", "Record review", hidden("requirement_id", reqID), hidden("revision", strconv.Itoa(revision)), Field{Name: "reviewed", Label: "I reviewed this implementation and its supporting evidence", Type: "checkbox"}, area("notes", "Review notes", "")))
	}
	gaps, err := app.Store.List(r.Context(), a, id, "gap", reqID, 100, 0)
	if err != nil {
		failure(w, r, err)
		return
	}
	for _, gap := range gaps {
		g, e := assessment.Decode[assessment.Gap](gap)
		if e != nil {
			failure(w, r, e)
			return
		}
		p.Forms = append(p.Forms, form(r, action("gap"), "Update gap", "Save gap", hidden("requirement_id", reqID), hidden("id", gap.ID), hidden("expected_revision", strconv.Itoa(gap.Revision)), area("explanation", "Explanation", g.Explanation), choice("status", "Status", g.Status, "open", "resolved")))
	}
	p.Forms = append(p.Forms, form(r, action("gap"), "Record a gap", "Save gap", hidden("requirement_id", reqID), hidden("id", uuid.NewString()), hidden("expected_revision", "0"), hidden("status", "open"), area("explanation", "Missing evidence or implementation", "")))
	render(w, r, p)
}

func integer(r *http.Request, name string) int {
	n, err := strconv.Atoi(r.FormValue(name))
	if err != nil {
		return -1
	}
	return n
}
func parse[T any](raw string) (T, error) {
	var v T
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	if err != nil {
		return v, assessment.ErrInvalid
	}
	return v, nil
}
func (app *App) action(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("service")
	a := auth.FromContext(r.Context())
	reqID := r.FormValue("requirement_id")
	revision := integer(r, "expected_revision")
	ctx := r.Context()
	var err error
	target := base(id)
	switch r.PathValue("action") {
	case "evidence-source":
		var source assessment.EvidenceSource
		source, err = parse[assessment.EvidenceSource](r.FormValue("payload"))
		if err == nil {
			_, err = app.Store.SaveEvidenceSource(ctx, a, id, r.FormValue("id"), revision, source)
		}
		target += "?view=sources"
	case "collect":
		var batch checks.Batch
		batch, err = app.Runner.CollectNow(ctx, a, id, r.FormValue("key"))
		if err == nil {
			redirectAction(w, r, base(id)+"?view=collection&batch="+batch.ID)
			return
		}
	case "collection-config":
		var c assessment.CollectionConfiguration
		c, err = parse[assessment.CollectionConfiguration](r.FormValue("payload"))
		if err == nil {
			_, err = app.Store.ConfigureCollection(ctx, a, id, revision, c)
		}
		target += "?view=collection"
	case "scope":
		var v assessment.Scope
		v, err = parse[assessment.Scope](r.FormValue("payload"))
		if err == nil {
			_, err = app.Store.SaveScope(ctx, a, id, revision, v)
		}
		target += "?view=settings"
	case "preview-profile":
		p := catalog.Profile{CatalogID: r.FormValue("catalog_id"), Type: "20x", Path: r.FormValue("path"), Class: r.FormValue("class"), Context: r.FormValue("context"), Date: r.FormValue("date")}
		if v := r.FormValue("assessment_started"); v != "unknown" {
			started := v == "yes"
			p.NextAssessmentStarted = &started
		}
		var changes []assessment.ProfileChange
		changes, err = app.Store.PreviewProfile(ctx, a, id, p)
		if err == nil {
			render(w, r, Page{Title: "Preview profile changes", Sections: []Section{{"Changed obligations", pretty(changes)}}, Forms: []Form{form(r, base(id)+"/actions/profile", "Confirm profile change", "Apply profile", hidden("payload", pretty(p)), hidden("expected_revision", strconv.Itoa(revision)))}})
			return
		}
	case "profile":
		var v catalog.Profile
		v, err = parse[catalog.Profile](r.FormValue("payload"))
		if err == nil {
			_, err = app.Store.SaveProfile(ctx, a, id, revision, v)
		}
	case "measure":
		v := assessment.Measure{Name: r.FormValue("name"), Rationale: r.FormValue("rationale"), CheckID: r.FormValue("check_id"), FreshnessHours: integer(r, "freshness_hours"), ScopeRevision: integer(r, "scope_revision"), Expected: []string{}}
		for _, s := range strings.Split(r.FormValue("expected"), "\n") {
			if s = strings.TrimSpace(s); s != "" {
				v.Expected = append(v.Expected, s)
			}
		}
		_, err = app.Store.SaveMeasure(ctx, a, id, r.FormValue("id"), reqID, revision, v)
	case "evidence":
		var observed time.Time
		observed, err = time.Parse(time.RFC3339, r.FormValue("observed_at"))
		if err != nil {
			err = assessment.ErrInvalid
		} else {
			_, err = app.Store.AttachEvidence(ctx, a, id, reqID, r.FormValue("key"), assessment.Evidence{Source: r.FormValue("source"), URL: r.FormValue("url"), Description: r.FormValue("description"), ObservedAt: observed, ScopeRevision: integer(r, "scope_revision")})
		}
	case "draft":
		var ev, m []assessment.Ref
		ev, err = parse[[]assessment.Ref](r.FormValue("evidence"))
		if err == nil {
			m, err = parse[[]assessment.Ref](r.FormValue("measures"))
		}
		if err == nil {
			_, err = app.Store.SaveDraft(ctx, a, id, reqID, revision, assessment.Draft{Explanation: r.FormValue("explanation"), Evidence: ev, Measures: m, ProfileRevision: integer(r, "profile_revision"), ScopeRevision: integer(r, "scope_revision")})
		}
	case "review":
		if r.FormValue("reviewed") != "yes" {
			err = assessment.ErrInvalid
		} else {
			_, err = app.Store.Review(ctx, a, id, reqID, assessment.Review{Assessment: assessment.Ref{ID: reqID, Revision: integer(r, "revision")}, Notes: r.FormValue("notes")})
		}
	case "gap":
		_, err = app.Store.SaveGap(ctx, a, id, r.FormValue("id"), reqID, revision, assessment.Gap{Explanation: r.FormValue("explanation"), Status: r.FormValue("status")})
	case "check":
		var job checks.Job
		job, err = app.Runner.Enqueue(ctx, a, id, reqID, r.FormValue("check_id"), r.FormValue("key"), checks.Input{ScopeRevision: integer(r, "scope_revision"), Measure: assessment.Ref{ID: r.FormValue("id"), Revision: integer(r, "revision")}})
		if err == nil {
			redirectAction(w, r, base(id)+"/runs/"+job.ID)
			return
		}
	case "token":
		var token string
		actions := []string{"read", "edit", "check", "export"}
		if r.FormValue("connect") == "yes" {
			actions = append(actions, "connect")
		}
		token, err = auth.IssueToken(ctx, app.Store.DB, a, id, actions, time.Duration(integer(r, "hours"))*time.Hour)
		if err == nil {
			render(w, r, Page{Title: "Agent token", Notice: "Copy this token now; it is only shown once. Send it as Authorization: Bearer <token> to /mcp.", Sections: []Section{{"Token", token}}})
			return
		}
	case "revoke-token":
		err = auth.RevokeToken(ctx, app.Store.DB, a, id, r.FormValue("hash"))
		target += "?view=settings"
	case "destination":
		_, err = app.Store.SaveDestination(ctx, a, id, revision, assessment.Destination{Name: r.FormValue("name"), AdapterType: r.FormValue("adapter_type"), ConfigurationRef: r.FormValue("configuration_ref")})
		target += "?view=settings"
	case "prepare":
		_, err = app.Store.PreparePublication(ctx, a, id, r.FormValue("key"))
		target += "?view=history"
	default:
		err = assessment.ErrInvalid
	}
	if err != nil {
		failure(w, r, err)
		return
	}
	if reqID != "" {
		target = base(id) + "/requirements/" + url.PathEscape(reqID)
	}
	redirectAction(w, r, target)
}

func (app *App) run(w http.ResponseWriter, r *http.Request) {
	j, err := app.Runner.Get(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("id"))
	if err != nil {
		failure(w, r, err)
		return
	}
	if isUI(r) {
		p := Page{Title: "Collection run", Notice: "Server collector · " + strings.NewReplacer(".", " · ", "-", " ").Replace(j.CheckID), Metrics: []Metric{{"Run state", j.State}, {"Updated", j.UpdatedAt.UTC().Format("02 Jan 2006 · 15:04 UTC")}}, Activities: []Link{{"Open requirement", base(j.ServiceID) + "/requirements/" + j.RequirementID}, {"Review source access", base(j.ServiceID) + "?page=Sources"}, {"Inspect gathered evidence", base(j.ServiceID) + "?page=Evidence&requirement=" + j.RequirementID}}, Diagnostics: []Section{{"Exact run details", pretty(j)}}}
		if j.Result != nil {
			if j.State == "succeeded" {
				switch j.Result.Outcome {
				case "error":
					p.Metrics[0].Value = "Completed with collection errors"
				case "fail":
					p.Metrics[0].Value = "Completed · findings need attention"
				case "unknown":
					p.Metrics[0].Value = "Completed · incomplete evidence"
				default:
					p.Metrics[0].Value = "Completed"
				}
			}
			p.Metrics = append(p.Metrics, Metric{"Targets read", strconv.Itoa(j.Result.Observed) + " / " + strconv.Itoa(j.Result.Expected)})
			errorTargets := map[string][]string{}
			errorOrder := []string{}
			for _, o := range j.Result.Observations {
				if o.Error != "" {
					if _, ok := errorTargets[o.Error]; !ok {
						errorOrder = append(errorOrder, o.Error)
					}
					errorTargets[o.Error] = append(errorTargets[o.Error], o.Scope)
				} else {
					p.Sections = append(p.Sections, Section{o.Scope, "Read completed · " + o.Outcome})
					for _, ev := range o.Evaluations {
						if ev.Outcome != "pass" {
							p.Findings = append(p.Findings, Section{ev.Condition, ev.Explanation})
						}
					}
				}
			}
			for _, code := range errorOrder {
				e := observation.ExplainError(code, "", "server collector")
				if app.Connections != nil {
					p.Links = append(p.Links, Link{"Resolve access · " + e.Title, base(j.ServiceID) + "?page=Connections&provider=" + url.QueryEscape(e.ConnectionProvider)})
				}
				p.Sections = append(p.Sections, Section{e.Title + " · " + strconv.Itoa(len(errorTargets[code])) + " targets", e.Explanation + "\n\nAffected targets:\n" + strings.Join(errorTargets[code], "\n")})
			}
		}
		p.Forms = append(p.Forms, form(r, base(j.ServiceID)+"/actions/collect", "Retry supported collection", "Collect again", hidden("key", uuid.NewString())))
		render(w, r, p)
		return
	}
	render(w, r, Page{Title: "Check run", Notice: "Refresh to see the latest job state.", Links: []Link{{"Requirement", base(j.ServiceID) + "/requirements/" + j.RequirementID}}, Sections: []Section{{"Run and coverage", pretty(j)}}})
}
func (app *App) export(w http.ResponseWriter, r *http.Request) {
	b, contentType, err := app.Store.Export(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("id"), r.PathValue("format"))
	if err != nil {
		failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=assessment."+r.PathValue("format"))
	_, _ = w.Write(b)
}
func (app *App) artifact(w http.ResponseWriter, r *http.Request) {
	var b []byte
	var err error
	if raw := r.URL.Query().Get("revision"); raw != "" {
		rev, e := strconv.Atoi(raw)
		if e != nil || rev < 1 {
			failure(w, r, assessment.ErrInvalid)
			return
		}
		b, err = app.Store.ArtifactRevision(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), assessment.Ref{ID: r.PathValue("id"), Revision: rev})
	} else {
		b, err = app.Store.Artifact(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("id"))
	}
	if err != nil {
		failure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=evidence.json")
	_, _ = w.Write(b)
}
func (app *App) audit(ctx context.Context, a auth.Actor, service string) ([]map[string]any, error) {
	if err := auth.Authorize(ctx, app.Store.DB, a, service, "read"); err != nil {
		return nil, err
	}
	rows, err := app.Store.DB.Query(ctx, `SELECT actor_id,action,target,created_at FROM audit_events WHERE service_id=$1 ORDER BY id DESC LIMIT 100`, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var actor, action, target string
		var at time.Time
		if err = rows.Scan(&actor, &action, &target, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"actor": actor, "action": action, "target": target, "at": at})
	}
	return out, rows.Err()
}
