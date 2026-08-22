---
baley_record: 1
record_id: "9b8c4663-76c6-4d26-ac60-74df56edddc5"
task_id: 139
task_key: "task-summary-input-display"
record_type: completion-report
run_id: "0d078aae-da9a-4f48-8305-b6f362c50969"
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #139 completion report

## Delivered

Tasks now accept an optional `currentSummary` on create and update. It is stored separately from the detailed description, appears immediately beneath the status in the Viewer Inspector, and preserves the detailed description below it. Tasks without a summary keep the prior description-only view.

The Baley work skill now instructs the LLM to write one or two plain-language sentences focused on user value and expected result. Task #139 was updated as a live verification example.

## Verification

- `go test ./internal/domain ./internal/application ./cmd/baley-mcp` passed.
- `npm run typecheck` passed.
- `npm test` passed: 70 tests.
- `npm run build` passed.
- Live API verification returned the saved `currentSummary` for Task #139 after deployment.

## Residual risk

Summary clarity is a writing-quality rule for the LLM, not a semantic classifier. Existing Tasks remain unchanged until a later update supplies a summary.
