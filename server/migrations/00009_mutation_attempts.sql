-- +goose Up
CREATE TABLE mutation_attempts (
  id text PRIMARY KEY,
  workspace_id text NOT NULL,
  command_name text NOT NULL CHECK (btrim(command_name) <> ''),
  source text NOT NULL CHECK (source IN ('command_service','database_trigger')),
  outcome text NOT NULL CHECK (outcome IN ('succeeded','rejected','failed','idempotent')),
  entity_type text,
  entity_id text,
  initiated_by_actor_id text,
  executed_by_actor_id text,
  idempotency_key_hash text,
  argument_digest text,
  request_fingerprint text,
  command_hash text,
  command_id text,
  event_ids text[] NOT NULL DEFAULT '{}',
  expected_workspace_revision bigint,
  observed_workspace_revision bigint,
  diagnostic_codes text[] NOT NULL DEFAULT '{}',
  duration_ms bigint NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mutation_attempts_workspace_time_idx
  ON mutation_attempts(workspace_id, occurred_at DESC, id DESC);
CREATE INDEX mutation_attempts_workspace_command_idx
  ON mutation_attempts(workspace_id, command_name, occurred_at DESC);

-- +goose StatementBegin
CREATE FUNCTION reject_mutation_attempt_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'mutation_attempts is append-only';
END $$;
-- +goose StatementEnd
CREATE TRIGGER mutation_attempts_append_only
  BEFORE UPDATE OR DELETE OR TRUNCATE ON mutation_attempts
  FOR EACH STATEMENT EXECUTE FUNCTION reject_mutation_attempt_change();

-- +goose StatementBegin
CREATE FUNCTION audit_direct_task_write() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  row_workspace text;
  row_entity text;
BEGIN
  IF coalesce(current_setting('baley.mutation_attempt_id', true), '') <> '' THEN
    RETURN coalesce(NEW, OLD);
  END IF;
  row_workspace := CASE WHEN TG_OP = 'DELETE' THEN OLD.workspace_id ELSE NEW.workspace_id END;
  row_entity := CASE WHEN TG_OP = 'DELETE' THEN OLD.id ELSE NEW.id END;
  INSERT INTO mutation_attempts(
    id, workspace_id, command_name, source, outcome, entity_type, entity_id
  ) VALUES (
    gen_random_uuid()::text, row_workspace, 'sql.tasks.' || lower(TG_OP),
    'database_trigger', 'succeeded', 'task', row_entity
  );
  RETURN coalesce(NEW, OLD);
END $$;
-- +goose StatementEnd
CREATE TRIGGER tasks_direct_write_audit
  AFTER INSERT OR UPDATE OR DELETE ON tasks
  FOR EACH ROW EXECUTE FUNCTION audit_direct_task_write();

-- +goose StatementBegin
CREATE FUNCTION audit_direct_task_truncate() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER tasks_direct_truncate_audit
  BEFORE TRUNCATE ON tasks
  FOR EACH STATEMENT EXECUTE FUNCTION audit_direct_task_truncate();

-- +goose Down
DROP TRIGGER IF EXISTS tasks_direct_truncate_audit ON tasks;
DROP FUNCTION IF EXISTS audit_direct_task_truncate();
DROP TRIGGER IF EXISTS tasks_direct_write_audit ON tasks;
DROP FUNCTION IF EXISTS audit_direct_task_write();
DROP TRIGGER IF EXISTS mutation_attempts_append_only ON mutation_attempts;
DROP FUNCTION IF EXISTS reject_mutation_attempt_change();
DROP TABLE mutation_attempts;
