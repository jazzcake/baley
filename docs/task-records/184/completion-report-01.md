---
baley_record: 1
record_id: "f6c3ed48-bfad-4f6e-abcb-0e8830af4ee9"
task_id: 184
record_type: completion-report
run_id: "pending-registration"
created_at: "2026-09-10T01:40:00+09:00"
created_by: "codex-worker-term_0678b4a3"
supersedes: null
status: ready_for_registration
---

# Task #184 completion report draft

## Delivered

- Migration 27 deterministically backfills pre-migration-26 Task lifecycle Events into the append-only Task Journal.
- The exact sanitized #183 history includes the `run.started` Event emitted by the original `run.start` command and yields exactly four rows: created, run_started, implemented, and confirmed.
- Allow-listed malformed payloads, missing Task/actor links, entity mismatch, duplicate command candidates, and mismatched existing rows fail the entire migration transaction.
- API readiness expects schema 27, container builds carry commit/version/time provenance, and the operational script/runbook separate preflight, backup, restore drill, migration, verification, service replacement, and application rollback.

No historical rationale, goal, alternative, completion contract, or outcome is inferred. A missing narrative is represented only as `narrative = NULL` with `_backfill.narrativeState = "not_recorded"`.

## Verification evidence

- `go test ./... -count=1`: passed, including the complete integration package against a fresh disposable PostgreSQL 17 container.
- `go vet ./...`: passed.
- `npm test -- --run`: 17 files and 108 tests passed.
- `npm run build`: passed; Vite retained its existing large-chunk warning.
- Disposable migration verification passed for 25→26→27, migration-27 down/up idempotency, exact #183 four-row oracle and approval linkage, paired ordering, Workspace isolation, and fail-closed missing Task/executor/entity, malformed payload, duplicate command, and existing-row conflict cases.
- Isolated `C:\dev-bin\baley\task-184\` executables passed schema-27 migration and MCP diagnostics against a separate disposable PostgreSQL container.
- Server and Viewer Docker images built successfully and both OCI revision labels matched the pinned source commit used for the build.
- The final commit SHA and push receipt are reported by the completing worker after this record is committed.

## Rollout and rollback

Follow `docs/task-journal-rollout-operations.md`. Migration 27 is forward-only and application rollback retains schema 27 plus all backfilled rows; old API and Viewer images are restored by exact image ID. A schema-25 dump restore is disaster recovery, not routine application rollback, and loses writes after the freeze boundary.

## Residual risks

- Historical allow-listed Events outside the sanitized #183 fixture may contain a malformed shape; migration intentionally stops instead of skipping them.
- The Viewer displays only its bounded newest page; complete historical inspection remains available through HTTP and MCP paired cursors.
- Production backup, migration, service replacement, and canary are explicitly outside this implementation run and require the reviewed operational procedure.
