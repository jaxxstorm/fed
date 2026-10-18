package checks

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/rs/zerolog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type Batch struct {
	State         string                       `json:"state"`
	CompletedJobs int                          `json:"completed_jobs"`
	ID            string                       `json:"id"`
	ServiceID     string                       `json:"service_id"`
	PlanRevision  int                          `json:"plan_revision"`
	Jobs          []Job                        `json:"jobs"`
	Blocked       []assessment.CollectionEntry `json:"blocked"`
}

func (b *Batch) summarize() {
	b.State = "completed"
	b.CompletedJobs = 0
	partial := len(b.Blocked) > 0
	pending := false
	for _, j := range b.Jobs {
		if j.State == "queued" || j.State == "running" {
			pending = true
			continue
		}
		b.CompletedJobs++
		if j.State != "succeeded" || j.Result == nil || j.Result.Outcome == "error" {
			partial = true
		}
	}
	if pending {
		b.State = "running"
	} else if partial {
		b.State = "partial"
	}
	if len(b.Jobs) == 0 && len(b.Blocked) > 0 {
		b.State = "blocked"
	}
}

func (r *Runner) CollectNow(ctx context.Context, a auth.Actor, service, key string) (Batch, error) {
	if err := auth.Authorize(ctx, r.DB, a, service, "check"); err != nil {
		return Batch{}, err
	}
	if key == "" || len(key) > 256 {
		return Batch{}, assessment.ErrInvalid
	}
	store := assessment.Store{DB: r.DB, Artifacts: r.Artifacts}
	if _, err := store.ReconcileCollection(ctx, service); err != nil {
		return Batch{}, err
	}
	return r.enqueueBatch(ctx, a, service, "manual:"+a.ID+":"+key, false)
}
func (r *Runner) enqueueBatch(ctx context.Context, a auth.Actor, service, slot string, scheduled bool) (Batch, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Batch{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "check"); err != nil {
		return Batch{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return Batch{}, err
	}
	var existing string
	err = tx.QueryRow(ctx, `SELECT id FROM collection_batches WHERE service_id=$1 AND slot=$2`, service, slot).Scan(&existing)
	if err == nil {
		tx.Rollback(ctx)
		return r.GetBatch(ctx, a, service, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, err
	}
	if scheduled {
		var due bool
		if err = tx.QueryRow(ctx, `SELECT NOT paused AND next_run_at<=now() FROM collection_schedules WHERE service_id=$1 FOR UPDATE`, service).Scan(&due); err != nil {
			return Batch{}, err
		}
		if !due {
			return Batch{}, nil
		}
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT data FROM collection_plans WHERE service_id=$1 ORDER BY revision DESC LIMIT 1`, service).Scan(&raw); err != nil {
		return Batch{}, err
	}
	var plan assessment.CollectionPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		return Batch{}, err
	}
	batch := Batch{ID: uuid.NewString(), ServiceID: service, PlanRevision: plan.Revision, Jobs: []Job{}, Blocked: []assessment.CollectionEntry{}}
	if _, err = tx.Exec(ctx, `INSERT INTO collection_batches(service_id,id,plan_revision,actor_id,slot) VALUES($1,$2,$3,$4,$5)`, service, batch.ID, plan.Revision, a.ID, slot); err != nil {
		return batch, err
	}
	groups := map[string][]assessment.CollectionEntry{}
	for _, entry := range plan.Entries {
		if entry.CheckID == "" {
			continue
		}
		if _, ok := r.Registry[entry.CheckID]; !ok {
			entry.State = "unsupported_automation"
			batch.Blocked = append(batch.Blocked, entry)
			continue
		}
		_, m, e := loadInput(ctx, tx, service, entry.RequirementID, entry.CheckID, Input{ScopeRevision: plan.ScopeRevision, Measure: entry.Measure, ConfigurationRevision: plan.ConfigurationRevision}, true)
		if e != nil {
			return batch, e
		}
		parameters, _ := json.Marshal(struct {
			Expected []string
			Policy   map[string]json.RawMessage
		}{m.Expected, m.Policy})
		group := entry.CheckID + ":" + storage.Hash(parameters)
		groups[group] = append(groups[group], entry)
	}
	for _, entries := range groups {
		first := entries[0]
		check := first.CheckID
		input := Input{ScopeRevision: plan.ScopeRevision, ProfileRevision: plan.ProfileRevision, ConfigurationRevision: plan.ConfigurationRevision, Measure: first.Measure, Bindings: entries}
		if _, _, err = loadInput(ctx, tx, service, first.RequirementID, check, input, true); err != nil {
			return batch, err
		}
		j := Job{ID: uuid.NewString(), ServiceID: service, ActorID: a.ID, RequirementID: first.RequirementID, CheckID: check, Input: input, State: "queued"}
		b, _ := json.Marshal(input)
		var token any
		if a.TokenHash != "" {
			token = a.TokenHash
		}
		if err = tx.QueryRow(ctx, `INSERT INTO jobs(id,service_id,actor_id,token_hash,requirement_id,check_id,input,state,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,'queued',now()+interval '10 minutes') RETURNING created_at,updated_at`, j.ID, service, a.ID, token, j.RequirementID, check, b).Scan(&j.CreatedAt, &j.UpdatedAt); err != nil {
			return batch, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO collection_batch_jobs(service_id,batch_id,job_id) VALUES($1,$2,$3)`, service, batch.ID, j.ID); err != nil {
			return batch, err
		}
		batch.Jobs = append(batch.Jobs, j)
	}
	if scheduled {
		if _, err = tx.Exec(ctx, `UPDATE collection_schedules SET next_run_at=now()+make_interval(hours=>cadence_hours),updated_at=now() WHERE service_id=$1`, service); err != nil {
			return batch, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'collection.queued',$4)`, a.OrganizationID, service, a.ID, batch.ID); err != nil {
		return batch, err
	}
	batch.summarize()
	err = logging.Commit(ctx, tx, logging.Fields{Event: "collection.batch.enqueued", Component: "worker", ServiceID: service, ActorID: a.ID, BatchID: batch.ID, Count: len(batch.Jobs)})
	if err == nil {
		for _, j := range batch.Jobs {
			j.BatchID = batch.ID
			logging.Emit(ctx, zerolog.InfoLevel, jobFields(j, "enqueued", "success"))
		}
	}
	return batch, err
}

func (r *Runner) GetBatch(ctx context.Context, a auth.Actor, service, id string) (Batch, error) {
	if err := auth.Authorize(ctx, r.DB, a, service, "read"); err != nil {
		return Batch{}, err
	}
	b := Batch{ID: id, ServiceID: service, Jobs: []Job{}, Blocked: []assessment.CollectionEntry{}}
	var raw []byte
	err := r.DB.QueryRow(ctx, `SELECT b.plan_revision,p.data FROM collection_batches b JOIN collection_plans p ON p.service_id=b.service_id AND p.revision=b.plan_revision WHERE b.service_id=$1 AND b.id=$2`, service, id).Scan(&b.PlanRevision, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, assessment.ErrNotFound
	}
	if err != nil {
		return b, err
	}
	var plan assessment.CollectionPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		return b, err
	}
	for _, entry := range plan.Entries {
		if entry.CheckID != "" {
			if _, ok := r.Registry[entry.CheckID]; !ok {
				entry.State = "unsupported_automation"
				b.Blocked = append(b.Blocked, entry)
			}
		}
	}
	rows, err := r.DB.Query(ctx, `SELECT job_id FROM collection_batch_jobs WHERE service_id=$1 AND batch_id=$2 ORDER BY job_id`, service, id)
	if err != nil {
		return b, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return b, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return b, err
	}
	for _, id := range ids {
		j, err := r.Get(ctx, a, service, id)
		if err != nil {
			return b, err
		}
		b.Jobs = append(b.Jobs, j)
	}
	b.summarize()
	return b, nil
}

func (r *Runner) Schedule(ctx context.Context) error {
	store := assessment.Store{DB: r.DB, Artifacts: r.Artifacts}
	rows, err := r.DB.Query(ctx, `SELECT service_id,generation FROM collection_outbox ORDER BY updated_at LIMIT 10`)
	if err != nil {
		return err
	}
	type pending struct {
		service    string
		generation int64
	}
	pendingItems := []pending{}
	for rows.Next() {
		var p pending
		if err = rows.Scan(&p.service, &p.generation); err != nil {
			rows.Close()
			return err
		}
		pendingItems = append(pendingItems, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range pendingItems {
		_, err = store.ReconcileCollection(ctx, p.service)
		if err != nil && !errors.Is(err, assessment.ErrNotFound) {
			return err
		}
		if _, err = r.DB.Exec(ctx, `DELETE FROM collection_outbox WHERE service_id=$1 AND generation=$2`, p.service, p.generation); err != nil {
			return err
		}
	}
	rows, err = r.DB.Query(ctx, `SELECT s.service_id,s.actor_id,i.organization_id,s.next_run_at FROM collection_schedules s JOIN identities i ON i.id=s.actor_id WHERE NOT s.paused AND s.next_run_at<=now() ORDER BY next_run_at LIMIT 10`)
	if err != nil {
		return err
	}
	type due struct {
		service string
		actor   auth.Actor
		at      time.Time
	}
	items := []due{}
	for rows.Next() {
		var d due
		d.actor.Kind = "automation"
		if err = rows.Scan(&d.service, &d.actor.ID, &d.actor.OrganizationID, &d.at); err != nil {
			rows.Close()
			return err
		}
		items = append(items, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range items {
		if _, err = r.enqueueBatch(ctx, d.actor, d.service, "schedule:"+storage.Hash([]byte(d.at.UTC().Format(time.RFC3339Nano))), true); err != nil {
			if errors.Is(err, assessment.ErrConflict) {
				continue
			}
			return err
		}
	}
	return nil
}
