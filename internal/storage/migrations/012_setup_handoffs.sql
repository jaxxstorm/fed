CREATE TABLE connection_setup_handoffs (
    hash text PRIMARY KEY,
    service_id text NOT NULL,
    setup_id text NOT NULL,
    setup_revision integer NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    FOREIGN KEY(service_id,setup_id) REFERENCES connection_setups(service_id,id)
);
