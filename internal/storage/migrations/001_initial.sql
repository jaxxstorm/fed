CREATE TABLE organizations (
    id text PRIMARY KEY,
    name text NOT NULL
);
CREATE TABLE identities (
    id text PRIMARY KEY,
    organization_id text NOT NULL REFERENCES organizations(id),
    kind text NOT NULL CHECK (kind IN ('human', 'agent')),
    issuer text NOT NULL DEFAULT '',
    subject text NOT NULL,
    name text NOT NULL,
    UNIQUE (issuer, subject)
);
CREATE TABLE services (
    id text PRIMARY KEY,
    organization_id text NOT NULL REFERENCES organizations(id),
    name text NOT NULL,
    revision integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE permissions (
    identity_id text NOT NULL REFERENCES identities(id),
    service_id text NOT NULL,
    action text NOT NULL CHECK (action IN ('read','edit','review','admin','check','export')),
    PRIMARY KEY (identity_id, service_id, action)
);
CREATE TABLE api_tokens (
    hash text PRIMARY KEY,
    identity_id text NOT NULL REFERENCES identities(id),
    service_ids text[] NOT NULL,
    actions text[] NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
    hash text PRIMARY KEY,
    identity_id text NOT NULL REFERENCES identities(id),
    csrf_token text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE login_attempts (
    state_hash text PRIMARY KEY,
    nonce text NOT NULL,
    verifier text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE catalogs (
    id text PRIMARY KEY,
    commit_sha text NOT NULL,
    source_url text NOT NULL,
    payload_hash text NOT NULL,
    schema_hash text NOT NULL,
    payload bytea NOT NULL,
    schema_payload bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE requirements (
    catalog_id text NOT NULL REFERENCES catalogs(id),
    id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('KSI','FRR')),
    data jsonb NOT NULL,
    PRIMARY KEY (catalog_id,id)
);
CREATE TABLE catalog_activation (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    catalog_id text NOT NULL REFERENCES catalogs(id)
);
-- Immutable versioned domain documents share one persistence primitive. Kind
-- and payload validation are enforced by domain services, never HTTP callers.
CREATE TABLE records (
    service_id text NOT NULL REFERENCES services(id),
    kind text NOT NULL CHECK (kind IN ('scope','profile','measure','evidence','assessment','review','gap','destination','snapshot','publication')),
    id text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    requirement_id text NOT NULL DEFAULT '',
    actor_id text NOT NULL REFERENCES identities(id),
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,kind,id,revision)
);
CREATE INDEX records_lookup ON records(service_id,kind,requirement_id,id,revision DESC);
CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id text NOT NULL REFERENCES organizations(id),
    service_id text,
    actor_id text NOT NULL REFERENCES identities(id),
    action text NOT NULL,
    target text NOT NULL,
    revision integer,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE idempotency (
    actor_id text NOT NULL REFERENCES identities(id),
    service_id text NOT NULL REFERENCES services(id),
    operation text NOT NULL,
    key text NOT NULL,
    request_hash text NOT NULL,
    response jsonb NOT NULL,
    PRIMARY KEY (actor_id,service_id,operation,key)
);
CREATE TABLE jobs (
    id text PRIMARY KEY,
    service_id text NOT NULL REFERENCES services(id),
    actor_id text NOT NULL REFERENCES identities(id),
    token_hash text REFERENCES api_tokens(hash),
    requirement_id text NOT NULL,
    check_id text NOT NULL,
    input jsonb NOT NULL,
    state text NOT NULL CHECK (state IN ('queued','running','succeeded','failed','timed-out')),
    attempts integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    lease_id text,
    lease_until timestamptz,
    deadline timestamptz NOT NULL,
    result jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_claim ON jobs(state,lease_until,created_at);
CREATE TABLE job_attempts (
    job_id text NOT NULL REFERENCES jobs(id),
    attempt integer NOT NULL,
    lease_id text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    outcome text,
    PRIMARY KEY (job_id,attempt)
);
