ALTER TABLE connection_discovery_sessions ADD COLUMN metadata jsonb NOT NULL DEFAULT '{}';
CREATE TABLE connection_discovery_seen (
    service_id text NOT NULL,
    session_id text NOT NULL,
    tenant text NOT NULL,
    resource_kind text NOT NULL,
    resource_id text NOT NULL,
    PRIMARY KEY(service_id,session_id,tenant,resource_kind,resource_id),
    FOREIGN KEY(service_id,session_id) REFERENCES connection_discovery_sessions(service_id,id)
);
CREATE TABLE connection_inventory_cursors (
    id text PRIMARY KEY,
    service_id text NOT NULL,
    connection_id text NOT NULL,
    generation bigint NOT NULL,
    query_hash text NOT NULL,
    after_kind text NOT NULL,
    after_id text NOT NULL,
    expires_at timestamptz NOT NULL,
    FOREIGN KEY(service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE INDEX connection_cursor_expiry ON connection_inventory_cursors(expires_at);
