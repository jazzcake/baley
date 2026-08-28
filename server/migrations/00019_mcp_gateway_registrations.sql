-- +goose Up
ALTER TABLE mcp_connection_requests
  ADD COLUMN gateway_id text NOT NULL DEFAULT '';

CREATE TABLE mcp_gateway_registrations (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  account_actor_id text NOT NULL REFERENCES actors(id),
  agent_actor_id text NOT NULL REFERENCES actors(id),
  gateway_id text NOT NULL,
  gateway_secret_hash bytea NOT NULL CHECK (octet_length(gateway_secret_hash) = 32),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked','replaced')),
  generation integer NOT NULL DEFAULT 1 CHECK (generation > 0),
  created_at timestamptz NOT NULL,
  revoked_at timestamptz,
  revoked_by_actor_id text REFERENCES actors(id),
  revoke_reason text,
  UNIQUE(workspace_id, gateway_id),
  CHECK ((status='active' AND revoked_at IS NULL AND revoked_by_actor_id IS NULL AND revoke_reason IS NULL) OR
         (status IN ('revoked','replaced') AND revoked_at IS NOT NULL AND revoked_by_actor_id IS NOT NULL AND revoke_reason IS NOT NULL))
);
CREATE INDEX mcp_gateway_registrations_member_lookup
  ON mcp_gateway_registrations(workspace_id, account_actor_id) WHERE status='active';

ALTER TABLE agent_tokens
  ADD COLUMN gateway_registration_id text REFERENCES mcp_gateway_registrations(id);
CREATE INDEX agent_tokens_gateway_registration_lookup
  ON agent_tokens(gateway_registration_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX agent_tokens_gateway_registration_lookup;
ALTER TABLE agent_tokens DROP COLUMN gateway_registration_id;
DROP INDEX mcp_gateway_registrations_member_lookup;
DROP TABLE mcp_gateway_registrations;
ALTER TABLE mcp_connection_requests DROP COLUMN gateway_id;
