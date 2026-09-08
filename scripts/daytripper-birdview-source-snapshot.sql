\set ON_ERROR_STOP on
\set QUIET on

BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;

SELECT count(*) = 1 AS source_identity_ok
FROM workspaces
WHERE id = '410f335e-ddb2-443f-be3c-7d1d18ccd534'
  AND name = 'DayTripper'
\gset
\if :source_identity_ok
\else
  \echo 'Refusing snapshot: expected DayTripper Workspace identity was not found exactly once.'
  \quit 41
\endif

SELECT coalesce(max(version_id) FILTER (WHERE is_applied), 0) = 25 AS source_schema_ok
FROM goose_db_version
\gset
\if :source_schema_ok
\else
  \echo 'Refusing snapshot: operating source schema is not exactly version 25.'
  \quit 42
\endif

SELECT array_agg(tablename ORDER BY tablename) = ARRAY[
  'account_credentials','account_external_identities','account_sessions','accounts','actors',
  'agent_tokens','approval_grants','auth_login_limits','backlog_items','commands',
  'commit_references','events','evidence_profiles','gate_entry_tasks','gate_tasks','gates',
  'goose_db_version','human_approval_attestations','lanes','mcp_connection_requests',
  'mcp_gateway_registrations','mutation_attempts','oidc_authorization_flows','phases',
  'repositories','run_git_observations','runs','security_events','task_acceptance_assignments',
  'task_acceptance_evidence','task_dependencies','task_record_indexes','tasks',
  'workspace_acceptance_policies','workspace_counters','workspace_memberships','workspaces'
]::name[] AS source_inventory_ok
FROM pg_tables
WHERE schemaname = 'public'
\gset
\if :source_inventory_ok
\else
  \echo 'Refusing snapshot: public table inventory differs from the reviewed schema-25 allowlist.'
  \quit 43
\endif

COPY (
  WITH constants AS (
    SELECT
      '410f335e-ddb2-443f-be3c-7d1d18ccd534'::text AS workspace_id,
      '17800000-0000-4000-8000-000000000001'::text AS review_actor_id
  )
  SELECT jsonb_build_object(
    'formatVersion', 1,
    'capturedAt', transaction_timestamp(),
    'source', jsonb_build_object(
      'database', current_database(),
      'schemaVersion', 25,
      'workspaceId', workspace.id,
      'workspaceName', workspace.name,
      'workspaceRevision', workspace.revision
    ),
    'policy', jsonb_build_object(
      'includedTables', jsonb_build_array(
        'workspaces','workspace_counters','phases','lanes','evidence_profiles',
        'workspace_acceptance_policies','tasks','task_dependencies','backlog_items','gates',
        'gate_tasks','gate_entry_tasks','repositories','runs','run_git_observations',
        'commit_references','task_record_indexes','task_acceptance_assignments',
        'task_acceptance_evidence'
      ),
      'excludedTables', jsonb_build_array(
        'accounts','account_credentials','account_external_identities','account_sessions','actors',
        'workspace_memberships','agent_tokens','approval_grants','auth_login_limits','commands','events',
        'human_approval_attestations','mcp_connection_requests','mcp_gateway_registrations',
        'mutation_attempts','oidc_authorization_flows','security_events'
      ),
      'targetOnlyTables', jsonb_build_array(
        'bird_views','bird_view_nodes','bird_view_edges','bird_view_phase_bindings',
        'bird_view_gate_bindings','bird_view_task_bindings','bird_view_backlog_bindings',
        'bird_view_commands','bird_view_events'
      ),
      'transformations', jsonb_build_array(
        'actor references -> isolated review actor',
        'runs.session_ref -> null',
        'runs.lease_token_hash -> inert deterministic redaction'
      )
    ),
    'tables', jsonb_build_object(
      'workspaces', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM workspaces value, constants WHERE value.id = constants.workspace_id) row_value),
      'workspace_counters', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.workspace_id), '[]'::jsonb) FROM (SELECT value.* FROM workspace_counters value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'phases', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.position, row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM phases value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'lanes', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM lanes value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'evidence_profiles', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM evidence_profiles value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'workspace_acceptance_policies', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.policy_version), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.policy_version,value.default_mode,value.evidence_profile_id,
          CASE WHEN value.changed_by_actor_id IS NULL THEN NULL ELSE constants.review_actor_id END AS changed_by_actor_id,
          value.changed_at
        FROM workspace_acceptance_policies value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value),
      'tasks', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.public_id), '[]'::jsonb) FROM (SELECT value.* FROM tasks value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'task_dependencies', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.from_task_id, row_value.to_task_id), '[]'::jsonb) FROM (SELECT value.* FROM task_dependencies value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'backlog_items', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.public_id), '[]'::jsonb) FROM (SELECT value.* FROM backlog_items value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'gates', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.public_id), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.id,value.name,value.from_phase_id,value.to_phase_id,value.criteria_revision,value.passed_at,
          CASE WHEN value.passed_by_actor_id IS NULL THEN NULL ELSE constants.review_actor_id END AS passed_by_actor_id,
          value.updated_at,value.public_id,value.alias
        FROM gates value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value),
      'gate_tasks', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.id), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.id,value.gate_id,value.task_id,value.passed_at,
          CASE WHEN value.passed_by_actor_id IS NULL THEN NULL ELSE constants.review_actor_id END AS passed_by_actor_id,
          value.pass_reason,value.updated_at
        FROM gate_tasks value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value),
      'gate_entry_tasks', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.gate_id, row_value.task_id), '[]'::jsonb) FROM (SELECT value.* FROM gate_entry_tasks value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'repositories', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM repositories value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'runs', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.started_at, row_value.id), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.id,value.task_id,value.client_run_id,value.kind,value.status,
          constants.review_actor_id AS operator_actor_id,NULL::text AS session_ref,value.parent_run_id,value.target_run_id,
          'redacted:daytripper-snapshot:' || value.id AS lease_token_hash,value.heartbeat_at,value.lease_expires_at,
          value.version,value.started_at,value.ended_at,value.result_summary,value.error_summary,value.created_at
        FROM runs value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value),
      'run_git_observations', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.observed_at, row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM run_git_observations value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'commit_references', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.created_at, row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM commit_references value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'task_record_indexes', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.created_at, row_value.id), '[]'::jsonb) FROM (SELECT value.* FROM task_record_indexes value, constants WHERE value.workspace_id = constants.workspace_id) row_value),
      'task_acceptance_assignments', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.task_id, row_value.assignment_version), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.id,value.task_id,value.assignment_version,value.requested_mode,value.effective_mode,
          value.policy_version,value.evidence_profile_id,value.reason,value.evidence_reference,
          CASE WHEN value.approved_by_actor_id IS NULL THEN NULL ELSE constants.review_actor_id END AS approved_by_actor_id,
          value.supersedes_assignment_id,value.created_at
        FROM task_acceptance_assignments value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value),
      'task_acceptance_evidence', (SELECT coalesce(jsonb_agg(to_jsonb(row_value) ORDER BY row_value.task_id, row_value.evidence_version), '[]'::jsonb) FROM (
        SELECT value.workspace_id,value.id,value.task_id,value.evidence_version,value.completion_report_record_id,
          value.verification_verdict,value.verification_reference,value.verification_reference_kind,
          value.independent_review_record_id,value.review_verdict,value.unresolved_blocking_count,value.commit_reference_id,
          constants.review_actor_id AS reported_by_actor_id,value.reported_at
        FROM task_acceptance_evidence value, constants WHERE value.workspace_id = constants.workspace_id
      ) row_value)
    )
  )::text
  FROM workspaces workspace, constants
  WHERE workspace.id = constants.workspace_id
) TO STDOUT;

ROLLBACK;
