---
baley_record: 1
record_id: "1ed75684-d8a1-4b37-a1c9-8ccabb62d32c"
task_id: 186
record_type: completion-report
run_id: "687582d6-6f76-46e4-89c7-a0a485136a7c"
created_at: "2026-09-10T17:15:00+09:00"
created_by: "codex-worker-term_22739825"
supersedes: "644e3728-d3e7-4930-9f57-857323c2eeb2"
status: completed
---

# Task #186 schema-28 rollout report

## Outcome

Tasks #184-#186 are integrated on pushed commit `e64c2fbe38561ce68755f162e3e37351d6c0d31b`. The live Baley stack advanced from schema 27 to schema 28, and API, Viewer, and MCP now run artifacts built from that exact commit. Task #184 remains `implemented`; the UTF-8 canary #185 returned to `implemented`; #186 is ready for the normal implemented report after this record is registered. Final Task confirmation remains a separate human-only decision.

## Source and immutable artifacts

- Branch `jazzcake/task-journal-finalize` was clean and pushed to `origin/jazzcake/task-journal-finalize` at `e64c2fbe38561ce68755f162e3e37351d6c0d31b` before rollout.
- API image: `sha256:f6e0269377da1cac2ef7f178a9feb93001778e57b4ad1a24bd39efb04b0dbfb7`; OCI revision `e64c2fbe38561ce68755f162e3e37351d6c0d31b`; schema label `28`.
- Viewer image: `sha256:dcb2e48c0324ba0b2fd611234bdf62382b8fc0be1c4be16e6de06c4f634721ad`; the same OCI revision.
- MCP executable: `C:\dev-bin\baley\task-186-rollout\e64c2fbe38561ce68755f162e3e37351d6c0d31b\baley-mcp.exe`; SHA-256 `7DBD719B56FEFE0643F4B989E7100CA7BF0A9F007DCE2ECC58F36E3710AA5783`.
- The repository-supported installer replaced only the loopback Gateway executable and retained the keychain-backed credential store and the compatibility-stable Codex registration.

## Backup, migration, and rollback proof

- Write freeze stopped the API and proved no listener remained on port 8080 before backup.
- Fresh schema-27 backup: `D:\Project_AI\baley-backups\task-186\20260910T075648Z\baley-schema27.dump`; SHA-256 `3ae629108ebf5b5387d65e018ccf6b3be308d30d12e61c7d4144906f11a7d3fe`.
- Backup metadata SHA-256: `443a35ea69f2f9233372eb2cef3f4ebfce60070e17612594b0bf5ded2a5c2ae8`. It pins all 38 public-table counts, Workspace revision 1413, and #184/#185/#186 identities and states.
- Isolated restore database `baley_task186_restore_20260910075652_a51ddf57` restored in one transaction, matched schema 27, every table count, Workspace revision, and all three Task identities/statuses, and was then removed.
- Migration 28 completed once and created the conversational evidence schema. Verification returned schema 28, #183 eligible Events 11, Journal rows 11, bidirectional set difference 0, and invalid provenance rows 0.
- The schema-28 rollback helper recreated the exact immutable API/Viewer pair, verified both exact container image IDs and healthy states, Viewer root HTTP 200, Viewer `/api/readyz`, API `/readyz`, and `/versionz` at schema 28 and the pinned revision. A schema-27 application image remains forbidden; database migration down was not attempted.

## Live API, Viewer, MCP, and Tailscale checks

- Local API `/healthz`, `/readyz`, and `/versionz` passed; readiness reported schema 28 and version `task-186`, while version reported the exact deployment commit.
- Viewer root returned HTTP 200 and its same-origin `/api/readyz` proxy reported schema 28. Tailnet `/api/readyz` and `/api/versionz` returned the same values.
- Ports 5174, 8080, and 8090 remained bound only to `127.0.0.1`.
- Tailscale remained `Running`, online, with no health findings and DNS `jazzcake-home.tail87e929.ts.net.`. The root Serve target remained `http://127.0.0.1:5174`; no Serve or firewall rule changed.
- Fresh MCP sessions returned 15 compact tools and 89 full tools. Both expose `baley_task_journal` and `baley_command_execute_with_approval`; neither exposes the removed alias `baley_command_execute_human`.
- The embedded Viewer reached the live login boundary, proving the deployed route and redirect. No login or human approval was automated; signed-in Inspector inspection is intentionally available to the final independent reviewer.

## UTF-8, Journal, DAG leaf, and conversational confirmation canaries

- #185 Run `b0ac5d58-94e0-4f48-93de-645b47382be5` recorded `스키마 28 UTF-8 왕복 검증 — 한글·é·🙂` through the compact MCP bridge. Journal/Event `573cad69-dd57-41d8-8e26-f2fa0eeb5f7b` and command `602df715-d535-4347-9961-8e698b64d633` matched the exact narrative, Korean context, Task, Event, and command provenance in PostgreSQL and an independently UTF-8-decoded MCP response. Its UTF-8 hex was recorded as `ec8aa4ed82a4eba788203238205554462d3820ec9995ebb3b520eab280eca69d20e2809420ed959ceab880c2b7c3a9c2b7f09f9982`.
- The Run succeeded through command `09d17e6f-f5f0-44b9-b767-4ab24160c486` and Event `7df4cc12-f62d-4784-bd08-3b7a5fdaaa92`. #185 returned to `implemented` through command `7b34c8e6-b511-46a7-b69b-bbcde18a54eb` and Journal/Event `4d9c8d23-18f5-4b31-92c0-355817f20922`.
- The #185 implemented preview emitted only the three expected missing-standalone-record warnings and no dangling/leaf topology warning. This proves an ordinary live DAG leaf needs no invented successor, Gate, or terminal-reason workaround.
- `task.confirm` preview for #185 returned the expected `human_approval_required` boundary. Compact-bridge calls with missing, vague, negated, and stale evidence returned local validation, `decision_evidence_mismatch`, `decision_evidence_mismatch`, and `stale_revision` respectively.
- After the negative probes, Workspace revision stayed 1416, #185 stayed `implemented`, the three decision IDs had zero evidence rows, and no new `task.confirmed` Event existed. No positive human decision was synthesized. DB-backed regression coverage already proves explicit success, linked human/Agent provenance, replay, cross-target, cross-revision, and unauthorized-member failures.

## Regression verification

- `scripts/test-task-journal-rollout.ps1`: 15/15 passed.
- `go test ./internal/application ./internal/domain ./cmd/baley-mcp -count=1`: passed.
- `go vet ./...`: passed.
- `npm test -- --run --reporter=dot`: 16 files and 100 tests passed.
- `npm run build`: passed with 2,111 modules; the existing large-chunk warning remains.
- The earlier fresh PostgreSQL 17.5 full suite remains bound to this commit: 15 packages and 663 test/subtest events passed, with only the intentional external `BALEY_MCP_E2E` skip.

## Record attachment correction

The first attachment attempt incorrectly expanded abbreviated commit `c7d517c` to nonexistent SHA `c7d517ce9408ab27e0b55b4f284786b2500b80d5`. Baley correctly rejected later replacement on the immutable rows with `record_hash_conflict`. This record and the other three #186 records therefore use new UUIDs that explicitly supersede those rows; the correction commit and each exact Git blob are attached from direct `git rev-parse` output. No database row was edited or deleted.

## Residual boundary

The live rollout is complete and recoverable. The external conversation transcript remains evidence conveyed by the authenticated linked Agent rather than something Baley can independently authenticate, so the conservative full-string grammar and all target/revision/hash/idempotency/account bindings remain essential. The final independent review should inspect the signed-in read-only Viewer and then present #184, #185, and #186 separately for explicit human confirmation; this report does not confirm any Task.
