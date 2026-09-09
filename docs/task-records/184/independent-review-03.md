---
baley_record: 1
record_id: "b6befc23-49ad-46c2-b88d-86fb0e25b680"
task_id: 184
record_type: independent-review
reviewed_commit: "bd5bc52e32eb091b8283263266cd3c07494e55d5"
created_at: "2026-09-10T03:24:41+09:00"
created_by: "codex-worker-term_bd02d9cb"
verdict: PASS
---

# Task #184 independent review 03

## Verdict

**PASS**

Independent re-review of final HEAD `bd5bc52e32eb091b8283263266cd3c07494e55d5` found zero blocker and zero material findings. Original findings F1-F4 and follow-up finding R2-F1 are resolved. No implementation source, prior record, production service or database, MCP registration/session, Tailscale setting, firewall rule, or `debug.log` was changed during this review.

## Finding disposition

| Finding | Result | Independent evidence |
| --- | --- | --- |
| F1 — pre-existing restore database deletion | **Resolved** | Restore ownership starts false and becomes true only after `createdb` succeeds. Cleanup calls `dropdb` only for an invocation-owned database; the collision case passed and recorded no `dropdb`. |
| F2 — rollback compatibility and readiness | **Resolved** | Rollback requires database schema 27, exact SHA-256 image IDs, an API schema-27 OCI label, identical 40-character API/Viewer revisions, exact running container image IDs, both health boundaries, Viewer root/proxy success, and direct API readiness/version agreement. A pre-schema-27 API is rejected before tag or container mutation. |
| F3 — backup/restore identity binding | **Resolved** | Format-2 metadata binds the dump SHA-256, every public table count, Workspace ID/revision, Task #183/#184 internal IDs/public IDs/statuses, and sorted Event, command, and approval IDs. The dump hash is checked before `createdb`, and every pinned field is compared after restore. |
| F4 — lifecycle canary approval boundary | **Resolved** | The runbook separates `task.create`, `run.start`, and `task.report_implemented` from a hard Agent/Operator write stop. Only a signed-in human may create and confirm the fresh Viewer preview; stale/mismatched grants require zero-write proof, and HTTP/Viewer/MCP provenance must agree. |
| R2-F1 — Viewer rollback could succeed while unavailable | **Resolved** | Container discovery uses `docker compose ps -q --all <service>`, so an exited Viewer remains discoverable for exact-image and state inspection. Rollback then waits for Viewer health, requires loopback root HTTP 200, and requires the same-origin `/api/readyz` proxy to report `ready` on schema 27 before direct API checks and success output. |

## Focused failure-path review

- Exact API and Viewer container IDs are obtained after forced recreation with `--all`; missing, ambiguous, or uninspectable output fails closed.
- Exact image identity is verified before either health or HTTP success can be accepted.
- `unhealthy`, `exited`, and `dead` states fail immediately; an unready container times out nonzero.
- Viewer root transport/HTTP failure, malformed proxy JSON, proxy status failure, or proxy schema mismatch prevents direct API checks and prevents success output.
- Direct API `/readyz` and `/versionz` remain independent final boundaries, including schema 27 and immutable revision agreement.
- The Compose Viewer healthcheck probes its internal root on `127.0.0.1:5174`, while the helper independently probes the loopback-published root and same-origin API proxy.

## Verification executed

- `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test-task-journal-rollout.ps1`: **PASS**, 10/10 safety cases, including restore ownership, metadata mismatch/hash rejection, rollback compatibility, exact containers/both health states, exited Viewer, unreachable Viewer root, incompatible Viewer proxy, and incompatible direct API readiness.
- `go test ./internal/application ./internal/transport/httpapi ./cmd/baley-mcp -count=1` from `server/`: **PASS**.
- `go vet ./...` from `server/`: **PASS**.
- `npm test -- --run`: **PASS**, 17 files / 108 tests.
- `npm run build`: **PASS**; the pre-existing large-chunk warning remains.
- `docker compose config --quiet`: **PASS**.
- `git diff --check ae74152e358e01fdac426ee453467d66e2750358..HEAD`: **PASS**.
- Static exact-discovery assertion for `docker compose ps -q --all $Service`: **PASS**.

## Commit and remote state

After an independent `git fetch origin jazzcake/task-journal-rollout`, local `HEAD` and `origin/jazzcake/task-journal-rollout` both resolved to `bd5bc52e32eb091b8283263266cd3c07494e55d5`. The worktree was clean before this review record was created.

## Residual nonblocking risks

The production backup/restore drill, migration, deployed API/Viewer/MCP smoke, full lifecycle canary, and signed-in human confirmation are intentionally operator-owned and were not executed in this source review. The UI build still emits the existing large-chunk warning. These are acceptance/operational follow-through items, not blocking or material defects in the reviewed commit.
