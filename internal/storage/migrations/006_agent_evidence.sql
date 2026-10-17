ALTER TABLE records DROP CONSTRAINT records_kind_check;
ALTER TABLE records ADD CONSTRAINT records_kind_check CHECK (kind IN ('scope','profile','measure','evidence','assessment','review','gap','destination','snapshot','publication','evidence_source','rule_result'));

CREATE TABLE rule_evidence_links (
    service_id text NOT NULL,
    requirement_id text NOT NULL,
    evidence_id text NOT NULL,
    evidence_revision integer NOT NULL,
    evidence_kind text NOT NULL DEFAULT 'evidence' CHECK(evidence_kind='evidence'),
    result_id text NOT NULL,
    result_revision integer NOT NULL,
    result_kind text NOT NULL DEFAULT 'rule_result' CHECK(result_kind='rule_result'),
    assertion_id text NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY(service_id,requirement_id,result_id,result_revision,assertion_id,evidence_id,evidence_revision),
    FOREIGN KEY(service_id,evidence_kind,evidence_id,evidence_revision) REFERENCES records(service_id,kind,id,revision),
    FOREIGN KEY(service_id,result_kind,result_id,result_revision) REFERENCES records(service_id,kind,id,revision)
);
CREATE INDEX rule_evidence_lookup ON rule_evidence_links(service_id,requirement_id,evidence_id,evidence_revision);

CREATE TABLE evidence_work_versions (
    service_id text PRIMARY KEY REFERENCES services(id),
    revision bigint NOT NULL DEFAULT 1
);
INSERT INTO evidence_work_versions(service_id) SELECT id FROM services;
CREATE FUNCTION advance_evidence_work_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO evidence_work_versions(service_id) VALUES(NEW.service_id)
    ON CONFLICT(service_id) DO UPDATE SET revision=evidence_work_versions.revision+1;
    RETURN NEW;
END $$;
CREATE TRIGGER evidence_work_record_changed AFTER INSERT ON records FOR EACH ROW EXECUTE FUNCTION advance_evidence_work_version();
CREATE TRIGGER evidence_work_observation_changed AFTER INSERT ON observations FOR EACH ROW EXECUTE FUNCTION advance_evidence_work_version();
