---
baley_record: 1
record_id: "370fe5a6-f2a0-4e14-94b3-8d0925e50b91"
task_id: 151
task_key: "workspace-mcp-context"
record_type: completion-report
run_id: "82f24ddf-f911-4a7e-b727-a0fcd607fa57"
created_at: "2026-08-28T12:00:00Z"
created_by: "codex"
supersedes: null
---

# #151 completion report

## Delivered

- Defined the compact response as a revisioned snapshot summary, with no implied
  graph-delta history; documented the compact-first, explicit-Phase-expand,
  explicit-full-graph compatibility contract.
- Enforced the MCP client 100-Task upper page bound and covered its default,
  bounded request path and invalid inputs.
- Added deterministic summary, Task-detail exclusion, cursor-scope, page-bound,
  unauthenticated, and cross-Workspace authorization regression coverage.
- Obtained an independent review with no release blockers and incorporated its
  two requested test refinements.

## Verification

`go test ./...`, `go vet ./...`, focused application/HTTP/MCP tests, `npm test`
(87 tests), `npm run typecheck`, and `npm run build` passed. Live Pilot smoke
reads returned a 794-revision compact active-Phase summary, then returned one
Task only after explicit `baley_phase_tasks` expansion.

## Scale evidence

| Tasks | compact bytes | full graph bytes | compact p95 | compact allocs |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 343 | 28,288 | 10.524 us | 24/op |
| 1,000 | 346 | 282,089 | 30.180 us | 24/op |
| 10,000 | 349 | 2,838,090 | 201.311 us | 24/op |

The compact payload stays below 350 bytes in this two-Lane benchmark while the
full graph grows linearly; the 10,000-Task compact payload is about 8,132 times
smaller. Benchmark timings are local Windows/AMD Ryzen 5 5600X measurements,
not a production latency guarantee.

## Git and release

Implementation and review records are committed and pushed as
`60ae82e59266e7c67e5acfb24f3b14dff686e118` on
`origin/jazzcake/baley-task-151-context`. A clean Linux/amd64 hosted-Pilot
release was built at `.tmp/hosted-pilot-releases/baley-60ae82e-linux-amd64` with
its `SHA256SUMS` manifest.

## Remaining blockers and truthful status

Production activation did not occur: the configured Lucy host is reachable with
non-interactive sudo, but has no `/srv/baley`, release directory, or Baley
systemd units. Its one-time host provisioning requires the Owner-controlled
database, migration, and lease-secret files before the documented activation
script can safely run. The review-response Record has since been registered;
this completion record and all record commit/blob attachments are being closed
out under the current completion-reporting Run. #151 is therefore **not yet
reportable as implemented or confirmed** until production activation is verified.
