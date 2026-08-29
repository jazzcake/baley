---
baley_record: 1
record_id: "eeaf25ea-d5c7-4ea6-952a-a7ed01d4eccd"
task_id: 155
task_key: "workspace-menu-archive"
record_type: completion-report
run_id: "ffae40f4-ac46-4736-b256-ee6f2f1b6b36"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 card menu placement correction completion report

The Owner Workspace lifecycle commands are now located on the intended
Workspace chooser cards: the `…` button opens Rename and Archive for active
Workspaces, or Restore for archived Workspaces. The card body remains a pure
Workspace selection control; the title switcher is selection-only.

The correction fixes the archive-revocation transition and separates menu,
form, and confirmation-dialog semantics. It was verified in a signed-in
deployed browser session, by independent review, 27 focused frontend tests,
and a production frontend build. The final commit and deployment are attached
after this report is registered.
