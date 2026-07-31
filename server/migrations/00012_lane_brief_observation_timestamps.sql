-- +goose Up
ALTER TABLE phases ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE lanes ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE task_dependencies ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE gates ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE gate_tasks ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- +goose StatementBegin
CREATE FUNCTION touch_lane_brief_entity_timestamp() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER phases_touch_lane_brief_timestamp
  BEFORE UPDATE ON phases FOR EACH ROW EXECUTE FUNCTION touch_lane_brief_entity_timestamp();
CREATE TRIGGER lanes_touch_lane_brief_timestamp
  BEFORE UPDATE ON lanes FOR EACH ROW EXECUTE FUNCTION touch_lane_brief_entity_timestamp();
CREATE TRIGGER gates_touch_lane_brief_timestamp
  BEFORE UPDATE ON gates FOR EACH ROW EXECUTE FUNCTION touch_lane_brief_entity_timestamp();
CREATE TRIGGER gate_tasks_touch_lane_brief_timestamp
  BEFORE UPDATE ON gate_tasks FOR EACH ROW EXECUTE FUNCTION touch_lane_brief_entity_timestamp();

-- +goose Down
DROP TRIGGER gate_tasks_touch_lane_brief_timestamp ON gate_tasks;
DROP TRIGGER gates_touch_lane_brief_timestamp ON gates;
DROP TRIGGER lanes_touch_lane_brief_timestamp ON lanes;
DROP TRIGGER phases_touch_lane_brief_timestamp ON phases;
DROP FUNCTION touch_lane_brief_entity_timestamp();
ALTER TABLE gate_tasks DROP COLUMN updated_at;
ALTER TABLE gates DROP COLUMN updated_at;
ALTER TABLE task_dependencies DROP COLUMN created_at;
ALTER TABLE lanes DROP COLUMN updated_at;
ALTER TABLE phases DROP COLUMN updated_at;
