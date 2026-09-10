---
baley_record: 1
record_id: "540d226e-1efe-456a-a514-5b1a5f968be7"
task_id: 184
record_type: independent-agent-review
run_id: "483bdcaf-1b97-42e4-9d1d-02ca01a117df"
reviewed_commit: "4b0a7792c686f0237a50c6349ab698984f0f7e9b"
created_at: "2026-09-10T11:40:46+09:00"
created_by: "codex-worker-term_ec31fe3f"
verdict: PASS
---

# Task #184 independent review 04

## Verdict

**PASS**

The final post-deploy review found zero blocking and zero material findings. I independently checked the operating Git state, backup and recovery controls, live schema and journal projection, deployed API/Viewer/MCP identities, network exposure, lifecycle canary audit trail, regression suites, and every rollout-only commit through `4b0a7792c686f0237a50c6349ab698984f0f7e9b`.

No implementation, schema or database data, service, MCP installation, Docker/Tailscale/firewall state, or user `debug.log` was changed. The only repository change made by this review is this record.

## Source and record identity

- After `git fetch origin jazzcake/mcp-login-membership-auth`, both `git rev-parse HEAD` and `git rev-parse origin/jazzcake/mcp-login-membership-auth` returned `4b0a7792c686f0237a50c6349ab698984f0f7e9b`.
- `git status --short --branch` showed only `?? debug.log`; `git ls-files --others --exclude-standard` returned only `debug.log`. Its size and SHA-256 remained 954 bytes and `BD7C6284F150EDBE5E256ED884D5BC23C6B08CC420CFE81F740512D703A58BED`.
- `rollout-report-01.md` uses stable record UUID `e6cb529d-c74e-4163-905d-461dcb1d1a1a`, Task `184`, and the contract-valid `completion-report` record type. Its Run `97491670-5d41-40b5-bbda-50224f78d151` exists for Task #184 and is `succeeded`, version 3.
- The report's terminal command `b7777f49-1792-4e15-9b18-a03a3cff5d00` is live `run.succeed` command revision 1384 and owns Event `391cd657-cff6-4545-a62d-b21133b78bbc` (`run.succeeded`). The rejected placeholder `79d027c7-f92f-46c2-8495-8e67a1910aba` matches no live Run ID or client Run ID.
- This review is bound to the actual live Baley Run `483bdcaf-1b97-42e4-9d1d-02ca01a117df`: kind `independent_agent_review`, Task #184, parent/target Run `97491670-5d41-40b5-bbda-50224f78d151`, session `orca:run_79b7d0f999c6/task_7e4c451358aa`. A preallocated UUID `ebd1bfa5-8a49-41ec-a3d2-fb356314a2b5` was never created and is intentionally not represented as a Baley Run.

## Backup and rollback evidence

- `Get-FileHash` independently returned `1C0D7630BFB7BCB8256229D7BF0D552C7AD19173CABEC9902FEAE57AAB07AD2F` for `baley-schema25.dump` and `13EECE548EA7E59759B59107939DF9D466BCE8CD60E4E85B82254E6EBF81F0CC` for `backup.json` under `D:\Project_AI\baley-backups\task-184\20260910T013125Z`.
- `pg_restore --list` accepted the custom dump and reported PostgreSQL 17.5, 291 TOC entries, gzip compression, and source database `baley`.
- The format-2 metadata binds schema 25, deploy SHA `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`, Workspace ID/revision 1379, every public table count, #183/#184 UUID/public ID/status, and sorted Event, command, and approval IDs. Its embedded dump digest equals the independently calculated digest.
- Static inspection of `scripts/task-journal-rollout.ps1 -Action VerifyRestore` confirmed that it checks the dump hash before database creation, restores with `--single-transaction --exit-on-error`, compares schema, all table counts, revision, Task identities, and fixed IDs, and drops only a database created by that invocation. The recorded isolated restore database name is absent after cleanup.
- `scripts/test-task-journal-rollout.ps1` passed 12/12, directly covering pre-existing database ownership, metadata mismatch and dump-hash rejection, bidirectional live projection drift, pre-schema-27 rollback rejection, exact image/container identity, both container health boundaries, Viewer root/proxy failure, and API readiness failure.
- Rollback remains application-only on schema 27. It requires exact SHA-256 image IDs, a schema-27 API label, identical API/Viewer 40-character revisions, exact recreated container images, both health checks, Viewer loopback root and same-origin proxy readiness, and direct API readiness/version agreement. The schema-25 dump is disaster recovery only: using it requires separate authorization and knowingly loses all post-freeze writes.

## Live database and journal

Read-only `psql` queries against database `baley` returned:

| Check | Result |
| --- | --- |
| applied schema | 27 |
| #183 migration-eligible Events | 11 |
| #183 Journal rows | 11 |
| bidirectional `(event_id, command_id)` set difference | 0 |
| invalid Event/command/time/actor provenance rows | 0 |
| #184 | `in_progress`, UUID `62786a24-2a62-4f55-9007-bd7c09441df3` |
| rollout Run | `97491670-5d41-40b5-bbda-50224f78d151`, `succeeded`, version 3 |
| #185 | `implemented`, UUID `c515c874-e1e7-4723-bdfb-abda21f7a5e4` |

`\d+ task_journal_entries` showed the primary key, unique Workspace/Event and Workspace/command constraints, Task/Workspace/time indexes, JSONB GIN index, foreign keys, lifecycle/context/time checks, and enabled `UPDATE OR DELETE` plus `TRUNCATE` append-only triggers. The eligibility query reproduced the migration allow-list, Task identity extraction, and exclusion of Events already carrying `taskJournal`; it did not use the obsolete fixed-four-row assumption.

## Deployed services and network boundary

- Local API `/healthz` returned HTTP 200 `{"status":"ok"}`. Local API and Viewer-proxied `/readyz` returned `ready`, schema 27, version `task-184`; `/versionz` returned schema 27 and exact application deploy SHA `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`.
- Tailnet root, `/api/readyz`, and `/api/versionz` returned HTTP 200 with the same schema and deploy SHA.
- `docker compose ps` and container labels identified API image `sha256:8fcfa31c09e23e1468c4d4af4778b361adc8e6b8632974de422d8d405da53985` and Viewer image `sha256:cdcd10bb99981cc161e56e1df7d0f2504a10a55c50fc9fa50f0c2e7fb8e8848a`, both healthy and revision-pinned; the API schema label is 27.
- `Get-NetTCPConnection -State Listen -LocalPort 8080,5174,8090` returned `127.0.0.1` for all three ports and no wildcard or Tailnet listener.
- `tailscale status --json` reported `Running`, online, no health findings, DNS `jazzcake-home.tail87e929.ts.net.`, and the reported IPv4/IPv6 addresses. `tailscale serve status --json` exactly retained root→5174, `/media`→5080/media, `/pipeline-ops`→8090, `/place-thumbs`→5080/place-thumbs, and the existing 8443-8450 mappings.

## MCP and canary evidence

- The process listening on 8090 is `C:\dev-bin\baley\task-184-rollout\04977a7cf7679c8b57462b99aac84c3d9ef6ffec\baley-mcp.exe serve-http`; the executable SHA-256 is `69C8B5BC278444A2EEF484CB416937B9BDF0A9E930AB36A65BD9917A4BCD4412`.
- Fresh MCP initialize/list calls returned 15 compact tools and 89 full tools. Both profiles expose `baley_task_journal`.
- `baley_task_journal(workspaceId=..., taskId=183, limit=4)` returned 4 rows and cursor `2026-09-08T09:26:15.110433Z` / `c87afef2-39bd-43e9-be92-d82e35966e30`; the paired-cursor continuation returned 7 rows, no overlap, complete command/Event/actor/time provenance, and an empty final cursor.
- Live `mutation_attempts` prove each #185 lifecycle write once, followed by an exact `idempotent` replay with the same command/Event IDs, then a changed-payload `idempotency_conflict` with empty command/Event IDs. The independent stale `run.start` at expected revision 1379 was rejected with `stale_revision`, empty command/Event IDs, and unchanged observed revision 1380.
- The successful IDs match the report: create `40627997...` / Event `8847f877...`; Run start `312a82c8...` / Events `ab2a95fc...`,`b6cb5c95...`; Run succeed `bc6b3b35...` / Event `c76090fe...`; implemented report `8f8090b0...` / Event `f13013b7...`. #185 has exactly the expected `created`, `run_started`, and `implemented` Journal rows; Run terminal state correctly has no Journal row.
- The probed approval grant `af52d5ca-b305-458d-851d-8c26f4e6b7b4` is already `consumed`, bound to #183 at revision 1352 and command `7217ce53...`. The #185 confirmation attempt has no command ID or Event IDs, Workspace revision did not advance, #185 remains `implemented`, and there are zero #185 confirmation attestations and zero post-canary `task.confirm` commands. No human confirmation was synthesized.

## Commit safety review

`git diff --check 04977a7c..4b0a7792` passed. The rollout-only history is exactly `6630c85`, `129a2e5`, `cc7c4a5`, followed by report commits `5e61194`, `a922917`, and `4b0a779`.

- `6630c85` replaces a false fixed-four-row production assertion with Workspace-scoped, bidirectional eligible Event/Journal equality while retaining malformed provenance rejection. Its two added regression cases passed.
- `129a2e5` accepts only an existing `baley-mcp.exe` whose resolved full path is under `C:\dev-bin\baley\`; normal builds also remain under that root.
- `cc7c4a5` changes the source gate to `git status --porcelain --untracked-files=no`. On the operating tree this returns empty despite `debug.log`, while Git still reports tracked working-tree and index changes through the same porcelain command. The installer contains no operation that moves, deletes, or rewrites untracked files.
- The three report commits correct the Run UUID, contract record type, #184 UUID, final counters/revision, and terminal command/Event identity. Read-only database checks confirmed those corrected values.

No rollout-only change weakens the schema, rollback, artifact pinning, network, MCP, Git-dirt, or human-approval boundaries.

## Verification commands and results

- `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test-task-journal-rollout.ps1`: PASS, 12/12.
- `go test ./internal/application ./internal/transport/httpapi ./cmd/baley-mcp -count=1` from `server/`: PASS.
- `go vet ./...` from `server/`: PASS.
- `npm test -- --run`: PASS, 17 files / 108 tests.
- `npm run build`: PASS, 2,112 modules; existing large-chunk warning only.
- `docker compose ps --format json`, local/Tailnet HTTP probes, process/hash inspection, MCP initialize/list/call, Tailscale status/Serve inspection, and read-only PostgreSQL queries: PASS with the exact results above.

No `go run` was used and no Go binary was created during this review.

## Residual nonblocking risks and human boundary

- I did not rerun the destructive restore drill or mutate the live database; I independently validated the dump, metadata, restore manifest, helper control flow, cleanup ownership, and 12 safety cases. A future disaster restore still requires explicit authorization and acceptance of post-freeze data loss.
- The signed-in Viewer Inspector was not available in this Agent session. HTTP-backed MCP pagination and direct database equality provide complete journal coverage, but human-session UI inspection remains an optional operational check.
- The Viewer build retains its pre-existing large-chunk warning.
- #185 intentionally remains `implemented`. #184 also remains `in_progress` until the ordinary Agent completion workflow is performed.

Task confirmation is human-only. This review does not confirm #184 or #185, does not issue or consume a browser approval grant, and does not treat chat, an Agent bearer, or this PASS verdict as human authority. A signed-in human must separately inspect the Task, create a fresh exact preview in the Viewer, and explicitly confirm it; stale, mismatched, or already-consumed grants remain invalid.
