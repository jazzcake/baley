---
baley_record: 1
record_id: "ece86ad5-8e41-475a-b552-564f804aa707"
task_id: 137
task_key: "mcp-connection-restart-durability"
record_type: implementation
run_id: "1cfd36d6-f688-4a79-a498-d3981062b3d3"
created_at: "2026-08-08T23:55:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# MCP connection approval restart durability

## Delivered

- Added migration 00018 with `mcp_connection_requests`. It retains only the
  request identifier, Workspace/Agent references, SHA-256 connection-secret
  verification value, status, timestamps, and deciding Owner provenance.
- Replaced the API-process memory broker with PostgreSQL reads and writes, so a
  new API process continues an unexpired request created by its predecessor.
- Approval records a durable decision. The raw Operator token is created only
  by the authenticated polling request, inside the transaction that changes an
  approved request to `consumed`; it is never kept in the request table.
- Added an Owner rejection endpoint and explicit rejected state. Expired
  requests are deleted before read/decision/poll and are indistinguishable from
  missing requests to the polling client.
- Refactored Agent-token issuance to be usable inside the connection-consume
  transaction, preserving Agent membership and security-event creation.
- Added a Workspace-header UUID copy action in the Viewer.

## Verification

- `go -C server test ./...`
- `npm test -- --run`
- `npm run build`
- PostgreSQL integration test creates a request, opens a fresh Repository to
  simulate API restart, approves and consumes it once, then verifies duplicate
  polling, rejection, expiry, and hash-only persistence.

## Residual risk

The new reject endpoint is server-complete but is not yet surfaced as a button
on the connection-approval screen; the existing approval flow is unchanged.
