---
baley_record: 1
record_id: "ce2d3f4a-9936-4ef1-a5de-eef017549e51"
task_id: 147
task_key: "task-phase-move"
record_type: completion-report
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #147 completion report

## Delivered

Implemented the routine Operator command `task.move` through the shared command path. A move keeps the existing Task identity and all task-owned evidence; the persistence write changes `phase_id` only and records a `task.moved` Event with the Task/public IDs and source/target Phase IDs.

The command is present in the literal command contract, domain mutation registry/planner, Command Service, PostgreSQL transaction, CLI command parsing, event audit/visibility maps, and MCP preview/execute tools.

## Safety checks

The command rejects a missing or completed target Phase, a no-op move, terminal Task, active Run, incompatible parent/child Phase relation, Gate condition/entry binding break, and standard stale revision/idempotency conflicts. It leaves dependencies intact and returns post-move `phase_order_inversion` diagnostics when applicable.

## Verification

`go test ./...` from `server/` passed, including application, domain, CLI, MCP, persistence, HTTP API, and integration packages.

## Operational follow-up

The running Baley server/MCP deployment must be rebuilt and restarted with this commit before live Task #126 and #127 can be moved using `task.move`. No direct database update, clone, discard, or human confirmation was performed.

## Residual risk

The command is covered by the full server suite, but the live production move remains a deployment-time operation and should be executed through the new preview/execute command after the server is updated.