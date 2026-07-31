-- +goose Up
ALTER TABLE actors DROP CONSTRAINT actors_actor_type_check;
ALTER TABLE actors
  ADD CONSTRAINT actors_actor_type_check
  CHECK (actor_type IN ('human','agent','system'));

CREATE TABLE accounts (
  id uuid PRIMARY KEY,
  actor_id text NOT NULL UNIQUE REFERENCES actors(id),
  login_id text NOT NULL,
  normalized_login_id text NOT NULL UNIQUE,
  display_name text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz,
  CHECK (length(normalized_login_id) BETWEEN 1 AND 128),
  CHECK ((status = 'active' AND disabled_at IS NULL) OR
         (status = 'disabled' AND disabled_at IS NOT NULL))
);

CREATE TABLE account_credentials (
  account_id uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  password_phc text NOT NULL,
  credential_version bigint NOT NULL DEFAULT 1 CHECK (credential_version > 0),
  password_changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE account_sessions (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  idle_expires_at timestamptz NOT NULL,
  absolute_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CHECK (idle_expires_at <= absolute_expires_at)
);
CREATE INDEX account_sessions_active_lookup
  ON account_sessions(token_hash)
  WHERE revoked_at IS NULL;

CREATE TABLE auth_login_limits (
  key_hash bytea PRIMARY KEY CHECK (octet_length(key_hash) = 32),
  window_started_at timestamptz NOT NULL,
  failure_count integer NOT NULL CHECK (failure_count >= 0),
  blocked_until timestamptz
);

CREATE TABLE workspace_memberships (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  actor_id text NOT NULL REFERENCES actors(id),
  role text NOT NULL CHECK (role IN ('viewer','operator','approver','owner')),
  active boolean NOT NULL DEFAULT true,
  created_by_actor_id text REFERENCES actors(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deactivated_at timestamptz,
  PRIMARY KEY (workspace_id,actor_id),
  CHECK ((active AND deactivated_at IS NULL) OR
         (NOT active AND deactivated_at IS NOT NULL))
);
CREATE INDEX workspace_memberships_actor_active
  ON workspace_memberships(actor_id,workspace_id)
  WHERE active;

-- Agent and system Actors can never receive a human approval/admin role.
-- +goose StatementBegin
CREATE FUNCTION enforce_workspace_membership_actor_role() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE kind text;
BEGIN
  SELECT actor_type INTO kind FROM actors WHERE id=NEW.actor_id;
  IF kind IN ('agent','system') AND NEW.role <> 'operator' THEN
    RAISE EXCEPTION 'non-human workspace membership must be operator';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER workspace_membership_actor_role
  BEFORE INSERT OR UPDATE OF actor_id,role ON workspace_memberships
  FOR EACH ROW EXECUTE FUNCTION enforce_workspace_membership_actor_role();

-- Direct SQL cannot leave an active Workspace without an active, account-linked
-- human Owner. The deferred trigger allows an atomic Owner transfer.
-- +goose StatementBegin
CREATE FUNCTION enforce_workspace_active_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE target_workspace text;
DECLARE workspace_state text;
DECLARE owner_count integer;
BEGIN
  target_workspace := COALESCE(NEW.workspace_id,OLD.workspace_id);
  -- Serialize deferred checks for concurrent direct-SQL changes to different
  -- Owner membership rows in the same Workspace.
  PERFORM 1 FROM workspaces WHERE id=target_workspace FOR UPDATE;
  SELECT state INTO workspace_state FROM workspaces WHERE id=target_workspace;
  IF workspace_state = 'active' THEN
    SELECT count(*) INTO owner_count
    FROM workspace_memberships membership
    JOIN actors actor ON actor.id=membership.actor_id AND actor.actor_type='human'
    JOIN accounts account ON account.actor_id=membership.actor_id AND account.status='active'
    WHERE membership.workspace_id=target_workspace
      AND membership.active
      AND membership.role='owner';
    IF owner_count = 0 THEN
      RAISE EXCEPTION 'active workspace requires an active account-linked owner';
    END IF;
  END IF;
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER workspace_active_owner_membership
  AFTER INSERT OR UPDATE OR DELETE ON workspace_memberships
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION enforce_workspace_active_owner();
-- +goose StatementBegin
CREATE FUNCTION enforce_account_active_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE target_workspace text;
DECLARE owner_count integer;
BEGIN
  FOR target_workspace IN
    SELECT workspace_id
    FROM workspace_memberships
    WHERE actor_id=NEW.actor_id AND role='owner'
  LOOP
    PERFORM 1 FROM workspaces WHERE id=target_workspace FOR UPDATE;
    IF (SELECT state FROM workspaces WHERE id=target_workspace) = 'active' THEN
      SELECT count(*) INTO owner_count
      FROM workspace_memberships membership
      JOIN actors actor ON actor.id=membership.actor_id AND actor.actor_type='human'
      JOIN accounts account ON account.actor_id=membership.actor_id AND account.status='active'
      WHERE membership.workspace_id=target_workspace
        AND membership.active
        AND membership.role='owner';
      IF owner_count = 0 THEN
        RAISE EXCEPTION 'active workspace requires an active account-linked owner';
      END IF;
    END IF;
  END LOOP;
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER workspace_active_owner_account
  AFTER UPDATE OF status ON accounts
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION enforce_account_active_owner();

CREATE TABLE agent_tokens (
  id uuid PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  actor_id text NOT NULL REFERENCES actors(id),
  name text NOT NULL,
  token_prefix text NOT NULL,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  scopes jsonb NOT NULL,
  created_by_actor_id text NOT NULL REFERENCES actors(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  last_used_at timestamptz,
  revoked_at timestamptz,
  UNIQUE (workspace_id,name)
);

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
  status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','consumed','revoked')),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  consumed_by_command_id text UNIQUE REFERENCES commands(id),
  revoked_at timestamptz,
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
ALTER TABLE commands
  ADD COLUMN authenticated_credential_kind text,
  ADD COLUMN authenticated_credential_id text;

CREATE TABLE security_events (
  id uuid PRIMARY KEY,
  workspace_id text REFERENCES workspaces(id) ON DELETE CASCADE,
  account_id uuid REFERENCES accounts(id),
  actor_id text REFERENCES actors(id),
  event_type text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (NOT payload ?| ARRAY[
    'password','passwordPhc','token','tokenHash','secret','csrfToken','grantToken'
  ])
);
CREATE INDEX security_events_workspace_time
  ON security_events(workspace_id,created_at,id);
-- +goose StatementBegin
CREATE FUNCTION reject_security_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'security_events is append-only';
END $$;
-- +goose StatementEnd
CREATE TRIGGER security_events_no_update_delete
  BEFORE UPDATE OR DELETE ON security_events
  FOR EACH ROW EXECUTE FUNCTION reject_security_event_mutation();
CREATE TRIGGER security_events_no_truncate
  BEFORE TRUNCATE ON security_events
  FOR EACH STATEMENT EXECUTE FUNCTION reject_security_event_mutation();

-- +goose Down
DROP TRIGGER security_events_no_truncate ON security_events;
DROP TRIGGER security_events_no_update_delete ON security_events;
DROP FUNCTION reject_security_event_mutation();
DROP TABLE security_events;
ALTER TABLE commands
  DROP COLUMN authenticated_credential_id,
  DROP COLUMN authenticated_credential_kind;
ALTER TABLE human_approval_attestations DROP COLUMN approval_grant_id;
DROP TABLE approval_grants;
DROP TABLE agent_tokens;
DROP TRIGGER workspace_active_owner_account ON accounts;
DROP FUNCTION enforce_account_active_owner();
DROP TRIGGER workspace_active_owner_membership ON workspace_memberships;
DROP FUNCTION enforce_workspace_active_owner();
DROP TRIGGER workspace_membership_actor_role ON workspace_memberships;
DROP FUNCTION enforce_workspace_membership_actor_role();
DROP TABLE workspace_memberships;
DROP TABLE auth_login_limits;
DROP TABLE account_sessions;
DROP TABLE account_credentials;
DROP TABLE accounts;
ALTER TABLE actors DROP CONSTRAINT actors_actor_type_check;
ALTER TABLE actors
  ADD CONSTRAINT actors_actor_type_check
  CHECK (actor_type IN ('human','agent'));
