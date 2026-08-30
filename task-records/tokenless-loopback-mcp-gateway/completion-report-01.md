---
baley_record: 1
record_id: "68c798fb-6da5-4ab5-9dc1-e65c991c3091"
task_id: 157
task_key: "tokenless-loopback-mcp-gateway"
record_type: completion-report
run_id: "f028ea64-2081-4133-a471-5d41d204c117"
created_at: "2026-08-30T17:20:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Completion report: tokenless loopback MCP Gateway

## Delivered

- Replaced per-Codex-session stdio operation with one scheduled, tokenless
  Streamable HTTP Gateway for the Windows user session. It only listens on
  literal `127.0.0.1` or `::1` and Codex is configured with
  `http://127.0.0.1:8090/mcp`, without a bearer-token environment variable or
  plaintext authorization header.
- A registered Keychain-backed device can now automatically enroll an active
  Workspace for the same Account when that Account has an active
  `workspace:operate` membership. The browser `mcp-connect` flow remains the
  first-device and post-invalidation recovery boundary.
- Membership removal, logout, archive, gateway replacement, and suspected
  compromise remain credential-revocation boundaries. Viewer and Approver
  memberships are explicitly prevented from minting an Operator Agent token.
- The credential store is safe for concurrent Codex processes via OS-managed
  file locks. The Windows installer rejects a handover while legacy stdio
  processes are live, verifies the exact Gateway process and MCP initialize,
  then changes Codex configuration.

## Verification

- Focused Go packages and `go test ./...` from `server` passed.
- PostgreSQL integration coverage passed against the disposable test database.
- Frontend suite: 93 tests passed; production frontend build passed.
- Docker API rebuild/deploy reached `/readyz` schema version 22.
- `C:\dev-bin\baley\baley-mcp.exe` was built and the scheduled local Gateway
  was started. A live JSON-RPC initialize succeeded and diagnostics confirmed
  the Keychain-backed store.
- A live member Workspace (`410f335e-ddb2-443f-be3c-7d1d18ccd534`) opened via
  automatic device enrollment without returning `workspace_login_required` or
  requiring an `mcp-connect` browser approval.
- Independent review passed after authorization, loopback-address, crash-lock,
  and installer-lifecycle remediation.

## Commit and operational scope

Implementation is committed and pushed as `1c56ba99b44e435e18ca7ae169d47d0c62cb0292`.
The loopback service denies external network access but is same-machine trust,
not a user-isolation transport for shared/untrusted Windows desktops; that
constraint is documented. Existing legacy stdio processes may finish naturally;
new Codex Desktop/CLI sessions use the single loopback Gateway.
