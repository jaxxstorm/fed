ALTER TABLE assessment_reporting_runs ADD COLUMN owner_id text REFERENCES identities(id);
ALTER TABLE assessment_reporting_runs ADD COLUMN configuration jsonb NOT NULL DEFAULT '{}';
ALTER TABLE assessment_reporting_runs ADD COLUMN retry_at timestamptz NOT NULL DEFAULT now();
CREATE TABLE assessment_reporting_notifications (
 service_id text NOT NULL,
 run_id text NOT NULL,
 owner_id text NOT NULL REFERENCES identities(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 message text NOT NULL,
 PRIMARY KEY(service_id,run_id),
 FOREIGN KEY(service_id,run_id) REFERENCES assessment_reporting_runs(service_id,id)
);
