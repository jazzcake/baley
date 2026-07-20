---
baley_record: 1
record_id: "608a1256-f4c1-440d-b5a8-d65fedc11bd9"
task_id: 111
task_key: "api-runtime-contract"
record_type: review-response
run_id: "50d06c62-f932-4b45-97a7-0a97fa703564"
created_at: "2026-07-21T00:30:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# API Runtime Contract Review Response

## Outcome

The independent review's two interim Low findings were resolved. Final findings are High 0, Medium 0, Low 0.

## Resolutions

1. Stale bootstrap statements that still described Records as pending or required a schema reload were updated to the actual Task #111, registered Record, and current-source 18080 state.
2. Temporary MCP invocation and runtime launcher helpers were removed from the working tree. The running loopback API uses the separately built `C:\tmp\baley-runtime\baley-server.exe`; no secret or runtime binary is part of the repository diff.

## Residual operational risk

The elevated legacy listener on 8080 cannot be terminated from this process. New MCP processes select the current-source 18080 runtime through the user-level `BALEY_SERVER_URL`. Clients that explicitly use 8080 or do not inherit the user environment may still reach the legacy contract. The roadmap keeps cleanup in the owning launch context as a follow-up; the reviewer classified this as non-blocking for Task #111.
