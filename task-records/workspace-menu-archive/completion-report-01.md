---
baley_record: 1
record_id: "a8f11e76-e3c2-4b45-88d6-75ee0d7ec3a1"
task_id: 155
task_key: "workspace-menu-archive"
record_type: completion-report
run_id: "28d3b630-a1c6-458b-8fe7-d0a0a7eaf8c7"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 completion report

Implemented Owner-only Workspace rename, archive, and restore. The Workspace
switcher now gives each Owner Workspace an overflow menu that is independent
of Workspace selection and offers Rename / Archive; archived Owner Workspaces
are shown separately and offer Restore.

Archive is a non-destructive persisted state. It preserves Task, Gate, Run,
record, repository, and audit history while atomically revoking active Agent
tokens, local MCP gateway registrations, pending MCP connections, and sessions
for active human members. Normal Workspace routes, account lists, Agent token
validation, and MCP gateway enrollment/resume accept active Workspaces only.
The explicit restore endpoint is the narrow Owner-only archived-state exception;
restoring requires fresh login and local gateway enrollment.

Verification: focused frontend API/interaction tests (25 passing), full Go
suite (`go test ./...`, passing), production frontend build, Docker Compose
rebuild, API health, and schema version 22 all passed. Independent review and
review response are recorded separately.
