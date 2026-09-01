---
baley_record: 1
record_id: "bcea22cb-8721-4f7b-a252-148323ffa0e0"
task_id: 160
task_key: "task-description-skill-policy"
record_type: review-response
run_id: "12fc4c9a-da0c-4e4b-a3c2-3ba789c26b81"
created_at: "2026-09-02T00:20:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Baley Task description Skill policy review response

## Response

All independent-review findings were accepted and resolved.

## Changes made

- Updated Task create and update examples to always include `currentSummary` and a four-section multiline description.
- Defined content refresh versus read-only behavior for existing Tasks.
- Added robust Backlog promotion normalization with re-read, re-preview, idempotent retry, and a downstream-work stop condition.
- Corrected command-reference `description` examples to use the MCP contract's string type.
- Reinstalled the canonical source through the official cachebuster workflow.

## Re-verification

- Skill quick validation: PASS
- plugin manifest validation: PASS
- canonical/personal/cache hashes: identical
- independent final review: approved, Blocker/P1/P2 all zero
