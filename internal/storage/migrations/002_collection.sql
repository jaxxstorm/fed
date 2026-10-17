-- Additive collection state; existing record revisions remain authoritative.
ALTER TABLE identities DROP CONSTRAINT identities_kind_check;
ALTER TABLE identities ADD CONSTRAINT identities_kind_check CHECK(kind IN ('human','agent','automation'));
CREATE TABLE collection_outbox (
    service_id text PRIMARY KEY REFERENCES services(id),
    generation bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE collection_plans (
    service_id text NOT NULL REFERENCES services(id),
    revision integer NOT NULL CHECK(revision>0),
    scope_revision integer NOT NULL CHECK(scope_revision>0),
    profile_revision integer NOT NULL CHECK(profile_revision>0),
    template_version text NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(service_id,revision),
    UNIQUE(service_id,scope_revision,profile_revision,template_version)
);
CREATE TABLE collection_entries (
    service_id text NOT NULL,
    plan_revision integer NOT NULL,
    id text NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY(service_id,plan_revision,id),
    FOREIGN KEY(service_id,plan_revision) REFERENCES collection_plans(service_id,revision)
);
CREATE TABLE managed_measures (
    service_id text NOT NULL REFERENCES services(id),
    measure_id text NOT NULL,
    template_id text NOT NULL,
    template_version text NOT NULL,
    overridden boolean NOT NULL DEFAULT false,
    PRIMARY KEY(service_id,measure_id)
);
CREATE TABLE collection_schedules (
    service_id text PRIMARY KEY REFERENCES services(id),
    actor_id text NOT NULL UNIQUE REFERENCES identities(id),
    revision integer NOT NULL DEFAULT 1,
    paused boolean NOT NULL DEFAULT false,
    cadence_hours integer NOT NULL DEFAULT 24 CHECK(cadence_hours BETWEEN 1 AND 8760),
    freshness_hours integer NOT NULL DEFAULT 48 CHECK(freshness_hours BETWEEN 1 AND 8760),
    overrides jsonb NOT NULL DEFAULT '{}',
    next_run_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE collection_batches (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    plan_revision integer NOT NULL,
    actor_id text NOT NULL REFERENCES identities(id),
    slot text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(service_id,id),
    UNIQUE(service_id,slot),
    FOREIGN KEY(service_id,plan_revision) REFERENCES collection_plans(service_id,revision)
);
ALTER TABLE jobs ADD CONSTRAINT jobs_service_id_id_unique UNIQUE(service_id,id);
CREATE TABLE collection_batch_jobs (
    service_id text NOT NULL,
    batch_id text NOT NULL,
    job_id text NOT NULL,
    PRIMARY KEY(service_id,batch_id,job_id),
    FOREIGN KEY(service_id,batch_id) REFERENCES collection_batches(service_id,id),
    FOREIGN KEY(service_id,job_id) REFERENCES jobs(service_id,id)
);
CREATE TABLE observations (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    scope_revision integer NOT NULL CHECK(scope_revision>0),
    profile_revision integer NOT NULL CHECK(profile_revision>0),
    actor_id text NOT NULL REFERENCES identities(id),
    source_kind text NOT NULL CHECK(source_kind IN ('agent_observation','collector_observation')),
    artifact_hash text NOT NULL CHECK(artifact_hash ~ '^[a-f0-9]{64}$'),
    observed_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    target_key text NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY(service_id,id)
);
CREATE INDEX observations_current ON observations(service_id,scope_revision,profile_revision,observed_at DESC);
CREATE TABLE observation_links (
    service_id text NOT NULL,
    observation_id text NOT NULL,
    requirement_id text NOT NULL,
    measure_id text NOT NULL,
    measure_revision integer NOT NULL,
    evidence_id text NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY(service_id,observation_id,requirement_id,measure_id),
    FOREIGN KEY(service_id,observation_id) REFERENCES observations(service_id,id)
);
CREATE INDEX observation_requirement ON observation_links(service_id,requirement_id,measure_id);
CREATE TABLE observation_ingestions (
    service_id text NOT NULL,
    actor_id text NOT NULL REFERENCES identities(id),
    operation text NOT NULL,
    key text NOT NULL,
    request_hash text NOT NULL,
    observation_id text NOT NULL,
    response jsonb NOT NULL,
    PRIMARY KEY(service_id,actor_id,operation,key),
    FOREIGN KEY(service_id,observation_id) REFERENCES observations(service_id,id)
);
CREATE TABLE finding_events (
    service_id text NOT NULL,
    finding_id text NOT NULL,
    observation_id text NOT NULL,
    condition text NOT NULL,
    outcome text NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(service_id,finding_id,observation_id),
    FOREIGN KEY(service_id,observation_id) REFERENCES observations(service_id,id)
);
CREATE INDEX finding_latest ON finding_events(service_id,finding_id,created_at DESC);
