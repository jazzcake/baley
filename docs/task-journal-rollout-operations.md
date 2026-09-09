# Task Journal historical backfill rollout

Migration 27 deterministically projects explicitly recorded facts from pre-migration-26 Task lifecycle Events into the append-only Task Journal. It does not infer rationale, goals, alternatives, completion contracts, or outcomes. The rollout is forward-only: an application rollback retains schema 27 and every backfilled row.

## Preconditions and stop conditions

Run this only after the reviewed deployment commit contains migrations 26 and 27, the API expects schema 27, and API, Viewer, and MCP binaries/images were built from that same commit. Do not alter firewall rules, Tailscale Serve, the shared PostgreSQL network or volume, Credential Manager data, or an operator's untracked files.

Stop before migration if any of these are true: the checkout is not the reviewed commit, schema is not 25, the schema-25 backup cannot be restored into an isolated database, candidate counts changed after write freeze, or the exact #183 fixture does not produce `created`, `run_started`, `implemented`, and `confirmed` rows. Stop after migration without replacing Viewer or MCP if schema is not 27, migration 27 reports malformed provenance, or the four #183 rows/provenance checks fail.

## Preflight, freeze, backup, and restore drill

Record the deployment SHA, API and Viewer image IDs, current MCP executable path and SHA-256, schema version, Workspace revision, Task #183/#184 status, table and allow-list Event counts, and `tailscale serve status`. Preserve any unrelated dirty or untracked file; do not delete it to satisfy a deployment helper.

```powershell
.\scripts\task-journal-rollout.ps1 -Action Preflight
docker compose stop api
Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort 8080 -ErrorAction SilentlyContinue

$stamp = [DateTimeOffset]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$backup = "D:\Project_AI\baley-backups\task-184\$stamp"
.\scripts\task-journal-rollout.ps1 -Action Backup -BackupDirectory $backup
$restoreDb = 'baley_task184_restore_' + [DateTimeOffset]::UtcNow.ToString('yyyyMMddHHmmss') + '_' + ([guid]::NewGuid().ToString('N').Substring(0,8))
.\scripts\task-journal-rollout.ps1 -Action VerifyRestore -BackupFile "$backup\baley-schema25.dump" -RestoreDatabase $restoreDb
```

The accepted recovery point objective is the write-freeze boundary. If restore verification fails, restart the old API image and do not migrate. Keep the dump and `backup.json` outside the repository.

## Immutable build and migration

Pin the reviewed commit and old image IDs before building. The Dockerfiles embed the build version, commit, time, and OCI revision; `/versionz` must later report the same commit.

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

Finally create a canary Task with only facts the operator actually states, start one Run, and confirm the new Event-backed row appears through API, Viewer, and MCP. Do not invent missing context to populate the canary.

## Rollback

If migration 27 itself fails, it is transactional: fix or explicitly review the historical data before retrying. Do not bypass fail-closed checks. If service smoke fails after schema 27 succeeds, restore the exact old API and Viewer image IDs:

```powershell
.\scripts\task-journal-rollout.ps1 -Action Rollback -RollbackApiImage 'sha256:<64 hex>' -RollbackViewerImage 'sha256:<64 hex>'
```

Do not run migration 26 down as an application rollback: migration 27 down intentionally retains history, while migration 26 down removes the table and is incompatible with the old API's expected schema. Keep schema 27 and the append-only rows, roll back only application images, and diagnose forward. Restore the schema-25 dump only for a separately authorized disaster recovery event that accepts losing all writes after the freeze boundary.
