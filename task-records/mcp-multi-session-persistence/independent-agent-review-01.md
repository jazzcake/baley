---
baley_record: 1
record_id: "5dff57fd-49a4-4633-8c21-d13132f94fc3"
task_id: 156
task_key: "mcp-multi-session-persistence"
record_type: independent-agent-review
run_id: "97ee759d-4e31-4cbd-9f2b-5f9c0db054be"
created_at: "2026-08-29T14:45:00Z"
created_by: "independent-codex-review"
registration_state: pending
supersedes: null
---

# Independent review: multi-session MCP gateway persistence

## Initial findings

1. A gateway replacement could race an in-flight gateway resume, permitting an
   old-secret session token to escape the replacement's revoke query.
2. Browser logout only revoked the web session, leaving active MCP gateway
   credentials usable.

## Resolution review

- Enrollment locks the existing `(workspace_id, gateway_id)` registration
  before token revocation and secret rotation. Resume locks the same row, so a
  racing operation either issues before replacement and is revoked, or observes
  the replaced/revoked registration and is rejected.
- Logout locks and revokes every active gateway registration for the Account in
  the same transaction as the browser session revocation.
- Integration evidence proves two live sessions remain valid after ordinary
  resume, while logout invalidates both token use and gateway resume.

## Verdict

Pass. The prior two P1 findings are resolved. A deterministic replacement-vs-
resume interleaving test would be a useful future hardening addition, but is not
a release blocker because the shared registration row lock provides the required
serialization.
