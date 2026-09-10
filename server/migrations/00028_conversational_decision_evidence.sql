-- +goose Up
CREATE TABLE conversational_decision_evidence (
  id uuid PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  linked_account_id uuid NOT NULL REFERENCES accounts(id),
  linked_human_actor_id text NOT NULL REFERENCES actors(id),
  executed_by_agent_actor_id text NOT NULL REFERENCES actors(id),
  gateway_registration_id text NOT NULL REFERENCES mcp_gateway_registrations(id),
  action text NOT NULL CHECK (action='task.confirm'),
  entity_type text NOT NULL CHECK (entity_type='task'),
  entity_id text NOT NULL,
  task_public_id bigint NOT NULL CHECK (task_public_id > 0),
  workspace_revision bigint NOT NULL CHECK (workspace_revision > 0),
  command_hash text NOT NULL,
  decision_scope text NOT NULL CHECK (decision_scope IN ('task','all_awaiting_confirmation')),
  source text NOT NULL CHECK (source='conversation'),
  conversation_ref text NOT NULL CHECK (length(btrim(conversation_ref)) > 0),
  statement_hash text NOT NULL CHECK (length(statement_hash) = 64),
  idempotency_key_hash text NOT NULL CHECK (length(idempotency_key_hash) = 64),
  executed_command_id text NOT NULL UNIQUE REFERENCES commands(id),
  consumed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversational_decision_evidence_task_audit
  ON conversational_decision_evidence(workspace_id,task_public_id,consumed_at DESC);

ALTER TABLE human_approval_attestations
  ADD COLUMN decision_evidence_id uuid UNIQUE REFERENCES conversational_decision_evidence(id);

-- +goose Down
ALTER TABLE human_approval_attestations DROP COLUMN decision_evidence_id;
DROP TABLE conversational_decision_evidence;
