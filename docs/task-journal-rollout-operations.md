# Task Journal historical backfill rollout

Migration 27 deterministically projects explicitly recorded facts from pre-migration-26 Task lifecycle Events into the append-only Task Journal. It does not infer rationale, goals, alternatives, completion contracts, or outcomes. The rollout is forward-only: an application rollback retains schema 27 and every backfilled row.

## Preconditions and stop conditions

Run this only after the reviewed deployment commit contains migrations 26 and 27, the API expects schema 27, and API, Viewer, and MCP binaries/images were built from that same commit. Do not alter firewall rules, Tailscale Serve, the shared PostgreSQL network or volume, Credential Manager data, or an operator's untracked files.

Stop before migration if any of these are true: the checkout is not the reviewed commit, schema is not 25, the schema-25 backup cannot be restored into an isolated database, candidate counts changed after write freeze, or the exact #183 fixture does not produce `created`, `run_started`, `implemented`, and `confirmed` rows. Stop after migration without replacing Viewer or MCP if schema is not 27, migration 27 reports malformed provenance, or the four #183 rows/provenance checks fail.

## Preflight, freeze, backup, and restore drill

Record the deployment SHA, API and Viewer image IDs, current MCP executable path and SHA-256, schema version, Workspace revision, Task #183/#184 status, every public table count, and `tailscale serve status`. Preserve any unrelated dirty or untracked file; do not delete it to satisfy a deployment helper. Set `$workspaceId` to the exact Workspace UUID that owns Tasks #183 and #184; the helper refuses to infer it.

```powershell
.\scripts\task-journal-rollout.ps1 -Action Preflight
docker compose stop api
Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort 8080 -ErrorAction SilentlyContinue

$workspaceId = '00000000-0000-4000-8000-000000000001'
$stamp = [DateTimeOffset]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$backup = "D:\Project_AI\baley-backups\task-184\$stamp"
.\scripts\task-journal-rollout.ps1 -Action Backup -WorkspaceId $workspaceId -BackupDirectory $backup
$restoreDb = 'baley_task184_restore_' + [DateTimeOffset]::UtcNow.ToString('yyyyMMddHHmmss') + '_' + ([guid]::NewGuid().ToString('N').Substring(0,8))
.\scripts\task-journal-rollout.ps1 -Action VerifyRestore -WorkspaceId $workspaceId -BackupFile "$backup\baley-schema25.dump" -BackupMetadataFile "$backup\backup.json" -RestoreDatabase $restoreDb
```

`backup.json` format 2 pins the dump SHA-256, every public table count, Workspace ID/revision, Task #183/#184 database IDs and statuses, and the sorted Event, command, and approval IDs associated with those Tasks and their Runs. `VerifyRestore` checks the dump hash before creating a database, restores it, and compares every pinned value. Any mismatch exits non-zero. A database is dropped only if that invocation's `createdb` succeeded; a pre-existing database with the requested name is never dropped. Keep the dump and `backup.json` outside the repository.

The accepted recovery point objective is the write-freeze boundary. If restore verification fails, restart the schema-25 API image while the database is still schema 25 and do not migrate.

## Immutable build and migration

Pin the reviewed commit and old image IDs before building. The API image labels its exact OCI revision and `org.opencontainers.image.baley.schema-version=27`; the Viewer labels the same revision. `/versionz` must later report that revision and schema 27.

```powershell
$deploySha = (git rev-parse HEAD).Trim()
$env:BALEY_BUILD_VERSION = 'task-184'
$env:BALEY_BUILD_COMMIT = $deploySha
$env:BALEY_BUILD_TIME = [DateTimeOffset]::UtcNow.ToString('O')
docker compose build api viewer
docker image inspect baley-api:latest baley-viewer:latest

.\scripts\task-journal-rollout.ps1 -Action Migrate -DeploySha $deploySha
.\scripts\task-journal-rollout.ps1 -Action Verify
```

The one-shot migration is separate from service start. Migration 27 is transactional; malformed allow-listed payloads, missing Tasks or actors, invalid entity provenance, duplicate candidate commands, or a conflicting existing projection leave Goose at version 26 and insert no candidate rows.

## Service replacement and smoke checks

Replace the API first, then Viewer, then the local MCP Gateway. Do not rely on the API entrypoint to mix migration and service replacement.

```powershell
docker compose up -d --no-deps api
Invoke-WebRequest http://127.0.0.1:8080/healthz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8080/readyz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8080/versionz -UseBasicParsing

docker compose up -d --no-deps viewer
Invoke-WebRequest http://127.0.0.1:5174/ -UseBasicParsing
Invoke-WebRequest https://jazzcake-home.tail87e929.ts.net/api/readyz -UseBasicParsing
Invoke-WebRequest https://jazzcake-home.tail87e929.ts.net/api/versionz -UseBasicParsing
```

Both local and tailnet endpoints must report schema 27 and the pinned deployment commit. Confirm ports 8080, 5174, and 8090 remain loopback-bound and `tailscale serve status` is unchanged. Build the MCP executable under `C:\dev-bin\baley\` only, preserve the existing credential-store metadata, and verify `baley_task_journal` returns the same four #183 rows and paired cursor order as HTTP. In the signed-in Viewer, inspect #183's read-only Journal; the UI intentionally shows only the newest 50 rows, so use HTTP/MCP cursors for a complete history.

## Lifecycle canary and approval stop point

Use one explicitly disposable canary Task and preserve each exact request envelope, response, command ID, Event ID, Journal ID, actor ID, timestamp, Workspace revision, and `contextNote`. Do not invent context. The lifecycle is strictly:

1. `task.create` with the operator-stated goal and completion contract.
2. `run.start(kind=implementation)` with the observed rollout-start fact.
3. `task.report_implemented` with the observed outcome and residual risks.
4. **Stop Agent/Operator writes.** A signed-in human opens the canary Task Inspector, reads its journal/evidence, clicks `Confirm task` to create a fresh preview, and explicitly confirms that exact preview. The browser-bound grant is not copied into chat or manufactured by an Agent.

At steps 1 through 3, send the command once over the authenticated HTTP command endpoint and save the full envelope. Re-send the byte-equivalent envelope with the same idempotency key and expected revision: it must return the original result, command ID, and Journal ID without changing the Workspace revision or any table count. Then change one payload field while retaining the same idempotency key: it must return `idempotency_conflict`, with zero new command, Event, Journal, Task, or Run rows. Send the otherwise-valid next command with a stale Workspace revision: it must fail with the stale-revision diagnostic and the same zero-write proof. These negative checks use test/canary requests only; never alter a successful command's preserved envelope.

For step 4, record counts immediately before the human preview. An old preview or grant bound to an earlier Workspace revision, a different command hash, target, browser session, or warning acknowledgement must fail. Record counts again and prove zero writes. The human then creates a new preview in the signed-in Viewer and confirms only that exact preview. This is the human-only stop point: the Agent may observe the result after the Viewer completes it, but may not execute or synthesize the approval.

After every successful step, compare all three projections:

| Surface | Required provenance match |
| --- | --- |
| HTTP Task/Workspace journal routes | Event ID, Journal ID, command ID/name, lifecycle stage, actor, timestamp, context, Task/Run identity, Workspace revision |
| Viewer Task Inspector | Same newest journal rows and human approval actor/time; read-only, no invented narrative |
| MCP `baley_task_journal` | Same fields and paired cursor order as HTTP; retry returns the same IDs |

For `task.confirm`, additionally match the `task.confirmed` Event, its command, the consumed `human_approval_attestation`, approval grant binding, human actor, and final `confirmed` status across HTTP, Viewer, and MCP. Any missing or divergent field is a failed canary; do not report the rollout complete. Run `task.confirm` only once through the signed-in Viewer after negative stale-grant testing has proved zero writes.

## Rollback

If migration 27 itself fails, it is transactional: fix or explicitly review the historical data before retrying. Do not bypass fail-closed checks. After schema 27 succeeds, a pre-rollout API is forbidden even if its image ID is known. Roll forward, or use only an exact API image that declares schema 27 compatibility and a Viewer image carrying the identical OCI revision:

```powershell
.\scripts\task-journal-rollout.ps1 -Action Rollback -RollbackApiImage 'sha256:<64 hex>' -RollbackViewerImage 'sha256:<64 hex>'
```

The helper inspects the immutable images before changing tags. It rejects an API without `org.opencontainers.image.baley.schema-version=27`, rejects mismatched or absent 40-character API/Viewer OCI revisions, verifies that the database is still schema 27, starts only API and Viewer, and verifies both containers use the requested exact image IDs. It then waits for both Compose healthchecks, requires the Viewer root to return HTTP 200 over its loopback-bound origin, requires the Viewer's same-origin `/api/readyz` proxy to return `ready` on schema 27, and independently checks the API `/readyz` and `/versionz` for schema 27 and the artifact revision. An exited, unhealthy, unreachable, non-200, malformed, or schema-mismatched Viewer/API condition returns non-zero; rollback success is not emitted until every boundary passes.

| Database | API artifact | Viewer/MCP artifact | Allowed outcome |
| --- | --- | --- | --- |
| schema 25 | reviewed pre-rollout schema-25 API | matching pre-rollout artifacts | Allowed only before migration 27 or after separately authorized destructive recovery |
| schema 27 | API image labeled schema 27, `/versionz` revision equals the immutable image revision | Same-commit Viewer; same-commit MCP is preferred, older read-only-compatible Viewer/MCP only with explicit compatibility evidence | Allowed application rollback/roll-forward target |
| schema 27 | schema-25/pre-rollout API, missing compatibility label, or unknown revision | any | Forbidden; `/readyz` would fail and mutation availability is not recoverable |
| schema 25 | schema-27-only API | any | Forbidden; `/readyz` must fail closed |

Do not run migration 26 down as an application rollback: migration 27 down intentionally retains history, while migration 26 down removes the table and is incompatible with schema-27 APIs. Keep schema 27 and the append-only rows, roll back only to a proven schema-27-compatible application set, and diagnose forward. Restore the schema-25 dump only for a separately authorized disaster recovery event that accepts losing all writes after the freeze boundary.
