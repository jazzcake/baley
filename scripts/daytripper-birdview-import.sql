\set ON_ERROR_STOP on
\set QUIET on

BEGIN;

-- The direct-SQL Task audit trigger is for accidental product mutations. This
-- transaction is an explicit snapshot import, so suppress synthetic audit rows
-- rather than creating a misleading clone history.
SET LOCAL baley.mutation_attempt_id = 'daytripper-snapshot-import';

CREATE TEMP TABLE daytripper_snapshot(payload jsonb NOT NULL);
COPY daytripper_snapshot(payload) FROM :'snapshot_path';

CREATE TEMP TABLE clone_assertion(label text NOT NULL, ok boolean NOT NULL CHECK (ok));
INSERT INTO clone_assertion
SELECT 'snapshot_identity', count(*) = 1
  AND min(payload->>'formatVersion') = '1'
  AND min(payload#>>'{source,workspaceId}') = :'expected_workspace_id'
  AND min(payload#>>'{source,workspaceName}') = :'expected_workspace_name'
  AND min(payload#>>'{source,schemaVersion}') = '25'
FROM daytripper_snapshot;

INSERT INTO actors(id,display_name,actor_type)
VALUES(:'review_actor_id', :'review_display_name', 'human');

INSERT INTO workspaces SELECT * FROM jsonb_populate_recordset(NULL::workspaces, (SELECT payload#>'{tables,workspaces}' FROM daytripper_snapshot));
INSERT INTO workspace_counters SELECT * FROM jsonb_populate_recordset(NULL::workspace_counters, (SELECT payload#>'{tables,workspace_counters}' FROM daytripper_snapshot));
INSERT INTO phases SELECT * FROM jsonb_populate_recordset(NULL::phases, (SELECT payload#>'{tables,phases}' FROM daytripper_snapshot));
INSERT INTO lanes SELECT * FROM jsonb_populate_recordset(NULL::lanes, (SELECT payload#>'{tables,lanes}' FROM daytripper_snapshot));
-- Migration 10 installs an AFTER INSERT default trigger. Remove only those
-- target-generated defaults before restoring the source policy snapshot.
DELETE FROM workspace_acceptance_policies;
DELETE FROM evidence_profiles;
INSERT INTO evidence_profiles SELECT * FROM jsonb_populate_recordset(NULL::evidence_profiles, (SELECT payload#>'{tables,evidence_profiles}' FROM daytripper_snapshot));
INSERT INTO workspace_acceptance_policies SELECT * FROM jsonb_populate_recordset(NULL::workspace_acceptance_policies, (SELECT payload#>'{tables,workspace_acceptance_policies}' FROM daytripper_snapshot));
INSERT INTO tasks SELECT * FROM jsonb_populate_recordset(NULL::tasks, (SELECT payload#>'{tables,tasks}' FROM daytripper_snapshot));
INSERT INTO task_dependencies SELECT * FROM jsonb_populate_recordset(NULL::task_dependencies, (SELECT payload#>'{tables,task_dependencies}' FROM daytripper_snapshot));
INSERT INTO backlog_items SELECT * FROM jsonb_populate_recordset(NULL::backlog_items, (SELECT payload#>'{tables,backlog_items}' FROM daytripper_snapshot));
INSERT INTO gates SELECT * FROM jsonb_populate_recordset(NULL::gates, (SELECT payload#>'{tables,gates}' FROM daytripper_snapshot));
INSERT INTO gate_tasks SELECT * FROM jsonb_populate_recordset(NULL::gate_tasks, (SELECT payload#>'{tables,gate_tasks}' FROM daytripper_snapshot));
INSERT INTO gate_entry_tasks SELECT * FROM jsonb_populate_recordset(NULL::gate_entry_tasks, (SELECT payload#>'{tables,gate_entry_tasks}' FROM daytripper_snapshot));
INSERT INTO repositories SELECT * FROM jsonb_populate_recordset(NULL::repositories, (SELECT payload#>'{tables,repositories}' FROM daytripper_snapshot));
INSERT INTO runs SELECT * FROM jsonb_populate_recordset(NULL::runs, (SELECT payload#>'{tables,runs}' FROM daytripper_snapshot));
INSERT INTO run_git_observations SELECT * FROM jsonb_populate_recordset(NULL::run_git_observations, (SELECT payload#>'{tables,run_git_observations}' FROM daytripper_snapshot));
INSERT INTO commit_references SELECT * FROM jsonb_populate_recordset(NULL::commit_references, (SELECT payload#>'{tables,commit_references}' FROM daytripper_snapshot));
INSERT INTO task_record_indexes SELECT * FROM jsonb_populate_recordset(NULL::task_record_indexes, (SELECT payload#>'{tables,task_record_indexes}' FROM daytripper_snapshot));
INSERT INTO task_acceptance_assignments SELECT * FROM jsonb_populate_recordset(NULL::task_acceptance_assignments, (SELECT payload#>'{tables,task_acceptance_assignments}' FROM daytripper_snapshot));
INSERT INTO task_acceptance_evidence SELECT * FROM jsonb_populate_recordset(NULL::task_acceptance_evidence, (SELECT payload#>'{tables,task_acceptance_evidence}' FROM daytripper_snapshot));

INSERT INTO clone_assertion
SELECT 'count_parity',
  (SELECT count(*) FROM workspaces) = jsonb_array_length(payload#>'{tables,workspaces}') AND
  (SELECT count(*) FROM workspace_counters) = jsonb_array_length(payload#>'{tables,workspace_counters}') AND
  (SELECT count(*) FROM phases) = jsonb_array_length(payload#>'{tables,phases}') AND
  (SELECT count(*) FROM lanes) = jsonb_array_length(payload#>'{tables,lanes}') AND
  (SELECT count(*) FROM evidence_profiles) = jsonb_array_length(payload#>'{tables,evidence_profiles}') AND
  (SELECT count(*) FROM workspace_acceptance_policies) = jsonb_array_length(payload#>'{tables,workspace_acceptance_policies}') AND
  (SELECT count(*) FROM tasks) = jsonb_array_length(payload#>'{tables,tasks}') AND
  (SELECT count(*) FROM task_dependencies) = jsonb_array_length(payload#>'{tables,task_dependencies}') AND
  (SELECT count(*) FROM backlog_items) = jsonb_array_length(payload#>'{tables,backlog_items}') AND
  (SELECT count(*) FROM gates) = jsonb_array_length(payload#>'{tables,gates}') AND
  (SELECT count(*) FROM gate_tasks) = jsonb_array_length(payload#>'{tables,gate_tasks}') AND
  (SELECT count(*) FROM gate_entry_tasks) = jsonb_array_length(payload#>'{tables,gate_entry_tasks}') AND
  (SELECT count(*) FROM repositories) = jsonb_array_length(payload#>'{tables,repositories}') AND
  (SELECT count(*) FROM runs) = jsonb_array_length(payload#>'{tables,runs}') AND
  (SELECT count(*) FROM run_git_observations) = jsonb_array_length(payload#>'{tables,run_git_observations}') AND
  (SELECT count(*) FROM commit_references) = jsonb_array_length(payload#>'{tables,commit_references}') AND
  (SELECT count(*) FROM task_record_indexes) = jsonb_array_length(payload#>'{tables,task_record_indexes}') AND
  (SELECT count(*) FROM task_acceptance_assignments) = jsonb_array_length(payload#>'{tables,task_acceptance_assignments}') AND
  (SELECT count(*) FROM task_acceptance_evidence) = jsonb_array_length(payload#>'{tables,task_acceptance_evidence}')
FROM daytripper_snapshot;

INSERT INTO clone_assertion
SELECT 'foreign_key_orphans', NOT EXISTS (
  SELECT 1 FROM (
    SELECT task.id FROM tasks task LEFT JOIN lanes lane ON lane.workspace_id=task.workspace_id AND lane.id=task.lane_id WHERE lane.id IS NULL
    UNION ALL SELECT task.id FROM tasks task LEFT JOIN phases phase ON phase.workspace_id=task.workspace_id AND phase.id=task.phase_id WHERE phase.id IS NULL
    UNION ALL SELECT task.id FROM tasks task LEFT JOIN tasks parent ON parent.workspace_id=task.workspace_id AND parent.id=task.parent_task_id WHERE task.parent_task_id IS NOT NULL AND parent.id IS NULL
    UNION ALL SELECT policy.workspace_id FROM workspace_acceptance_policies policy LEFT JOIN evidence_profiles profile ON profile.workspace_id=policy.workspace_id AND profile.id=policy.evidence_profile_id LEFT JOIN actors actor ON actor.id=policy.changed_by_actor_id WHERE profile.id IS NULL OR (policy.changed_by_actor_id IS NOT NULL AND actor.id IS NULL)
    UNION ALL SELECT item.id FROM backlog_items item LEFT JOIN lanes lane ON lane.workspace_id=item.workspace_id AND lane.id=item.lane_id WHERE lane.id IS NULL
    UNION ALL SELECT item.id FROM backlog_items item LEFT JOIN tasks task ON task.workspace_id=item.workspace_id AND task.id=item.promoted_task_id WHERE item.promoted_task_id IS NOT NULL AND task.id IS NULL
    UNION ALL SELECT dependency.from_task_id FROM task_dependencies dependency LEFT JOIN tasks task ON task.workspace_id=dependency.workspace_id AND task.id=dependency.from_task_id WHERE task.id IS NULL
    UNION ALL SELECT dependency.to_task_id FROM task_dependencies dependency LEFT JOIN tasks task ON task.workspace_id=dependency.workspace_id AND task.id=dependency.to_task_id WHERE task.id IS NULL
    UNION ALL SELECT gate.id FROM gates gate LEFT JOIN phases phase ON phase.workspace_id=gate.workspace_id AND phase.id=gate.from_phase_id WHERE phase.id IS NULL
    UNION ALL SELECT gate.id FROM gates gate LEFT JOIN phases phase ON phase.workspace_id=gate.workspace_id AND phase.id=gate.to_phase_id WHERE phase.id IS NULL
    UNION ALL SELECT relation.id FROM gate_tasks relation LEFT JOIN gates gate ON gate.workspace_id=relation.workspace_id AND gate.id=relation.gate_id LEFT JOIN tasks task ON task.workspace_id=relation.workspace_id AND task.id=relation.task_id WHERE gate.id IS NULL OR task.id IS NULL
    UNION ALL SELECT relation.gate_id FROM gate_entry_tasks relation LEFT JOIN gates gate ON gate.workspace_id=relation.workspace_id AND gate.id=relation.gate_id LEFT JOIN tasks task ON task.workspace_id=relation.workspace_id AND task.id=relation.task_id WHERE gate.id IS NULL OR task.id IS NULL
    UNION ALL SELECT run.id FROM runs run LEFT JOIN tasks task ON task.workspace_id=run.workspace_id AND task.id=run.task_id LEFT JOIN actors actor ON actor.id=run.operator_actor_id LEFT JOIN runs parent ON parent.workspace_id=run.workspace_id AND parent.id=run.parent_run_id LEFT JOIN runs target ON target.workspace_id=run.workspace_id AND target.id=run.target_run_id WHERE task.id IS NULL OR actor.id IS NULL OR (run.parent_run_id IS NOT NULL AND parent.id IS NULL) OR (run.target_run_id IS NOT NULL AND target.id IS NULL)
    UNION ALL SELECT observation.id::text FROM run_git_observations observation LEFT JOIN runs run ON run.workspace_id=observation.workspace_id AND run.id=observation.run_id LEFT JOIN repositories repository ON repository.workspace_id=observation.workspace_id AND repository.id=observation.repository_id WHERE run.id IS NULL OR repository.id IS NULL
    UNION ALL SELECT reference.id::text FROM commit_references reference LEFT JOIN tasks task ON task.workspace_id=reference.workspace_id AND task.id=reference.task_id LEFT JOIN repositories repository ON repository.workspace_id=reference.workspace_id AND repository.id=reference.repository_id LEFT JOIN runs run ON run.workspace_id=reference.workspace_id AND run.id=reference.run_id WHERE task.id IS NULL OR repository.id IS NULL OR (reference.run_id IS NOT NULL AND run.id IS NULL)
    UNION ALL SELECT record.id::text FROM task_record_indexes record LEFT JOIN tasks task ON task.workspace_id=record.workspace_id AND task.id=record.task_id LEFT JOIN repositories repository ON repository.workspace_id=record.workspace_id AND repository.id=record.repository_id LEFT JOIN runs run ON run.workspace_id=record.workspace_id AND run.id=record.run_id LEFT JOIN task_record_indexes superseded ON superseded.workspace_id=record.workspace_id AND superseded.id=record.supersedes_record_id WHERE task.id IS NULL OR repository.id IS NULL OR (record.run_id IS NOT NULL AND run.id IS NULL) OR (record.supersedes_record_id IS NOT NULL AND superseded.id IS NULL)
    UNION ALL SELECT assignment.id::text FROM task_acceptance_assignments assignment LEFT JOIN tasks task ON task.workspace_id=assignment.workspace_id AND task.id=assignment.task_id LEFT JOIN evidence_profiles profile ON profile.workspace_id=assignment.workspace_id AND profile.id=assignment.evidence_profile_id LEFT JOIN actors approver ON approver.id=assignment.approved_by_actor_id LEFT JOIN task_acceptance_assignments superseded ON superseded.workspace_id=assignment.workspace_id AND superseded.id=assignment.supersedes_assignment_id WHERE task.id IS NULL OR profile.id IS NULL OR (assignment.approved_by_actor_id IS NOT NULL AND approver.id IS NULL) OR (assignment.supersedes_assignment_id IS NOT NULL AND superseded.id IS NULL)
    UNION ALL SELECT evidence.id::text FROM task_acceptance_evidence evidence LEFT JOIN tasks task ON task.workspace_id=evidence.workspace_id AND task.id=evidence.task_id LEFT JOIN task_record_indexes completion ON completion.workspace_id=evidence.workspace_id AND completion.id=evidence.completion_report_record_id LEFT JOIN task_record_indexes review ON review.workspace_id=evidence.workspace_id AND review.id=evidence.independent_review_record_id LEFT JOIN commit_references reference ON reference.workspace_id=evidence.workspace_id AND reference.id=evidence.commit_reference_id LEFT JOIN actors reporter ON reporter.id=evidence.reported_by_actor_id WHERE task.id IS NULL OR completion.id IS NULL OR review.id IS NULL OR reporter.id IS NULL OR (evidence.commit_reference_id IS NOT NULL AND reference.id IS NULL)
  ) orphan
);

INSERT INTO clone_assertion VALUES
  ('actors', (SELECT count(*) FROM actors) = 1),
  ('accounts', (SELECT count(*) FROM accounts) = 0),
  ('account_credentials', (SELECT count(*) FROM account_credentials) = 0),
  ('account_external_identities', (SELECT count(*) FROM account_external_identities) = 0),
  ('account_sessions', (SELECT count(*) FROM account_sessions) = 0),
  ('workspace_memberships', (SELECT count(*) FROM workspace_memberships) = 0),
  ('agent_tokens', (SELECT count(*) FROM agent_tokens) = 0),
  ('approval_grants', (SELECT count(*) FROM approval_grants) = 0),
  ('auth_login_limits', (SELECT count(*) FROM auth_login_limits) = 0),
  ('commands', (SELECT count(*) FROM commands) = 0),
  ('events', (SELECT count(*) FROM events) = 0),
  ('human_approval_attestations', (SELECT count(*) FROM human_approval_attestations) = 0),
  ('mcp_connection_requests', (SELECT count(*) FROM mcp_connection_requests) = 0),
  ('mcp_gateway_registrations', (SELECT count(*) FROM mcp_gateway_registrations) = 0),
  ('mutation_attempts', (SELECT count(*) FROM mutation_attempts) = 0),
  ('oidc_authorization_flows', (SELECT count(*) FROM oidc_authorization_flows) = 0),
  ('security_events', (SELECT count(*) FROM security_events) = 0),
  ('sanitized_runs', (SELECT count(*) FROM runs WHERE session_ref IS NOT NULL OR lease_token_hash NOT LIKE 'redacted:daytripper-snapshot:%') = 0);

COMMIT;
