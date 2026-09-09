---
baley_record: 1
record_id: "1970e53d-c35d-4fda-82c3-59970b1361ce"
task_id: 184
record_type: review-response
run_id: "pending-registration"
created_at: "2026-09-10T03:12:30+09:00"
created_by: "codex-worker-term_e4c314e8"
responds_to: "independent-review-02.md"
status: implemented
---

# Task #184 review response 02

## Outcome

The sole HIGH finding R2-F1 in `independent-review-02.md` is corrected without editing or replacing either independent review. Rollback now remains fail-closed until the exact API and Viewer containers are healthy, the Viewer root responds successfully over its loopback-bound origin, the Viewer's same-origin `/api/readyz` proxy reports schema 27, and the direct API readiness/version checks pass.

No operating branch, production database or service, MCP registration/session, Tailscale setting, firewall rule, or user `debug.log` was changed. The Docker smoke used uniquely named disposable images, containers, and a network, then removed them.

## Finding response

### R2-F1 — Viewer availability was not proven

`docker-compose.yml` now gives the Viewer an internal root healthcheck. The rollback helper validates a new loopback-only `ViewerBaseUrl`, verifies the exact Viewer image ID, waits on the Viewer health state as well as the API health state, requests the Viewer root, and requests `/api/readyz` through that same Viewer origin. It returns nonzero if the exact Viewer container is exited, dead, unhealthy, cannot be inspected, never becomes healthy, does not serve the loopback root with HTTP 200, or cannot proxy a valid `ready` response with `schemaVersion: 27`.

The success object is emitted only after all Viewer and API boundaries pass and now reports separate API/Viewer health plus Viewer root and proxy evidence. The operations runbook describes these required checks and explicitly states that failed Viewer conditions are not recovered rollback outcomes.

## Direct regression evidence

The PowerShell safety suite preserves the original seven cases and adds three direct Viewer cases:

1. An exact-image Viewer whose inspected state is `exited` returns nonzero before HTTP checks.
2. A healthy exact-image Viewer whose loopback root is unreachable returns nonzero.
3. A healthy Viewer whose same-origin `/api/readyz` proxy reports schema 26 returns nonzero before direct API checks.

The positive rollback case additionally asserts that the exact Viewer container health boundary was inspected and that Viewer root/proxy checks passed.

## Verification

- `scripts/test-task-journal-rollout.ps1`: passed, 10/10 safety cases (the original 7 plus 3 Viewer regressions).
- `go test ./internal/application ./internal/transport/httpapi ./cmd/baley-mcp -count=1`: passed.
- `go vet ./...`: passed.
- `npm test -- --run`: passed, 17 files / 108 tests.
- `npm run build`: passed; the pre-existing large-chunk warning remains.
- `docker compose config --quiet`: passed.
- Isolated Docker smoke: disposable PostgreSQL 17, schema-27 API, and Viewer containers passed Viewer health, loopback root HTTP 200, and same-origin `/api/readyz` schema 27; all disposable containers, network, and tagged images were removed.
- Final diff/whitespace, commit, and push evidence are supplied in the worker completion handoff.

## Residual boundary

This correction closes the implementation gap identified by R2-F1. Production rollout, the full lifecycle canary, and signed-in human confirmation remain operator-owned execution; no production state was exercised here.
