ALTER TABLE jobs ADD COLUMN available_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX jobs_available ON jobs(state,available_at,created_at);
