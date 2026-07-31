---
baley_record: 1
record_id: "810d2902-67b2-4758-bd3b-40f6c57f4b39"
task_id: 129
task_key: "gate-entry-unlocks"
record_type: independent-agent-review
run_id: "a688bff8-3215-4ef5-b812-6511410d2fff"
created_at: "2026-07-27T00:06:00+09:00"
created_by: "codex-review-agent"
registration_state: registered
supersedes: null
---

# Task #129 independent review

## Verdict

PASS. No blocking or nonblocking findings remain.

## Reviewed boundaries

- `gate_tasks` conditions remain from-Phase-only; entry bindings remain to-Phase-only.
- Persisted entry rows are explicit-only. Automatic entries are deterministic same-Phase DAG-root projections ordered by Task public ID.
- Explicit attach/detach cannot mutate a passed Gate, increments Gate criteria revision, and does not change readiness, dependencies, or Task state.
- Gate pass decision hashes and `gate.passed` Events bind the resolved entry Task IDs and `explicit|automatic` sources. The entry snapshot key is mandatory while an empty set remains valid when the target Phase has no Tasks.
- CLI, typed MCP, HTTP graph projection, Viewer `Gate → Task` unlock direction, command contracts, and Operator references agree.
- Migration 00013 removes v12-legal legacy automatic rows before enforcing explicit-only persistence.

## Verification

- `go test ./...`
- `go vet ./...`
- isolated PostgreSQL integration suite, including downgrade/legacy insert/upgrade cleanup and zero-target-Phase Gate pass
- frontend: 10 files, 39 tests
- production TypeScript/Vite build
- `git diff --check`
