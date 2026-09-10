# Task Journal historical backfill rollout

Migration 27 deterministically projects explicitly recorded facts from pre-migration-26 Task lifecycle Events into the append-only Task Journal. Migration 28 adds the conversational decision-evidence audit boundary. Neither infers rationale, goals, alternatives, completion contracts, outcomes, or human approval. The rollout is forward-only: an application rollback retains schema 28 and every backfilled or consumed evidence row.

## Preconditions and stop conditions

Run this only after the reviewed deployment commit contains migrations 26 through 28, the API expects schema 28, and API, Viewer, and MCP binaries/images were built from that same commit. Do not alter firewall rules, Tailscale Serve, the shared PostgreSQL network or volume, Credential Manager data, or an operator's untracked files.

Stop before migration if any of these are true: the checkout is not the reviewed commit, schema is not 25, the schema-25 backup cannot be restored into an isolated database, candidate counts changed after write freeze, or the sanitized #183 fixture does not produce its exact four `created`, `run_started`, `implemented`, and `confirmed` rows. Stop after migration without replacing Viewer or MCP if schema is not 28, migration 27 reports malformed provenance, migration 28 lacks its evidence table/attestation link, or the live #183 migration-eligible Event set and Task Journal set are not bidirectionally equal by Event and command ID.

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

Pin the reviewed commit and old image IDs before building. The API image labels its exact OCI revision and `org.opencontainers.image.baley.schema-version=28`; the Viewer labels the same revision. `/versionz` must later report that revision and schema 28.

```powershell
$deploySha = (git rev-parse HEAD).Trim()
$env:BALEY_BUILD_VERSION = 'task-184'
$env:BALEY_BUILD_COMMIT = $deploySha
$env:BALEY_BUILD_TIME = [DateTimeOffset]::UtcNow.ToString('O')
docker compose build api viewer
docker image inspect baley-api:latest baley-viewer:latest

.\scripts\task-journal-rollout.ps1 -Action Migrate -DeploySha $deploySha
.\scripts\task-journal-rollout.ps1 -Action Verify -WorkspaceId $workspaceId
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

Both local and tailnet endpoints must report schema 28 and the pinned deployment commit. Confirm ports 8080, 5174, and 8090 remain loopback-bound and `tailscale serve status` is unchanged. Build the MCP executable under `C:\dev-bin\baley\` only, preserve the existing credential-store metadata, and verify `baley_task_journal` returns the same complete live #183 history and paired cursor order as HTTP. The sanitized migration fixture remains the exact four-row lifecycle example; a live Task may correctly have additional eligible lifecycle Events such as multiple Runs. In the signed-in Viewer, inspect #183's read-only Journal; the UI intentionally shows only the newest 50 rows, so use HTTP/MCP cursors for a complete history.

## Lifecycle canary and approval stop point

Use one explicitly disposable canary Task and preserve each exact request envelope, response, command ID, Event ID, Journal ID, actor ID, timestamp, Workspace revision, and `contextNote`. Do not invent context. The lifecycle is strictly:

1. `task.create` with the operator-stated goal and completion contract.
2. `run.start(kind=implementation)` with the observed rollout-start fact.
3. `task.report_implemented` with the observed outcome and residual risks.
4. The signed-in human states an explicit decision in the current conversation, such as `confirm #<id>`. The Agent fresh-previews that exact Task and executes `task.confirm` over MCP with a unique, target-bound `decisionEvidence`; the Viewer remains read-only.

At steps 1 through 3, send the command once over the authenticated HTTP command endpoint and save the full envelope. Re-send the byte-equivalent envelope with the same idempotency key and expected revision: it must return the original result, command ID, and Journal ID without changing the Workspace revision or any table count. Then change one payload field while retaining the same idempotency key: it must return `idempotency_conflict`, with zero new command, Event, Journal, Task, or Run rows. Send the otherwise-valid next command with a stale Workspace revision: it must fail with the stale-revision diagnostic and the same zero-write proof. These negative checks use test/canary requests only; never alter a successful command's preserved envelope.

For step 4, record counts immediately before the MCP preview. Missing, vague, negated, stale, replayed, cross-target, or cross-revision evidence must fail with zero writes. A linked member who no longer has `task:approve` must also fail. The Agent may transmit only the human's explicit current-conversation statement; it may not synthesize or infer approval. Preserve the decision ID, conversation reference, statement hash, linked human, executing Agent, command hash, idempotency hash, and resulting attestation/Journal linkage.

After every successful step, compare all three projections:

| Surface | Required provenance match |
| --- | --- |
| HTTP Task/Workspace journal routes | Event ID, Journal ID, command ID/name, lifecycle stage, actor, timestamp, context, Task/Run identity, Workspace revision |
| Viewer Task Inspector | Same newest journal rows and human approval actor/time; read-only, no invented narrative |
| MCP `baley_task_journal` | Same fields and paired cursor order as HTTP; retry returns the same IDs |

For `task.confirm`, additionally match the `task.confirmed` Event, its command, consumed `conversational_decision_evidence`, linked `human_approval_attestation`, initiating human, executing Agent, and final `confirmed` status across HTTP, Viewer, and MCP. Any missing or divergent field is a failed canary; do not report the rollout complete. Run the successful `task.confirm` only once through the compatibility-stable compact tool after negative evidence tests have proved zero writes.

## Rollback

If migration 27 or 28 fails, it is transactional: fix or explicitly review the data before retrying. Do not bypass fail-closed checks. After schema 28 succeeds, a pre-rollout API is forbidden even if its image ID is known. Roll forward, or use only an exact API image that declares schema 28 compatibility and a Viewer image carrying the identical OCI revision:

```powershell
.\scripts\task-journal-rollout.ps1 -Action Rollback -RollbackApiImage 'sha256:<64 hex>' -RollbackViewerImage 'sha256:<64 hex>'
```

The helper inspects the immutable images before changing tags. It rejects an API without `org.opencontainers.image.baley.schema-version=28`, rejects mismatched or absent 40-character API/Viewer OCI revisions, verifies that the database is still schema 28, starts only API and Viewer, and verifies both containers use the requested exact image IDs. It then waits for both Compose healthchecks, requires the Viewer root to return HTTP 200 over its loopback-bound origin, requires the Viewer's same-origin `/api/readyz` proxy to return `ready` on schema 28, and independently checks the API `/readyz` and `/versionz` for schema 28 and the artifact revision. An exited, unhealthy, unreachable, non-200, malformed, or schema-mismatched Viewer/API condition returns non-zero; rollback success is not emitted until every boundary passes.

| Database | API artifact | Viewer/MCP artifact | Allowed outcome |
| --- | --- | --- | --- |
| schema 25 | reviewed pre-rollout schema-25 API | matching pre-rollout artifacts | Allowed only before migration 27 or after separately authorized destructive recovery |
| schema 28 | API image labeled schema 28, `/versionz` revision equals the immutable image revision | Same-commit Viewer and MCP required for conversational confirmation | Allowed application rollback/roll-forward target |
| schema 28 | schema-25/pre-rollout API, missing compatibility label, or unknown revision | any | Forbidden; `/readyz` would fail and mutation availability is not recoverable |
| schema 25 | schema-28-only API | any | Forbidden; `/readyz` must fail closed |

Do not run migrations down as an application rollback: migration 28 down removes evidence linkage, migration 27 down intentionally retains history, and migration 26 down removes the Journal table. Keep schema 28 and all audit rows, roll back only to a proven schema-28-compatible application set, and diagnose forward. Restore the schema-25 dump only for a separately authorized disaster recovery event that accepts losing all writes after the freeze boundary.
