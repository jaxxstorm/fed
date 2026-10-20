-- New documents reuse the immutable, revisioned domain-record primitive.
-- Existing rows retain their original meaning; no approval is backfilled.
ALTER TABLE records DROP CONSTRAINT records_kind_check;
ALTER TABLE records ADD CONSTRAINT records_kind_check CHECK (kind IN (
    'scope','profile','measure','evidence','assessment','review','gap',
    'destination','snapshot','publication','evidence_source','rule_result',
    'evidence_request','assessment_issue','assessment_decision','service_context',
    'assessment_release','assessment_release_event','reporting_cycle',
    'assessor_designation'
));

CREATE INDEX evidence_request_queue ON records(service_id,requirement_id,id,revision DESC)
    WHERE kind='evidence_request';
CREATE INDEX assessment_issue_queue ON records(service_id,requirement_id,id,revision DESC)
    WHERE kind='assessment_issue';

-- Each decision records exact immutable dependencies, not a mutable current ID.
CREATE TABLE assessment_review_dependencies (
    service_id text NOT NULL,
    decision_kind text NOT NULL DEFAULT 'assessment_decision' CHECK(decision_kind='assessment_decision'),
    decision_id text NOT NULL,
    decision_revision integer NOT NULL,
    dependency_kind text NOT NULL,
    dependency_id text NOT NULL,
    dependency_revision integer NOT NULL,
    PRIMARY KEY(service_id,decision_id,decision_revision,dependency_kind,dependency_id,dependency_revision),
    FOREIGN KEY(service_id,decision_kind,decision_id,decision_revision)
        REFERENCES records(service_id,kind,id,revision),
    FOREIGN KEY(service_id,dependency_kind,dependency_id,dependency_revision)
        REFERENCES records(service_id,kind,id,revision)
);
CREATE INDEX assessment_dependency_reviews ON assessment_review_dependencies
    (service_id,dependency_kind,dependency_id,dependency_revision);

-- A manifest is frozen as revision 1. Lifecycle changes are separate events.
CREATE TABLE assessment_releases (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    manifest_kind text NOT NULL DEFAULT 'assessment_release' CHECK(manifest_kind='assessment_release'),
    manifest_revision integer NOT NULL DEFAULT 1 CHECK(manifest_revision=1),
    manifest_hash text NOT NULL CHECK(manifest_hash ~ '^[a-f0-9]{64}$'),
    audience_hash text NOT NULL CHECK(audience_hash ~ '^[a-f0-9]{64}$'),
    archive_key text NOT NULL,
    state text NOT NULL CHECK(state IN ('prepared','approved','published','withdrawn')),
    revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
    prepared_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    withdrawn_at timestamptz,
    receipt jsonb,
    PRIMARY KEY(service_id,id),
    FOREIGN KEY(service_id,manifest_kind,id,manifest_revision)
        REFERENCES records(service_id,kind,id,revision),
    CHECK(state NOT IN ('published','withdrawn') OR (published_at IS NOT NULL AND receipt IS NOT NULL)),
    CHECK(state<>'withdrawn' OR withdrawn_at IS NOT NULL)
);
CREATE INDEX assessment_published_releases ON assessment_releases(service_id,published_at DESC,id)
    WHERE state='published';

CREATE TABLE trust_centres (
    service_id text PRIMARY KEY REFERENCES services(id),
    slug text NOT NULL UNIQUE CHECK(slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    enabled boolean NOT NULL DEFAULT false,
    latest_release_id text,
    FOREIGN KEY(service_id,latest_release_id) REFERENCES assessment_releases(service_id,id)
);

-- External readers are not organization members. All access paths must check
-- these grants separately, including expiry/revocation and release membership.
CREATE TABLE trust_access_requests (
    id text PRIMARY KEY,
    service_id text NOT NULL REFERENCES services(id),
    identity_id text NOT NULL REFERENCES identities(id),
    release_id text,
    state text NOT NULL CHECK(state IN ('pending','approved','denied','cancelled')),
    requested_at timestamptz NOT NULL DEFAULT now(),
    decided_by text REFERENCES identities(id),
    decided_at timestamptz,
    FOREIGN KEY(service_id,release_id) REFERENCES assessment_releases(service_id,id)
);
CREATE INDEX trust_pending_access ON trust_access_requests(service_id,requested_at,id) WHERE state='pending';
CREATE TABLE trust_access_grants (
    id text PRIMARY KEY,
    service_id text NOT NULL REFERENCES services(id),
    identity_id text NOT NULL REFERENCES identities(id),
    release_id text,
    capacity text NOT NULL CHECK(capacity IN ('reader','assessor')),
    granted_by text NOT NULL REFERENCES identities(id),
    granted_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    FOREIGN KEY(service_id,release_id) REFERENCES assessment_releases(service_id,id),
    CHECK(expires_at>granted_at)
);
CREATE INDEX trust_identity_grants ON trust_access_grants(identity_id,service_id,release_id,expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE assessment_reporting_schedules (
    service_id text PRIMARY KEY REFERENCES services(id),
    revision integer NOT NULL CHECK(revision>0),
    enabled boolean NOT NULL DEFAULT false,
    owner_id text NOT NULL REFERENCES identities(id),
    interval_hours integer NOT NULL CHECK(interval_hours>0),
    next_period_start timestamptz NOT NULL,
    next_run_at timestamptz NOT NULL,
    configuration jsonb NOT NULL
);
CREATE INDEX assessment_reporting_due ON assessment_reporting_schedules(next_run_at,service_id) WHERE enabled;
CREATE TABLE assessment_reporting_runs (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    period_start timestamptz NOT NULL,
    period_end timestamptz NOT NULL,
    schedule_revision integer NOT NULL CHECK(schedule_revision>0),
    state text NOT NULL CHECK(state IN ('queued','running','succeeded','failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
    lease_id text,
    lease_until timestamptz,
    candidate_id text,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(service_id,id),
    UNIQUE(service_id,period_start,period_end,schedule_revision),
    CHECK(period_end>=period_start)
);
CREATE INDEX assessment_reporting_work ON assessment_reporting_runs(state,lease_until,created_at);
