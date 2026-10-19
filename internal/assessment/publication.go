package assessment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/integrations"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

type Destination struct {
	Name             string `json:"name"`
	AdapterType      string `json:"adapter_type"`
	ConfigurationRef string `json:"configuration_ref"`
}
type Publication struct {
	SnapshotID  string                `json:"snapshot_id"`
	Destination *Ref                  `json:"destination,omitempty"`
	State       string                `json:"state"`
	Error       string                `json:"error,omitempty"`
	Receipt     *integrations.Receipt `json:"receipt,omitempty"`
}
type Bundle struct {
	ReadinessWork            []json.RawMessage           `json:"readiness_work,omitempty"`
	Guidance                 []readiness.GuidanceContext `json:"guidance,omitempty"`
	CollectionPlans          []json.RawMessage           `json:"collection_plans"`
	CollectionConfigurations []json.RawMessage           `json:"collection_configurations"`
	Observations             []json.RawMessage           `json:"observations"`
	Format                   string                      `json:"format"`
	Label                    string                      `json:"label"`
	ServiceID                string                      `json:"service_id"`
	PreparedAt               time.Time                   `json:"prepared_at"`
	Profile                  catalog.Profile             `json:"profile"`
	Requirements             []catalog.Resolved          `json:"requirements"`
	Records                  []Record                    `json:"records"`
	Statuses                 []Status                    `json:"statuses"`
}

func (s *Store) SaveDestination(ctx context.Context, a auth.Actor, service string, expected int, v Destination) (Record, error) {
	if v.Name == "" || v.AdapterType == "" {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "destination", "destination", "", v), expected, "admin", nil)
}

func (s *Store) PreparePublication(ctx context.Context, a auth.Actor, service, key string) (Record, error) {
	if key == "" {
		return Record{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "export"); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return Record{}, err
	}
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(a.ID+"/"+service+"/snapshot/"+key)).String()
	if old, err := latest(ctx, tx, service, "snapshot", id); err == nil {
		return old, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	profileRecord, err := latest(ctx, tx, service, "profile", "profile")
	if err != nil {
		return Record{}, err
	}
	profile, err := Decode[catalog.Profile](profileRecord)
	if err != nil {
		return Record{}, err
	}
	all, err := catalog.Load(ctx, tx, profile.CatalogID)
	if err != nil {
		return Record{}, err
	}
	resolved, err := catalog.Resolve(all, profile)
	if err != nil {
		return Record{}, err
	}
	bundle := Bundle{Format: "ksi-assessment-bundle/v1", Label: "Application assessment bundle; not a complete certification package or schema-validated official SDR", ServiceID: service, PreparedAt: time.Now().UTC(), Profile: profile, Requirements: resolved, Records: []Record{}}
	if s.Guidance != nil {
		bundle.Guidance, err = s.Guidance.ExportContexts(ctx, tx, a, service, profile.CatalogID, resolved, bundle.PreparedAt)
		if err != nil {
			return Record{}, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM records WHERE service_id=$1 AND kind IN ('scope','profile','measure','evidence','assessment','review','gap','evidence_source','rule_result') ORDER BY kind,id,revision`, service)
	if err != nil {
		return Record{}, err
	}
	for rows.Next() {
		var r Record
		if err = rows.Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt); err != nil {
			rows.Close()
			return Record{}, err
		}
		bundle.Records = append(bundle.Records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Record{}, err
	}
	for _, section := range []struct {
		query       string
		destination *[]json.RawMessage
	}{
		{`SELECT data FROM collection_plans WHERE service_id=$1 ORDER BY revision`, &bundle.CollectionPlans},
		{`SELECT data FROM collection_configurations WHERE service_id=$1 ORDER BY revision`, &bundle.CollectionConfigurations},
		{`SELECT to_jsonb(o) FROM observations o WHERE service_id=$1 ORDER BY observed_at,id`, &bundle.Observations},
		{`SELECT to_jsonb(w) FROM readiness_work w WHERE service_id=$1 ORDER BY id,revision`, &bundle.ReadinessWork},
	} {
		rows, e := tx.Query(ctx, section.query, service)
		if e != nil {
			return Record{}, e
		}
		for rows.Next() {
			var b []byte
			if e = rows.Scan(&b); e != nil {
				rows.Close()
				return Record{}, e
			}
			*section.destination = append(*section.destination, json.RawMessage(b))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return Record{}, e
		}
	}
	view := *s
	view.reader = tx
	for _, item := range resolved {
		status, err := view.Status(ctx, a, service, item.Requirement.ID, bundle.PreparedAt)
		if err != nil {
			return Record{}, err
		}
		bundle.Statuses = append(bundle.Statuses, status)
	}
	r := record(service, "snapshot", id, "", bundle)
	r.Revision = 1
	r.ActorID = a.ID
	if len(r.Data) > storage.MaxArtifactBytes {
		return Record{}, errors.New("snapshot exceeds export size limit")
	}
	err = tx.QueryRow(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'snapshot',$2,1,'',$3,$4) RETURNING created_at`, service, id, a.ID, r.Data).Scan(&r.CreatedAt)
	if err != nil {
		return Record{}, err
	}
	publication := Publication{SnapshotID: id, State: "prepared"}
	destination, err := latest(ctx, tx, service, "destination", "destination")
	if err == nil {
		publication.Destination = &Ref{destination.ID, destination.Revision}
	} else if !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'publication',$2,1,'',$3,$4)`, service, uuid.NewString(), a.ID, data(publication)); err != nil {
		return Record{}, err
	}
	if err = audit(ctx, tx, a, service, "snapshot.prepared", id, 1); err != nil {
		return Record{}, err
	}
	return r, commitOperation(ctx, tx, a, service, "export.prepared", r)
}

func (s *Store) Export(ctx context.Context, a auth.Actor, service, id, format string) ([]byte, string, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "export"); err != nil {
		return nil, "", err
	}
	r, err := s.Get(ctx, a, service, "snapshot", id)
	if err != nil {
		return nil, "", err
	}
	var output []byte
	contentType := "application/json"
	switch format {
	case "json":
		output, err = json.MarshalIndent(json.RawMessage(r.Data), "", "  ")
	case "html":
		contentType = "text/html; charset=utf-8"
		var b bytes.Buffer
		pretty, e := json.MarshalIndent(json.RawMessage(r.Data), "", "  ")
		if e != nil {
			return nil, "", e
		}
		err = exportTemplate.Execute(&b, struct{ ID, JSON string }{r.ID, string(pretty)})
		output = b.Bytes()
	default:
		return nil, "", ErrInvalid
	}
	if err != nil {
		return nil, "", err
	}
	_, err = s.mutate(ctx, a, record(service, "publication", uuid.NewString(), "", Publication{SnapshotID: id, State: "exported"}), 0, "export", nil)
	return output, contentType, err
}

var exportTemplate = template.Must(template.New("bundle").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Assessment bundle {{.ID}}</title></head><body><h1>Application assessment bundle</h1><p>This is not a complete certification package or schema-validated official SDR.</p><p>Snapshot: {{.ID}}</p><pre>{{.JSON}}</pre></body></html>`))

// Live vendor connectors are deliberately absent from this release. Record the
// unsuccessful attempt against the exact destination version, never publication.
func (s *Store) Publish(ctx context.Context, a auth.Actor, service, snapshotID string) error {
	if err := auth.Authorize(ctx, s.DB, a, service, "export"); err != nil {
		return err
	}
	if _, err := s.Get(ctx, a, service, "snapshot", snapshotID); err != nil {
		return err
	}
	p := Publication{SnapshotID: snapshotID, State: "failed", Error: integrations.ErrUnavailable.Error()}
	d, err := s.Get(ctx, a, service, "destination", "destination")
	if err == nil {
		p.Destination = &Ref{d.ID, d.Revision}
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if _, err = s.mutate(ctx, a, record(service, "publication", uuid.NewString(), "", p), 0, "export", nil); err != nil {
		return err
	}
	return integrations.ErrUnavailable
}
