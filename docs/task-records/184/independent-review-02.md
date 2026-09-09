---
baley_record: 1
record_id: "b722d529-84a0-44f5-a2f5-949143979e6a"
task_id: 184
record_type: independent-review
reviewed_commit: "ae74152e358e01fdac426ee453467d66e2750358"
created_at: "2026-09-10T03:04:57+09:00"
created_by: "codex-worker-term_d034504e"
verdict: CHANGES_REQUIRED
---

# Task #184 independent review 02

## Verdict

**CHANGES_REQUIRED**

The correction closes F1, F3, and F4 and closes the schema-compatibility portion of F2. It does not make the two-service application rollback fail-closed: the helper can return success while the Viewer container is stopped or otherwise unavailable. There are zero blocker findings and one HIGH material finding, so the PASS condition is not met.

No implementation source was modified during this review. Production services, databases, MCP registration, Tailscale configuration, and firewall state were not changed.

## Finding

### R2-F1 — HIGH — rollback reports success without proving Viewer availability

- **File:line:** `scripts/task-journal-rollout.ps1:270-283`; `docker-compose.yml:53-67`; `scripts/test-task-journal-rollout.ps1:152-182`; `docs/task-journal-rollout-operations.md:95-105`
- **Evidence:** After Compose recreates both services, the helper verifies the Viewer container's image ID but never checks its state, health, or HTTP response. Only the API container enters `Wait-HealthyContainer`, and only API `/readyz` and `/versionz` are requested. The Viewer service has no Compose healthcheck, and the regression test asserts API health only.
- **Focused repro:** A command-level mock represented the Viewer as `exited` whenever `.State` was queried, while returning the expected immutable Viewer image ID. The helper returned `RESULT=SUCCESS`; the captured calls ended with `inspect viewer-container --format {{.Image}}` followed only by API health, `/readyz`, and `/versionz`. No Viewer state query occurred, so the simulated `exited` condition was never observed.
- **Impact:** A wrong-entrypoint, immediately crashing, or non-serving Viewer image with the expected revision label can be accepted as a successful rollback. This contradicts the runbook's fail-closed recovery claim and can leave the signed-in human confirmation surface unavailable even though the helper reports recovery.
- **Required correction:** Add a Viewer liveness/readiness boundary after exact-image verification and before success. At minimum, require the Viewer container to remain running and verify its loopback HTTP endpoint; preferably add a Compose healthcheck and wait for it. Add a regression case in which the Viewer is exited or its HTTP endpoint fails and assert a non-zero result. The output must not report rollback success until both API and Viewer checks pass.

## F1-F4 disposition

| Original finding | Result | Independent evidence |
| --- | --- | --- |
| F1 — pre-existing restore DB deletion | **Fixed** | Ownership is recorded only after successful `createdb`; cleanup is conditional. The collision regression proves no `dropdb` call, and the matching/mismatch cases prove invocation-owned cleanup. |
| F2 — schema-compatible, fail-closed rollback | **Partially fixed / material gap** | Schema 27, immutable API schema label, matching API/Viewer revision, exact image IDs, API health, `/readyz`, and `/versionz` are enforced. R2-F1 shows Viewer failure is not detected. |
| F3 — dump and restored dataset binding | **Fixed** | Format-2 metadata binds dump SHA-256, all public table counts, Workspace ID/revision, #183/#184 internal/public identity and status, and sorted Event/command/approval IDs. Restore checks the hash before `createdb` and automatically compares every pinned field. |
| F4 — complete lifecycle canary | **Fixed** | The runbook explicitly sequences `task.create`, `run.start`, `task.report_implemented`, then the signed-in human-only `task.confirm` boundary. It requires HTTP/Viewer/MCP provenance agreement, byte-equivalent retry, changed-payload conflict, stale revision, stale/mismatched grant zero-write proof, and a fresh Viewer preview before confirmation. |

## Regression and metadata review

- Adding explicit `baley-api:latest` and `baley-viewer:latest` Compose image names makes immutable-ID retagging compatible with `docker compose up --no-build`; `docker compose config --quiet` passed.
- The API Dockerfile now carries schema version 27 and exact revision labels; the Viewer already carries the revision label. The rollback helper checks the API label and requires the Viewer label to match the API revision before changing tags.
- Restore identifiers, Workspace UUIDs, image IDs, and loopback API origins are constrained before use. The dump hash is checked before any restore database creation.
- The existing large JavaScript chunk warning remains and is unrelated to this correction.

## Verification executed

- `& .\scripts\test-task-journal-rollout.ps1`: **PASS**, 7/7 safety cases.
- `go test ./internal/application ./internal/transport/httpapi ./cmd/baley-mcp -count=1`: **PASS**.
- `go vet ./...`: **PASS**.
- `npm test -- --run`: **PASS**, 17 files / 108 tests.
- `npm run build`: **PASS**, with the pre-existing large-chunk warning.
- `docker compose config --quiet`: **PASS**.
- `git diff --check 9ac96f4..HEAD`: **PASS**.
- Focused Viewer-exited rollback mock: **FAIL as expected for the review** — the helper incorrectly returned success, establishing R2-F1.

Windows PowerShell was used for the safety suite because `pwsh` is not installed in this environment; the script itself passed when invoked in the available PowerShell host.

## Commit and push state

After `git fetch origin jazzcake/task-journal-rollout`, local `HEAD` and `origin/jazzcake/task-journal-rollout` both resolved to `ae74152e358e01fdac426ee453467d66e2750358`. The tree was clean before this review document was created. Per the task contract, this CHANGES_REQUIRED review document is intentionally left uncommitted and unpushed; no implementation files were changed.

## Remaining acceptance

Correct R2-F1, rerun the seven safety cases plus a Viewer-unavailable rollback case, and obtain a fresh independent review. Production backup/restore, migration, deployment smoke, full lifecycle canary, and signed-in human confirmation remain operator-owned execution.
