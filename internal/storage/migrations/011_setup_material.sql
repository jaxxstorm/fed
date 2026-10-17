ALTER TABLE connection_setup_authorizations ADD COLUMN installation_hint text NOT NULL DEFAULT '';
CREATE TABLE connection_setup_material (
    service_id text NOT NULL,
    setup_id text NOT NULL,
    account_id text NOT NULL,
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    ciphertext bytea NOT NULL,
    PRIMARY KEY(service_id,setup_id),
    FOREIGN KEY(service_id,setup_id) REFERENCES connection_setups(service_id,id)
);
