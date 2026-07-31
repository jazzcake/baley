-- +goose Up
CREATE TABLE evidence_profiles (
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  id text NOT NULL,
  version text NOT NULL,
  required_completion_reports integer NOT NULL DEFAULT 1 CHECK (required_completion_reports > 0),
  required_verifications integer NOT NULL DEFAULT 1 CHECK (required_verifications > 0),
  required_independent_reviews integer NOT NULL DEFAULT 1 CHECK (required_independent_reviews > 0),
  allowed_reference_kinds text[] NOT NULL DEFAULT ARRAY['task_record','run','commit_reference','artifact'],
  verification_reference_required boolean NOT NULL DEFAULT true,
  review_requires_zero_blockers boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id)
);

CREATE TABLE workspace_acceptance_policies (
  workspace_id text PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  policy_version text NOT NULL,
  default_mode text NOT NULL CHECK (default_mode IN ('delegated','human_required')),
  evidence_profile_id text NOT NULL,
  changed_by_actor_id text REFERENCES actors(id),
  changed_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (workspace_id,evidence_profile_id) REFERENCES evidence_profiles(workspace_id,id)
);

ALTER TABLE tasks
  ADD COLUMN requested_acceptance_mode text NOT NULL DEFAULT 'human_required'
    CHECK (requested_acceptance_mode IN ('delegated','human_required','inherit')),
  ADD COLUMN effective_acceptance_mode text NOT NULL DEFAULT 'human_required'
    CHECK (effective_acceptance_mode IN ('delegated','human_required')),
  ADD COLUMN acceptance_policy_version text NOT NULL DEFAULT 'migration-v1',
  ADD COLUMN evidence_profile_id text NOT NULL DEFAULT 'technical-v1';

CREATE TABLE task_acceptance_assignments (
  workspace_id text NOT NULL,
  id uuid NOT NULL,
  task_id text NOT NULL,
  assignment_version integer NOT NULL CHECK (assignment_version > 0),
  requested_mode text NOT NULL CHECK (requested_mode IN ('delegated','human_required','inherit')),
  effective_mode text NOT NULL CHECK (effective_mode IN ('delegated','human_required')),
  policy_version text NOT NULL,
  evidence_profile_id text NOT NULL,
  reason text,
  evidence_reference text,
  approved_by_actor_id text REFERENCES actors(id),
  supersedes_assignment_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,task_id,assignment_version),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,evidence_profile_id) REFERENCES evidence_profiles(workspace_id,id),
  FOREIGN KEY (workspace_id,supersedes_assignment_id) REFERENCES task_acceptance_assignments(workspace_id,id)
);

CREATE TABLE task_acceptance_evidence (
  workspace_id text NOT NULL,
  id uuid NOT NULL,
  task_id text NOT NULL,
  evidence_version integer NOT NULL CHECK (evidence_version > 0),
  completion_report_record_id uuid NOT NULL,
  verification_verdict text NOT NULL CHECK (verification_verdict IN ('passed','failed','unavailable')),
  verification_reference text,
  verification_reference_kind text CHECK (verification_reference_kind IN ('task_record','run','commit_reference','artifact')),
  independent_review_record_id uuid NOT NULL,
  review_verdict text NOT NULL CHECK (review_verdict IN ('pass','fail','unavailable')),
  unresolved_blocking_count integer NOT NULL CHECK (unresolved_blocking_count >= 0),
  commit_reference_id uuid,
  reported_by_actor_id text NOT NULL REFERENCES actors(id),
  reported_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id,id),
  UNIQUE (workspace_id,task_id,evidence_version),
  FOREIGN KEY (workspace_id,task_id) REFERENCES tasks(workspace_id,id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id,completion_report_record_id) REFERENCES task_record_indexes(workspace_id,id),
  FOREIGN KEY (workspace_id,independent_review_record_id) REFERENCES task_record_indexes(workspace_id,id),
  FOREIGN KEY (workspace_id,commit_reference_id) REFERENCES commit_references(workspace_id,id)
);

-- +goose StatementBegin
CREATE FUNCTION reject_acceptance_history_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'TRUNCATE' AND TG_TABLE_NAME = 'task_acceptance_assignments'
     AND NOT EXISTS (SELECT 1 FROM task_acceptance_assignments) THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'TRUNCATE' AND TG_TABLE_NAME = 'task_acceptance_evidence'
     AND NOT EXISTS (SELECT 1 FROM task_acceptance_evidence) THEN
    RETURN NULL;
  END IF;
  RAISE EXCEPTION 'acceptance history is append-only';
END $$;
-- +goose StatementEnd
CREATE TRIGGER task_acceptance_assignments_append_only
  BEFORE UPDATE OR DELETE OR TRUNCATE ON task_acceptance_assignments
  FOR EACH STATEMENT EXECUTE FUNCTION reject_acceptance_history_change();
CREATE TRIGGER task_acceptance_evidence_append_only
  BEFORE UPDATE OR DELETE OR TRUNCATE ON task_acceptance_evidence
  FOR EACH STATEMENT EXECUTE FUNCTION reject_acceptance_history_change();

-- +goose StatementBegin
CREATE FUNCTION seed_workspace_acceptance_defaults() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO evidence_profiles(workspace_id,id,version)
  VALUES (NEW.id,'technical-v1','1');
  INSERT INTO workspace_acceptance_policies(workspace_id,policy_version,default_mode,evidence_profile_id)
  VALUES (NEW.id,'migration-v1','human_required','technical-v1');
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER workspaces_acceptance_defaults
  AFTER INSERT ON workspaces
  FOR EACH ROW EXECUTE FUNCTION seed_workspace_acceptance_defaults();

INSERT INTO evidence_profiles(workspace_id,id,version)
SELECT id,'technical-v1','1' FROM workspaces;

INSERT INTO workspace_acceptance_policies(workspace_id,policy_version,default_mode,evidence_profile_id)
SELECT id,'migration-v1','human_required','technical-v1' FROM workspaces;

INSERT INTO task_acceptance_assignments(
  workspace_id,id,task_id,assignment_version,requested_mode,effective_mode,
  policy_version,evidence_profile_id,reason
)
SELECT workspace_id,gen_random_uuid(),id,1,'human_required','human_required',
       'migration-v1','technical-v1','existing-task migration'
FROM tasks;

ALTER TABLE tasks
  ADD CONSTRAINT tasks_acceptance_profile_fk
  FOREIGN KEY (workspace_id,evidence_profile_id) REFERENCES evidence_profiles(workspace_id,id);

-- +goose Down
ALTER TABLE tasks DROP CONSTRAINT tasks_acceptance_profile_fk;
DROP TRIGGER workspaces_acceptance_defaults ON workspaces;
DROP FUNCTION seed_workspace_acceptance_defaults();
DROP TRIGGER task_acceptance_evidence_append_only ON task_acceptance_evidence;
DROP TRIGGER task_acceptance_assignments_append_only ON task_acceptance_assignments;
DROP FUNCTION reject_acceptance_history_change();
DROP TABLE task_acceptance_evidence;
DROP TABLE task_acceptance_assignments;
ALTER TABLE tasks
  DROP COLUMN evidence_profile_id,
  DROP COLUMN acceptance_policy_version,
  DROP COLUMN effective_acceptance_mode,
  DROP COLUMN requested_acceptance_mode;
DROP TABLE workspace_acceptance_policies;
DROP TABLE evidence_profiles;
