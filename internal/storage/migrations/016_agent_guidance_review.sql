-- Existing human-admin reviews/releases stay immutable and historical. New
-- agent-review records are separately scoped to the service grant that created
-- them; a grant in one service cannot publish guidance for its neighbors.
CREATE TABLE readiness_agent_reviews (
    organization_id text NOT NULL REFERENCES organizations(id),
    service_id text NOT NULL REFERENCES services(id),
    catalog_id text NOT NULL REFERENCES catalogs(id),
    playbook_id text NOT NULL,
    content_hash text NOT NULL,
    reviewer_id text NOT NULL REFERENCES identities(id),
    reviewer_type text NOT NULL CHECK (reviewer_type IN ('agent','human')),
    comparison jsonb NOT NULL,
    reviewed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, service_id, catalog_id, playbook_id, content_hash)
);
CREATE TABLE readiness_service_releases (
    organization_id text NOT NULL REFERENCES organizations(id),
    service_id text NOT NULL REFERENCES services(id),
    catalog_id text NOT NULL REFERENCES catalogs(id),
    revision integer NOT NULL CHECK (revision > 0),
    content_hash text NOT NULL,
    payload jsonb NOT NULL,
    publisher_id text NOT NULL REFERENCES identities(id),
    published_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, service_id, catalog_id, revision)
);
CREATE TRIGGER readiness_agent_reviews_immutable
    BEFORE UPDATE OR DELETE ON readiness_agent_reviews
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
CREATE TRIGGER readiness_service_releases_immutable
    BEFORE UPDATE OR DELETE ON readiness_service_releases
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
