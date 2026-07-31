-- +goose Up
ALTER TABLE mutation_attempts
  ADD COLUMN IF NOT EXISTS command_hash text;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_direct_task_truncate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF coalesce(current_setting('baley.mutation_attempt_id', true), '') <> '' THEN
    RETURN NULL;
  END IF;
  INSERT INTO mutation_attempts(
    id, workspace_id, command_name, source, outcome, entity_type, entity_id
  )
  SELECT gen_random_uuid()::text, workspace_id, 'sql.tasks.truncate',
         'database_trigger', 'succeeded', 'task', '*'
  FROM tasks
  GROUP BY workspace_id;
  RETURN NULL;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgname = 'tasks_direct_truncate_audit'
      AND tgrelid = 'tasks'::regclass
  ) THEN
    CREATE TRIGGER tasks_direct_truncate_audit
      BEFORE TRUNCATE ON tasks
      FOR EACH STATEMENT EXECUTE FUNCTION audit_direct_task_truncate();
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- The current 00009 baseline already contains this column and trigger. This
-- compatibility migration only upgrades databases that applied an older
-- 00009, so rolling it back must preserve the current 00009 schema.
SELECT 1;
