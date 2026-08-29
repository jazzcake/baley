---
baley_record: 1
record_id: "7d8ebc99-0662-4ca9-b35f-2bce8474bc60"
task_id: 155
task_key: "workspace-menu-archive"
record_type: independent-agent-review
run_id: "83c4c85c-289d-403f-9644-3edfaeae5fd3"
created_at: "2026-08-29T00:00:00Z"
created_by: "independent-agent"
registration_state: pending
---

# #155 card menu independent review

Initial review of the card-menu placement found two P1 defects: archive success
did not immediately clear the local session, and ARIA menu content mixed a form
and dialog inside the menu container.

The remediation review passed. Archive now calls the local session-expiry
transition after server revocation. The action list remains an ARIA menu while
the rename form and archive confirmation are sibling popovers. Focus lands on
the appropriate first control and Escape returns focus to the overflow trigger.
No release blocker remains.
