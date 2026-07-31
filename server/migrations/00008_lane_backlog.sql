-- +goose Up
ALTER TABLE workspace_counters
  ADD COLUMN next_backlog_public_id integer NOT NULL DEFAULT 1 CHECK (next_backlog_public_id > 0);

CREATE TABLE backlog_items (
  workspace_id text NOT NULL,
  id text NOT NULL,
  public_id integer NOT NULL CHECK (public_id > 0),
  lane_id text NOT NULL,
  title text NOT NULL CHECK (btrim(title) <> ''),
  description text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('active','promoted','discarded')),
  position integer,
  promoted_task_id text,
  discard_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,public_id),
  FOREIGN KEY (workspace_id,lane_id) REFERENCES lanes(workspace_id,id),
  FOREIGN KEY (workspace_id,promoted_task_id) REFERENCES tasks(workspace_id,id),
  CHECK (position IS NULL OR position > 0),
  CHECK (
    (status='active' AND position IS NOT NULL AND promoted_task_id IS NULL AND discard_reason IS NULL) OR
    (status='promoted' AND position IS NULL AND promoted_task_id IS NOT NULL AND discard_reason IS NULL) OR
    (status='discarded' AND position IS NULL AND promoted_task_id IS NULL AND btrim(discard_reason) <> '')
  )
);
CREATE UNIQUE INDEX backlog_items_active_position_uq ON backlog_items(workspace_id,lane_id,position) WHERE status='active';
CREATE UNIQUE INDEX backlog_items_promoted_task_uq ON backlog_items(workspace_id,promoted_task_id) WHERE promoted_task_id IS NOT NULL;
CREATE INDEX backlog_items_active_lane_idx ON backlog_items(workspace_id,lane_id,position,public_id) WHERE status='active';

-- +goose Down
DROP TABLE backlog_items;
ALTER TABLE workspace_counters DROP COLUMN next_backlog_public_id;
