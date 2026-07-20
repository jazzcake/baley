---
baley_record: 1
record_id: "dc035a7a-025e-40c0-9567-63bf0645fca3"
task_id: 111
task_key: "mcp-task-create-bootstrap"
record_type: handoff
run_id: "e589d695-ac21-40d5-9f9a-e9183977e872"
created_at: "2026-07-20T23:50:23+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# MCP Task Create Bootstrap Handoff

## Completed

`baley_task_create_preview` and `baley_task_create_execute` are implemented with schema and forwarding unit tests, real PostgreSQL integration coverage, and an MCP stdio end-to-end test. The independent review is complete after addressing both Low findings; final findings are High 0, Medium 0, Low 0. No live Baley mutation was performed.

Verification passed:

- `go test -count=1 ./...`
- `go vet ./...`
- real PostgreSQL `go test -count=1 ./integration`
- real API plus MCP stdio end-to-end test
- frontend tests: 13/13
- frontend production build
- Baley skill validator

The race detector remains unavailable because the Windows Go toolchain has `CGO_ENABLED=0`.

## Resolved resume boundary

The root manifest remained stale, but the repository's official Baley MCP stdio server exposed both tools. Its typed preview and execute created Task #111 without HTTP, SQL, fixture, or database bypass. The earlier sub-agent hangs were isolated to accumulated host child connections.

New MCP processes use the persisted user-level `BALEY_SERVER_URL=http://127.0.0.1:18080`, which points to the current-source runtime.

## First Phase 3 Task proposal

- Lane: `server`
- Phase: active `validate` Phase
- Title: `기본 Baley API runtime 계약 정렬`
- Description: restart the default API from the current source using the existing stable lease-token secret, verify that source and runtime expose the same warning-capable MCP/HTTP contract, and record independent-review evidence.
- Predecessor: Task #110 only if preview confirms the intended graph and warnings are acknowledged through the typed execute contract.

Use `baley_task_create_preview` first. Report the structural preview, then execute with the exact `commandHash` and any required warning acknowledgements. Start a Run only after the Task exists. Keep user confirmation as the final pending operation; do not perform it.

## Live baseline

- Workspace: `00000000-0000-4000-8000-000000000001`
- Workspace revision at Task creation: 12
- Active Phase: Validate
- Tasks #101, #104, #106, and #110: confirmed
- Task #111: created in Server/Validate with #110 as predecessor
- Outstanding human decisions: none
- Repository and bootstrap Record indexes: registered

The legacy API on port 8080 is an older elevated process (`baley-server.exe`, PID 17232, started 2026-07-19 11:55:50 KST; parent `go.exe`, PID 37516). Windows denied termination after exact PID, listener, executable, and parent verification. It remains as a fallback. The current-source runtime listens on 18080 with a stable user-level lease secret and is selected for new MCP processes.

## Records

- `detailed-plan-01.md`
- `independent-agent-review-01.md`
- `review-response-01.md`
- `completion-report-01.md`
- this handoff

All listed records are registered to Task #111 and its implementation Run.
