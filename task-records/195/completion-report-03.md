---
baley_record: 1
record_id: "1f6c9a42-7d35-4b80-a164-2e8f5c3d9071"
task_id: 195
record_type: completion-report
run_id: "2b196ab6-8ec9-46a3-b77d-1269f7d8cf15"
created_at: "2026-09-18T16:10:00+09:00"
created_by: "codex"
supersedes: "c9d3b74e-aea7-4293-83f2-703d2d0b83cb"
status: implemented
---

# Task #195 conversational Task discard rollout

Task #195 now treats ordinary `task.confirm` and `task.discard` as the same
linked-account conversational decision boundary. A user can explicitly confirm
or discard a Task in chat without a Viewer approval grant. In an established
exact-Task context, direct discard wording such as `삭제합시다` is accepted;
questions, negations, conditions, vague agreement, cross-action evidence and
wrong targets still fail closed.

The original defect was a split contract. The application evaluator,
PostgreSQL execution boundary, MCP compact bridge and typed tool schema
special-cased only `task.confirm`; `task.discard` therefore fell through to
the browser-grant path. The fix unifies both Task decisions while retaining a
fresh preview, current Workspace revision, canonical command hash, exact action
and target, unique decision UUID, linked human capability revalidation,
single-use evidence and separate human/Agent provenance.

Migration 30 extends the persisted conversational evidence action constraint to
`task.discard`. Unit and MCP tests cover direct and compound Korean/English
discard statements, contextual `삭제합시다`, cross-action rejection and the
absence of a browser grant. Disposable PostgreSQL tests verify migration
up/down behavior, the real `in_progress -> discarded` transition, evidence
persistence and linked-account provenance.

Verification completed:

- `go test ./... -skip '^TestCommandRemoteGitVerifier' -count=1`: PASS
- `go vet ./...`: PASS
- full integration suite against an isolated loopback PostgreSQL 17 container:
  PASS
- implementation commit `269007e2e72edcbb3f7531d2c6d0251c574f504e`
  atomically pushed to `main`, the operating branch and the Task branch
- deployed API reports `task-195-discard`, schema 30 and healthy readiness
- deployed MCP binary:
  `C:\dev-bin\baley\releases\269007e2e72e\baley-mcp.exe`
  with SHA-256
  `0707C632C00F344638C4E20095AAEE869998D17007D8487B2845FB3335A589F8`
- MCP diagnostics report the updated full schema size 50492
- personal plugin `baley@personal` reinstalled as
  `0.1.0+codex.20260918070752`; installed Skill text contains both Task
  decision actions and the no-Viewer-grant rule

The attempted live DayTripper compound execution was stopped before mutation by
the host action-safety layer because the current user turn did not freshly name
the individual Task decisions. No workaround was attempted. DayTripper Tasks
#61 through #65 remain unchanged. This does not indicate a Baley API rejection:
the deployed path passed its isolated PostgreSQL integration test, and a new
Codex thread can load the refreshed plugin and execute a fresh explicit
Task-specific command without Viewer UI.
