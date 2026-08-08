-- +goose Up
CREATE TABLE mcp_connection_requests (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  agent_actor_id text NOT NULL REFERENCES actors(id),
  secret_hash bytea NOT NULL CHECK (octet_length(secret_hash) = 32),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','consumed')),
  approved_by_actor_id text REFERENCES actors(id),
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  approved_at timestamptz,
  rejected_at timestamptz,
  consumed_at timestamptz,
  CHECK (expires_at > created_at),
  CHECK ((status='pending' AND approved_at IS NULL AND rejected_at IS NULL AND consumed_at IS NULL AND approved_by_actor_id IS NULL) OR
         (status='approved' AND approved_at IS NOT NULL AND rejected_at IS NULL AND consumed_at IS NULL AND approved_by_actor_id IS NOT NULL) OR
         (status='rejected' AND approved_at IS NULL AND rejected_at IS NOT NULL AND consumed_at IS NULL AND approved_by_actor_id IS NOT NULL) OR
         (status='consumed' AND approved_at IS NOT NULL AND rejected_at IS NULL AND consumed_at IS NOT NULL AND approved_by_actor_id IS NOT NULL))
);
CREATE INDEX mcp_connection_requests_workspace_lookup ON mcp_connection_requests(workspace_id,id);
CREATE INDEX mcp_connection_requests_expiry_lookup ON mcp_connection_requests(expires_at);

-- +goose Down
DROP TABLE mcp_connection_requests;
