---
baley_record: 1
record_id: "e6cb529d-c74e-4163-905d-461dcb1d1a1a"
task_id: 184
record_type: rollout-report
run_id: "97491670-5d41-40b5-8b6e30e4355c"
created_at: "2026-09-10T11:10:00+09:00"
created_by: "codex-worker-term_76c61e49"
supersedes: null
status: completed
---

# Task #184 rollout report 01

## Outcome

Task Journal was deployed forward from schema 25 to schema 27 on the existing local Baley PostgreSQL/API/Viewer/MCP stack. API, Viewer, and MCP application artifacts were built from reviewed deploy commit `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`; the operating branch then received rollout-only helper, installer, and report commits.

The initial rollout Run `2e80d5d1-4c16-48c6-9b69-f9863735dd16` was interrupted after Orca terminal loss with zero rollout changes. Continuation Run `97491670-5d41-40b5-8b6e30e4355c` performed this deployment. The stale planning placeholder `79d027c7-f92f-46c2-8495-8e67a1910aba` was not present in the live database and is not claimed as an actual Run.

## Source and artifact pinning

- Source rollout worktree: clean `jazzcake/task-journal-rollout` at `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`.
- Operating branch fast-forwarded from `ca44415cd2776981b558755c60a11886a25296ec` to the reviewed deploy commit and pushed before deployment.
- API image before: `sha256:b59a6e2334cd067c7dbb07431a462da82b4e34b81acfb707f2d367db85c8181e`.
- Viewer image before: `sha256:c8361e1e55c2d6330562ec389cef165e39bbbae308c481a22a77a390cb6cb042`.
- API image after: `sha256:8fcfa31c09e23e1468c4d4af4778b361adc8e6b8632974de422d8d405da53985`; OCI revision `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`, schema label `27`.
- Viewer image after: `sha256:cdcd10bb99981cc161e56e1df7d0f2504a10a55c50fc9fa50f0c2e7fb8e8848a`; OCI revision `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`.
- MCP before: `C:\dev-bin\baley\releases\39c58946924c\baley-mcp.exe`, SHA-256 `BF3ADA6C53E3C50F2E0D53FC625F5A8E0F1FD65A008FD4974537FD6BF49DFCF6`.
- MCP after: `C:\dev-bin\baley\task-184-rollout\04977a7cf7679c8b57462b99aac84c3d9ef6ffec\baley-mcp.exe`, SHA-256 `69C8B5BC278444A2EEF484CB416937B9BDF0A9E930AB36A65BD9917A4BCD4412`.
- The per-user `Baley MCP Gateway` scheduled task still launches hidden PowerShell and the repository-supported launcher. Its routing-only credential metadata file stayed 145 bytes with SHA-256 `7D29C1E7A2F1FE3041563F59B9AD60803719BB1FCF4281502BA2220A937A0BF0`; no credential content was printed or copied.
- User `debug.log` remained 954 bytes with SHA-256 `BD7C6284F150EDBE5E256ED884D5BC23C6B08CC420CFE81F740512D703A58BED`. It is the only untracked operating-worktree file.

## Freeze, backup, and restore drill

- API/write path was stopped and `127.0.0.1:8080` had no listener before backup.
- Backup directory: `D:\Project_AI\baley-backups\task-184\20260910T013125Z`.
- Dump: `baley-schema25.dump`, SHA-256 `1C0D7630BFB7BCB8256229D7BF0D552C7AD19173CABEC9902FEAE57AAB07AD2F`.
- Metadata: `backup.json`, format 2, SHA-256 `13EECE548EA7E59759B59107939DF9D466BCE8CD60E4E85B82254E6EBF81F0CC`.
- Isolated restore database: `baley_task184_restore_20260910013125_f16bc070`. The helper verified dump hash, schema 25, every table count, Workspace revision 1379, #183/#184 identity/status, and fixed Event/command/approval IDs, then dropped only the database it created.
- At freeze, #183 was `confirmed` (database ID `0393bf4a-c53d-4ccd-937c-50528b044c32`) and #184 was `in_progress` (database ID `c95aeaf0-8be3-4cc5-a99d-bdb0db23f50f`).

## Database before and after

Schema moved from 25 to 27. The helper's first live fixed-four-row check correctly stopped after migration because #183 had eight eligible historical Runs. The authorized correction now derives migration eligibility exactly and proves bidirectional equality by Event/command ID: #183 eligible Events 11, Journal rows 11, set difference 0, invalid provenance 0. The sanitized fixture remains exactly four rows.

| Public table | Freeze count | Post-canary count |
|---|---:|---:|
| account_credentials | 1 | 1 |
| account_external_identities | 1 | 1 |
| account_sessions | 94 | 94 |
| accounts | 2 | 2 |
| actors | 3 | 3 |
| agent_tokens | 68 | 71 |
| approval_grants | 2 | 2 |
| auth_login_limits | 2 | 2 |
| backlog_items | 74 | 74 |
| commands | 3313 | 3317 |
| commit_references | 68 | 68 |
| events | 3231 | 3236 |
| evidence_profiles | 6 | 6 |
| gate_entry_tasks | 0 | 0 |
| gate_tasks | 23 | 23 |
| gates | 10 | 10 |
| goose_db_version | 26 | 28 |
| human_approval_attestations | 90 | 90 |
| lanes | 22 | 22 |
| mcp_connection_requests | 1 | 1 |
| mcp_gateway_registrations | 5 | 5 |
| mutation_attempts | 4092 | 4106 |
| oidc_authorization_flows | 28 | 28 |
| phases | 16 | 16 |
| repositories | 5 | 5 |
| run_git_observations | 5 | 5 |
| runs | 713 | 714 |
| security_events | 227 | 230 |
| task_acceptance_assignments | 213 | 214 |
| task_acceptance_evidence | 18 | 18 |
| task_dependencies | 184 | 184 |
| task_journal_entries | absent | 1189 |
| task_record_indexes | 490 | 490 |
| tasks | 213 | 214 |
| workspace_acceptance_policies | 6 | 6 |
| workspace_counters | 6 | 6 |
| workspace_memberships | 12 | 12 |
| workspaces | 6 | 6 |

The post-canary Workspace revision is 1383. Agent-token, mutation-attempt, and security-event growth includes normal MCP gateway credential renewal and the explicitly recorded negative probes; successful domain writes account for four commands and five Events.

## Service and network verification

- `http://127.0.0.1:8080/healthz`: `{"status":"ok"}`.
- Local and tailnet `readyz`: schema 27, status `ready`, version `task-184`.
- Local and tailnet `versionz`: exact commit `04977a7cf7679c8b57462b99aac84c3d9ef6ffec`, schema 27.
- Viewer root `http://127.0.0.1:5174/`: HTTP 200.
- Tailnet URLs: `https://jazzcake-home.tail87e929.ts.net/`, `/api/readyz`, and `/api/versionz`.
- Ports 8080, 5174, and 8090 are all bound only to `127.0.0.1`.
- Tailscale remained Running/online with no health findings, DNS name `jazzcake-home.tail87e929.ts.net.`, IPs `100.71.243.110` and `fd7a:115c:a1e0::1c01:f3b4`.
- Tailscale Serve targets were unchanged: root to 5174, `/media` to 5080/media, `/pipeline-ops` to 8090, `/place-thumbs` to 5080/place-thumbs, plus the unchanged 8443-8450 mappings.

## Journal and MCP verification

- Compact catalog: 15 tools; full catalog: 89 tools; both expose `baley_task_journal`.
- #183 MCP pagination returned 4 rows then 7 rows using the exact paired cursor `2026-09-08T09:26:15.110433Z` / `c87afef2-39bd-43e9-be92-d82e35966e30`, with no overlap and an empty final cursor.
- All 11 #183 rows contain Event ID, command ID/name, actor, timestamp, lifecycle stage, Task/Workspace identity, and historical provenance. The live Event and Journal sets are exactly equal.
- The MCP tool performs the authenticated GET against the HTTP journal route; the independent raw browser GET correctly returned `unauthenticated` because no Baley browser session was signed in. Viewer Task Inspector verification was therefore not available and no login or approval was automated.

## Disposable lifecycle canary

- Task: #185, UUID `c515c874-e1e7-4723-bdfb-abda21f7a5e4`, `implemented`.
- Implementation Run: `3b723b9a-cffd-4716-b1c1-bf597fd10b4e`, client Run `9c29660a-6b85-4e80-9d1c-4e660ad47860`, `succeeded`.
- `task.create`: command `40627997-55ae-4a37-bec7-f1c590deb9c1`; Event/Journal `8847f877-ed03-47b9-92d9-c87fae59f78a`.
- `run.start`: command `312a82c8-7779-4fcc-bae0-f32efdcee04f`; Events `ab2a95fc-b188-4744-80d7-2b3b2c26dcd2`, `b6cb5c95-25c4-4413-87c4-2c343d48bfde`; Journal `b6cb5c95-25c4-4413-87c4-2c343d48bfde`.
- `run.succeed`: command `bc6b3b35-a6e2-426f-baa3-2352359120f6`; Event `c76090fe-a878-4a8a-accc-4bf7942a0d37`; no Task Journal row is expected for Run terminal state.
- `task.report_implemented`: command `8f8090b0-23e5-420a-9d24-e135a15cb992`; Event/Journal `f13013b7-1d46-4b11-a888-723913157d14`.
- Exact retries returned the same command/Event/Journal IDs with `idempotent=true` and no count or revision changes.
- Same-key changed-payload retries returned `idempotency_conflict` with zero writes.
- The otherwise-valid `run.start` probe at stale revision 1379 returned `stale_revision` with zero writes.
- Two real browser grants existed but were already consumed. A canary confirm preview returned `human_approval_required`; a probe using an old consumed grant returned HTTP 409 and left commands, Events, Journal, attestations, Tasks, Runs, and Workspace revision unchanged.
- No fresh signed-in browser grant existed. The Agent stopped before confirmation; #185 remains `implemented`.

Exact successful envelopes are preserved outside the repository next to the backup:

| Envelope | SHA-256 |
|---|---|
| `canary-create-execute.json` | `CBDBA9AB91392FBCDCA5EFA85AB084DDCEC6BC603702761FFCFF3DB8D0DCB8B9` |
| `canary-run-start-execute.json` | `AAAF54DED68EB691D92DEBBBE5943EAF2F45E8DF674EDF083594E4D650FFEFAB` |
| `canary-run-succeed-execute.json` | `E17CBB11E3692ABA998A5ED77A8A8CE3903D907CD87A343F710CF1539A4ADA12` |
| `canary-report-execute.json` | `7CC7356C35E6A713120ACC9A5E38BF52261447F7CEB2B49265EB46A5437CAE89` |
| `canary-confirm-stale-grant.json` | `8576505EA8B069B9181E4731BF2B4DA14AF54808706E71214430F97540757B46` |

## Tests, rollback, and residual risk

- Rollout safety suite: all 12 tests passed after adding live Event/Journal equality checks.
- API/Viewer immutable builds passed; Viewer production build retained the pre-existing large-chunk warning and npm reported dependency audit findings not introduced by this rollout.
- MCP compact/full catalog and cursor checks passed after repository-supported scheduled-task replacement. Installer hardening now ignores unrelated untracked operator files while still rejecting tracked/staged source changes.
- Schema 27 is forward-only. The pre-rollout API image has no schema-27 compatibility label and is forbidden after migration. The immediate application recovery target is the exact pinned schema-27 API/Viewer pair above, or a reviewed roll-forward build with matching revision labels.
- The schema-25 dump is disaster-recovery evidence only; restoring it requires separate authorization and accepts loss of every post-freeze write.
- Nonblocking residuals: Viewer Inspector was not accessible without a signed-in human session; #185 confirmation was intentionally not performed; Task #184 stays `in_progress` until an independent post-deploy review.
