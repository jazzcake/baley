---
baley_record: 1
record_id: "ae58d32f-d091-4ce1-9b44-eec1fc982c15"
task_id: 184
record_type: completion-report
run_id: "97491670-5d41-40b5-bbda-50224f78d151"
created_at: "2026-09-10T17:57:25+09:00"
created_by: "codex-worker-term_6035acbf"
supersedes: "e6cb529d-c74e-4163-905d-461dcb1d1a1a"
status: immutable_correction
---

# Task #184 rollout evidence locator correction

This immutable completion Record corrects only the repository locator of the
prior rollout evidence. The superseded Baley row pointed to
`task-records/184/rollout-report-01.md`, but the attached blob existed only at
`docs/task-records/184/rollout-report-01.md`.

The succeeded rollout Run above remains the source of the operational result:
schema 27 rollout, deterministic #183 journal recovery, backup/restore checks,
pinned API/Viewer/MCP artifacts, local and tailnet smoke checks, and canary
safety verification completed without firewall or Tailscale changes.

