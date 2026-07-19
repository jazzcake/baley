-- +goose Up
ALTER TABLE lanes
  ADD COLUMN goal text NOT NULL DEFAULT '',
  ADD COLUMN summary text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE lanes DROP COLUMN summary, DROP COLUMN goal;
