CREATE FUNCTION queue_collection_reconciliation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind IN ('scope','profile') THEN
        INSERT INTO collection_outbox(service_id) VALUES(NEW.service_id)
        ON CONFLICT(service_id) DO UPDATE SET generation=collection_outbox.generation+1,updated_at=now();
    END IF;
    IF NEW.kind='measure' THEN
        UPDATE managed_measures SET overridden=true WHERE service_id=NEW.service_id AND measure_id=NEW.id
        AND NOT EXISTS(SELECT 1 FROM identities WHERE id=NEW.actor_id AND kind='automation');
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collection_record_changed AFTER INSERT ON records FOR EACH ROW EXECUTE FUNCTION queue_collection_reconciliation();
INSERT INTO collection_outbox(service_id) SELECT id FROM services ON CONFLICT DO NOTHING;
