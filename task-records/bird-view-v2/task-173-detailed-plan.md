---
baley_record: 1
record_id: "ba6fc108-98de-4a65-9103-f2a485785058"
task_id: 173
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-05T07:20:00Z"
created_by: "codex"
registration_state: pending
---

# Task #173 detailed implementation plan

## Outcome

Correct Bird View V2 focus so the embedded Workspace retains usable Task interaction without inventing Task XY persistence. Human operators with `workspace:operate` can edit title, description, and current summary through `task.update`, and move a non-terminal Task to an eligible Phase through `task.move`.

## Work

1. Instrument selection, preview, execution, failure, controller, and DOM boundaries with development-only structured traces.
2. Replace the blanket read-only badge and LLM-only hint with the precise contract: Task content and Phase are editable, canvas layout is automatic.
3. Add Inspector forms that use fresh server preview then exact execution, preserve revision/idempotency, reject stale targets, respect membership and terminal-state guards, and recover by discarding failed previews while the graph refreshes from the server.
4. Keep React Flow Task nodes auto-positioned; do not add Task position columns or reinterpret visual dragging as `task.move`.
5. Fix the empty-external-reference focus grid row so the lower graph always occupies the flexible row and remains pointer hit-testable.
6. Add focused tests for update, Phase move, authorization/terminal guards, failure recovery, and absence of XY data; browser-verify Task #110 through Tailnet HTTPS.

## Non-goals

No new Task coordinates, drag-to-Phase gesture, cross-Workspace layout, human confirmation, deployment, or operational database changes.
