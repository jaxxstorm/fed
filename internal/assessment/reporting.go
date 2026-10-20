package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type ReportingSchedule struct {
	Revision        int                 `json:"revision"`
	Enabled         bool                `json:"enabled"`
	IntervalHours   int                 `json:"interval_hours"`
	NextPeriodStart time.Time           `json:"next_period_start"`
	NextRunAt       time.Time           `json:"next_run_at"`
	Configuration   PrepareReleaseInput `json:"configuration"`
}

type ReportingRun struct {
	ID           string           `json:"id"`
	Period       AssessmentPeriod `json:"period"`
	State        string           `json:"state"`
	CandidateID  string           `json:"candidate_id"`
	Attempts     int              `json:"attempts"`
	Error        string           `json:"error"`
	Notification string           `json:"notification"`
}

func (s *Store) RunReporting(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := s.RunReportingOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Store) SaveReportingSchedule(ctx context.Context, a auth.Actor, service string, expected int, v ReportingSchedule) error {
	if a.Kind != "human" || a.TokenHash != "" {
		return auth.ErrForbidden
	}
	if v.IntervalHours < 1 || v.IntervalHours > 24*366 || v.NextPeriodStart.IsZero() || !v.NextRunAt.After(v.NextPeriodStart) || v.Configuration.Public.ServiceName == "" {
		return ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "export"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return err
	}
	var revision int
	err = tx.QueryRow(ctx, `SELECT revision FROM assessment_reporting_schedules WHERE service_id=$1`, service).Scan(&revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if expected != revision {
		return ErrConflict
	}
	v.Configuration.ServiceID = service
	v.Configuration.IdempotencyKey = ""
	_, err = tx.Exec(ctx, `INSERT INTO assessment_reporting_schedules(service_id,revision,enabled,owner_id,interval_hours,next_period_start,next_run_at,configuration) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(service_id) DO UPDATE SET revision=excluded.revision,enabled=excluded.enabled,owner_id=excluded.owner_id,interval_hours=excluded.interval_hours,next_period_start=excluded.next_period_start,next_run_at=excluded.next_run_at,configuration=excluded.configuration`, service, revision+1, v.Enabled, a.ID, v.IntervalHours, v.NextPeriodStart, v.NextRunAt, data(v.Configuration))
	if err != nil {
		return err
	}
	return commitOperation(ctx, tx, a, service, "assessment.reporting_configured", Record{ID: "schedule", Revision: revision + 1})
}

func (s *Store) QueueReportingCandidate(ctx context.Context, a auth.Actor, in PrepareReleaseInput) (string, error) {
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || in.Period.Start.IsZero() || !in.Period.End.After(in.Period.Start) || in.Period.End.After(s.now()) || in.Public.ServiceName == "" {
		return "", ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "export"); err != nil {
		return "", err
	}
	if a.Kind != "human" || a.TokenHash != "" {
		return "", auth.ErrForbidden
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return "", err
	}
	digest := storage.Hash(data(in))
	var saved struct {
		ID string `json:"id"`
	}
	if found, e := readOperation(ctx, tx, a, in.ServiceID, "queue_reporting_candidate", in.IdempotencyKey, digest, &saved); found || e != nil {
		return saved.ID, e
	}
	operationKey := in.IdempotencyKey
	saved.ID = uuid.NewString()
	in.IdempotencyKey = "reporting/" + saved.ID
	// Ad-hoc runs use a reserved revision distinct from configured schedules.
	inserted, err := tx.Exec(ctx, `INSERT INTO assessment_reporting_runs(service_id,id,period_start,period_end,schedule_revision,state,owner_id,configuration) VALUES($1,$2,$3,$4,2147483647,'queued',$5,$6) ON CONFLICT(service_id,period_start,period_end,schedule_revision) DO NOTHING`, in.ServiceID, saved.ID, in.Period.Start, in.Period.End, a.ID, data(in))
	if err != nil {
		return "", err
	}
	if inserted.RowsAffected() != 1 {
		return "", ErrConflict
	}
	if err = saveOperation(ctx, tx, a, in.ServiceID, "queue_reporting_candidate", operationKey, digest, saved); err != nil {
		return "", err
	}
	return saved.ID, commitOperation(ctx, tx, a, in.ServiceID, "assessment.reporting_queued", Record{ID: saved.ID})
}

func (s *Store) ReportingRuns(ctx context.Context, a auth.Actor, service string, limit, offset int) ([]ReportingRun, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT r.id,r.period_start,r.period_end,r.state,coalesce(r.candidate_id,''),r.attempts,r.error,coalesce(n.message,'') FROM assessment_reporting_runs r LEFT JOIN assessment_reporting_notifications n ON n.service_id=r.service_id AND n.run_id=r.id WHERE r.service_id=$1 ORDER BY r.created_at DESC,r.id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReportingRun{}
	for rows.Next() {
		var v ReportingRun
		if err = rows.Scan(&v.ID, &v.Period.Start, &v.Period.End, &v.State, &v.CandidateID, &v.Attempts, &v.Error, &v.Notification); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// RunReportingOnce bounds each pass to one due schedule and one leased job.
// It prepares candidates only; no agent invocation or publication occurs here.
func (s *Store) RunReportingOnce(ctx context.Context) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var service, owner string
	var revision, hours int
	var start, end time.Time
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT service_id,owner_id,revision,interval_hours,next_period_start,next_run_at,configuration FROM assessment_reporting_schedules WHERE enabled AND next_run_at<=now() ORDER BY next_run_at,service_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&service, &owner, &revision, &hours, &start, &end, &raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		var in PrepareReleaseInput
		if err = json.Unmarshal(raw, &in); err != nil {
			return err
		}
		id := uuid.NewString()
		in.ServiceID = service
		in.Period = AssessmentPeriod{Start: start, End: end}
		in.IdempotencyKey = "reporting/" + id
		if _, err = tx.Exec(ctx, `INSERT INTO assessment_reporting_runs(service_id,id,period_start,period_end,schedule_revision,state,owner_id,configuration) VALUES($1,$2,$3,$4,$5,'queued',$6,$7) ON CONFLICT(service_id,period_start,period_end,schedule_revision) DO NOTHING`, service, id, start, end, revision, owner, data(in)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE assessment_reporting_schedules SET next_period_start=$2,next_run_at=$3 WHERE service_id=$1`, service, end, end.Add(time.Duration(hours)*time.Hour)); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	lease := uuid.NewString()
	var id string
	err = s.DB.QueryRow(ctx, `UPDATE assessment_reporting_runs SET state='running',attempts=attempts+1,lease_id=$1,lease_until=now()+interval '10 minutes' WHERE (service_id,id)=(SELECT service_id,id FROM assessment_reporting_runs WHERE owner_id IS NOT NULL AND ((state IN ('queued','failed') AND retry_at<=now() AND attempts<3) OR (state='running' AND lease_until<now())) ORDER BY created_at,service_id,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING service_id,id,owner_id,configuration`, lease).Scan(&service, &id, &owner, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var in PrepareReleaseInput
	if err = json.Unmarshal(raw, &in); err != nil {
		return err
	}
	a := auth.Actor{ID: owner, Kind: "human"}
	if err = s.DB.QueryRow(ctx, `SELECT organization_id FROM identities WHERE id=$1 AND kind='human'`, owner).Scan(&a.OrganizationID); err != nil {
		return err
	}
	jobCtx, cancel := context.WithTimeout(ctx, 9*time.Minute)
	defer cancel()
	release, prepareErr := s.PrepareAssessmentRelease(jobCtx, a, in)
	state, message, diagnostic := "succeeded", "Assessment candidate prepared. Review current freshness, evidence requests and changed review inputs before approval.", ""
	if prepareErr != nil {
		state = "failed"
		message = "Assessment candidate preparation failed. Check service context, evidence availability and owner permissions. Automatic retries are bounded to three attempts."
		diagnostic = "candidate_preparation_failed"
	}
	finish, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer finish.Rollback(ctx)
	result, err := finish.Exec(ctx, `UPDATE assessment_reporting_runs SET state=$4,candidate_id=nullif($5,''),error=$6,retry_at=now()+interval '5 minutes',lease_id=NULL,lease_until=NULL WHERE service_id=$1 AND id=$2 AND lease_id=$3 AND lease_until>now()`, service, id, lease, state, release.ID, diagnostic)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	if _, err = finish.Exec(ctx, `INSERT INTO assessment_reporting_notifications(service_id,run_id,owner_id,message) VALUES($1,$2,$3,$4) ON CONFLICT(service_id,run_id) DO UPDATE SET message=excluded.message,created_at=now()`, service, id, owner, message); err != nil {
		return err
	}
	return finish.Commit(ctx)
}
