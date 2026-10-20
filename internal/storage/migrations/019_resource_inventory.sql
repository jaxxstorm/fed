CREATE TABLE resource_inventories (
 service_id text NOT NULL REFERENCES services(id),
 job_id text NOT NULL REFERENCES jobs(id),
 partition text NOT NULL,
 scope_revision integer NOT NULL,
 profile_revision integer NOT NULL,
 connection_id text NOT NULL,
 generation bigint NOT NULL,
 source_id text NOT NULL,
 source_revision integer NOT NULL,
 binding_revision integer NOT NULL,
 complete boolean NOT NULL,
 target_keys text[] NOT NULL,
 observed_at timestamptz NOT NULL,
 PRIMARY KEY(service_id,job_id,partition),
 FOREIGN KEY(service_id,connection_id) REFERENCES evidence_connections(service_id,id)
);
CREATE INDEX resource_inventory_current ON resource_inventories(service_id,partition,scope_revision,profile_revision,observed_at DESC);
CREATE TABLE observation_inventory_bindings (
 service_id text NOT NULL,
 observation_id text NOT NULL,
 job_id text NOT NULL,
 partition text NOT NULL,
 PRIMARY KEY(service_id,observation_id),
 FOREIGN KEY(service_id,observation_id) REFERENCES observations(service_id,id),
 FOREIGN KEY(service_id,job_id,partition) REFERENCES resource_inventories(service_id,job_id,partition)
);
-- Old observations with no verified generation provenance remain conservatively
-- visible. A complete inventory may retire only covered, compatible identities.
CREATE VIEW current_resource_observations AS
SELECT o.* FROM observations o WHERE NOT EXISTS (
 SELECT 1 FROM observation_inventory_bindings b
 JOIN resource_inventories origin ON origin.service_id=b.service_id AND origin.job_id=b.job_id AND origin.partition=b.partition
 JOIN LATERAL (
   SELECT i.* FROM resource_inventories i
   JOIN evidence_connections c ON c.service_id=i.service_id AND c.id=i.connection_id AND c.generation=i.generation AND c.lifecycle='active'
   JOIN connection_source_bindings sb ON sb.service_id=i.service_id AND sb.source_id=i.source_id AND sb.connection_id=i.connection_id AND sb.revision=i.binding_revision AND sb.lifecycle='enabled'
   WHERE i.service_id=origin.service_id AND i.partition=origin.partition
   AND i.scope_revision=o.scope_revision AND i.profile_revision=o.profile_revision
   AND i.connection_id=origin.connection_id AND i.generation=origin.generation
   AND i.source_id=origin.source_id AND i.source_revision=origin.source_revision AND i.binding_revision=origin.binding_revision
   AND i.source_revision=(SELECT max(revision) FROM records WHERE service_id=i.service_id AND kind='evidence_source' AND id=i.source_id)
   ORDER BY i.observed_at DESC,i.job_id DESC LIMIT 1
 ) latest ON latest.complete AND latest.observed_at>o.observed_at AND NOT o.target_key=ANY(latest.target_keys)
 WHERE b.service_id=o.service_id AND b.observation_id=o.id
);
