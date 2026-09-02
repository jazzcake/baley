---
baley_record: 1
record_id: "b25e9469-9755-431f-bb92-21c2f8daaa92"
task_id: 163
task_key: "mcp-loopback-only-security"
record_type: detailed-plan
run_id: "0ba7dde4-118c-469b-b54d-78da09a13c48"
created_at: "2026-09-02T21:30:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #163 MCP loopback-only transport and connection security plan

## Easy explanation

Baley MCP will have one supported transport: a single device-local Streamable
HTTP Gateway bound to loopback. The retired per-Codex-session stdio server and
all instructions that could recreate it will be removed.

## Why it is needed

Codex configuration already points at `http://127.0.0.1:8090/mcp`, but the
binary still defaults to stdio when started without arguments. Legacy tests,
scripts, images, and active documentation can therefore recreate the old
multi-process behavior. The final #162 review also found that a browser could
complete a device link, log out, and leave its two-minute redemption code valid.

## What changes when complete

1. Require an explicit supported `baley-mcp` subcommand and remove the
   `mcp.StdioTransport` execution branch. Apply the HTTPS-or-loopback server URL
   rule consistently to every command that contacts Baley.
2. Replace or remove stdio-only E2E coverage, keep transport contract coverage
   on Streamable HTTP, and make preflight reject command-based MCP registration,
   token environment variables, and Authorization headers.
3. Remove retired stdio setup helpers and retired Docker MCP packaging. Update
   current installation, operations, handoff, and pilot documentation to the
   single loopback Gateway model. Preserve immutable Task Records as historical
   evidence and preserve unrelated CLI stdin/stdout handling.
4. Bind every browser-completed pending link to the exact active account
   session. Redemption atomically rechecks the session, Account actor,
   membership, code, and device secret. Logout or expiry makes redemption fail,
   and logout eagerly removes pending links issued by that session.
5. Keep old currently running stdio processes alive until the user closes their
   Codex sessions or reboots, avoiding work loss. The newly built binary cannot
   start stdio, so no persisted setting can recreate them after reboot.

## Scope and exclusions

- Included: Go command transport selection and URL validation, MCP integration
  tests, PostgreSQL schema and repository code, browser link API, Windows
  install/preflight scripts, active product/operations docs, Docker deployment,
  live loopback smoke, security regression tests, independent review, and
  commit/push evidence.
- Excluded: generic stdin/stdout used by non-MCP CLI commands, deletion or
  rewriting of historical Task Records, firewall mutation, and forced
  termination of the user's currently open legacy Codex sessions.

## Verification

- Targeted tests for explicit subcommands, URL safety, Streamable HTTP MCP
  initialize/list/call, browser-session link binding, logout-before-redeem,
  expiry, wrong session/actor, replay, membership removal, revoke, and restart.
- PowerShell and shell syntax checks, preflight registration assertions, full
  Go suite including migrations and disposable PostgreSQL integration,
  frontend suite/typecheck/production build, release binary under
  `C:\dev-bin\baley\`, Docker deployment/readiness, live MCP read, and a final
  repository scan proving no executable or active-instruction MCP stdio path.
- A fresh independent Agent reviews the complete diff for security and
  regression risk. Blocking findings are fixed and re-reviewed before the Task
  is reported implemented.
