-- Baley Task #182: sanitized, read-only development-decision audit.
--
-- Supply values through an already trusted psql session. Never place a database
-- URL, password, token, account/session ID, or human display name in this file.
-- Required psql variables:
--   workspace_id  target Workspace ID
--   snapshot_at   UTC audit cutoff, for example 2026-09-08T02:55:35.52239Z
--
-- Append-only rows are restricted to snapshot_at where their table has a time
-- column. Current-state rows such as tasks cannot be reconstructed exactly after
-- later mutations because the database is not event-sourced.

BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY;

-- 1. Target verification. Confirm this against the running API's configured DB
-- target before treating any following result as an operational measurement.
SELECT current_database() AS database_name,
       pg_is_in_recovery() AS is_replica,
       current_setting('transaction_read_only') AS transaction_read_only,
       transaction_timestamp() AS query_started_at,
       (SELECT revision FROM workspaces WHERE id = :'workspace_id') AS workspace_revision,
       (SELECT COUNT(*) FROM tasks WHERE workspace_id = :'workspace_id') AS task_count;

-- 2. Actual tables and columns used by this audit.
SELECT table_name, ordinal_position, column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN (
    'actors', 'commands', 'events', 'tasks', 'task_dependencies', 'runs',
    'task_record_indexes', 'task_acceptance_assignments',
    'task_acceptance_evidence', 'evidence_profiles',
    'workspace_acceptance_policies', 'commit_references',
    'run_git_observations', 'human_approval_attestations', 'approval_grants',
    'mutation_attempts', 'security_events'
  )
ORDER BY table_name, ordinal_position;

-- 3. Real foreign-key edges. mutation_attempts and Event entity_type/entity_id
-- are intentionally absent where no physical FK exists.
SELECT child.relname AS child_table,
       con.conname AS constraint_name,
       string_agg(child_col.attname, ', ' ORDER BY cols.ord) AS child_columns,
       parent.relname AS parent_table,
       string_agg(parent_col.attname, ', ' ORDER BY cols.ord) AS parent_columns
FROM pg_constraint con
JOIN pg_class child ON child.oid = con.conrelid
JOIN pg_namespace ns ON ns.oid = child.relnamespace AND ns.nspname = 'public'
JOIN pg_class parent ON parent.oid = con.confrelid
JOIN LATERAL unnest(con.conkey, con.confkey)
  WITH ORDINALITY AS cols(child_attnum, parent_attnum, ord) ON true
JOIN pg_attribute child_col
  ON child_col.attrelid = child.oid AND child_col.attnum = cols.child_attnum
JOIN pg_attribute parent_col
  ON parent_col.attrelid = parent.oid AND parent_col.attnum = cols.parent_attnum
WHERE con.contype = 'f'
  AND child.relname IN (
    'commands', 'events', 'tasks', 'task_dependencies', 'runs',
    'task_record_indexes', 'task_acceptance_assignments',
    'task_acceptance_evidence', 'commit_references',
    'run_git_observations', 'human_approval_attestations', 'approval_grants',
    'mutation_attempts'
  )
GROUP BY child.relname, con.conname, parent.relname
ORDER BY child.relname, con.conname;

-- 4. Layer counts and time bounds at the audit cutoff.
SELECT layer, row_count, first_at_kst, last_at_kst
FROM (
  SELECT 'tasks' AS layer,
         COUNT(*)::bigint AS row_count,
         MIN(created_event.created_at) AT TIME ZONE 'Asia/Seoul' AS first_at_kst,
         MAX(t.updated_at) AT TIME ZONE 'Asia/Seoul' AS last_at_kst
  FROM tasks t
  LEFT JOIN events created_event
    ON created_event.workspace_id = t.workspace_id
   AND created_event.entity_id = t.id
   AND created_event.event_type = 'task.created'
   AND created_event.created_at <= TIMESTAMPTZ :'snapshot_at'
  WHERE t.workspace_id = :'workspace_id'
  UNION ALL
  SELECT 'commands', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM commands
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'events', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM events
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'runs', COUNT(*),
         MIN(started_at) AT TIME ZONE 'Asia/Seoul',
         MAX(COALESCE(ended_at, heartbeat_at)) AT TIME ZONE 'Asia/Seoul'
  FROM runs
  WHERE workspace_id = :'workspace_id' AND started_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'task_record_indexes', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM task_record_indexes
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'task_acceptance_assignments', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM task_acceptance_assignments
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'task_acceptance_evidence', COUNT(*),
         MIN(reported_at) AT TIME ZONE 'Asia/Seoul',
         MAX(reported_at) AT TIME ZONE 'Asia/Seoul'
  FROM task_acceptance_evidence
  WHERE workspace_id = :'workspace_id' AND reported_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'commit_references', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM commit_references
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'run_git_observations', COUNT(*),
         MIN(observed_at) AT TIME ZONE 'Asia/Seoul',
         MAX(observed_at) AT TIME ZONE 'Asia/Seoul'
  FROM run_git_observations
  WHERE workspace_id = :'workspace_id' AND observed_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'human_approval_attestations', COUNT(*),
         MIN(recorded_at) AT TIME ZONE 'Asia/Seoul',
         MAX(recorded_at) AT TIME ZONE 'Asia/Seoul'
  FROM human_approval_attestations
  WHERE workspace_id = :'workspace_id' AND recorded_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'approval_grants', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM approval_grants
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'mutation_attempts', COUNT(*),
         MIN(occurred_at) AT TIME ZONE 'Asia/Seoul',
         MAX(occurred_at) AT TIME ZONE 'Asia/Seoul'
  FROM mutation_attempts
  WHERE workspace_id = :'workspace_id' AND occurred_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL
  SELECT 'security_events', COUNT(*),
         MIN(created_at) AT TIME ZONE 'Asia/Seoul',
         MAX(created_at) AT TIME ZONE 'Asia/Seoul'
  FROM security_events
  WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
) counts
ORDER BY layer;

-- 5. Task-level recoverability coverage.
WITH task_features AS (
  SELECT t.id,
         NULLIF(BTRIM(t.description), '') IS NOT NULL AS has_description,
         NULLIF(BTRIM(t.current_summary), '') IS NOT NULL AS has_summary,
         NULLIF(BTRIM(t.next_action), '') IS NOT NULL AS has_next_action,
         NULLIF(BTRIM(t.terminal_reason), '') IS NOT NULL AS has_terminal_reason,
         NULLIF(BTRIM(t.implemented_assessment), '') IS NOT NULL AS has_assessment,
         EXISTS (
           SELECT 1 FROM events e
           WHERE e.workspace_id = t.workspace_id AND e.entity_id = t.id
             AND e.event_type = 'task.created'
             AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_created_event,
         EXISTS (
           SELECT 1 FROM events e
           WHERE e.workspace_id = t.workspace_id AND e.entity_type = 'task'
             AND e.entity_id = t.id AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_any_task_event,
         EXISTS (
           SELECT 1 FROM runs r
           WHERE r.workspace_id = t.workspace_id AND r.task_id = t.id
             AND r.started_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_run,
         EXISTS (
           SELECT 1 FROM task_record_indexes x
           WHERE x.workspace_id = t.workspace_id AND x.task_id = t.id
             AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_record,
         EXISTS (
           SELECT 1 FROM task_record_indexes x
           WHERE x.workspace_id = t.workspace_id AND x.task_id = t.id
             AND x.record_type = 'detailed-plan'
             AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_detailed_plan,
         EXISTS (
           SELECT 1 FROM task_record_indexes x
           WHERE x.workspace_id = t.workspace_id AND x.task_id = t.id
             AND x.record_type = 'independent-agent-review'
             AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_independent_review,
         EXISTS (
           SELECT 1 FROM task_record_indexes x
           WHERE x.workspace_id = t.workspace_id AND x.task_id = t.id
             AND x.record_type = 'completion-report'
             AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_completion_report,
         EXISTS (
           SELECT 1 FROM commit_references c
           WHERE c.workspace_id = t.workspace_id AND c.task_id = t.id
             AND c.created_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_commit_reference,
         EXISTS (
           SELECT 1 FROM task_acceptance_evidence a
           WHERE a.workspace_id = t.workspace_id AND a.task_id = t.id
             AND a.reported_at <= TIMESTAMPTZ :'snapshot_at'
         ) AS has_typed_acceptance_evidence
  FROM tasks t
  WHERE t.workspace_id = :'workspace_id'
), metrics AS (
  SELECT 'description_nonempty' metric, COUNT(*) FILTER (WHERE has_description)::bigint matched, COUNT(*)::bigint total FROM task_features
  UNION ALL SELECT 'current_summary_nonempty', COUNT(*) FILTER (WHERE has_summary), COUNT(*) FROM task_features
  UNION ALL SELECT 'next_action_nonempty', COUNT(*) FILTER (WHERE has_next_action), COUNT(*) FROM task_features
  UNION ALL SELECT 'terminal_reason_nonempty', COUNT(*) FILTER (WHERE has_terminal_reason), COUNT(*) FROM task_features
  UNION ALL SELECT 'implemented_assessment_nonempty', COUNT(*) FILTER (WHERE has_assessment), COUNT(*) FROM task_features
  UNION ALL SELECT 'task.created_event', COUNT(*) FILTER (WHERE has_created_event), COUNT(*) FROM task_features
  UNION ALL SELECT 'any_task_event', COUNT(*) FILTER (WHERE has_any_task_event), COUNT(*) FROM task_features
  UNION ALL SELECT 'any_run', COUNT(*) FILTER (WHERE has_run), COUNT(*) FROM task_features
  UNION ALL SELECT 'any_record', COUNT(*) FILTER (WHERE has_record), COUNT(*) FROM task_features
  UNION ALL SELECT 'detailed_plan_record', COUNT(*) FILTER (WHERE has_detailed_plan), COUNT(*) FROM task_features
  UNION ALL SELECT 'independent_review_record', COUNT(*) FILTER (WHERE has_independent_review), COUNT(*) FROM task_features
  UNION ALL SELECT 'completion_report_record', COUNT(*) FILTER (WHERE has_completion_report), COUNT(*) FROM task_features
  UNION ALL SELECT 'commit_reference', COUNT(*) FILTER (WHERE has_commit_reference), COUNT(*) FROM task_features
  UNION ALL SELECT 'typed_acceptance_evidence', COUNT(*) FILTER (WHERE has_typed_acceptance_evidence), COUNT(*) FROM task_features
)
SELECT metric, matched, total,
       ROUND(100.0 * matched / NULLIF(total, 0), 1) AS percent
FROM metrics
ORDER BY metric;

-- 6. Record, commit, evidence and supersedes linkage.
SELECT metric, matched, total,
       ROUND(100.0 * matched / NULLIF(total, 0), 1) AS percent
FROM (
  SELECT 'records_with_run' metric,
         COUNT(*) FILTER (WHERE run_id IS NOT NULL)::bigint matched,
         COUNT(*)::bigint total
  FROM task_record_indexes WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'records_with_commit_sha', COUNT(*) FILTER (WHERE commit_sha IS NOT NULL), COUNT(*)
  FROM task_record_indexes WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'records_with_blob_sha', COUNT(*) FILTER (WHERE blob_sha IS NOT NULL), COUNT(*)
  FROM task_record_indexes WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'records_with_supersedes', COUNT(*) FILTER (WHERE supersedes_record_id IS NOT NULL), COUNT(*)
  FROM task_record_indexes WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'commit_refs_with_run', COUNT(*) FILTER (WHERE run_id IS NOT NULL), COUNT(*)
  FROM commit_references WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'commit_refs_remote_verified', COUNT(*) FILTER (WHERE verification_state = 'remote_verified'), COUNT(*)
  FROM commit_references WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'evidence_with_commit_ref', COUNT(*) FILTER (WHERE commit_reference_id IS NOT NULL), COUNT(*)
  FROM task_acceptance_evidence WHERE workspace_id = :'workspace_id' AND reported_at <= TIMESTAMPTZ :'snapshot_at'
  UNION ALL SELECT 'assignments_with_supersedes', COUNT(*) FILTER (WHERE supersedes_assignment_id IS NOT NULL), COUNT(*)
  FROM task_acceptance_assignments WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
) linkage
ORDER BY metric;

-- 7. Physical orphans and historical gaps. The first group should be zero when
-- the declared FKs are healthy; the second group measures missing history.
SELECT metric, orphan_count
FROM (
  SELECT 'runs_missing_task' metric, COUNT(*)::bigint orphan_count
  FROM runs r LEFT JOIN tasks t ON t.workspace_id = r.workspace_id AND t.id = r.task_id
  WHERE r.workspace_id = :'workspace_id' AND r.started_at <= TIMESTAMPTZ :'snapshot_at' AND t.id IS NULL
  UNION ALL SELECT 'records_missing_task', COUNT(*)
  FROM task_record_indexes x LEFT JOIN tasks t ON t.workspace_id = x.workspace_id AND t.id = x.task_id
  WHERE x.workspace_id = :'workspace_id' AND x.created_at <= TIMESTAMPTZ :'snapshot_at' AND t.id IS NULL
  UNION ALL SELECT 'records_missing_nonnull_run', COUNT(*)
  FROM task_record_indexes x LEFT JOIN runs r ON r.workspace_id = x.workspace_id AND r.id = x.run_id
  WHERE x.workspace_id = :'workspace_id' AND x.created_at <= TIMESTAMPTZ :'snapshot_at' AND x.run_id IS NOT NULL AND r.id IS NULL
  UNION ALL SELECT 'commit_refs_missing_task', COUNT(*)
  FROM commit_references c LEFT JOIN tasks t ON t.workspace_id = c.workspace_id AND t.id = c.task_id
  WHERE c.workspace_id = :'workspace_id' AND c.created_at <= TIMESTAMPTZ :'snapshot_at' AND t.id IS NULL
  UNION ALL SELECT 'events_missing_command', COUNT(*)
  FROM events e LEFT JOIN commands c ON c.workspace_id = e.workspace_id AND c.id = e.command_id
  WHERE e.workspace_id = :'workspace_id' AND e.created_at <= TIMESTAMPTZ :'snapshot_at' AND c.id IS NULL
  UNION ALL SELECT 'acceptance_evidence_missing_task', COUNT(*)
  FROM task_acceptance_evidence a LEFT JOIN tasks t ON t.workspace_id = a.workspace_id AND t.id = a.task_id
  WHERE a.workspace_id = :'workspace_id' AND a.reported_at <= TIMESTAMPTZ :'snapshot_at' AND t.id IS NULL
) physical
ORDER BY metric;

SELECT gap, COUNT(*) AS row_count,
       ARRAY_AGG(public_id ORDER BY public_id) AS task_public_ids
FROM (
  SELECT t.public_id,
         CASE
           WHEN NOT EXISTS (
             SELECT 1 FROM events e
             WHERE e.workspace_id = t.workspace_id AND e.entity_type = 'task'
               AND e.entity_id = t.id AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
           ) THEN 'no_task_event'
           WHEN NOT EXISTS (
             SELECT 1 FROM events e
             WHERE e.workspace_id = t.workspace_id AND e.entity_id = t.id
               AND e.event_type = 'task.created'
               AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
           ) THEN 'no_task_created_event'
         END AS gap
  FROM tasks t
  WHERE t.workspace_id = :'workspace_id'
) missing_history
WHERE gap IS NOT NULL
GROUP BY gap
ORDER BY gap;

SELECT metric, row_count
FROM (
  SELECT 'runs_without_run_started_event' metric, COUNT(*)::bigint row_count
  FROM runs r
  WHERE r.workspace_id = :'workspace_id' AND r.started_at <= TIMESTAMPTZ :'snapshot_at'
    AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.workspace_id = r.workspace_id AND e.entity_type = 'run'
        AND e.entity_id = r.id AND e.event_type = 'run.started'
        AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
    )
  UNION ALL
  SELECT 'records_without_registered_event', COUNT(*)
  FROM task_record_indexes x
  WHERE x.workspace_id = :'workspace_id' AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
    AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.workspace_id = x.workspace_id AND e.entity_type = 'task_record'
        AND e.entity_id = x.id::text AND e.event_type = 'record.registered'
        AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
    )
  UNION ALL
  SELECT 'terminal_runs_without_any_terminal_event', COUNT(*)
  FROM runs r
  WHERE r.workspace_id = :'workspace_id' AND r.started_at <= TIMESTAMPTZ :'snapshot_at'
    AND r.status <> 'running'
    AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.workspace_id = r.workspace_id AND e.entity_type = 'run'
        AND e.entity_id = r.id
        AND e.event_type IN ('run.succeeded', 'run.failed', 'run.interrupted', 'run.cancelled', 'run.corrected')
        AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
    )
  UNION ALL
  SELECT 'commands_without_event_all', COUNT(*)
  FROM commands c
  WHERE c.workspace_id = :'workspace_id' AND c.created_at <= TIMESTAMPTZ :'snapshot_at'
    AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.workspace_id = c.workspace_id AND e.command_id = c.id
        AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
    )
  UNION ALL
  SELECT 'commands_without_event_excluding_heartbeat', COUNT(*)
  FROM commands c
  WHERE c.workspace_id = :'workspace_id' AND c.created_at <= TIMESTAMPTZ :'snapshot_at'
    AND c.command_name <> 'run.heartbeat'
    AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.workspace_id = c.workspace_id AND e.command_id = c.id
        AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
    )
) history
ORDER BY metric;

-- 8. Actor completeness without displaying identities or names.
SELECT role, actor_type, row_count
FROM (
  SELECT 'event_initiator' role, COALESCE(a.actor_type, '[missing]') actor_type, COUNT(*)::bigint row_count
  FROM events e LEFT JOIN actors a ON a.id = e.initiated_by_actor_id
  WHERE e.workspace_id = :'workspace_id' AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
  GROUP BY a.actor_type
  UNION ALL
  SELECT 'event_executor', COALESCE(a.actor_type, '[missing]'), COUNT(*)
  FROM events e LEFT JOIN actors a ON a.id = e.executed_by_actor_id
  WHERE e.workspace_id = :'workspace_id' AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
  GROUP BY a.actor_type
  UNION ALL
  SELECT 'event_approver', COALESCE(a.actor_type, '[missing]'), COUNT(*)
  FROM events e LEFT JOIN actors a ON a.id = e.approved_by_actor_id
  WHERE e.workspace_id = :'workspace_id' AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
  GROUP BY a.actor_type
  UNION ALL
  SELECT 'run_operator', COALESCE(a.actor_type, '[missing]'), COUNT(*)
  FROM runs r LEFT JOIN actors a ON a.id = r.operator_actor_id
  WHERE r.workspace_id = :'workspace_id' AND r.started_at <= TIMESTAMPTZ :'snapshot_at'
  GROUP BY a.actor_type
) actor_coverage
ORDER BY role, actor_type;

-- 9. Weekly distribution.
WITH weekly AS (
  SELECT 'task.created_events' layer,
         date_trunc('week', created_at AT TIME ZONE 'Asia/Seoul')::date week_start,
         COUNT(*)::bigint row_count
  FROM events
  WHERE workspace_id = :'workspace_id' AND event_type = 'task.created'
    AND created_at <= TIMESTAMPTZ :'snapshot_at'
  GROUP BY 2
  UNION ALL SELECT 'events', date_trunc('week', created_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
  UNION ALL SELECT 'runs', date_trunc('week', started_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM runs WHERE workspace_id = :'workspace_id' AND started_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
  UNION ALL SELECT 'task_records', date_trunc('week', created_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM task_record_indexes WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
  UNION ALL SELECT 'commit_refs', date_trunc('week', created_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM commit_references WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
  UNION ALL SELECT 'typed_evidence', date_trunc('week', reported_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM task_acceptance_evidence WHERE workspace_id = :'workspace_id' AND reported_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
  UNION ALL SELECT 'approvals', date_trunc('week', recorded_at AT TIME ZONE 'Asia/Seoul')::date, COUNT(*)
  FROM human_approval_attestations WHERE workspace_id = :'workspace_id' AND recorded_at <= TIMESTAMPTZ :'snapshot_at' GROUP BY 2
)
SELECT week_start, layer, row_count
FROM weekly
ORDER BY week_start, layer;

-- 10. Decision-semantic payload coverage. Exact quoted-key patterns avoid
-- matching prose values.
SELECT semantic, row_count
FROM (
  SELECT 'implemented_event_assessment_nonempty' semantic, COUNT(*)::bigint row_count
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'task.implemented_reported' AND NULLIF(BTRIM(payload->>'assessment'), '') IS NOT NULL
  UNION ALL SELECT 'implemented_event_proceed_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'task.implemented_reported' AND NULLIF(BTRIM(payload->>'proceedReason'), '') IS NOT NULL
  UNION ALL SELECT 'run_succeeded_summary_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'run.succeeded' AND NULLIF(BTRIM(payload->>'resultSummary'), '') IS NOT NULL
  UNION ALL SELECT 'run_interrupted_summary_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'run.interrupted' AND NULLIF(BTRIM(payload->>'errorSummary'), '') IS NOT NULL
  UNION ALL SELECT 'dependency_patch_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'dependency.patched' AND NULLIF(BTRIM(payload->>'proceedReason'), '') IS NOT NULL
  UNION ALL SELECT 'task_confirm_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'task.confirmed' AND NULLIF(BTRIM(payload->>'proceedReason'), '') IS NOT NULL
  UNION ALL SELECT 'task_discard_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'task.discarded' AND NULLIF(BTRIM(payload->>'reason'), '') IS NOT NULL
  UNION ALL SELECT 'task_rework_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'task.rework_started' AND NULLIF(BTRIM(payload->>'reason'), '') IS NOT NULL
  UNION ALL SELECT 'run_correction_reason_nonempty', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND event_type = 'run.corrected' AND NULLIF(BTRIM(payload->>'reason'), '') IS NOT NULL
  UNION ALL SELECT 'event_payload_has_alternative_key', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND payload::text ~* '"(alternative|alternatives|options|considered_options)"[[:space:]]*:'
  UNION ALL SELECT 'event_payload_has_rationale_key', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND payload::text ~* '"(rationale|decision_rationale|why)"[[:space:]]*:'
  UNION ALL SELECT 'event_payload_has_supersedes_or_amends_key', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND payload::text ~* '"(supersedes|supersedes_id|supersedes_record_id|amends)"[[:space:]]*:'
  UNION ALL SELECT 'event_payload_has_expected_outcome_key', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND payload::text ~* '"(expected_outcome|expectedOutcome)"[[:space:]]*:'
  UNION ALL SELECT 'event_payload_has_actual_outcome_key', COUNT(*)
  FROM events WHERE workspace_id = :'workspace_id' AND created_at <= TIMESTAMPTZ :'snapshot_at'
    AND payload::text ~* '"(actual_outcome|actualOutcome)"[[:space:]]*:'
) semantic_coverage
ORDER BY semantic;

-- 11. Required recent cases. Hashes are shortened and actor identities omitted.
SELECT public_id, title, status, description, current_summary, next_action,
       terminal_reason, implemented_assessment,
       updated_at AT TIME ZONE 'Asia/Seoul' AS updated_kst
FROM tasks
WHERE workspace_id = :'workspace_id' AND public_id IN (179, 180, 181)
ORDER BY public_id;

SELECT t.public_id,
       e.created_at AT TIME ZONE 'Asia/Seoul' AS occurred_kst,
       e.workspace_revision,
       e.event_type,
       c.command_name,
       COALESCE(
         NULLIF(e.payload->>'reason', ''),
         NULLIF(e.payload->>'proceedReason', ''),
         NULLIF(e.payload->>'assessment', ''),
         ''
       ) AS decision_or_assessment
FROM tasks t
JOIN events e
  ON e.workspace_id = t.workspace_id AND e.entity_type = 'task' AND e.entity_id = t.id
JOIN commands c ON c.workspace_id = e.workspace_id AND c.id = e.command_id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
  AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY t.public_id, e.created_at, e.command_event_index;

SELECT t.public_id, r.kind, r.status,
       r.started_at AT TIME ZONE 'Asia/Seoul' AS started_kst,
       r.ended_at AT TIME ZONE 'Asia/Seoul' AS ended_kst,
       NULLIF(r.result_summary, '') AS result_summary,
       NULLIF(r.error_summary, '') AS error_summary,
       r.parent_run_id IS NOT NULL AS has_parent_run,
       r.target_run_id IS NOT NULL AS has_target_run
FROM tasks t
JOIN runs r ON r.workspace_id = t.workspace_id AND r.task_id = t.id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
  AND r.started_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY t.public_id, r.started_at;

SELECT t.public_id, x.record_type, x.state,
       x.created_at AT TIME ZONE 'Asia/Seoul' AS created_kst,
       x.relative_path, x.short_summary,
       x.run_id IS NOT NULL AS linked_run,
       x.commit_sha IS NOT NULL AS linked_commit,
       x.supersedes_record_id IS NOT NULL AS supersedes_prior
FROM tasks t
JOIN task_record_indexes x ON x.workspace_id = t.workspace_id AND x.task_id = t.id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
  AND x.created_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY t.public_id, x.created_at;

SELECT t.public_id,
       c.created_at AT TIME ZONE 'Asia/Seoul' AS created_kst,
       c.relation, c.verification_state,
       LEFT(c.commit_sha, 12) AS commit_prefix,
       c.run_id IS NOT NULL AS linked_run
FROM tasks t
JOIN commit_references c ON c.workspace_id = t.workspace_id AND c.task_id = t.id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
  AND c.created_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY t.public_id, c.created_at;

SELECT t.public_id, a.evidence_version,
       a.reported_at AT TIME ZONE 'Asia/Seoul' AS reported_kst,
       a.verification_verdict, a.verification_reference_kind,
       a.review_verdict, a.unresolved_blocking_count,
       a.commit_reference_id IS NOT NULL AS linked_commit
FROM tasks t
JOIN task_acceptance_evidence a
  ON a.workspace_id = t.workspace_id AND a.task_id = t.id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
  AND a.reported_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY t.public_id, a.evidence_version;

SELECT t.public_id,
       p.public_id AS parent_public_id,
       ARRAY_REMOVE(ARRAY_AGG(DISTINCT pred.public_id ORDER BY pred.public_id), NULL) AS predecessor_public_ids,
       ARRAY_REMOVE(ARRAY_AGG(DISTINCT succ.public_id ORDER BY succ.public_id), NULL) AS successor_public_ids
FROM tasks t
LEFT JOIN tasks p ON p.workspace_id = t.workspace_id AND p.id = t.parent_task_id
LEFT JOIN task_dependencies incoming
  ON incoming.workspace_id = t.workspace_id AND incoming.to_task_id = t.id
LEFT JOIN tasks pred
  ON pred.workspace_id = incoming.workspace_id AND pred.id = incoming.from_task_id
LEFT JOIN task_dependencies outgoing
  ON outgoing.workspace_id = t.workspace_id AND outgoing.from_task_id = t.id
LEFT JOIN tasks succ
  ON succ.workspace_id = outgoing.workspace_id AND succ.id = outgoing.to_task_id
WHERE t.workspace_id = :'workspace_id' AND t.public_id IN (179, 180, 181)
GROUP BY t.public_id, p.public_id
ORDER BY t.public_id;

-- 12. Old judgment-change case: the one persisted Task rework.
SELECT t.public_id, t.title, t.status AS current_status,
       e.created_at AT TIME ZONE 'Asia/Seoul' AS rework_started_kst,
       e.payload->>'reason' AS reason,
       e.workspace_revision
FROM events e
JOIN tasks t ON t.workspace_id = e.workspace_id AND t.id = e.entity_id
WHERE e.workspace_id = :'workspace_id'
  AND e.entity_type = 'task' AND e.event_type = 'task.rework_started'
  AND e.created_at <= TIMESTAMPTZ :'snapshot_at'
ORDER BY e.created_at;

-- 13. Recovery boundary that explains missing historical rows.
SELECT e.created_at AT TIME ZONE 'Asia/Seoul' AS occurred_kst,
       e.workspace_revision,
       e.payload->>'sourceRevision' AS source_revision,
       e.payload->>'physicalRecovery' AS physical_recovery,
       CASE WHEN e.payload ? 'recoveryImageSha256' THEN 'present-redacted' ELSE 'absent' END AS recovery_image_hash_state,
       e.payload->>'source' AS source
FROM events e
WHERE e.workspace_id = :'workspace_id'
  AND e.event_type = 'recovery.reconstructed'
  AND e.created_at <= TIMESTAMPTZ :'snapshot_at';

SELECT c.result->'limitations' AS limitations,
       c.result->'recordsIndexed' AS records_indexed,
       c.result->'sourceRevision' AS source_revision
FROM commands c
WHERE c.workspace_id = :'workspace_id'
  AND c.command_name = 'recovery.reconstruct'
  AND c.created_at <= TIMESTAMPTZ :'snapshot_at';

ROLLBACK;
