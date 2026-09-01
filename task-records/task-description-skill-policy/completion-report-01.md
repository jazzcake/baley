---
baley_record: 1
record_id: "6f2c1fa4-bbc3-4324-bbab-c2915b938f08"
task_id: 160
task_key: "task-description-skill-policy"
record_type: completion-report
run_id: "30cf862c-dc32-474e-bf76-d50c7a841ec4"
created_at: "2026-09-02T00:29:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Baley Task description Skill policy completion report

## Outcome

Task #160 is implemented. The canonical Baley management Skill now requires a plain-language `currentSummary` and a four-section description whenever Task content is created or changed, and applies the same normalization to an explicit refresh of an existing non-terminal Task.

## Delivered behavior

- `task.create` and content-changing `task.update` always carry `currentSummary` plus description sections for easy explanation, need, completed outcome, and scope/exclusions.
- Read-only Task inspection never mutates content.
- `backlog.promote` is not complete until the promoted Task is re-read and normalized; stale revision and interruption require idempotent re-preview/retry, and downstream work is forbidden until it succeeds.
- Command examples use the actual MCP wire type: `description` is a multiline string, not an object.
- The repository source was deployed through the official plugin installer rather than editing an installed cache.

## Distribution evidence

- installed plugin: `baley@personal 0.1.0+codex.20260901151925`
- canonical, personal source, and installed cache hashes match for `SKILL.md` and `references/commands.md`
- Skill quick validation: PASS
- plugin manifest validation: PASS

## Review and verification

- independent review: approved after all P1/P2 findings were resolved
- full Go suite against migrated disposable PostgreSQL schema 23: PASS
- frontend: 16 files / 94 tests PASS
- production frontend build: PASS
- working tree diff check: PASS

## Approval boundary

This reports implementation only. Human confirmation remains a separate user-only action.
