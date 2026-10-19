CREATE TABLE readiness_work (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    actor_id text NOT NULL REFERENCES identities(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    binding jsonb NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY(service_id,id,revision)
);
CREATE TABLE readiness_work_requests (
    service_id text NOT NULL REFERENCES services(id),
    actor_id text NOT NULL REFERENCES identities(id),
    idempotency_key text NOT NULL,
    request_hash text NOT NULL,
    response jsonb NOT NULL,
    PRIMARY KEY(service_id,actor_id,idempotency_key)
);
CREATE TRIGGER readiness_work_immutable BEFORE UPDATE OR DELETE ON readiness_work
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
CREATE TRIGGER readiness_work_requests_immutable BEFORE UPDATE OR DELETE ON readiness_work_requests
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
