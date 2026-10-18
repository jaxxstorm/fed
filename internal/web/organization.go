package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/jaxxstorm/fed/internal/auth"
)

func organizationInput(w http.ResponseWriter, r *http.Request, out any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return auth.ErrMembershipInvalid
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return auth.ErrMembershipInvalid
	}
	return nil
}

func (app *App) registerOrganizationAPI(m *http.ServeMux) {
	m.HandleFunc("GET /organization", frontendPage)
	m.HandleFunc("GET /api/ui/v1/organization", func(w http.ResponseWriter, r *http.Request) {
		out, err := auth.GetOrganization(r.Context(), app.Store.DB, auth.FromContext(r.Context()))
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("POST /api/ui/v1/organization", func(w http.ResponseWriter, r *http.Request) {
		var in auth.OrganizationUpdate
		if err := organizationInput(w, r, &in); err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		out, err := auth.UpdateOrganization(r.Context(), app.Store.DB, auth.FromContext(r.Context()), in)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("GET /api/ui/v1/members", func(w http.ResponseWriter, r *http.Request) {
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				uiFailure(w, auth.ErrMembershipInvalid, r.Context())
				return
			}
			offset = n
		}
		out, err := auth.ListMembers(r.Context(), app.Store.DB, auth.FromContext(r.Context()), 50, offset)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("POST /api/ui/v1/members", func(w http.ResponseWriter, r *http.Request) {
		var in auth.MembershipChange
		if err := organizationInput(w, r, &in); err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		out, err := auth.ChangeMembership(r.Context(), app.Store.DB, auth.FromContext(r.Context()), in)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
}
