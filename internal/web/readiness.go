package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/readiness"
)

type ReadinessWorkPage struct {
	Items      []readiness.WorkRecord `json:"items"`
	NextOffset *int                   `json:"next_offset,omitempty"`
}

func (app *App) registerReadinessAPI(m *http.ServeMux) {
	m.HandleFunc("GET /api/ui/v1/services/{service}/readiness/requirements", func(w http.ResponseWriter, r *http.Request) {
		in := assessment.ReadinessQueueInput{ServiceID: r.PathValue("service"), Group: r.URL.Query().Get("group"), Cursor: r.URL.Query().Get("cursor"), Limit: 25}
		if value := r.URL.Query().Get("limit"); value != "" {
			n, e := strconv.Atoi(value)
			if e != nil {
				uiFailure(w, assessment.ErrInvalid, r.Context())
				return
			}
			in.Limit = n
		}
		out, e := app.Store.ReadinessQueue(r.Context(), auth.FromContext(r.Context()), in)
		if e != nil {
			uiFailure(w, e, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("GET /api/ui/v1/services/{service}/readiness/capabilities/{id}", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 25, 0
		for key, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
			if value := r.URL.Query().Get(key); value != "" {
				n, err := strconv.Atoi(value)
				if err != nil {
					uiFailure(w, readiness.ErrInvalid, r.Context())
					return
				}
				*dest = n
			}
		}
		out, err := app.Store.Guidance.Capability(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), r.PathValue("id"), limit, offset)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("GET /api/ui/v1/services/{service}/readiness/coverage", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 25, 0
		for key, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
			if value := r.URL.Query().Get(key); value != "" {
				n, err := strconv.Atoi(value)
				if err != nil {
					uiFailure(w, readiness.ErrInvalid, r.Context())
					return
				}
				*dest = n
			}
		}
		out, err := app.Store.Guidance.Quality(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), limit, offset)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("GET /api/ui/v1/services/{service}/readiness/plan", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 25, 0
		for key, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
			if value := r.URL.Query().Get(key); value != "" {
				n, err := strconv.Atoi(value)
				if err != nil {
					uiFailure(w, readiness.ErrInvalid, r.Context())
					return
				}
				*dest = n
			}
		}
		out, err := app.Store.Guidance.Plan(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), limit, offset)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("GET /api/ui/v1/services/{service}/readiness/work", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 25, 0
		for key, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
			if value := r.URL.Query().Get(key); value != "" {
				n, err := strconv.Atoi(value)
				if err != nil {
					uiFailure(w, readiness.ErrInvalid, r.Context())
					return
				}
				*dest = n
			}
		}
		items, err := app.Store.Guidance.Work(r.Context(), auth.FromContext(r.Context()), r.PathValue("service"), limit, offset)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		out := ReadinessWorkPage{Items: items}
		if len(items) == limit {
			next := offset + limit
			out.NextOffset = &next
		}
		jsonResponse(w, 200, out)
	})
	m.HandleFunc("POST /api/ui/v1/services/{service}/readiness/work", func(w http.ResponseWriter, r *http.Request) {
		var in readiness.WorkChange
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
		d.DisallowUnknownFields()
		if err := d.Decode(&in); err != nil {
			uiFailure(w, readiness.ErrInvalid, r.Context())
			return
		}
		if err := d.Decode(new(any)); err != io.EOF {
			uiFailure(w, readiness.ErrInvalid, r.Context())
			return
		}
		if in.ServiceID != "" && in.ServiceID != r.PathValue("service") {
			uiFailure(w, readiness.ErrInvalid, r.Context())
			return
		}
		in.ServiceID = r.PathValue("service")
		out, err := app.Store.Guidance.SaveWork(r.Context(), auth.FromContext(r.Context()), in)
		if err != nil {
			uiFailure(w, err, r.Context())
			return
		}
		jsonResponse(w, 200, out)
	})
}
