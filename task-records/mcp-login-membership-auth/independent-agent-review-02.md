---
baley_record: 1
record_id: "05b69f0d-6db7-431c-8fb1-b750e21af5be"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: independent-agent-review
run_id: "7ba5f8e0-b7c8-4113-84b8-94bd089b7059"
created_at: "2026-09-02T14:42:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "2e1a3f8a-df04-4721-b988-f43077108a92"
---

# #162 final independent review

## Verdict

FAIL. One Medium security boundary defect and one Low transition defect remain.
The reviewer inspected the complete branch diff independently of earlier PASS
reports and did not modify repository files.

## Medium — logout can be followed by pending-link redemption

The browser `complete` endpoint stores the Actor and a two-minute code, but the
login link is not bound to the browser session that completed it. Logout revokes
already registered Gateways but does not invalidate the pending login link.
Redeem then validates the code, device secret, and current membership without
revalidating the issuing browser session.

Consequently this sequence can issue a fresh Gateway credential after logout:

1. Click `Connect local Gateway`.
2. Log out before the loopback callback redeems the code.
3. Resume the callback within two minutes.
4. Redeem issues a new Gateway and Agent token.

Evidence:

- `server/internal/transport/httpapi/mcp_connection.go:233`
- `server/internal/persistence/postgres/mcp_connection.go:90`
- `server/internal/persistence/postgres/account_access.go:422`
- migration 24 has no browser-session binding for the pending link.

Required remediation: bind the link/code to the issuing browser session and
recheck that session atomically at redeem, or invalidate the Actor's pending
links during logout. Add a `begin → complete → logout → redeem denied`
PostgreSQL integration regression test.

## Low — v6/v7 credential-store writer transition

The Windows installer deliberately leaves existing stdio sessions running and
macOS does not detect them. A v6 process does not understand v7 `pendingLinks`;
if it writes the store during the transition, it can serialize the old schema
and remove the new pending-link field. File locking prevents byte corruption but
does not prevent this semantic downgrade, so a new connection attempt can be
lost and require another login.

Required remediation: delay the switch until old writers drain, or introduce a
schema-generation/CAS guard and a v6/v7 concurrent-writer test that proves newer
fields cannot be lost.

## Checks that passed

- No active MCP approve/reject API, `approvalUrl`, `mcp-connect`, or
  `BALEY_MCP_APPROVAL_ORIGIN` remains.
- Remaining approval terms belong to human-only Task/Gate/Lane/Workspace-close
  grants or historical migration compatibility.
- Viewer/Approver derive read-only Agent scope; Owner/Operator derive only the
  Agent-safe Operator catalog.
- Code replay, wrong code, Gateway replacement, membership removal, archive,
  and revoke paths are covered.
- Human-only command grants remain enforced.
- PostgreSQL migration 1 through 24, focused Go packages, MCP persistence and
  human-grant integration tests, React authentication tests, PowerShell/Bash
  syntax checks, and `git diff --check` passed.
