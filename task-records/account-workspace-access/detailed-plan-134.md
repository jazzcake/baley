---
baley_record: 1
record_id: "876920df-1538-4fcc-a295-e790b3f5f0a2"
task_id: 134
task_key: "workspace-membership-authorization"
record_type: detailed-plan
run_id: "11e787b3-93ba-4a61-a231-4fc7c2152509"
created_at: "2026-07-28T00:34:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Workspace membership and authorization detailed plan

Task #134 makes every Workspace read and write depend on the authenticated
principal's active membership.

1. Add persistent human membership roles (`viewer`, `operator`, `approver`,
   `owner`) and keep Agent membership restricted to `operator`.
2. Reuse the existing capability catalog and default-deny HTTP and MCP requests in
   enforced mode.
3. Derive initiated, executed, and approving Actor provenance from authentication
   context rather than trusting command JSON.
4. Add member list, add, role change, deactivate/reactivate, and atomic Owner
   transfer operations.
5. Protect the final active human Owner with a Workspace lock, application checks,
   and a database trigger.
6. Keep an explicit internal system principal for lease cleanup only.
7. Verify cross-Workspace denial, inactive Account/membership denial, forged Actor
   rejection, concurrent Owner changes, and legacy-to-enforced cutover.
