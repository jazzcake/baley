---
baley_record: 1
record_id: "6c0be69a-25fa-4d7a-bf8b-97d82db6362d"
task_id: 184
record_type: review-response
run_id: "pending-registration"
created_at: "2026-09-10T02:23:13+09:00"
created_by: "codex"
responds_to: "independent-review-01.md"
status: implemented
---

# Task #184 review response 01

## Outcome

All four findings in `independent-review-01.md` were addressed without editing or replacing the independent review. The rollout remains documentation and local implementation work only; no operational branch, production `baley` database data/schema, service, MCP registration/session, Tailscale configuration, or firewall rule was changed.

During the first PowerShell mock test attempt, a scope error allowed the helper to reach `local-dev-postgres`: it created the fixed disposable database `baley_task184_restore_20260910010101_deadbeef`, rejected the invalid dump, and immediately dropped only that owned database. The production `baley` database was never targeted. A final read-only catalog check returned zero databases with that exact name, and the corrected regression harness proves its Docker mock does not leak into the caller.

## Finding responses

### F1 — restore database ownership and non-deletion

`VerifyRestore` now records ownership only after `createdb` succeeds and calls `dropdb` only when that ownership flag is true. A failed `createdb`, including collision with a pre-existing database, exits non-zero and leaves that database untouched. The PowerShell safety suite directly simulates the collision and asserts that no `dropdb` call is made; it separately proves that a database created by the invocation is cleaned after success or later verification failure.

### F2 — schema-27 rollback compatibility and readiness

Routine rollback after migration 27 now accepts only an exact API image whose immutable metadata declares `org.opencontainers.image.baley.schema-version=27`. API and Viewer images must declare the same 40-character OCI revision. The helper verifies the live database is schema 27 before changing local tags, recreates only API and Viewer from the exact requested image IDs, waits for API container health, then checks `/readyz` and `/versionz` for schema 27 and the API artifact revision. Any mismatch exits non-zero.

The runbook now includes an explicit artifact/database compatibility matrix. Pre-rollout/schema-25 API images are forbidden on schema 27; destructive schema-25 restore remains a separately authorized disaster-recovery operation, not application rollback.

### F3 — backup metadata identity and automatic restore comparison

`Backup` requires an explicit Workspace UUID and writes metadata format 2. It pins the dump SHA-256, schema, every public table count, Workspace revision, Task #183/#184 IDs and statuses, and sorted Event, command, and approval IDs for the two Tasks and their Runs. `VerifyRestore` requires the metadata path explicitly, checks the hash before creating a database, and automatically compares every pinned value after restore. Any missing field or mismatch exits non-zero.

The safety suite covers a matching restore, hash mismatch before `createdb`, and restored metadata mismatch with non-zero failure and owned-database cleanup.

### F4 — complete lifecycle canary and human stop point

The runbook now specifies `task.create -> run.start -> task.report_implemented -> signed-in Viewer task.confirm`. It marks the boundary after `task.report_implemented` as a hard Agent/Operator stop point: only a signed-in human may create and consume the exact browser-bound confirmation grant.

For HTTP, Viewer, and MCP it requires matching Event, Journal, command, actor/time, context, Task/Run identity, revision, approval attestation, and cursor provenance. It also specifies byte-equivalent retry with the same envelope and IDs, changed-payload idempotency conflict, stale-revision zero-write proof, stale/mismatched-grant zero-write proof, and a fresh Viewer preview after negative testing.

## Verification

- `scripts/test-task-journal-rollout.ps1`: 7/7 safety cases passed, including pre-existing database non-deletion, metadata/hash mismatch failure, owned cleanup, schema-25 API rejection, rollback health/version success, and incompatible `/readyz` failure.
- `go test ./... -count=1 -parallel=1 -p=1`: passed after schema-27 bootstrap against a fresh disposable PostgreSQL 17 container; the integration package completed in 52.976 seconds and the container was removed.
- `go vet ./...`: passed.
- `npm test -- --run`: 17 files and 108 tests passed.
- `npm run build`: passed; the existing large-chunk warning remains.
- Isolated executables under `C:\dev-bin\baley\task-184\`: server migration/readiness/version and MCP diagnostics passed against a separate disposable PostgreSQL container and temporary credential path.
- Isolated production Docker images: schema-27/revision labels, API readiness/version, Viewer response, and Viewer same-origin `/api/readyz` proxy passed on a unique Docker network; smoke containers, network, and image tags were removed.
- Final diff check, commit, and push are reported in the completing worker handoff.

## Residual boundary

The production rollout and the signed-in human canary confirmation remain operator-owned execution. This response strengthens and tests the helper/runbook but does not exercise production services, the production database, MCP, Tailscale, or firewall state.
