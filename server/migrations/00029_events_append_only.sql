-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION reject_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'events is append-only';
END $$;
-- +goose StatementEnd
CREATE TRIGGER events_no_update_delete
  BEFORE UPDATE OR DELETE ON events
  FOR EACH ROW EXECUTE FUNCTION reject_event_mutation();
CREATE TRIGGER events_no_truncate
  BEFORE TRUNCATE ON events
  FOR EACH STATEMENT EXECUTE FUNCTION reject_event_mutation();

-- +goose Down
DROP TRIGGER events_no_truncate ON events;
DROP TRIGGER events_no_update_delete ON events;
DROP FUNCTION reject_event_mutation();
