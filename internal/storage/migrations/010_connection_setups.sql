-- Preserve existing metadata/evidence JSON and credential semantics.
ALTER TABLE evidence_connections ADD COLUMN access_strategy text NOT NULL DEFAULT 'legacy'
    CHECK (access_strategy IN ('legacy','delegated_role','workspace_consent','github_installation'));

CREATE TABLE connection_setups (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    provider text NOT NULL CHECK (provider IN ('aws','github','google_workspace')),
    strategy text NOT NULL CHECK (strategy IN ('delegated_role','workspace_consent','github_installation')),
    step text NOT NULL CHECK (step IN ('prerequisites','configure','authorize','confirm','complete','cancelled','expired')),
    revision integer NOT NULL CHECK (revision > 0),
    input jsonb NOT NULL DEFAULT '{}',
    verified_account_id text NOT NULL DEFAULT '',
    connection_id text,
    expected_generation bigint,
    actor_id text NOT NULL REFERENCES identities(id),
    token_hash text REFERENCES api_tokens(hash),
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,id),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id),
    CHECK ((provider='aws' AND strategy='delegated_role') OR
           (provider='github' AND strategy='github_installation') OR
           (provider='google_workspace' AND strategy='workspace_consent')),
    CHECK (step <> 'complete' OR (connection_id IS NOT NULL AND verified_account_id <> ''))
);
CREATE INDEX connection_setup_expiry ON connection_setups(expires_at)
    WHERE step NOT IN ('complete','cancelled','expired');

CREATE TABLE connection_setup_secrets (
    service_id text NOT NULL,
    setup_id text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    ciphertext bytea NOT NULL,
    PRIMARY KEY (service_id,setup_id),
    FOREIGN KEY (service_id,setup_id) REFERENCES connection_setups(service_id,id)
);

-- Initial consent cannot reference a connection that does not exist yet.
CREATE TABLE connection_setup_authorizations (
    service_id text NOT NULL,
    setup_id text NOT NULL,
    id text NOT NULL,
    setup_revision integer NOT NULL CHECK (setup_revision > 0),
    stage text NOT NULL CHECK (stage IN ('installation','oauth')),
    state_hash text NOT NULL UNIQUE,
    browser_actor_id text REFERENCES identities(id),
    browser_session_hash text,
    redirect_uri text NOT NULL,
    verifier_key_id text,
    verifier_nonce bytea,
    verifier_ciphertext bytea,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    PRIMARY KEY (service_id,id),
    FOREIGN KEY (service_id,setup_id) REFERENCES connection_setups(service_id,id)
);
