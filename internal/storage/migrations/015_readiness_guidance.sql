-- Content approval is separate from service assessment review. A release never
-- updates a previous package or review, and is scoped to its reviewing tenant.
CREATE TABLE readiness_content_reviews (
    organization_id text NOT NULL REFERENCES organizations(id),
    catalog_id text NOT NULL REFERENCES catalogs(id),
    playbook_id text NOT NULL,
    content_hash text NOT NULL,
    reviewer_id text NOT NULL REFERENCES identities(id),
    reviewed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, catalog_id, playbook_id, content_hash)
);

CREATE TABLE readiness_guidance_releases (
    organization_id text NOT NULL REFERENCES organizations(id),
    catalog_id text NOT NULL REFERENCES catalogs(id),
    revision integer NOT NULL CHECK (revision > 0),
    content_hash text NOT NULL,
    payload jsonb NOT NULL,
    publisher_id text NOT NULL REFERENCES identities(id),
    published_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, catalog_id, revision)
);

CREATE FUNCTION readiness_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'readiness guidance history is immutable';
END;
$$;
CREATE TRIGGER readiness_content_reviews_immutable
    BEFORE UPDATE OR DELETE ON readiness_content_reviews
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
CREATE TRIGGER readiness_guidance_releases_immutable
    BEFORE UPDATE OR DELETE ON readiness_guidance_releases
    FOR EACH ROW EXECUTE FUNCTION readiness_immutable();
