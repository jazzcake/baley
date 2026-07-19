-- +goose Up
CREATE TABLE repositories (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (length(trim(name)) > 0),
  remote_url text NOT NULL CHECK (length(trim(remote_url)) > 0),
  default_branch text,
  is_record_repository boolean NOT NULL DEFAULT false,
  task_records_root text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  CHECK ((is_record_repository AND task_records_root IS NOT NULL AND length(trim(task_records_root)) > 0) OR
         (NOT is_record_repository AND task_records_root IS NULL))
);

CREATE TABLE task_record_indexes (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  task_id text NOT NULL,
  run_id text,
  record_type text NOT NULL CHECK (record_type IN ('detailed-plan','handoff','independent-agent-review','review-response','completion-report')),
  repository_id uuid NOT NULL,
  relative_path text NOT NULL,
  working_tree_hash text,
  commit_sha text,
  blob_sha text,
  state text NOT NULL CHECK (state IN ('reported_uncommitted','committed_unverified','verified')),
  short_summary text NOT NULL CHECK (length(trim(short_summary)) > 0),
  supersedes_record_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,repository_id,relative_path),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id),
  FOREIGN KEY (workspace_id,run_id) REFERENCES runs(workspace_id,id),
  FOREIGN KEY (workspace_id,repository_id) REFERENCES repositories(workspace_id,id),
  FOREIGN KEY (workspace_id,supersedes_record_id) REFERENCES task_record_indexes(workspace_id,id),
  CHECK ((state='reported_uncommitted' AND commit_sha IS NULL AND blob_sha IS NULL) OR
         (state IN ('committed_unverified','verified') AND commit_sha IS NOT NULL AND blob_sha IS NOT NULL))
);

CREATE TABLE commit_references (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  task_id text NOT NULL,
  run_id text,
  repository_id uuid NOT NULL,
  commit_sha text NOT NULL,
  relation text NOT NULL CHECK (relation IN ('base','produced','reviewed','superseded')),
  verification_state text NOT NULL CHECK (verification_state IN ('reported','remote_verified')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,task_id,repository_id,commit_sha,relation),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id),
  FOREIGN KEY (workspace_id,run_id) REFERENCES runs(workspace_id,id),
  FOREIGN KEY (workspace_id,repository_id) REFERENCES repositories(workspace_id,id)
);

CREATE TABLE run_git_observations (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  run_id text NOT NULL,
  repository_id uuid NOT NULL,
  observed_at timestamptz NOT NULL,
  head_commit_sha text,
  branch_hint text,
  worktree_label text,
  dirty boolean,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  FOREIGN KEY (workspace_id,run_id) REFERENCES runs(workspace_id,id),
  FOREIGN KEY (workspace_id,repository_id) REFERENCES repositories(workspace_id,id)
);

-- +goose Down
DROP TABLE run_git_observations;
DROP TABLE commit_references;
DROP TABLE task_record_indexes;
DROP TABLE repositories;
