-- Fail before changing authorization data when legacy grants cannot be mapped
-- without widening access. All migrations execute in one locked transaction.
DO $$
DECLARE occupied integer; total integer; ambiguous text;
BEGIN
    SELECT count(*) INTO occupied FROM organizations o WHERE
        EXISTS(SELECT 1 FROM identities WHERE organization_id=o.id) OR
        EXISTS(SELECT 1 FROM services WHERE organization_id=o.id);
    SELECT count(*) INTO total FROM organizations;
    IF occupied > 1 OR (occupied=0 AND total>1) THEN
        RAISE EXCEPTION 'organization migration requires one unambiguous organization';
    END IF;
    SELECT string_agg(i.id, ',' ORDER BY i.id) INTO ambiguous FROM identities i
    WHERE i.kind='human' AND NOT EXISTS(
        SELECT 1 FROM permissions p WHERE p.identity_id=i.id AND p.service_id='*' AND p.action='admin');
    IF ambiguous IS NOT NULL THEN
        RAISE EXCEPTION 'organization migration requires explicit human role mapping for identity IDs: %', ambiguous;
    END IF;
END $$;

INSERT INTO organizations(id,name)
SELECT 'default','Default organization' WHERE NOT EXISTS(SELECT 1 FROM organizations);

CREATE TABLE organization_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
    organization_id text NOT NULL UNIQUE REFERENCES organizations(id),
    allowed_domains text[] NOT NULL DEFAULT '{}',
    revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
    bootstrap_complete boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO organization_settings(organization_id,bootstrap_complete)
SELECT o.id,EXISTS(SELECT 1 FROM identities i WHERE i.organization_id=o.id AND i.kind='human')
FROM organizations o
ORDER BY (EXISTS(SELECT 1 FROM identities WHERE organization_id=o.id) OR
          EXISTS(SELECT 1 FROM services WHERE organization_id=o.id)) DESC,o.id LIMIT 1;

CREATE TABLE organization_memberships (
    identity_id text PRIMARY KEY REFERENCES identities(id),
    organization_id text NOT NULL REFERENCES organization_settings(organization_id),
    email text,
    email_verified boolean NOT NULL DEFAULT false,
    legacy_subject boolean NOT NULL DEFAULT false,
    category text NOT NULL DEFAULT 'member' CHECK(category='member'),
    role text CHECK(role IN ('admin','user','viewer')),
    state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','active','disabled')),
    revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(organization_id,email),
    CHECK(state<>'active' OR role IS NOT NULL),
    CHECK(state<>'pending' OR role IS NULL),
    CHECK(email IS NOT NULL OR legacy_subject),
    CHECK(NOT email_verified OR email IS NOT NULL)
);
INSERT INTO organization_memberships(identity_id,organization_id,legacy_subject,role,state)
SELECT id,organization_id,true,'admin','active' FROM identities WHERE kind='human';

CREATE TABLE organization_audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id text NOT NULL REFERENCES organizations(id),
    actor_id text REFERENCES identities(id),
    database_actor text NOT NULL DEFAULT current_user,
    target_id text REFERENCES identities(id),
    action text NOT NULL,
    revision integer NOT NULL CHECK(revision>0),
    before_state jsonb NOT NULL DEFAULT '{}',
    after_state jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE api_tokens ADD COLUMN issuer_identity_id text REFERENCES identities(id);
CREATE INDEX api_tokens_issuer ON api_tokens(issuer_identity_id) WHERE revoked_at IS NULL;
ALTER TABLE sessions ADD COLUMN policy_fingerprint text NOT NULL DEFAULT '';
DELETE FROM sessions;
