---
baley_record: 1
record_id: "e127c5d7-45a2-4a31-bfea-582bbd7805a5"
task_id: 155
task_key: "workspace-menu-archive"
record_type: handoff
run_id: "fb076f45-38dc-4b2e-bb42-5b4d0fb59cfb"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 independent review handoff

Review the pending Workspace rename/archive implementation for security and
correctness. Focus on archive access denial, restore-only exception handling,
session and MCP credential revocation, Owner authorization, migration safety,
and whether the Workspace switcher overflow interaction can accidentally select
or expose actions on a non-Owner Workspace.

Changed areas: `server/migrations/00022_workspace_archive.sql`, account access
and MCP gateway persistence, HTTP routing/auth middleware, Workspace switcher,
API client, contracts, tests, and this Task Record directory. Do not modify
files; return findings with severity and exact locations.
