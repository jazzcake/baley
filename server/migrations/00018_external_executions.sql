-- +goose Up
CREATE TABLE external_executions (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  task_id text NOT NULL,
  provider text NOT NULL CHECK (provider IN ('orca')),
  external_id text,
  provider_instance_id text,
  host_id text,
  status text NOT NULL CHECK (status IN ('creating','active','review','settled','lost')),
  attempt_number integer NOT NULL CHECK (attempt_number > 0),
  client_execution_id uuid NOT NULL,
  context_snapshot_hash text,
  last_terminal_handle text,
  started_at timestamptz NOT NULL,
  last_observed_at timestamptz,
  settled_at timestamptz,
  settlement_reason text CHECK (settlement_reason IN ('completed','abandoned','rejected','superseded','creation_failed','external_deleted_after_recovery')),
  created_by_actor_id text NOT NULL REFERENCES actors(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,client_execution_id),
  UNIQUE (workspace_id,task_id,provider,attempt_number),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id),
  CHECK ((status='creating' AND external_id IS NULL AND provider_instance_id IS NULL AND host_id IS NULL) OR
         (status<>'creating' AND external_id IS NOT NULL AND provider_instance_id IS NOT NULL AND host_id IS NOT NULL) OR
         (status='lost')),
  CHECK ((status='settled' AND settled_at IS NOT NULL AND settlement_reason IS NOT NULL) OR
         (status<>'settled' AND settled_at IS NULL AND settlement_reason IS NULL))
);

CREATE UNIQUE INDEX external_executions_one_open_per_task_provider
  ON external_executions(workspace_id,task_id,provider)
  WHERE status IN ('creating','active','review','lost');

ALTER TABLE runs ADD COLUMN external_execution_id uuid;
ALTER TABLE runs ADD FOREIGN KEY (workspace_id,external_execution_id)
  REFERENCES external_executions(workspace_id,id);

-- +goose Down
ALTER TABLE runs DROP COLUMN external_execution_id;
DROP TABLE external_executions;
