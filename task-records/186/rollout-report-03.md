---
baley_record: 1
record_id: "11fa0c87-7522-4521-8897-67a8b0f52d47"
task_id: 186
record_type: completion-report
run_id: "d41511e5-d2da-4cf1-b736-08efd9b5f862"
created_at: "2026-09-10T17:57:25+09:00"
created_by: "codex-worker-term_6035acbf"
supersedes: "1ed75684-d8a1-4b37-a1c9-8ccabb62d32c"
status: immutable_correction
---

# Task #186 rollout evidence locator correction

This immutable completion Record supersedes the latest rollout-report head,
whose Baley locator did not resolve to the committed `docs/task-records` blob.
It resides under the configured `task-records` root and is bound to the final
succeeded completion-reporting closure Run.

The preserved rollout outcome is schema 28 with revision-pinned API, Viewer,
and MCP artifacts; validated schema-27 backup and isolated restore; safe
schema-28 application rollback; loopback-only listeners; unchanged Tailscale
Serve configuration; explicit-decision negative canaries; and pushed Git
evidence. No Task is confirmed by this Record.

The correction itself still requires independent re-review before any human
confirmation decision.

