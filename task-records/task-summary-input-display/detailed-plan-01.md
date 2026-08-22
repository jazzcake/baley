---
baley_record: 1
record_id: "4def2722-bf9d-4547-b9d1-1cf4771d8190"
task_id: 139
task_key: "task-summary-input-display"
record_type: detailed-plan
run_id: "0d078aae-da9a-4f48-8305-b6f362c50969"
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #139 detailed plan

1. Add optional `currentSummary` to Task create and update command inputs.
2. Preserve it in the domain update, event before/after payload, projection, and existing persistence column.
3. Expose it through typed MCP inputs and teach the Baley work skill to write a short plain-language explanation.
4. Render it immediately below the status in the Viewer Inspector; leave older Tasks without a summary unchanged.
5. Verify server, MCP, Viewer build/tests, then update Task #139 as a live example.
