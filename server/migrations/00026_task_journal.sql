-- +goose Up
-- Composite keys keep every journal link inside one Workspace even though
-- command and Event IDs are deployment-wide primary keys.
ALTER TABLE commands
  ADD CONSTRAINT commands_workspace_id_id_unique UNIQUE (workspace_id,id);
ALTER TABLE events
  ADD CONSTRAINT events_workspace_id_id_unique UNIQUE (workspace_id,id);

CREATE TABLE task_journal_entries (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  task_id text NOT NULL,
  event_id text NOT NULL,
  command_id text NOT NULL,
  lifecycle_stage text NOT NULL CHECK (lifecycle_stage IN (
    'created','run_started','updated','rework_started','blocked','unblocked',
    'implemented','confirmed','discarded'
  )),
  narrative text,
  schema_version integer NOT NULL CHECK (schema_version > 0),
  context jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(context) = 'object'),
  initiated_by_actor_id text REFERENCES actors(id),
  executed_by_actor_id text NOT NULL REFERENCES actors(id),
  approved_by_actor_id text REFERENCES actors(id),
  occurred_at timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id,event_id),
  UNIQUE (workspace_id,command_id),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,event_id) REFERENCES events(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,command_id) REFERENCES commands(workspace_id,id) ON DELETE CASCADE,
  CHECK (narrative IS NULL OR btrim(narrative) <> ''),
  CHECK (narrative IS NOT NULL OR context <> '{}'::jsonb),
  CHECK (recorded_at >= occurred_at)
);

CREATE INDEX task_journal_entries_workspace_time_idx
  ON task_journal_entries(workspace_id,recorded_at DESC,id DESC);
CREATE INDEX task_journal_entries_task_time_idx
  ON task_journal_entries(workspace_id,task_id,recorded_at DESC,id DESC);
CREATE INDEX task_journal_entries_context_gin_idx
  ON task_journal_entries USING gin(context jsonb_path_ops);

-- +goose StatementBegin
CREATE FUNCTION reject_task_journal_entry_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'TRUNCATE' AND NOT EXISTS (SELECT 1 FROM task_journal_entries) THEN
		RETURN NULL;
	END IF;
  RAISE EXCEPTION 'task_journal_entries is append-only';
END $$;
-- +goose StatementEnd
CREATE TRIGGER task_journal_entries_append_only
  BEFORE UPDATE OR DELETE ON task_journal_entries
  FOR EACH ROW EXECUTE FUNCTION reject_task_journal_entry_change();
CREATE TRIGGER task_journal_entries_truncate_append_only
  BEFORE TRUNCATE ON task_journal_entries
  FOR EACH STATEMENT EXECUTE FUNCTION reject_task_journal_entry_change();

-- +goose Down
DROP TABLE task_journal_entries;
DROP FUNCTION reject_task_journal_entry_change();
ALTER TABLE events DROP CONSTRAINT events_workspace_id_id_unique;
ALTER TABLE commands DROP CONSTRAINT commands_workspace_id_id_unique;
