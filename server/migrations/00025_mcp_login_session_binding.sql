-- +goose Up
-- Codes created before session binding cannot be proven to belong to a live
-- browser session. Fail closed instead of carrying them across the upgrade.
DELETE FROM mcp_connection_requests
  WHERE status = 'linked' OR (status = 'pending' AND login_code_hash IS NOT NULL);

ALTER TABLE mcp_connection_requests
  DROP CONSTRAINT mcp_connection_requests_status_check,
  DROP CONSTRAINT mcp_connection_requests_link_state_check;

ALTER TABLE mcp_connection_requests
  ADD COLUMN login_session_id uuid REFERENCES account_sessions(id) ON DELETE CASCADE;

CREATE INDEX mcp_connection_requests_login_session_pending
  ON mcp_connection_requests(login_session_id)
  WHERE status = 'pending' AND login_session_id IS NOT NULL;

ALTER TABLE mcp_connection_requests
  ADD CONSTRAINT mcp_connection_requests_status_check
    CHECK (status IN ('pending','consumed')),
  ADD CONSTRAINT mcp_connection_requests_link_state_check
    CHECK ((status='pending' AND linked_at IS NULL AND consumed_at IS NULL AND linked_by_actor_id IS NULL AND
             ((login_code_hash IS NULL AND login_code_expires_at IS NULL AND login_actor_id IS NULL AND login_session_id IS NULL) OR
              (login_code_hash IS NOT NULL AND login_code_expires_at IS NOT NULL AND login_actor_id IS NOT NULL AND login_session_id IS NOT NULL))) OR
           (status='consumed' AND linked_at IS NOT NULL AND consumed_at IS NOT NULL AND linked_by_actor_id IS NOT NULL));

-- +goose Down
ALTER TABLE mcp_connection_requests
  DROP CONSTRAINT mcp_connection_requests_status_check,
  DROP CONSTRAINT mcp_connection_requests_link_state_check;

DROP INDEX mcp_connection_requests_login_session_pending;

ALTER TABLE mcp_connection_requests
  DROP COLUMN login_session_id;

ALTER TABLE mcp_connection_requests
  ADD CONSTRAINT mcp_connection_requests_status_check
    CHECK (status IN ('pending','linked','consumed')),
  ADD CONSTRAINT mcp_connection_requests_link_state_check
    CHECK ((status='pending' AND linked_at IS NULL AND consumed_at IS NULL AND linked_by_actor_id IS NULL AND
             ((login_code_hash IS NULL AND login_code_expires_at IS NULL AND login_actor_id IS NULL) OR
              (login_code_hash IS NOT NULL AND login_code_expires_at IS NOT NULL AND login_actor_id IS NOT NULL))) OR
           (status='linked' AND linked_at IS NOT NULL AND consumed_at IS NULL AND linked_by_actor_id IS NOT NULL) OR
           (status='consumed' AND linked_at IS NOT NULL AND consumed_at IS NOT NULL AND linked_by_actor_id IS NOT NULL));
