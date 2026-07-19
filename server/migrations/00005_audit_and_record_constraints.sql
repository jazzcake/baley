-- +goose Up
ALTER TABLE commands
  ADD COLUMN initiated_by_actor_id text REFERENCES actors(id),
  ADD COLUMN executed_by_actor_id text REFERENCES actors(id);

ALTER TABLE events
  ADD COLUMN entity_type text,
  ADD COLUMN entity_id text,
  ADD COLUMN initiated_by_actor_id text REFERENCES actors(id),
  ADD COLUMN executed_by_actor_id text REFERENCES actors(id),
  ADD COLUMN approved_by_actor_id text REFERENCES actors(id),
  ADD CONSTRAINT events_entity_envelope
    CHECK ((entity_type IS NULL AND entity_id IS NULL) OR
           (length(trim(entity_type)) > 0 AND length(trim(entity_id)) > 0));

CREATE INDEX events_workspace_entity_history
  ON events(workspace_id,entity_type,entity_id,workspace_revision);

CREATE UNIQUE INDEX repositories_one_record_repository
  ON repositories(workspace_id) WHERE is_record_repository;

ALTER TABLE task_record_indexes
  ADD CONSTRAINT task_record_short_summary_length
    CHECK (length(short_summary) <= 500);

-- +goose Down
ALTER TABLE task_record_indexes DROP CONSTRAINT task_record_short_summary_length;
DROP INDEX repositories_one_record_repository;
DROP INDEX events_workspace_entity_history;
ALTER TABLE events
  DROP CONSTRAINT events_entity_envelope,
  DROP COLUMN approved_by_actor_id,
  DROP COLUMN executed_by_actor_id,
  DROP COLUMN initiated_by_actor_id,
  DROP COLUMN entity_id,
  DROP COLUMN entity_type;
ALTER TABLE commands
  DROP COLUMN executed_by_actor_id,
  DROP COLUMN initiated_by_actor_id;
