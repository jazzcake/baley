-- +goose Up
-- Bounds explicit Phase Task expansion by its selection and cursor order.
CREATE INDEX tasks_workspace_phase_public_id_idx ON tasks(workspace_id, phase_id, public_id);

-- +goose Down
DROP INDEX IF EXISTS tasks_workspace_phase_public_id_idx;
