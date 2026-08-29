-- +goose Up
ALTER TABLE workspaces DROP CONSTRAINT workspaces_state_check;
ALTER TABLE workspaces
  ADD CONSTRAINT workspaces_state_check
  CHECK (state IN ('draft','active','closed','archived'));

-- +goose Down
ALTER TABLE workspaces DROP CONSTRAINT workspaces_state_check;
ALTER TABLE workspaces
  ADD CONSTRAINT workspaces_state_check
  CHECK (state IN ('draft','active','closed'));
