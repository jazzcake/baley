---
baley_record: 1
record_id: "d48d91f0-7295-49d3-9592-5fc2eaa1a538"
task_id: 155
task_key: "workspace-menu-archive"
record_type: detailed-plan
run_id: "ae27a96d-0a83-4c61-b315-12a1f709b0a5"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 Workspace menu rename and archive plan

## Outcome

Replace destructive Workspace deletion with an Owner-only archive lifecycle.
The Workspace switcher keeps its normal selection action, while an Owner-only
overflow command on each Workspace row opens a small contextual action menu
for rename, archive, or (for an archived Workspace) restore.

## Lifecycle and access boundary

1. Add `archived` as a persisted Workspace state. No task, Gate, Run, Record,
   repository, or audit row is deleted.
2. Add Owner-authorized HTTP endpoints for rename, archive, and restore. Names
   use the existing creation constraints (trimmed, non-empty, maximum 120
   Unicode code points). Each successful lifecycle operation increments the
   Workspace revision and creates an append-only security event.
3. Archive atomically revokes Agent tokens and local MCP gateway registrations
   for the Workspace and revokes sessions for its active human members. A
   restored Workspace therefore requires a fresh human login and fresh local
   gateway enrollment before MCP use resumes.
4. All ordinary workspace routes, command authorization, snapshots, MCP
   gateway enrollment/resume, and default account workspace lists treat an
   archived Workspace as inaccessible. The sole exception is the explicit
   restore route, which rechecks the Owner membership while allowing the
   archived state. `GET /v1/workspaces?includeArchived=true` returns archived
   memberships only to their Owners so the restore command remains
   discoverable after re-login.

## Viewer interaction

1. Render normal active Workspaces and a clearly separated archived group.
   Active rows retain radio-menu semantics and selection behavior.
2. An Owner row receives an overflow button. Its click stops propagation and
   opens a nested contextual menu; it never selects/navigates the Workspace.
3. Rename uses an inline popover with validation/error feedback. Archive uses
   a deliberate confirmation popover, then returns the user to a remaining
   active Workspace or the Workspace chooser. Restore refreshes membership
   state and opens the restored Workspace.
4. Development-only Viewer traces record overflow click, selected action,
   calculated target workspace, API result, refreshed membership state, and
   DOM menu state. Keyboard Escape/outside click return focus predictably.

## Verification

- Repository and HTTP tests cover authorization, name validation, archive
  access denial, Owner-only archived discovery, session/token/gateway revoke,
  restore, revision, and audit preservation.
- Frontend API and interaction tests cover row selection isolation, Owner-only
  actions, rename, archive confirmation/fallback navigation, restore, focus,
  and errors.
- Run focused and full Go/frontend suites, production builds, Docker/deployed
  smoke checks, independent review, review response, Task Record registration,
  committed evidence, and push before reporting implementation.
