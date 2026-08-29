---
baley_record: 1
record_id: "32f1b251-21c9-4980-b7a4-790c87f5f630"
task_id: 155
task_key: "workspace-menu-archive"
record_type: review-response
run_id: "7eed2329-8dd1-4e71-bc65-03272fccdb99"
created_at: "2026-08-29T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #155 review response

All blocking review findings were corrected.

- Auth bootstrap and membership refresh now request archived owner memberships;
  the Workspace chooser separates active Workspaces from archived owner
  Workspaces and exposes `Restore Workspace`. This covers the only-Workspace
  archive case after the archive-triggered session revocation.
- `Repository.WorkspaceIDs` now returns active Workspaces only, preventing
  expiration sweeps from changing archived history.
- Transactional `membershipFromQuerier` joins the Workspace and requires
  `state='active'`, so direct command execution has the same archived-state
  boundary as HTTP routes.

The focused UI suite now includes the final archived Workspace restore path.
