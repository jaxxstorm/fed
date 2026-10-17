ALTER TABLE collection_plans ADD COLUMN configuration_revision integer NOT NULL DEFAULT 1;
DO $$ DECLARE c record; BEGIN
    FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='collection_plans'::regclass AND contype='u' LOOP
        EXECUTE format('ALTER TABLE collection_plans DROP CONSTRAINT %I',c.conname);
    END LOOP;
END $$;
ALTER TABLE collection_plans ADD CONSTRAINT collection_plan_binding UNIQUE(service_id,scope_revision,profile_revision,configuration_revision,template_version);
CREATE TABLE collection_configurations (
    service_id text NOT NULL REFERENCES services(id),
    revision integer NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(service_id,revision)
);
INSERT INTO collection_configurations(service_id,revision,data) SELECT service_id,revision,to_jsonb(s) FROM collection_schedules s;
CREATE FUNCTION snapshot_collection_configuration() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='INSERT' OR NEW.revision<>OLD.revision THEN
        INSERT INTO collection_configurations(service_id,revision,data) VALUES(NEW.service_id,NEW.revision,to_jsonb(NEW));
        INSERT INTO collection_outbox(service_id) VALUES(NEW.service_id)
        ON CONFLICT(service_id) DO UPDATE SET generation=collection_outbox.generation+1,updated_at=now();
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collection_configuration_changed AFTER INSERT OR UPDATE ON collection_schedules FOR EACH ROW EXECUTE FUNCTION snapshot_collection_configuration();
CREATE OR REPLACE FUNCTION queue_collection_reconciliation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind IN ('scope','profile') THEN
        INSERT INTO collection_outbox(service_id) VALUES(NEW.service_id)
        ON CONFLICT(service_id) DO UPDATE SET generation=collection_outbox.generation+1,updated_at=now();
    END IF;
    IF NEW.kind='measure' AND EXISTS(SELECT 1 FROM managed_measures WHERE service_id=NEW.service_id AND measure_id=NEW.id)
    AND NOT EXISTS(SELECT 1 FROM identities WHERE id=NEW.actor_id AND kind='automation') THEN
        UPDATE managed_measures SET overridden=true WHERE service_id=NEW.service_id AND measure_id=NEW.id;
        UPDATE collection_schedules SET revision=revision+1,updated_at=now() WHERE service_id=NEW.service_id;
    END IF;
    RETURN NEW;
END $$;
