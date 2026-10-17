ALTER TABLE permissions DROP CONSTRAINT permissions_action_check;
ALTER TABLE permissions ADD CONSTRAINT permissions_action_check
    CHECK (action IN ('read','edit','review','admin','check','export','connect'));

-- Metadata revisions are separate from credential generations and evidence work.
CREATE TABLE evidence_connections (
    service_id text NOT NULL REFERENCES services(id),
    id text NOT NULL,
    provider text NOT NULL CHECK (provider IN ('github','google_workspace','aws','kubernetes','documents')),
    revision integer NOT NULL CHECK (revision > 0),
    generation bigint NOT NULL CHECK (generation > 0),
    lifecycle text NOT NULL CHECK (lifecycle IN ('active','disconnected','deleted')),
    data jsonb NOT NULL,
    access_health jsonb NOT NULL DEFAULT '{"state":"not_configured"}',
    actor_id text NOT NULL REFERENCES identities(id),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,id)
);
CREATE TABLE connection_revisions (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    data jsonb NOT NULL,
    actor_id text NOT NULL REFERENCES identities(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,connection_id,revision),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE TABLE connection_credentials (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    ciphertext bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,connection_id),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE TABLE connection_source_bindings (
    service_id text NOT NULL,
    source_id text NOT NULL,
    connection_id text NOT NULL,
    provider text NOT NULL,
    tenant text NOT NULL,
    resource_kind text NOT NULL,
    resource_id text NOT NULL,
    lifecycle text NOT NULL CHECK (lifecycle IN ('enabled','disabled','retired')),
    revision integer NOT NULL CHECK (revision > 0),
    actor_id text NOT NULL REFERENCES identities(id),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,source_id),
    UNIQUE (service_id,provider,tenant,resource_kind,resource_id),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE INDEX connection_source_children ON connection_source_bindings(service_id,connection_id,lifecycle);

CREATE TABLE connection_operations (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    id text NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    actor_id text NOT NULL REFERENCES identities(id),
    token_hash text REFERENCES api_tokens(hash),
    operation text NOT NULL,
    state text NOT NULL CHECK (state IN ('pending','running','succeeded','partial','failed','cancelled','expired')),
    result jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id,id),
    UNIQUE (service_id,connection_id,id),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE TABLE connection_authorizations (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    operation_id text NOT NULL,
    provider text NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    state_hash text NOT NULL UNIQUE,
    initiating_actor_id text NOT NULL REFERENCES identities(id),
    browser_actor_id text REFERENCES identities(id),
    browser_session_hash text,
    redirect_uri text NOT NULL,
    verifier_key_id text,
    verifier_nonce bytea,
    verifier_ciphertext bytea,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    PRIMARY KEY (service_id,operation_id),
    FOREIGN KEY (service_id,connection_id,operation_id) REFERENCES connection_operations(service_id,connection_id,id)
);
CREATE TABLE connection_discovery_sessions (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    id text NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    actor_id text NOT NULL REFERENCES identities(id),
    mode text NOT NULL CHECK (mode IN ('server_managed','server_environment','agent_managed','manual')),
    state text NOT NULL CHECK (state IN ('running','complete','partial','failed')),
    coverage jsonb NOT NULL DEFAULT '[]',
    next_cursor text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    PRIMARY KEY (service_id,id),
    UNIQUE (service_id,connection_id,id),
    FOREIGN KEY (service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE TABLE connection_discovery_pages (
    service_id text NOT NULL,
    session_id text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash text NOT NULL,
    receipt jsonb NOT NULL,
    PRIMARY KEY (service_id,session_id,idempotency_key),
    FOREIGN KEY (service_id,session_id) REFERENCES connection_discovery_sessions(service_id,id)
);
CREATE TABLE connection_inventory (
    service_id text NOT NULL,
    connection_id text NOT NULL,
    resource_kind text NOT NULL,
    resource_id text NOT NULL,
    tenant text NOT NULL,
    data jsonb NOT NULL,
    session_id text NOT NULL,
    available boolean NOT NULL DEFAULT true,
    observed_at timestamptz NOT NULL,
    PRIMARY KEY (service_id,connection_id,tenant,resource_kind,resource_id),
    FOREIGN KEY (service_id,connection_id,session_id) REFERENCES connection_discovery_sessions(service_id,connection_id,id)
);
CREATE INDEX connection_inventory_page ON connection_inventory(service_id,connection_id,resource_kind,resource_id);
