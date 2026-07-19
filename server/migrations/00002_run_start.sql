-- +goose Up
ALTER TABLE workspaces
  ADD COLUMN state text NOT NULL DEFAULT 'draft'
  CHECK (state IN ('draft','active','closed'));
UPDATE workspaces w SET state='active'
WHERE EXISTS (SELECT 1 FROM phases p WHERE p.workspace_id=w.id AND p.state='active');

ALTER TABLE events ADD COLUMN command_event_index integer;
WITH ranked AS (
  SELECT id, row_number() OVER (PARTITION BY command_id ORDER BY created_at,id) - 1 AS event_index
  FROM events
)
UPDATE events SET command_event_index=ranked.event_index
FROM ranked WHERE events.id=ranked.id;
ALTER TABLE events ALTER COLUMN command_event_index SET NOT NULL;
ALTER TABLE events ALTER COLUMN command_event_index SET DEFAULT 0;
ALTER TABLE events ADD CHECK (command_event_index >= 0);
CREATE UNIQUE INDEX events_command_sequence ON events(command_id,command_event_index);

CREATE TABLE runs (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  task_id text NOT NULL,
  client_run_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('detailed_planning','implementation','independent_agent_review','review_response','completion_reporting')),
  status text NOT NULL CHECK (status IN ('running','succeeded','failed','interrupted','cancelled')),
  operator_actor_id text NOT NULL REFERENCES actors(id),
  session_ref text,
  parent_run_id text,
  target_run_id text,
  lease_token_hash text NOT NULL,
  heartbeat_at timestamptz NOT NULL,
  lease_expires_at timestamptz NOT NULL,
  version bigint NOT NULL CHECK (version > 0),
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  result_summary text,
  error_summary text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,client_run_id),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id),
  FOREIGN KEY (workspace_id,parent_run_id) REFERENCES runs(workspace_id,id),
  FOREIGN KEY (workspace_id,target_run_id) REFERENCES runs(workspace_id,id),
  CHECK (lease_expires_at > heartbeat_at),
  CHECK ((status = 'running' AND ended_at IS NULL) OR (status <> 'running' AND ended_at IS NOT NULL))
);

-- +goose Down
DROP TABLE runs;
DROP INDEX events_command_sequence;
ALTER TABLE events DROP COLUMN command_event_index;
ALTER TABLE workspaces DROP COLUMN state;
