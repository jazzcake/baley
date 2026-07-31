-- +goose Up
ALTER TABLE gate_entry_tasks DROP CONSTRAINT gate_entry_tasks_selection_source_check;
-- Versions <=12 permitted persisted "automatic" rows even though automatic
-- entries are a read-only DAG-root projection. Remove that legacy cache-like
-- state before enforcing explicit-only persistence.
DELETE FROM gate_entry_tasks WHERE selection_source <> 'explicit';
ALTER TABLE gate_entry_tasks ADD CONSTRAINT gate_entry_tasks_selection_source_check CHECK (selection_source = 'explicit');

-- +goose Down
ALTER TABLE gate_entry_tasks DROP CONSTRAINT gate_entry_tasks_selection_source_check;
ALTER TABLE gate_entry_tasks ADD CONSTRAINT gate_entry_tasks_selection_source_check CHECK (selection_source IN ('explicit','automatic'));
