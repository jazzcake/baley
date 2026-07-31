-- +goose Up
CREATE TABLE gate_entry_tasks (
  workspace_id text NOT NULL,
  gate_id text NOT NULL,
  task_id text NOT NULL,
  selection_source text NOT NULL DEFAULT 'explicit' CHECK (selection_source = 'explicit'),
  PRIMARY KEY (workspace_id,gate_id,task_id),
  FOREIGN KEY (workspace_id,gate_id) REFERENCES gates(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE
);

-- +goose StatementBegin
CREATE FUNCTION enforce_gate_entry_task_to_phase() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE gate_phase text; task_phase text;
BEGIN
  SELECT to_phase_id INTO gate_phase FROM gates WHERE workspace_id=NEW.workspace_id AND id=NEW.gate_id;
  SELECT phase_id INTO task_phase FROM tasks WHERE workspace_id=NEW.workspace_id AND id=NEW.task_id;
  IF gate_phase IS DISTINCT FROM task_phase THEN RAISE EXCEPTION 'gate entry task must belong to gate to phase'; END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER gate_entry_tasks_to_phase BEFORE INSERT OR UPDATE ON gate_entry_tasks FOR EACH ROW EXECUTE FUNCTION enforce_gate_entry_task_to_phase();

-- +goose Down
DROP TRIGGER gate_entry_tasks_to_phase ON gate_entry_tasks;
DROP FUNCTION enforce_gate_entry_task_to_phase();
DROP TABLE gate_entry_tasks;
