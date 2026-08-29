---
baley_record: 1
record_id: "2111b6f4-9322-495d-8bcb-4f790928c86b"
task_id: 155
task_key: "workspace-menu-archive"
record_type: review-response
run_id: "0ccf5025-6e7b-47b4-bb60-7a3ff883c392"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 card menu review response

The Workspace lifecycle actions were moved from the title switcher to each
Owner card's overflow menu. Independent review identified two blockers:
archive success left a stale chooser session, and the action menu incorrectly
contained both a form and dialog.

Archive now calls the authenticated application's session-expiry transition as
soon as the server revokes the session. The chooser is replaced immediately.
The action list is the only ARIA menu; Rename is a sibling form popover and
Archive is a sibling non-modal confirmation dialog. Initial focus is assigned
to the input/cancel action, and Escape restores focus to the card overflow
trigger. Focused tests cover the archive state transition and Escape behavior.
