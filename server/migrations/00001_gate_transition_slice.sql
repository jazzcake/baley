-- +goose Up
CREATE TABLE actors (
  id text PRIMARY KEY,
  display_name text NOT NULL,
  actor_type text NOT NULL CHECK (actor_type IN ('human','agent'))
);

CREATE TABLE workspaces (
  id text PRIMARY KEY,
  name text NOT NULL,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE phases (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  name text NOT NULL,
  position integer NOT NULL CHECK (position >= 0),
  state text NOT NULL CHECK (state IN ('planned','active','completed')),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,position)
);
CREATE UNIQUE INDEX phases_one_active_per_workspace ON phases(workspace_id) WHERE state='active';

CREATE TABLE lanes (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  name text NOT NULL,
  state text NOT NULL CHECK (state IN ('active','closed_out','discarded')),
  PRIMARY KEY (workspace_id,id)
);

CREATE TABLE tasks (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  public_id integer NOT NULL CHECK (public_id > 0),
  lane_id text NOT NULL,
  phase_id text NOT NULL,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('pending','in_progress','implemented','confirmed','discarded')),
  blocked_at timestamptz,
  blocker_reason text,
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,public_id),
  FOREIGN KEY (workspace_id,lane_id) REFERENCES lanes(workspace_id,id),
  FOREIGN KEY (workspace_id,phase_id) REFERENCES phases(workspace_id,id),
  CHECK ((blocked_at IS NULL) = (blocker_reason IS NULL))
);

CREATE TABLE task_dependencies (
  workspace_id text NOT NULL,
  from_task_id text NOT NULL,
  to_task_id text NOT NULL,
  PRIMARY KEY (workspace_id,from_task_id,to_task_id),
  FOREIGN KEY (workspace_id,from_task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,to_task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE,
  CHECK (from_task_id <> to_task_id)
);

CREATE TABLE gates (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  name text NOT NULL,
  from_phase_id text NOT NULL,
  to_phase_id text NOT NULL,
  criteria_revision bigint NOT NULL DEFAULT 1 CHECK (criteria_revision > 0),
  passed_at timestamptz,
  passed_by_actor_id text REFERENCES actors(id),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,from_phase_id),
  UNIQUE (workspace_id,to_phase_id),
  FOREIGN KEY (workspace_id,from_phase_id) REFERENCES phases(workspace_id,id),
  FOREIGN KEY (workspace_id,to_phase_id) REFERENCES phases(workspace_id,id),
  CHECK ((passed_at IS NULL) = (passed_by_actor_id IS NULL))
);

CREATE TABLE gate_tasks (
  workspace_id text NOT NULL,
  id text NOT NULL,
  gate_id text NOT NULL,
  task_id text NOT NULL,
  passed_at timestamptz,
  passed_by_actor_id text REFERENCES actors(id),
  pass_reason text,
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,gate_id,task_id),
  FOREIGN KEY (workspace_id,gate_id) REFERENCES gates(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id),
  CHECK ((passed_at IS NULL AND passed_by_actor_id IS NULL AND pass_reason IS NULL) OR
         (passed_at IS NOT NULL AND passed_by_actor_id IS NOT NULL AND length(trim(pass_reason)) > 0))
);

CREATE TABLE commands (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id),
  idempotency_key text NOT NULL,
  command_name text NOT NULL,
  command_hash text NOT NULL,
  request_fingerprint text NOT NULL,
  workspace_revision bigint NOT NULL,
  result jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id,idempotency_key)
);

CREATE TABLE human_approval_attestations (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id),
  approved_by_actor_id text NOT NULL REFERENCES actors(id),
  approved_command_hash text NOT NULL,
  decision_snapshot_hash text,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  workspace_revision bigint NOT NULL,
  executed_command_id text NOT NULL UNIQUE REFERENCES commands(id),
  statement_hash text,
  conversation_ref text,
  approved_at timestamptz,
  recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE events (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id),
  command_id text NOT NULL REFERENCES commands(id),
  workspace_revision bigint NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_counters (
  workspace_id text PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  next_task_public_id integer NOT NULL CHECK (next_task_public_id > 0)
);

-- +goose StatementBegin
CREATE FUNCTION enforce_gate_phase_order() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE from_position integer; to_position integer;
BEGIN
  SELECT position INTO from_position FROM phases WHERE workspace_id=NEW.workspace_id AND id=NEW.from_phase_id;
  SELECT position INTO to_position FROM phases WHERE workspace_id=NEW.workspace_id AND id=NEW.to_phase_id;
  IF to_position <> from_position + 1 THEN RAISE EXCEPTION 'gate must connect adjacent phases'; END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER gates_phase_order BEFORE INSERT OR UPDATE ON gates FOR EACH ROW EXECUTE FUNCTION enforce_gate_phase_order();

-- +goose StatementBegin
CREATE FUNCTION enforce_gate_task_from_phase() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE gate_phase text; task_phase text;
BEGIN
  SELECT from_phase_id INTO gate_phase FROM gates WHERE workspace_id=NEW.workspace_id AND id=NEW.gate_id;
  SELECT phase_id INTO task_phase FROM tasks WHERE workspace_id=NEW.workspace_id AND id=NEW.task_id;
  IF gate_phase IS DISTINCT FROM task_phase THEN RAISE EXCEPTION 'gate task must belong to gate from phase'; END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER gate_tasks_from_phase BEFORE INSERT OR UPDATE ON gate_tasks FOR EACH ROW EXECUTE FUNCTION enforce_gate_task_from_phase();

-- +goose Down
DROP TABLE workspace_counters;
DROP TABLE events;
DROP TABLE human_approval_attestations;
DROP TABLE commands;
DROP TABLE gate_tasks;
DROP TABLE gates;
DROP TABLE task_dependencies;
DROP TABLE tasks;
DROP TABLE lanes;
DROP INDEX phases_one_active_per_workspace;
DROP TABLE phases;
DROP TABLE workspaces;
DROP TABLE actors;
DROP FUNCTION enforce_gate_task_from_phase();
DROP FUNCTION enforce_gate_phase_order();
