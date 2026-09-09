-- +goose Up
-- Forward-only, deterministic projection of pre-migration-26 lifecycle Events.
-- Only facts explicitly present in the Event payload are copied. In particular,
-- Task text is never interpreted as rationale, a goal, an alternative, or an
-- outcome.
CREATE TEMP TABLE task_journal_backfill_v1_sources
ON COMMIT DROP AS
SELECT
  e.*,
  CASE WHEN e.event_type = 'task.created' THEN
    COALESCE(NULLIF(btrim(e.payload #>> '{task,id}'), ''),
             NULLIF(btrim(e.payload #>> '{task,ID}'), ''))
  ELSE NULLIF(btrim(e.payload ->> 'taskId'), '') END AS task_id
FROM events e
WHERE e.event_type IN (
  'task.created','run.started','task.updated','task.rework_started',
  'task.blocked','task.unblocked','task.implemented_reported',
  'task.confirmed','task.discarded'
)
AND NOT (e.payload ? 'taskJournal');

-- Fail closed before inserting anything. An allow-listed Event is historical
-- evidence: malformed payloads and broken provenance must be repaired or
-- explicitly excluded by a later reviewed migration, never silently skipped.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM task_journal_backfill_v1_sources s
    WHERE jsonb_typeof(s.payload) <> 'object'
       OR s.task_id IS NULL
       OR s.executed_by_actor_id IS NULL
       OR NOT EXISTS (SELECT 1 FROM tasks t WHERE t.workspace_id=s.workspace_id AND t.id=s.task_id)
       OR NOT EXISTS (SELECT 1 FROM actors a WHERE a.id=s.executed_by_actor_id)
       OR (s.initiated_by_actor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM actors a WHERE a.id=s.initiated_by_actor_id))
       OR (s.approved_by_actor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM actors a WHERE a.id=s.approved_by_actor_id))
       OR (s.event_type <> 'task.created' AND jsonb_typeof(s.payload -> 'taskId') <> 'string')
       OR (s.event_type = 'run.started' AND (
         s.entity_type <> 'run' OR s.entity_id IS NULL
         OR jsonb_typeof(s.payload -> 'runId') <> 'string'
         OR s.entity_id <> NULLIF(btrim(s.payload ->> 'runId'), '')
       ))
       OR (s.event_type <> 'run.started' AND (s.entity_type <> 'task' OR s.entity_id <> s.task_id))
  ) THEN
    RAISE EXCEPTION 'historical Task Journal backfill found invalid Task, actor, or entity provenance';
  END IF;

  IF EXISTS (
    SELECT 1 FROM task_journal_backfill_v1_sources s
    WHERE
      (s.event_type = 'task.created' AND (
        jsonb_typeof(s.payload -> 'task') <> 'object'
        OR ((s.payload #> '{task,id}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,id}') <> 'string')
        OR ((s.payload #> '{task,ID}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,ID}') <> 'string')
        OR ((s.payload #> '{task,title}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,title}') <> 'string')
        OR ((s.payload #> '{task,Title}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,Title}') <> 'string')
        OR ((s.payload #> '{task,description}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,description}') <> 'string')
        OR ((s.payload #> '{task,Description}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,Description}') <> 'string')
        OR ((s.payload #> '{task,currentSummary}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,currentSummary}') <> 'string')
        OR ((s.payload #> '{task,CurrentSummary}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,CurrentSummary}') <> 'string')
        OR ((s.payload #> '{task,terminalReason}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,terminalReason}') <> 'string')
        OR ((s.payload #> '{task,TerminalReason}') IS NOT NULL AND jsonb_typeof(s.payload #> '{task,TerminalReason}') <> 'string')
      ))
      OR (s.event_type = 'run.started' AND (
        jsonb_typeof(s.payload -> 'taskId') <> 'string'
        OR ((s.payload -> 'kind') IS NOT NULL AND jsonb_typeof(s.payload -> 'kind') <> 'string')
        OR ((s.payload -> 'clientRunId') IS NOT NULL AND jsonb_typeof(s.payload -> 'clientRunId') <> 'string')
        OR ((s.payload -> 'sessionRef') IS NOT NULL AND jsonb_typeof(s.payload -> 'sessionRef') <> 'string')
      ))
      OR (s.event_type = 'task.updated' AND (
        ((s.payload -> 'before') IS NOT NULL AND jsonb_typeof(s.payload -> 'before') <> 'object')
        OR ((s.payload -> 'after') IS NOT NULL AND jsonb_typeof(s.payload -> 'after') <> 'object')
      ))
      OR (s.event_type IN ('task.rework_started','task.blocked','task.unblocked','task.discarded')
          AND (s.payload -> 'reason') IS NOT NULL AND jsonb_typeof(s.payload -> 'reason') <> 'string')
      OR (s.event_type = 'task.implemented_reported'
          AND (s.payload -> 'assessment') IS NOT NULL AND jsonb_typeof(s.payload -> 'assessment') <> 'string')
      OR ((s.payload -> 'proceedReason') IS NOT NULL AND jsonb_typeof(s.payload -> 'proceedReason') <> 'string')
      OR ((s.payload -> 'warnings') IS NOT NULL AND jsonb_typeof(s.payload -> 'warnings') <> 'array')
      OR ((s.payload -> 'acknowledgedWarningCodes') IS NOT NULL AND jsonb_typeof(s.payload -> 'acknowledgedWarningCodes') <> 'array')
  ) THEN
    RAISE EXCEPTION 'historical Task Journal backfill found a malformed allow-listed payload';
  END IF;

  IF EXISTS (
    SELECT 1 FROM task_journal_backfill_v1_sources
    GROUP BY workspace_id,command_id HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'historical Task Journal backfill found multiple lifecycle Events for one command';
  END IF;
END $$;
-- +goose StatementEnd

CREATE TEMP TABLE task_journal_backfill_v1_candidates
ON COMMIT DROP AS
WITH explicit_context AS (
  SELECT s.*,
    CASE s.event_type
      WHEN 'task.created' THEN jsonb_strip_nulls(jsonb_build_object(
        'title', COALESCE(NULLIF(btrim(s.payload #>> '{task,title}'), ''), NULLIF(btrim(s.payload #>> '{task,Title}'), '')),
        'description', COALESCE(NULLIF(btrim(s.payload #>> '{task,description}'), ''), NULLIF(btrim(s.payload #>> '{task,Description}'), '')),
        'currentSummary', COALESCE(NULLIF(btrim(s.payload #>> '{task,currentSummary}'), ''), NULLIF(btrim(s.payload #>> '{task,CurrentSummary}'), '')),
        'terminalReason', COALESCE(NULLIF(btrim(s.payload #>> '{task,terminalReason}'), ''), NULLIF(btrim(s.payload #>> '{task,TerminalReason}'), ''))
      ))
      WHEN 'run.started' THEN jsonb_strip_nulls(jsonb_build_object(
        'kind', NULLIF(btrim(s.payload ->> 'kind'), ''),
        'clientRunId', NULLIF(btrim(s.payload ->> 'clientRunId'), ''),
        'sessionRef', NULLIF(btrim(s.payload ->> 'sessionRef'), '')
      ))
      WHEN 'task.updated' THEN jsonb_strip_nulls(jsonb_build_object('before',s.payload->'before','after',s.payload->'after'))
      WHEN 'task.rework_started' THEN jsonb_strip_nulls(jsonb_build_object('reason',NULLIF(btrim(s.payload->>'reason'),'')))
      WHEN 'task.blocked' THEN jsonb_strip_nulls(jsonb_build_object('reason',NULLIF(btrim(s.payload->>'reason'),'')))
      WHEN 'task.unblocked' THEN jsonb_strip_nulls(jsonb_build_object('reason',NULLIF(btrim(s.payload->>'reason'),'')))
      WHEN 'task.discarded' THEN jsonb_strip_nulls(jsonb_build_object('reason',NULLIF(btrim(s.payload->>'reason'),'')))
      WHEN 'task.implemented_reported' THEN jsonb_strip_nulls(jsonb_build_object('assessment',NULLIF(btrim(s.payload->>'assessment'),'')))
      ELSE '{}'::jsonb
    END || jsonb_strip_nulls(jsonb_build_object(
      'proceedReason',NULLIF(btrim(s.payload->>'proceedReason'),''),
      'warnings',s.payload->'warnings',
      'acknowledgedWarningCodes',s.payload->'acknowledgedWarningCodes'
    )) AS source_context,
    COALESCE(
      NULLIF(btrim(s.payload->>'reason'),''),
      NULLIF(btrim(s.payload->>'assessment'),''),
      NULLIF(btrim(s.payload->>'proceedReason'),'')
    ) AS narrative,
    CASE s.event_type
      WHEN 'task.created' THEN 'created' WHEN 'run.started' THEN 'run_started'
      WHEN 'task.updated' THEN 'updated' WHEN 'task.rework_started' THEN 'rework_started'
      WHEN 'task.blocked' THEN 'blocked' WHEN 'task.unblocked' THEN 'unblocked'
      WHEN 'task.implemented_reported' THEN 'implemented'
      WHEN 'task.confirmed' THEN 'confirmed' WHEN 'task.discarded' THEN 'discarded'
    END AS lifecycle_stage
  FROM task_journal_backfill_v1_sources s
)
SELECT
  e.id,e.workspace_id,e.task_id,e.id AS event_id,e.command_id,e.lifecycle_stage,e.narrative,
  1 AS schema_version,
  e.source_context || jsonb_build_object('_backfill',jsonb_build_object(
    'projectionVersion',1,'source','historical_event',
    'narrativeState',CASE WHEN e.narrative IS NULL THEN 'not_recorded' ELSE 'recorded' END
  )) AS context,
  e.initiated_by_actor_id,e.executed_by_actor_id,e.approved_by_actor_id,
  e.created_at AS occurred_at,e.created_at AS recorded_at
FROM explicit_context e;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM task_journal_backfill_v1_candidates candidate
    JOIN task_journal_entries existing
      ON existing.workspace_id=candidate.workspace_id
     AND (existing.event_id=candidate.event_id OR existing.command_id=candidate.command_id)
    WHERE existing.id IS DISTINCT FROM candidate.id
       OR existing.task_id IS DISTINCT FROM candidate.task_id
       OR existing.event_id IS DISTINCT FROM candidate.event_id
       OR existing.command_id IS DISTINCT FROM candidate.command_id
       OR existing.lifecycle_stage IS DISTINCT FROM candidate.lifecycle_stage
       OR existing.narrative IS DISTINCT FROM candidate.narrative
       OR existing.schema_version IS DISTINCT FROM candidate.schema_version
       OR existing.context IS DISTINCT FROM candidate.context
       OR existing.initiated_by_actor_id IS DISTINCT FROM candidate.initiated_by_actor_id
       OR existing.executed_by_actor_id IS DISTINCT FROM candidate.executed_by_actor_id
       OR existing.approved_by_actor_id IS DISTINCT FROM candidate.approved_by_actor_id
       OR existing.occurred_at IS DISTINCT FROM candidate.occurred_at
       OR existing.recorded_at IS DISTINCT FROM candidate.recorded_at
  ) THEN
    RAISE EXCEPTION 'historical Task Journal backfill conflicts with an existing projection';
  END IF;
END $$;
-- +goose StatementEnd

INSERT INTO task_journal_entries(
  id,workspace_id,task_id,event_id,command_id,lifecycle_stage,narrative,
  schema_version,context,initiated_by_actor_id,executed_by_actor_id,
  approved_by_actor_id,occurred_at,recorded_at
)
SELECT id,workspace_id,task_id,event_id,command_id,lifecycle_stage,narrative,
  schema_version,context,initiated_by_actor_id,executed_by_actor_id,
  approved_by_actor_id,occurred_at,recorded_at
FROM task_journal_backfill_v1_candidates
ON CONFLICT (workspace_id,event_id) DO NOTHING;

-- +goose Down
-- Forward-only: append-only history is retained. Down only moves Goose's
-- version marker so the following Up can validate deterministic idempotency.
SELECT 1;
