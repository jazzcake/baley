---
baley_record: 1
record_id: "dd4bc336-7ba8-464b-ba8b-37c4c35e4375"
task_id: 151
task_key: "workspace-mcp-context"
record_type: handoff
run_id: "0ca498c7-cfc1-4954-b77e-b1922f6b46d4"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
supersedes: null
---

# #151 Codex implementation handoff prompt

Implement and close out Baley Task #151, **대규모 Workspace MCP 컨텍스트
경량화·성능 보호**, in this worktree.

Read `AGENTS.md`, `.agents/skills/baley-manage-work/SKILL.md`, and
`task-records/workspace-mcp-context/detailed-plan-02.md` first. #151 is already
in progress. Inspect the current compact context and phase-task expansion
implementation before editing; it has already demonstrated the desired basic
flow in the live Pilot. Finish the remaining contract, security, benchmark,
compatibility, and operational evidence work rather than replacing working
behavior.

Default Codex discovery must return only non-completed active Phase summaries;
Task lists/details and completed Phase graph data load only through explicit,
bounded expansion or explicit full-graph opt-in. Preserve authorization,
audit, revision/stale behavior, Viewer compatibility, and every human-only
Task/Gate boundary. Add measured 100/1,000/10,000 Task performance evidence,
run focused plus full Go/frontend/build validation, deploy, and smoke test the
compact-then-expand route. Create completion/review Task Records, obtain an
independent-agent review and response, commit and push. Report the commit,
measurements, test/deploy results, record paths, residual risks, and whether
#151 can truthfully be reported implemented (never confirmed).

