---
baley_record: 1
record_id: "f9ba8788-ab61-4aba-994f-0630ca270bf7"
task_id: 129
task_key: "gate-entry-unlocks"
record_type: review-response
run_id: "f5130055-625b-4973-873b-baddf93b0a2f"
created_at: "2026-07-27T00:03:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #129 review response

The initial review found an upgrade blocker: migration 00013 tightened the entry source constraint without handling v12-legal persisted `automatic` rows. The response now deletes those legacy projection rows before adding the explicit-only constraint and fixes the behavior with an isolated migration integration test.

The response also:

- documents entry attach/detach in the Operator command reference;
- tests both CLI commands and typed MCP forwarding without approval fields;
- tests HTTP-to-Viewer `required` and `unlocks` mapping;
- requires the `entryTasks` snapshot key in Gate pass Event evidence;
- preserves an empty resolved entry set for target Phases with no Tasks.

Full Go, vet, isolated PostgreSQL, frontend, build, and diff validation pass.
