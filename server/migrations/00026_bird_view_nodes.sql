-- +goose Up
CREATE TABLE bird_views (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  title text NOT NULL CHECK (btrim(title) <> ''),
  description text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (account_id,id),
  CHECK ((status='active' AND archived_at IS NULL) OR
         (status='archived' AND archived_at IS NOT NULL))
);
CREATE INDEX bird_views_account_status_updated_idx
  ON bird_views(account_id,status,updated_at DESC,id);

CREATE TABLE bird_view_nodes (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  id uuid NOT NULL,
  title text NOT NULL CHECK (btrim(title) <> ''),
  summary text NOT NULL DEFAULT '',
  content text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','achieved','parked')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,id),
  UNIQUE (account_id,bird_view_id,id),
  FOREIGN KEY (account_id,bird_view_id) REFERENCES bird_views(account_id,id) ON DELETE CASCADE
);
CREATE INDEX bird_view_nodes_graph_idx ON bird_view_nodes(account_id,bird_view_id,created_at,id);

CREATE TABLE bird_view_edges (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  id uuid NOT NULL,
  from_node_id uuid NOT NULL,
  to_node_id uuid NOT NULL,
  label text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,id),
  UNIQUE (account_id,bird_view_id,id),
  UNIQUE (bird_view_id,from_node_id,to_node_id),
  FOREIGN KEY (account_id,bird_view_id) REFERENCES bird_views(account_id,id) ON DELETE CASCADE,
  FOREIGN KEY (account_id,bird_view_id,from_node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  FOREIGN KEY (account_id,bird_view_id,to_node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  CHECK (from_node_id <> to_node_id)
);
CREATE INDEX bird_view_edges_from_idx ON bird_view_edges(account_id,bird_view_id,from_node_id);
CREATE INDEX bird_view_edges_to_idx ON bird_view_edges(account_id,bird_view_id,to_node_id);

CREATE TABLE bird_view_phase_bindings (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  node_id uuid NOT NULL,
  workspace_id text NOT NULL,
  phase_id text NOT NULL,
  binding_state text NOT NULL CHECK (binding_state IN ('suggested','pinned','excluded')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,node_id,workspace_id,phase_id),
  FOREIGN KEY (account_id,bird_view_id,node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,phase_id) REFERENCES phases(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX bird_view_phase_bindings_target_idx ON bird_view_phase_bindings(workspace_id,phase_id);

CREATE TABLE bird_view_gate_bindings (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  node_id uuid NOT NULL,
  workspace_id text NOT NULL,
  gate_id text NOT NULL,
  binding_state text NOT NULL CHECK (binding_state IN ('suggested','pinned','excluded')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,node_id,workspace_id,gate_id),
  FOREIGN KEY (account_id,bird_view_id,node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,gate_id) REFERENCES gates(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX bird_view_gate_bindings_target_idx ON bird_view_gate_bindings(workspace_id,gate_id);

CREATE TABLE bird_view_task_bindings (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  node_id uuid NOT NULL,
  workspace_id text NOT NULL,
  task_id text NOT NULL,
  binding_state text NOT NULL CHECK (binding_state IN ('suggested','pinned','excluded')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,node_id,workspace_id,task_id),
  FOREIGN KEY (account_id,bird_view_id,node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX bird_view_task_bindings_target_idx ON bird_view_task_bindings(workspace_id,task_id);

CREATE TABLE bird_view_backlog_bindings (
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  node_id uuid NOT NULL,
  workspace_id text NOT NULL,
  backlog_id text NOT NULL,
  binding_state text NOT NULL CHECK (binding_state IN ('suggested','pinned','excluded')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bird_view_id,node_id,workspace_id,backlog_id),
  FOREIGN KEY (account_id,bird_view_id,node_id) REFERENCES bird_view_nodes(account_id,bird_view_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,backlog_id) REFERENCES backlog_items(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX bird_view_backlog_bindings_target_idx ON bird_view_backlog_bindings(workspace_id,backlog_id);

CREATE TABLE bird_view_commands (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  bird_view_id uuid NOT NULL,
  idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
  command_name text NOT NULL,
  command_hash text NOT NULL,
  request_fingerprint text NOT NULL,
  expected_bird_view_revision bigint,
  bird_view_revision bigint NOT NULL CHECK (bird_view_revision > 0),
  executed_by_actor_id text NOT NULL REFERENCES actors(id),
  initiated_by_actor_id text REFERENCES actors(id),
  result jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id,idempotency_key),
  UNIQUE (account_id,bird_view_id,bird_view_revision),
  UNIQUE (account_id,bird_view_id,bird_view_revision,id),
  FOREIGN KEY (account_id,bird_view_id) REFERENCES bird_views(account_id,id)
);
CREATE INDEX bird_view_commands_scope_idx ON bird_view_commands(account_id,bird_view_id,created_at,id);

CREATE TABLE bird_view_events (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL,
  bird_view_id uuid NOT NULL,
  command_id uuid NOT NULL,
  bird_view_revision bigint NOT NULL CHECK (bird_view_revision > 0),
  event_type text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  initiated_by_actor_id text REFERENCES actors(id),
  executed_by_actor_id text NOT NULL REFERENCES actors(id),
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (account_id,bird_view_id) REFERENCES bird_views(account_id,id) ON DELETE CASCADE,
  FOREIGN KEY (account_id,bird_view_id,bird_view_revision,command_id)
    REFERENCES bird_view_commands(account_id,bird_view_id,bird_view_revision,id)
);
CREATE INDEX bird_view_events_scope_idx ON bird_view_events(account_id,bird_view_id,bird_view_revision,id);

-- +goose Down
DROP TABLE bird_view_events;
DROP TABLE bird_view_commands;
DROP TABLE bird_view_backlog_bindings;
DROP TABLE bird_view_task_bindings;
DROP TABLE bird_view_gate_bindings;
DROP TABLE bird_view_phase_bindings;
DROP TABLE bird_view_edges;
DROP TABLE bird_view_nodes;
DROP TABLE bird_views;
