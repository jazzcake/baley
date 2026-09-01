---
baley_record: 1
record_id: "9da803d6-a600-4287-98ca-ad5b194ea058"
task_id: 160
task_key: "task-description-skill-policy"
record_type: independent-agent-review
run_id: "e9633727-6f41-4497-b5b2-dc41452fb0ab"
created_at: "2026-09-02T00:20:00+09:00"
created_by: "independent-review-agent"
registration_state: pending
supersedes: null
---

# Baley Task description Skill policy independent review

## Final result

- Blocker: 0
- P1: 0
- P2: 0

Approved after two review-response passes.

## Findings and disposition

1. Mandatory policy originally conflicted with Task creation examples that omitted `currentSummary` and the four-section description. Both Skill and command-reference examples now include them.
2. Existing-Task refresh was ambiguous. The Skill now distinguishes content-changing refresh requests from read-only inspection and sends title, description, and summary together.
3. Backlog promotion could leave an unnormalized Task after interruption or stale revision. The Skill now requires re-read, re-preview, idempotent retry, and forbids downstream work until normalization succeeds.
4. The first command-reference correction represented `description` as an object even though MCP requires a string. Both create and update patterns now use explicit multiline strings.

## Distribution reviewed

- canonical repository Skill, personal plugin source, and installed cache hashes match
- Skill validation and plugin validation pass
- installed and enabled plugin version: `baley@personal 0.1.0+codex.20260901151925`

## Final verdict

Approved with no remaining Blocker/P1/P2.
