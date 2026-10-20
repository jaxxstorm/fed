package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func (app *App) releaseWorkspace(r *http.Request, p *Page, service string) error {
	a := auth.FromContext(r.Context())
	runs, err := app.Store.ReportingRuns(r.Context(), a, service, 25, 0)
	if err != nil {
		return err
	}
	for _, run := range runs {
		p.Sections = append(p.Sections, Section{"Reporting candidate · " + run.State, run.ID + " · " + run.Notification + " · " + run.CandidateID})
	}
	var scheduleRevision int
	if err = app.Store.DB.QueryRow(r.Context(), `SELECT coalesce((SELECT revision FROM assessment_reporting_schedules WHERE service_id=$1),0)`, service).Scan(&scheduleRevision); err != nil {
		return err
	}
	p.Forms = append(p.Forms, form(r, base(service)+"/actions/reporting-schedule", "Recurring assessment candidates", "Save reporting schedule", hidden("expected_revision", strconv.Itoa(scheduleRevision)), choice("enabled", "Enable candidate preparation", "no", "no", "yes"), field("interval_hours", "Interval in hours", "720"), field("period_start", "First reporting period start (RFC3339)", ""), field("next_run_at", "First reporting period end / due time (RFC3339)", ""), area("configuration", "Release configuration (public preview and optional previous release)", pretty(assessment.PrepareReleaseInput{ServiceID: service, Public: assessment.PublicRelease{ServiceName: p.Title, Contacts: []string{}, Requirements: []assessment.PublicRequirement{}}}))))
	context := assessment.ServiceContext{Contacts: []string{}, Deliverables: []assessment.ReleaseDeliverable{}}
	revision := 0
	if record, err := app.Store.Get(r.Context(), a, service, "service_context", "service_context"); err == nil {
		context, err = assessment.Decode[assessment.ServiceContext](record)
		if err != nil {
			return err
		}
		revision = record.Revision
	} else if !errors.Is(err, assessment.ErrNotFound) {
		return err
	}
	p.Forms = append(p.Forms, form(r, base(service)+"/actions/service-context", "Assessment package · service context", "Save service context", hidden("expected_revision", strconv.Itoa(revision)), area("description", "Offering description", context.Description), area("boundary", "Authorization boundary", context.Boundary), area("responsibilities", "Provider and customer responsibilities", context.Responsibilities), area("contacts", "Points of contact (one per line)", strings.Join(context.Contacts, "\n")), area("deliverables", "Supporting deliverables: requirement IDs and exact evidence references", pretty(context.Deliverables))))
	p.Forms = append(p.Forms, form(r, base(service)+"/actions/prepare-release", "Prepare an immutable assessment package", "Prepare release", hidden("key", uuid.NewString()), choice("preparation", "Preparation mode", "now", "now", "queued"), field("period_start", "Period start (RFC3339)", ""), field("period_end", "Period end (RFC3339)", ""), field("previous_release_id", "Previous release ID (optional)", ""), area("public", "Public disclosure preview: only these fields will be public", pretty(assessment.PublicRelease{ServiceName: p.Title, Contacts: []string{}, Requirements: []assessment.PublicRequirement{}}))))
	p.Sections = append(p.Sections, Section{"Publication and disclosure", "Preparing or exporting a package does not publish it. Public downloads cannot be recalled after publication. Review the public summary separately from restricted evidence. Incomplete reports remain explicitly labeled incomplete."})
	offset := 0
	if raw := r.URL.Query().Get("release_offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return assessment.ErrInvalid
		}
		offset = n
	}
	releases, err := app.Store.ListReleases(r.Context(), a, service, 10, offset)
	if err != nil {
		return err
	}
	for _, release := range releases {
		p.Sections = append(p.Sections, Section{"Assessment release · " + release.State, release.ID + "\nManifest: " + release.ManifestHash + "\nAudience: " + release.AudienceHash + "\n" + release.Manifest.Label})
		p.Sections = append(p.Sections, Section{"Public disclosure preview · " + release.ID, pretty(release.Manifest.Public)})
		for _, blocker := range release.Manifest.Blockers {
			p.Sections = append(p.Sections, Section{"Release blocker", blocker})
		}
		for _, change := range release.Manifest.Changes {
			p.Sections = append(p.Sections, Section{"Change · " + change.Category, change.RequirementID + " · " + change.Explanation})
		}
		url := base(service) + "/assessment-releases/" + release.ID + "/"
		p.Links = append(p.Links, Link{"Preview assessment package", url}, Link{"Download release JSON", url + "?format=json"}, Link{"Download offline evidence archive", url + "?format=zip"})
		action := ""
		switch release.State {
		case "prepared":
			action = "approve"
		case "approved":
			action = "publish"
		case "published":
			action = "withdraw"
		}
		if action != "" {
			p.Forms = append(p.Forms, form(r, base(service)+"/actions/release-transition", action+" · "+release.ID, "Record "+action, hidden("id", release.ID), hidden("expected_revision", strconv.Itoa(release.Revision)), hidden("manifest_hash", release.ManifestHash), hidden("audience_hash", release.AudienceHash), hidden("transition", action), hidden("key", uuid.NewString()), area("reason", "Decision rationale", ""), choice("confirmed", "I reviewed this exact package and its public disclosure", "no", "no", "yes")))
		}
	}
	if len(releases) == 10 {
		p.Pagination = append(p.Pagination, Link{"Next releases", base(service) + "?page=History&release_offset=" + strconv.Itoa(offset+10)})
	}
	if offset > 0 {
		p.Pagination = append(p.Pagination, Link{"Previous releases", base(service) + "?page=History&release_offset=" + strconv.Itoa(max(0, offset-10))})
	}
	if auth.Authorize(r.Context(), app.Store.DB, a, service, "admin") == nil {
		p.Forms = append(p.Forms, form(r, base(service)+"/actions/trust-config", "Trust-centre delivery", "Save trust-centre configuration", field("slug", "Public service slug", ""), choice("enabled", "Enable delivery of published releases", "no", "no", "yes")))
		requests, err := app.Store.TrustRequests(r.Context(), a, service, 100, 0)
		if err != nil {
			return err
		}
		for _, request := range requests {
			if request.State != "pending" {
				continue
			}
			p.Forms = append(p.Forms, form(r, base(service)+"/actions/trust-access", "Evidence access request · "+request.IdentityID, "Record access decision", hidden("id", request.ID), choice("approve", "Approve this service/release access request", "no", "no", "yes"), choice("capacity", "Access capacity", "reader", "reader", "assessor"), field("expires_at", "Expiry (RFC3339)", "")))
		}
		grants, err := app.Store.TrustGrants(r.Context(), a, service, 100, 0)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			if grant.RevokedAt != nil {
				continue
			}
			p.Forms = append(p.Forms, form(r, base(service)+"/actions/trust-revoke", "Trust grant · "+grant.IdentityID+" · expires "+grant.ExpiresAt.Format(time.RFC3339), "Revoke access", hidden("id", grant.ID)))
		}
	}
	return nil
}

func (app *App) internalAssessmentRelease(w http.ResponseWriter, r *http.Request) {
	a := auth.FromContext(r.Context())
	service := r.PathValue("service")
	if r.URL.Query().Get("format") != "" {
		if err := auth.Authorize(r.Context(), app.Store.DB, a, service, "export"); err != nil {
			failure(w, r, err)
			return
		}
	}
	release, err := app.Store.GetRelease(r.Context(), a, service, r.PathValue("release"))
	if err != nil {
		failure(w, r, err)
		return
	}
	app.deliverRelease(w, r, release)
}

func (app *App) internalReleaseArtifact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	release, err := app.Store.GetRelease(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("release"))
	if err != nil {
		failure(w, r, err)
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
		http.NotFound(w, r)
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
