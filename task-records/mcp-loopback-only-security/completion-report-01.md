---
baley_record: 1
record_id: "c8f1533e-765c-4a24-bd31-1a5350875c5b"
task_id: 163
task_key: "mcp-loopback-only-security"
record_type: completion-report
run_id: "d8c023a3-f6db-41c3-b54b-a769dd2d35b6"
created_at: "2026-09-02T17:15:35+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #163 completion report

## Easy explanation

Baley MCP now has one supported runtime shape: every Codex Desktop and CLI
client shares one token-free Gateway on `127.0.0.1:8090`. The executable cannot
fall back to a per-session MCP process when it is launched without a command.

## Why it was needed

Old command-based setup files and an implicit transport branch could recreate
many per-session processes after a configuration rollback. The final #162
security review also proved that a browser could finish a device link, log out,
and still race the local callback before its short code expired.

## What changed

- Removed the MCP stdio execution branch, its setup scripts, Docker packaging,
  environment example, tests, and current documentation. All executable modes
  now require an explicit supported subcommand.
- Kept token/store migration, rollback, and redacted diagnostics as explicit
  maintenance commands required by the migration contract.
- Replaced command-transport E2E with a prebuilt-binary Streamable HTTP E2E and
  made preflight require exact loopback URL registration with no command,
  bearer environment variable, or Authorization header.
- Added schema 25. A completed device link is bound to the exact active browser
  session, Account Actor, and current Workspace membership. Polling never
  issues a token; only the one-time local callback redeem can do so.
- Logout eagerly removes its pending links and revokes existing Gateways. Link
  Begin, redeem, and logout use a consistent request-to-session lock order, so
  a simultaneous logout and callback cannot deadlock or leave a credential
  active.
- Human-only Task and Gate grants are unchanged.

## Verification and deployment

- Full Go tests and `go vet`: PASS.
- Disposable PostgreSQL full integration suite through migration 25: PASS.
- Expired/wrong-Actor session, membership removal before redeem, migration
  preservation/deletion, and logout/redeem race tests: PASS. The complete race
  test passed ten repeated executions.
- Frontend: 17 files and 107 tests PASS; production build PASS.
- Windows installer/preflight PowerShell AST and Git diff checks: PASS.
- Streamable HTTP initialize/list/call using the prebuilt MCP executable: PASS.
- Independent review found one lock-order defect; it was fixed and the
  independent re-review returned PASS with no remaining blocker.
- Commit `39c58946924cb50d6e70cae4f0d325cbff66c47c` was pushed to
  `origin/jazzcake/mcp-login-membership-auth`.
- Docker deployment is healthy at schema 25. Viewer and Tailnet landing page
  return HTTP 200, and the public provider endpoint exposes Google.
- Installed Gateway release
  `C:\dev-bin\baley\releases\39c58946924c\baley-mcp.exe` owns only
  `127.0.0.1:8090` with the explicit `serve-http` command. Both standalone
  Codex and Orca runtime homes contain URL-only Streamable HTTP registration,
  and a live Baley Workspace read succeeded through the restarted Gateway.
- A repository-wide active-source scan reports no MCP stdio,
  `StdioTransport`, or `CommandTransport` reference outside immutable historic
  Task Records. No firewall rule was created or changed.

## Remaining transition state

Fourteen old no-argument processes still belong to Codex sessions opened before
this deployment. They were intentionally not killed because the user has not
rebooted and their work may be unsaved. No persisted Codex or scheduled-start
configuration references that binary, so they terminate at reboot and cannot
be recreated. The new executable rejects no-argument startup with exit code 1.

Task #163 is ready to be reported `implemented`. Human confirmation remains a
separate user-only action.

