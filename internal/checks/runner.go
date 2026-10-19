// Package checks executes registered read-only collectors with durable leases.
package checks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/rs/zerolog"
)

type Input struct {
	ConfigurationRevision int                          `json:"configuration_revision,omitempty"`
	ProfileRevision       int                          `json:"profile_revision,omitempty"`
	Bindings              []assessment.CollectionEntry `json:"bindings,omitempty"`
	ScopeRevision         int                          `json:"scope_revision"`
	Measure               assessment.Ref               `json:"measure"`
}
type Observation struct {
	Target      *observation.Target      `json:"target,omitempty"`
	Evaluations []observation.Evaluation `json:"evaluations,omitempty"`
	Scope       string                   `json:"scope"`
	Outcome     string                   `json:"outcome"`
	Data        any                      `json:"data,omitempty"`
	Error       string                   `json:"error,omitempty"`
}
type Result struct {
	Outcome          string        `json:"outcome"`
	Expected         int           `json:"expected"`
	Observed         int           `json:"observed"`
	Observations     []Observation `json:"observations"`
	EvidenceIDs      []string      `json:"evidence_ids"`
	Limitations      []string      `json:"limitations"`
	CollectorVersion string        `json:"collector_version"`
}

// A fresh partition access observation supersedes an older partition error at
// the same target key without asserting that any resource control passed.
func partitionAccessObserved(scope, explanation string) Observation {
	return Observation{Scope: scope, Outcome: "unknown", Data: map[string]string{"inventory": "access_observed", "explanation": explanation}}
}

func accessObservation(o Observation) bool {
	v, ok := o.Data.(map[string]string)
	return ok && v["inventory"] == "access_observed" && o.Error == ""
}

func (r *Result) Summarize() {
	r.Outcome = "pass"
	r.Expected = len(r.Observations)
	r.Observed = 0
	if r.Expected == 0 {
		r.Outcome = "unknown"
	}
	for _, o := range r.Observations {
		if accessObservation(o) {
			r.Expected--
			continue
		}
		if o.Error == "" {
			r.Observed++
		}
		switch o.Outcome {
		case "error":
			r.Outcome = "error"
		case "unknown":
			if r.Outcome != "error" {
				r.Outcome = "unknown"
			}
		case "fail":
			if r.Outcome == "pass" {
				r.Outcome = "fail"
			}
		}
	}
	if r.Expected == 0 {
		r.Outcome = "unknown"
	}
}

type Collector interface {
	Collect(context.Context, assessment.Scope, assessment.Measure) (Result, error)
}
type Runner struct {
	Connections *connections.Service
	DB          *pgxpool.Pool
	Artifacts   storage.Artifacts
	Registry    map[string]Collector
}
type Job struct {
	BatchID       string    `json:"-"`
	ID            string    `json:"id"`
	ServiceID     string    `json:"service_id"`
	ActorID       string    `json:"actor_id"`
	TokenHash     *string   `json:"-"`
	RequirementID string    `json:"requirement_id"`
	CheckID       string    `json:"check_id"`
	Input         Input     `json:"input"`
	State         string    `json:"state"`
	Attempts      int       `json:"attempts"`
	LeaseID       string    `json:"-"`
	Result        *Result   `json:"result,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (r *Runner) Enqueue(ctx context.Context, a auth.Actor, service, requirement, check, key string, input Input) (Job, error) {
	if input.ProfileRevision != 0 || input.ConfigurationRevision != 0 || len(input.Bindings) > 0 {
		return Job{}, assessment.ErrInvalid
	}
	if _, ok := r.Registry[check]; !ok || key == "" || input.ScopeRevision < 1 || input.Measure.Revision < 1 {
		return Job{}, assessment.ErrInvalid
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "check"); err != nil {
		return Job{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return Job{}, err
	}
	request, _ := json.Marshal(struct {
		Requirement, Check string
		Input              Input
	}{requirement, check, input})
	hash := storage.Hash(request)
	var oldHash string
	var response []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='run_check' AND key=$3`, a.ID, service, key).Scan(&oldHash, &response)
	if err == nil {
		if oldHash != hash {
			return Job{}, assessment.ErrConflict
		}
		var j Job
		err = json.Unmarshal(response, &j)
		if err == nil {
			logging.Emit(ctx, zerolog.InfoLevel, jobFields(j, "enqueued", "replayed"))
		}
		return j, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, err
	}
	_, m, err := loadInput(ctx, tx, service, requirement, check, input, true)
	if err != nil {
		return Job{}, err
	}
	if m.CheckID != check {
		return Job{}, assessment.ErrInvalid
	}
	if err = tx.QueryRow(ctx, `SELECT coalesce(max(revision),0) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile'`, service).Scan(&input.ProfileRevision); err != nil {
		return Job{}, err
	}
	if input.ProfileRevision > 0 {
		input.Bindings = []assessment.CollectionEntry{{ID: input.Measure.ID, RequirementID: requirement, CheckID: check, Measure: input.Measure, Rationale: m.Rationale}}
		input.ConfigurationRevision = m.ConfigurationRevision
	}
	j := Job{ID: uuid.NewString(), ServiceID: service, ActorID: a.ID, RequirementID: requirement, CheckID: check, Input: input, State: "queued"}
	var token any
	if a.TokenHash != "" {
		token = a.TokenHash
	}
	b, _ := json.Marshal(input)
	err = tx.QueryRow(ctx, `INSERT INTO jobs(id,service_id,actor_id,token_hash,requirement_id,check_id,input,state,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,'queued',now()+interval '10 minutes') RETURNING created_at,updated_at`, j.ID, service, a.ID, token, requirement, check, b).Scan(&j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return Job{}, err
	}
	response, _ = json.Marshal(j)
	if _, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'run_check',$3,$4,$5)`, a.ID, service, key, hash, response); err != nil {
		return Job{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'check.queued',$4)`, a.OrganizationID, service, a.ID, j.ID); err != nil {
		return Job{}, err
	}
	return j, logging.Commit(ctx, tx, jobFields(j, "enqueued", "success"))
}

func loadInput(ctx context.Context, q auth.Querier, service, requirement, check string, in Input, current bool) (assessment.Scope, assessment.Measure, error) {
	var scope assessment.Scope
	var measure assessment.Measure
	var raw []byte
	var id string
	err := q.QueryRow(ctx, `SELECT data FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' AND revision=$2`, service, in.ScopeRevision).Scan(&raw)
	if err != nil {
		return scope, measure, assessment.ErrInvalid
	}
	if err = json.Unmarshal(raw, &scope); err != nil {
		return scope, measure, err
	}
	err = q.QueryRow(ctx, `SELECT data,requirement_id FROM records WHERE service_id=$1 AND kind='measure' AND id=$2 AND revision=$3`, service, in.Measure.ID, in.Measure.Revision).Scan(&raw, &id)
	if err != nil {
		return scope, measure, assessment.ErrInvalid
	}
	if err = json.Unmarshal(raw, &measure); err != nil {
		return scope, measure, err
	}
	if id != requirement || measure.CheckID != check || measure.ScopeRevision != in.ScopeRevision {
		return scope, measure, assessment.ErrInvalid
	}
	if current {
		if in.ProfileRevision > 0 {
			var profileRevision int
			if err = q.QueryRow(ctx, `SELECT coalesce(max(revision),0) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile'`, service).Scan(&profileRevision); err != nil {
				return scope, measure, err
			}
			if profileRevision != in.ProfileRevision {
				return scope, measure, assessment.ErrConflict
			}
		}
		var revision int
		if err = q.QueryRow(ctx, `SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope'`, service).Scan(&revision); err != nil {
			return scope, measure, err
		}
		if revision != in.ScopeRevision {
			return scope, measure, assessment.ErrConflict
		}
		if in.ConfigurationRevision > 0 {
			var currentConfig int
			if err = q.QueryRow(ctx, `SELECT revision FROM collection_schedules WHERE service_id=$1`, service).Scan(&currentConfig); err != nil {
				return scope, measure, err
			}
			if currentConfig != in.ConfigurationRevision {
				return scope, measure, assessment.ErrConflict
			}
		}
	}
	return scope, measure, nil
}

func (r *Runner) Get(ctx context.Context, a auth.Actor, service, id string) (Job, error) {
	if err := auth.Authorize(ctx, r.DB, a, service, "read"); err != nil {
		return Job{}, err
	}
	var j Job
	var input, result []byte
	err := r.DB.QueryRow(ctx, `SELECT id,service_id,actor_id,requirement_id,check_id,input,state,attempts,result,created_at,updated_at FROM jobs WHERE service_id=$1 AND id=$2`, service, id).Scan(&j.ID, &j.ServiceID, &j.ActorID, &j.RequirementID, &j.CheckID, &input, &j.State, &j.Attempts, &result, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, assessment.ErrNotFound
	}
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal(input, &j.Input); err != nil {
		return j, err
	}
	if result != nil {
		err = json.Unmarshal(result, &j.Result)
	}
	return j, err
}

func (r *Runner) List(ctx context.Context, a auth.Actor, service, requirement string) ([]Job, error) {
	if err := auth.Authorize(ctx, r.DB, a, service, "read"); err != nil {
		return nil, err
	}
	rows, err := r.DB.Query(ctx, `SELECT id FROM jobs WHERE service_id=$1 AND requirement_id=$2 ORDER BY created_at DESC LIMIT 25`, service, requirement)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []Job{}
	for _, id := range ids {
		job, err := r.Get(ctx, a, service, id)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (r *Runner) claim(ctx context.Context) (Job, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `UPDATE jobs SET state=CASE WHEN deadline<=now() THEN 'timed-out' ELSE 'failed' END,updated_at=now() WHERE state IN ('queued','running') AND (lease_until IS NULL OR lease_until<now()) AND (deadline<=now() OR attempts>=max_attempts) RETURNING id,service_id,attempts,state`)
	if err != nil {
		return Job{}, err
	}
	expired, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Job, error) {
		var j Job
		err := row.Scan(&j.ID, &j.ServiceID, &j.Attempts, &j.State)
		return j, err
	})
	if err != nil {
		return Job{}, err
	}
	commit := func() error {
		err := tx.Commit(ctx)
		if err == nil {
			for _, j := range expired {
				logging.Emit(ctx, zerolog.ErrorLevel, jobFields(j, "exhausted", j.State))
			}
		}
		return err
	}
	var j Job
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT id,service_id,actor_id,token_hash,requirement_id,check_id,input,attempts FROM jobs WHERE (state='queued' OR (state='running' AND lease_until<now())) AND deadline>now() AND attempts<max_attempts
AND available_at<=now() AND NOT EXISTS(SELECT 1 FROM collection_batch_jobs bj JOIN collection_batches b ON b.service_id=bj.service_id AND b.id=bj.batch_id JOIN collection_schedules s ON s.service_id=b.service_id WHERE bj.job_id=jobs.id AND bj.service_id=jobs.service_id AND b.slot LIKE 'schedule:%' AND s.paused)
ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&j.ID, &j.ServiceID, &j.ActorID, &j.TokenHash, &j.RequirementID, &j.CheckID, &raw, &j.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		if commitErr := commit(); commitErr != nil {
			return j, commitErr
		}
		return j, err
	}
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal(raw, &j.Input); err != nil {
		return j, err
	}
	recovered, err := tx.Exec(ctx, `UPDATE job_attempts SET finished_at=now(),outcome='lease_expired' WHERE job_id=$1 AND finished_at IS NULL`, j.ID)
	if err != nil {
		return j, err
	}
	j.Attempts++
	j.LeaseID = uuid.NewString()
	j.State = "running"
	if _, err = tx.Exec(ctx, `UPDATE jobs SET state='running',attempts=$2,lease_id=$3,lease_until=now()+interval '90 seconds',updated_at=now() WHERE id=$1`, j.ID, j.Attempts, j.LeaseID); err != nil {
		return j, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_attempts(job_id,attempt,lease_id) VALUES($1,$2,$3)`, j.ID, j.Attempts, j.LeaseID); err != nil {
		return j, err
	}
	err = commit()
	if err == nil {
		if recovered.RowsAffected() > 0 {
			logging.Emit(ctx, zerolog.WarnLevel, jobFields(j, "lease_recovered", "retry"))
		}
		logging.Emit(ctx, zerolog.InfoLevel, jobFields(j, "claimed", "success"))
	}
	return j, err
}

func (r *Runner) RunOnce(ctx context.Context) (bool, error) {
	ctx = logging.Operation(ctx)
	j, err := r.claim(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		logging.Emit(ctx, zerolog.DebugLevel, logging.Fields{Event: "collection.queue.idle", Component: "worker"})
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = r.DB.QueryRow(ctx, `SELECT batch_id FROM collection_batch_jobs WHERE service_id=$1 AND job_id=$2 LIMIT 1`, j.ServiceID, j.ID).Scan(&j.BatchID)
	ctx = logging.Identity(ctx, j.ActorID, j.ServiceID)
	logging.Emit(ctx, zerolog.InfoLevel, jobFields(j, "attempt.started", "running"))
	started := time.Now()
	a := auth.Actor{ID: j.ActorID}
	if j.TokenHash != nil {
		a.TokenHash = *j.TokenHash
	}
	err = r.DB.QueryRow(ctx, `SELECT organization_id,kind FROM identities WHERE id=$1`, a.ID).Scan(&a.OrganizationID, &a.Kind)
	if err == nil {
		err = auth.Authorize(ctx, r.DB, a, j.ServiceID, "check")
	}
	var result Result
	var connectionAccess []connections.CollectionAccess
	if err == nil {
		var scope assessment.Scope
		var measure assessment.Measure
		scope, measure, err = loadInput(ctx, r.DB, j.ServiceID, j.RequirementID, j.CheckID, j.Input, true)
		if err == nil {
			collector, ok := r.Registry[j.CheckID]
			if !ok {
				err = assessment.ErrInvalid
			} else {
				deadline, cancel := context.WithTimeout(ctx, 60*time.Second)
				result, connectionAccess, err = r.collectConnected(deadline, a, j.ServiceID, collector, scope, measure)
				fields := jobFields(j, "collector.completed", "success")
				fields.Duration = time.Since(started)
				fields.ErrorCode = logging.Code(err)
				level := zerolog.DebugLevel
				if err != nil {
					level = zerolog.WarnLevel
					fields.Outcome = "failed"
				}
				logging.Emit(ctx, level, fields)
				cancel()
			}
		}
	}
	if err != nil {
		finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return true, r.retry(finish, j, err)
	}
	result.CollectorVersion = "1"
	result.Summarize()
	if result.Observed < result.Expected {
		fields := jobFields(j, "coverage", "partial")
		fields.Count = result.Observed
		logging.Emit(ctx, zerolog.WarnLevel, fields)
	}
	if r.Artifacts == nil {
		return true, r.retry(ctx, j, errors.New("artifact storage unavailable"))
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return true, err
	}
	key, err := r.Artifacts.Put(ctx, raw)
	if err != nil {
		return true, r.retry(ctx, j, err)
	}
	if err = r.finish(ctx, a, j, result, key, connectionAccess...); err != nil {
		if errors.Is(err, assessment.ErrConflict) {
			return true, nil
		}
		finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return true, r.retry(finish, j, err)
	}
	return true, nil
}

func (r *Runner) retry(ctx context.Context, j Job, cause error) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	state := "queued"
	reason := "collection_error"
	if errors.Is(cause, auth.ErrForbidden) || errors.Is(cause, assessment.ErrInvalid) || errors.Is(cause, assessment.ErrConflict) {
		state = "failed"
		reason = "scope_or_permission_changed"
	}
	tag, err := tx.Exec(ctx, `UPDATE jobs SET state=CASE WHEN deadline<=now() THEN 'timed-out' WHEN attempts>=max_attempts THEN 'failed' ELSE $3 END,lease_until=NULL,available_at=now()+make_interval(secs=>least(60,attempts*attempts)),updated_at=now() WHERE id=$1 AND lease_id=$2 AND state='running'`, j.ID, j.LeaseID, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return assessment.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE job_attempts SET finished_at=now(),outcome=$3 WHERE job_id=$1 AND attempt=$2`, j.ID, j.Attempts, reason); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT state FROM jobs WHERE id=$1`, j.ID).Scan(&state); err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err == nil {
		event, level := "retry_scheduled", zerolog.WarnLevel
		if state != "queued" {
			event = "failed"
			level = zerolog.ErrorLevel
		}
		fields := jobFields(j, event, state)
		fields.ErrorCode = reason
		logging.Emit(ctx, level, fields)
	}
	return err
}

func (r *Runner) finish(ctx context.Context, a auth.Actor, j Job, result Result, key string, accesses ...connections.CollectionAccess) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, j.ServiceID, "check"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, j.ServiceID); err != nil {
		return err
	}
	var valid bool
	for _, access := range accesses {
		if err = connections.CheckCollectionTx(ctx, tx, j.ServiceID, access); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(ctx, `SELECT lease_id=$2 AND lease_until>now() AND deadline>now() AND state='running' FROM jobs WHERE id=$1 FOR UPDATE`, j.ID, j.LeaseID).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return assessment.ErrConflict
	}
	now := time.Now().UTC()
	var b []byte
	if j.Input.ProfileRevision > 0 && len(j.Input.Bindings) > 0 {
		result.EvidenceIDs = []string{}
		for index, o := range result.Observations {
			in := observation.Submission{ServiceID: j.ServiceID, ScopeRevision: j.Input.ScopeRevision, ProfileRevision: j.Input.ProfileRevision, IdempotencyKey: j.ID + ":" + strconv.Itoa(index), ObservedAt: now, Method: j.CheckID, ToolVersion: result.CollectorVersion, Limitations: result.Limitations, Evaluations: o.Evaluations, Coverage: observation.Coverage{Expected: 1, Observed: 1, Complete: o.Error == "", State: "observed", Error: o.Error}}
			if o.Error != "" {
				recovery := observation.ExplainError(o.Error, o.Scope, "server collector")
				in.Coverage.Recovery = &recovery
			}
			if o.Error != "" {
				in.Coverage.Observed = 0
				in.Coverage.State = "error"
			}
			if len(in.Evaluations) == 0 {
				in.Evaluations = []observation.Evaluation{{Condition: "collector_result", Outcome: o.Outcome, Explanation: "Registered collector result; inspect artifact for coverage and limitations."}}
			}
			if inventory, ok := o.Data.(map[string]string); ok && inventory["inventory"] == "no_resources" {
				in.Coverage = observation.Coverage{Complete: true, State: "no_resources"}
			}
			if accessObservation(o) {
				in.Coverage = observation.Coverage{Complete: true, State: "observed"}
				in.Evaluations = []observation.Evaluation{{Condition: "collection_access", Outcome: "unknown", Explanation: o.Data.(map[string]string)["explanation"]}}
			}
			if o.Target != nil {
				in.Target = *o.Target
			} else {
				in.Target = observation.Target{Kind: "collector_partition", ResourceID: o.Scope}
			}
			if artifact, ok := o.Data.(observation.Artifact); ok {
				in.Artifact = artifact
			} else {
				in.Artifact = observation.Artifact{Schema: j.CheckID + "/v1"}
			}
			for _, binding := range j.Input.Bindings {
				in.Links = append(in.Links, observation.Link{RequirementID: binding.RequirementID, Measure: observation.Ref{ID: binding.Measure.ID, Revision: binding.Measure.Revision}, MappingVersion: assessment.TemplateVersion, Rationale: binding.Rationale, Conditions: binding.Conditions})
			}
			receipt, err := assessment.StageCollected(ctx, tx, a, j.ID, j.LeaseID, key, in)
			if err != nil {
				return err
			}
			for _, ref := range receipt.Evidence {
				result.EvidenceIDs = append(result.EvidenceIDs, ref.ID)
			}
		}
	} else {
		id := uuid.NewString()
		e := assessment.Evidence{Source: "collector", Description: j.CheckID + " observations", ScopeRevision: j.Input.ScopeRevision, ObservedAt: now, CollectedAt: now, Verification: "collector_observation", ArtifactKey: key, Collector: j.CheckID + "@1", Limitations: result.Limitations}
		b, _ = json.Marshal(e)
		if _, err = tx.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'evidence',$2,1,$3,$4,$5)`, j.ServiceID, id, j.RequirementID, a.ID, b); err != nil {
			return err
		}
		result.EvidenceIDs = []string{id}
	}
	b, _ = json.Marshal(result)
	if _, err = tx.Exec(ctx, `UPDATE jobs SET state='succeeded',result=$2,lease_until=NULL,updated_at=now() WHERE id=$1`, j.ID, b); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE job_attempts SET finished_at=now(),outcome='succeeded' WHERE job_id=$1 AND attempt=$2`, j.ID, j.Attempts); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target) VALUES($1,$2,$3,'check.completed',$4)`, a.OrganizationID, j.ServiceID, a.ID, j.ID); err != nil {
		return err
	}
	return logging.Commit(ctx, tx, jobFields(j, "completed", "success"))
}

func jobFields(j Job, event, outcome string) logging.Fields {
	return logging.Fields{Event: "collection.job." + event, Component: "worker", JobID: j.ID, BatchID: j.BatchID, ServiceID: j.ServiceID, ActorID: j.ActorID, Attempt: j.Attempts, Outcome: outcome, Stage: j.CheckID}
}

func (r *Runner) Run(ctx context.Context) (runErr error) {
	logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "runtime.worker.starting", Component: "worker"})
	defer func() {
		level, outcome := zerolog.InfoLevel, "stopped"
		if runErr != nil {
			level = zerolog.ErrorLevel
			outcome = "failed"
		}
		logging.Emit(ctx, level, logging.Fields{Event: "runtime.worker.stopped", Component: "worker", Outcome: outcome, ErrorCode: logging.Code(runErr)})
	}()
	if _, err := r.DB.Exec(ctx, `INSERT INTO collection_outbox(service_id) SELECT id FROM services ON CONFLICT(service_id) DO UPDATE SET generation=collection_outbox.generation+1,updated_at=now()`); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "runtime.worker.ready", Component: "worker"})
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err := r.Schedule(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		worked, err := r.RunOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !worked {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	}
}
