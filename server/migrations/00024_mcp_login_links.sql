-- +goose Up
DELETE FROM mcp_connection_requests WHERE status = 'rejected';

ALTER TABLE mcp_connection_requests
  DROP CONSTRAINT mcp_connection_requests_status_check,
  DROP CONSTRAINT mcp_connection_requests_check1;

ALTER TABLE mcp_connection_requests
  RENAME COLUMN approved_by_actor_id TO linked_by_actor_id;
ALTER TABLE mcp_connection_requests
  RENAME COLUMN approved_at TO linked_at;
ALTER TABLE mcp_connection_requests
  DROP COLUMN rejected_at,
  ADD COLUMN login_code_hash bytea,
  ADD COLUMN login_code_expires_at timestamptz,
  ADD COLUMN login_actor_id text REFERENCES actors(id);

UPDATE mcp_connection_requests SET status = 'linked' WHERE status = 'approved';

ALTER TABLE mcp_connection_requests
  ADD CONSTRAINT mcp_connection_requests_status_check
    CHECK (status IN ('pending','linked','consumed')),
  ADD CONSTRAINT mcp_connection_requests_link_state_check
    CHECK ((status='pending' AND linked_at IS NULL AND consumed_at IS NULL AND linked_by_actor_id IS NULL AND
             ((login_code_hash IS NULL AND login_code_expires_at IS NULL AND login_actor_id IS NULL) OR
              (login_code_hash IS NOT NULL AND login_code_expires_at IS NOT NULL AND login_actor_id IS NOT NULL))) OR
           (status='linked' AND linked_at IS NOT NULL AND consumed_at IS NULL AND linked_by_actor_id IS NOT NULL) OR
           (status='consumed' AND linked_at IS NOT NULL AND consumed_at IS NOT NULL AND linked_by_actor_id IS NOT NULL));

-- +goose Down
ALTER TABLE mcp_connection_requests
  DROP CONSTRAINT mcp_connection_requests_status_check,
  DROP CONSTRAINT mcp_connection_requests_link_state_check;

UPDATE mcp_connection_requests SET status = 'approved' WHERE status = 'linked';

ALTER TABLE mcp_connection_requests
  ADD COLUMN rejected_at timestamptz;
ALTER TABLE mcp_connection_requests
  DROP COLUMN login_code_hash,
  DROP COLUMN login_code_expires_at,
  DROP COLUMN login_actor_id;
ALTER TABLE mcp_connection_requests
  RENAME COLUMN linked_at TO approved_at;
ALTER TABLE mcp_connection_requests
  RENAME COLUMN linked_by_actor_id TO approved_by_actor_id;

ALTER TABLE mcp_connection_requests
  ADD CONSTRAINT mcp_connection_requests_status_check
    CHECK (status IN ('pending','approved','rejected','consumed')),
  ADD CONSTRAINT mcp_connection_requests_check1
    CHECK ((status='pending' AND approved_at IS NULL AND rejected_at IS NULL AND consumed_at IS NULL AND approved_by_actor_id IS NULL) OR
           (status='approved' AND approved_at IS NOT NULL AND rejected_at IS NULL AND consumed_at IS NULL AND approved_by_actor_id IS NOT NULL) OR
           (status='rejected' AND approved_at IS NULL AND rejected_at IS NOT NULL AND consumed_at IS NULL AND approved_by_actor_id IS NOT NULL) OR
           (status='consumed' AND approved_at IS NOT NULL AND rejected_at IS NULL AND consumed_at IS NOT NULL AND approved_by_actor_id IS NOT NULL));
