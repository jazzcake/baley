-- +goose Up
ALTER TABLE human_approval_attestations
  DROP COLUMN approval_grant_id;

DROP TABLE approval_grants;

-- +goose Down
CREATE TABLE approval_grants (
  id uuid PRIMARY KEY,
  secret_hash bytea NOT NULL UNIQUE CHECK (octet_length(secret_hash) = 32),
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  approved_by_account_id uuid NOT NULL REFERENCES accounts(id),
  approved_by_actor_id text NOT NULL REFERENCES actors(id),
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  workspace_revision bigint NOT NULL CHECK (workspace_revision > 0),
  command_hash text NOT NULL,
  decision_snapshot_hash text,
  warning_codes jsonb NOT NULL DEFAULT '[]'::jsonb,
  proceed_reason_digest text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','consumed','revoked')),
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  consumed_by_command_id text REFERENCES commands(id),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (status='active' AND consumed_at IS NULL AND consumed_by_command_id IS NULL AND revoked_at IS NULL) OR
    (status='consumed' AND consumed_at IS NOT NULL AND consumed_by_command_id IS NOT NULL AND revoked_at IS NULL) OR
    (status='revoked' AND consumed_at IS NULL AND consumed_by_command_id IS NULL AND revoked_at IS NOT NULL)
  )
);

CREATE INDEX approval_grants_active_lookup
  ON approval_grants(workspace_id,id)
  WHERE status='active';

ALTER TABLE human_approval_attestations
  ADD COLUMN approval_grant_id uuid UNIQUE REFERENCES approval_grants(id);
