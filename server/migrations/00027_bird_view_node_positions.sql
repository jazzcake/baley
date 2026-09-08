-- +goose Up
ALTER TABLE bird_view_nodes
  ADD COLUMN position_x double precision NOT NULL DEFAULT 0,
  ADD COLUMN position_y double precision NOT NULL DEFAULT 0;

WITH ordered AS (
  SELECT bird_view_id, id,
         row_number() OVER (PARTITION BY bird_view_id ORDER BY created_at, id) - 1 AS ordinal
  FROM bird_view_nodes
)
UPDATE bird_view_nodes node
SET position_x = (ordered.ordinal % 4) * 320,
    position_y = floor(ordered.ordinal / 4.0) * 220
FROM ordered
WHERE node.bird_view_id = ordered.bird_view_id AND node.id = ordered.id;

ALTER TABLE bird_view_nodes
  ADD CONSTRAINT bird_view_nodes_position_x_range CHECK (position_x BETWEEN -1000000 AND 1000000),
  ADD CONSTRAINT bird_view_nodes_position_y_range CHECK (position_y BETWEEN -1000000 AND 1000000);

-- +goose Down
ALTER TABLE bird_view_nodes
  DROP CONSTRAINT bird_view_nodes_position_y_range,
  DROP CONSTRAINT bird_view_nodes_position_x_range,
  DROP COLUMN position_y,
  DROP COLUMN position_x;
