---
baley_record: 1
record_id: "089ab381-b0a0-462e-875a-cb471ea2daaf"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: review-response
run_id: "63e04f1d-1d0b-45d0-98fc-d4ce61fa8f16"
created_at: "2026-09-02T12:44:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #162 review response

## Applied changes

- Replaced page-load linking with an explicit device-link button.
- Added a loopback start boundary that accepts only a connection present in the
  initiating Gateway's local pending store.
- Added a two-minute one-time browser code and required both that code and the
  device connection secret in the server-side redeem transaction.
- Rechecked membership and role-derived Agent-safe scope atomically at redeem.
- Fixed the PostgreSQL migration actor FK type and consumed-link terminal UI.
- Replaced macOS per-session stdio packaging with a per-user loopback
  LaunchAgent matching the Windows single-Gateway model.
- Hardened Windows old/new PID migration against corrupt files and PID reuse by
  validating the exact executable, allowed path, command argument, listener
  address, port, and owning process before stopping it.
- Removed the obsolete stdio `run-baley-mcp.ps1` loader and corrected product,
  plugin, pilot, and operations documentation to the membership model.

## Verification after response

- Independent reviewer final verdict: PASS with no actionable findings.
- `go test ./...`: passed.
- Frontend: 17 files and 107 tests passed.
- TypeScript production build: passed.
- Installed Gateway: one `serve-http` process from the revisioned
  `C:\dev-bin\baley\releases\3a6ad7c68253\` path owns `127.0.0.1:8090`.
- Fresh MCP initialize exposed 78 tools and a real `baley_workspace_get` call
  succeeded without a Workspace reconnect step.
