---
baley_record: 1
record_id: "c6d2b97f-ca4d-4d72-9d8a-bf39be636dba"
task_id: 133
task_key: "authenticated-approval-agent-token"
record_type: detailed-plan
run_id: "69106084-1393-481c-80ea-3ff0bdfeeba6"
created_at: "2026-07-28T00:34:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Authenticated approval and Agent token detailed plan

Task #133 gives MCP Agents a Workspace-scoped identity while keeping human approval
exclusive to authenticated human sessions.

1. Issue, list, rotate, and revoke opaque Agent tokens; persist hashes and safe
   fingerprints only.
2. Limit Agent scopes to the Operator capability bundle and reject approval or
   administration scope escalation.
3. Let the MCP bridge send its token as a Bearer credential without including it in
   command JSON or logs.
4. Issue a short-lived approval grant only after the server recomputes a fresh
   preview for an authenticated human with the required capability.
5. Bind the grant to Workspace, command, target, revision, command hash, decision
   snapshot, warnings, and proceed reason.
6. Lock, revalidate, consume, and execute the grant atomically; rollback preserves
   the grant and concurrent or mismatched reuse fails.
7. Verify expiry, membership/role revocation, stale preview, cross-Workspace use,
   double use, and secret redaction.
