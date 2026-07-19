-- +goose Up
ALTER TABLE tasks
  ADD COLUMN parent_task_id text,
  ADD COLUMN current_summary text NOT NULL DEFAULT '',
  ADD COLUMN next_action text NOT NULL DEFAULT '',
  ADD COLUMN terminal_reason text,
  ADD COLUMN implemented_assessment text,
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
  ADD CONSTRAINT tasks_parent_fk
    FOREIGN KEY (workspace_id,parent_task_id) REFERENCES tasks(workspace_id,id),
  ADD CONSTRAINT tasks_terminal_reason_nonblank
    CHECK (terminal_reason IS NULL OR length(trim(terminal_reason)) > 0),
  ADD CONSTRAINT tasks_implemented_assessment_nonblank
    CHECK (implemented_assessment IS NULL OR length(trim(implemented_assessment)) > 0);

-- +goose Down
ALTER TABLE tasks
  DROP CONSTRAINT tasks_implemented_assessment_nonblank,
  DROP CONSTRAINT tasks_terminal_reason_nonblank,
  DROP CONSTRAINT tasks_parent_fk,
  DROP COLUMN updated_at,
  DROP COLUMN implemented_assessment,
  DROP COLUMN terminal_reason,
  DROP COLUMN next_action,
  DROP COLUMN current_summary,
  DROP COLUMN parent_task_id;
