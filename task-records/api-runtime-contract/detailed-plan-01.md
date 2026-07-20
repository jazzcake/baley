---
baley_record: 1
record_id: "c1c310db-fb55-4480-907d-f588326f0c73"
task_id: 111
task_key: "api-runtime-contract"
record_type: detailed-plan
run_id: "0a739e25-b3ea-482c-ba33-c76a1913d9f4"
created_at: "2026-07-21T00:16:53+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# API Runtime Contract Alignment Plan

## Objective

Operate Baley through a current-source API with a stable lease-token secret, and verify that the warning-capable MCP preview/execute contract works against the live Workspace.

## Plan

1. Build the current `baley-server` source and validate it against the live PostgreSQL database on an isolated loopback port.
2. Preserve the existing elevated 8080 listener when Windows denies termination, and establish `http://127.0.0.1:18080` as the user-level `BALEY_SERVER_URL` for subsequent MCP processes.
3. Persist a generated user-level `BALEY_LEASE_TOKEN_SECRET` only when no existing user-level value is present.
4. Prove the current-source contract with a write-free `task.create` preview and typed execute, then run the Task lifecycle through that runtime.
5. Register the repository and Task Records, run full verification, obtain an independent Agent review, and report Task #111 implemented. Leave human confirmation pending.

## Safety and rollback

- Do not terminate the elevated legacy listener after access is denied.
- Use the same live database and loopback-only binding.
- Never print or commit the lease secret.
- Keep the legacy listener as a fallback; the user-level MCP URL selects the current-source runtime for new processes.

## Acceptance criteria

- Current-source API serves the live Workspace on loopback.
- `task.create` preview is write-free and execute accepts the warning-capable envelope.
- Task #111 has successful planning, implementation, review, and reporting evidence.
- Independent review has no unresolved blocking finding.
- Task #111 ends in `implemented`, with only human `task.confirm` pending.
