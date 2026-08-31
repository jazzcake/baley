-- +goose Up
CREATE TABLE approval_grants (
  id uuid PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  approved_by_account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  approved_by_actor_id text NOT NULL REFERENCES actors(id),
  approved_by_session_id uuid NOT NULL REFERENCES account_sessions(id) ON DELETE CASCADE,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  workspace_revision bigint NOT NULL CHECK (workspace_revision > 0),
  command_hash text NOT NULL,
  decision_snapshot_hash text,
  warning_codes jsonb NOT NULL DEFAULT '[]'::jsonb,
  proceed_reason_digest text NOT NULL,
  status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','consumed','revoked','expired')),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  consumed_by_command_id text UNIQUE REFERENCES commands(id),
  revoked_at timestamptz,
  expired_at timestamptz,
  CHECK (
    (status='active' AND consumed_at IS NULL AND consumed_by_command_id IS NULL AND revoked_at IS NULL AND expired_at IS NULL) OR
    (status='consumed' AND consumed_at IS NOT NULL AND consumed_by_command_id IS NOT NULL AND revoked_at IS NULL AND expired_at IS NULL) OR
    (status='revoked' AND consumed_at IS NULL AND consumed_by_command_id IS NULL AND revoked_at IS NOT NULL AND expired_at IS NULL) OR
    (status='expired' AND consumed_at IS NULL AND consumed_by_command_id IS NULL AND revoked_at IS NULL AND expired_at IS NOT NULL)
  )
);
CREATE INDEX approval_grants_active_lookup
  ON approval_grants(workspace_id,id)
  WHERE status='active';
CREATE INDEX approval_grants_session_active
  ON approval_grants(approved_by_session_id)
  WHERE status='active';

ALTER TABLE human_approval_attestations
  ADD COLUMN approval_grant_id uuid UNIQUE REFERENCES approval_grants(id);

-- Revoking the browser session that issued a grant revokes every unused grant
-- from that session, even when the session is revoked by direct SQL.
-- +goose StatementBegin
CREATE FUNCTION revoke_session_approval_grants() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL THEN
    WITH revoked AS (
      UPDATE approval_grants
      SET status='revoked',revoked_at=NEW.revoked_at
      WHERE approved_by_session_id=NEW.id AND status='active'
      RETURNING id,workspace_id,approved_by_account_id,approved_by_actor_id
    )
    INSERT INTO security_events(id,workspace_id,account_id,actor_id,event_type,entity_type,entity_id,payload)
    SELECT gen_random_uuid(),workspace_id,approved_by_account_id,approved_by_actor_id,
           'approval_grant.revoked','approval_grant',id::text,
           jsonb_build_object('reason','session_revoked','sessionId',NEW.id::text)
    FROM revoked;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER account_sessions_revoke_approval_grants
  AFTER UPDATE OF revoked_at ON account_sessions
  FOR EACH ROW EXECUTE FUNCTION revoke_session_approval_grants();

-- Membership removal or deactivation immediately invalidates outstanding
-- approval authority for that Workspace and Actor.
-- +goose StatementBegin
CREATE FUNCTION revoke_membership_approval_grants() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target_workspace text;
DECLARE target_actor text;
BEGIN
  target_workspace := COALESCE(NEW.workspace_id,OLD.workspace_id);
  target_actor := COALESCE(NEW.actor_id,OLD.actor_id);
  IF TG_OP='DELETE' OR (OLD.active AND NOT NEW.active) THEN
    WITH revoked AS (
      UPDATE approval_grants
      SET status='revoked',revoked_at=now()
      WHERE workspace_id=target_workspace AND approved_by_actor_id=target_actor AND status='active'
      RETURNING id,workspace_id,approved_by_account_id,approved_by_actor_id
    )
    INSERT INTO security_events(id,workspace_id,account_id,actor_id,event_type,entity_type,entity_id,payload)
    SELECT gen_random_uuid(),workspace_id,approved_by_account_id,approved_by_actor_id,
           'approval_grant.revoked','approval_grant',id::text,
           jsonb_build_object('reason','membership_revoked')
    FROM revoked;
  END IF;
  IF TG_OP='DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER workspace_memberships_revoke_approval_grants
  AFTER UPDATE OF active OR DELETE ON workspace_memberships
  FOR EACH ROW EXECUTE FUNCTION revoke_membership_approval_grants();

-- Delegated acceptance no longer exists as current authority. Preserve the
-- append-only assignment history and append a human-required migration row.
UPDATE workspace_acceptance_policies
SET default_mode='human_required',policy_version='human-approval-p0-v1',changed_at=now()
WHERE default_mode='delegated';

INSERT INTO task_acceptance_assignments(
  workspace_id,id,task_id,assignment_version,requested_mode,effective_mode,
  policy_version,evidence_profile_id,reason,supersedes_assignment_id
)
SELECT task.workspace_id,gen_random_uuid(),task.id,latest.assignment_version+1,
       'human_required','human_required','human-approval-p0-v1',task.evidence_profile_id,
       'P0 migration: delegated acceptance removed',latest.id
FROM tasks task
JOIN LATERAL (
  SELECT assignment.id,assignment.assignment_version
  FROM task_acceptance_assignments assignment
  WHERE assignment.workspace_id=task.workspace_id AND assignment.task_id=task.id
  ORDER BY assignment.assignment_version DESC LIMIT 1
) latest ON true
WHERE task.effective_acceptance_mode='delegated';

UPDATE tasks
SET requested_acceptance_mode='human_required',effective_acceptance_mode='human_required',
    acceptance_policy_version='human-approval-p0-v1',updated_at=now()
WHERE requested_acceptance_mode='delegated' OR effective_acceptance_mode='delegated';

ALTER TABLE tasks DROP CONSTRAINT tasks_requested_acceptance_mode_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_requested_acceptance_mode_check
  CHECK (requested_acceptance_mode IN ('human_required','inherit'));
ALTER TABLE tasks DROP CONSTRAINT tasks_effective_acceptance_mode_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_effective_acceptance_mode_check
  CHECK (effective_acceptance_mode='human_required');
ALTER TABLE workspace_acceptance_policies DROP CONSTRAINT workspace_acceptance_policies_default_mode_check;
ALTER TABLE workspace_acceptance_policies ADD CONSTRAINT workspace_acceptance_policies_default_mode_check
  CHECK (default_mode='human_required');

-- +goose Down
ALTER TABLE workspace_acceptance_policies DROP CONSTRAINT workspace_acceptance_policies_default_mode_check;
ALTER TABLE workspace_acceptance_policies ADD CONSTRAINT workspace_acceptance_policies_default_mode_check
  CHECK (default_mode IN ('delegated','human_required'));
ALTER TABLE tasks DROP CONSTRAINT tasks_effective_acceptance_mode_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_effective_acceptance_mode_check
  CHECK (effective_acceptance_mode IN ('delegated','human_required'));
ALTER TABLE tasks DROP CONSTRAINT tasks_requested_acceptance_mode_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_requested_acceptance_mode_check
  CHECK (requested_acceptance_mode IN ('delegated','human_required','inherit'));
DROP TRIGGER workspace_memberships_revoke_approval_grants ON workspace_memberships;
DROP FUNCTION revoke_membership_approval_grants();
DROP TRIGGER account_sessions_revoke_approval_grants ON account_sessions;
DROP FUNCTION revoke_session_approval_grants();
ALTER TABLE human_approval_attestations DROP COLUMN approval_grant_id;
DROP TABLE approval_grants;
