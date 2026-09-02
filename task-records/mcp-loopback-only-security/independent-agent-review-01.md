---
baley_record: 1
record_id: "d3e53712-4de7-44ba-8767-a2674bf05752"
task_id: 163
task_key: "mcp-loopback-only-security"
record_type: independent-agent-review
run_id: "07e24bc1-9c08-4034-9b12-763f6246d2d2"
created_at: "2026-09-02T17:07:01+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #163 independent security and regression review

## Final verdict

PASS after review response. The independent reviewer found no remaining
blocker and did not modify repository files.

## Initial findings

- Medium: callback redeem locked the connection request before the browser
  session, while logout locked the session before deleting pending requests.
  A concurrent callback and logout could therefore deadlock and return a 500
  from one transaction, even though it did not create a durable authorization
  bypass.
- Low coverage gap: expired and wrong-Actor sessions, membership removal before
  redeem, the real logout/redeem race, and migration 25's legacy-row policy did
  not have direct regression coverage.

## Review response and re-review

- `RevokeSession` now locks every session-bound pending request in stable ID
  order before locking the session. Begin, redeem, and logout therefore share
  the request-to-session lock order. A still-unbound Begin loses to logout at
  the session lock and rolls back after observing revocation.
- Integration coverage now rejects expired and wrong-Actor sessions, rejects a
  redeem after the human membership is removed, races logout and redeem eight
  times, and proves any token issued by a winning redeem is revoked before the
  successful logout returns.
- Migration coverage proves that version 25 retains unlinked pending and
  already-consumed audit rows while deleting pre-session code-pending and
  linked rows that cannot be safely attributed to a live browser session.
- The reviewer independently reran the focused disposable-PostgreSQL tests and
  related Go packages. Re-review passed with no remaining finding.

## Boundaries that passed

- No active source, script, config, deployment, or current documentation path
  starts MCP through stdio, `StdioTransport`, `CommandTransport`, or an implicit
  no-argument command. Historical Task Records remain immutable evidence.
- Codex registration is URL-only Streamable HTTP on exact loopback, with no
  bearer-token environment variable or Authorization header.
- Polling cannot issue an Agent token. Only the device-secret and one-time-code
  callback redeem can issue it, while atomically rechecking browser session,
  Account/Actor, Workspace membership, and device binding.
- Human-only Task and Gate authorization remains unchanged and cannot be
  exercised by the MCP Agent bearer alone.

