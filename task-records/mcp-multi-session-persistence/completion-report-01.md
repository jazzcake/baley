---
baley_record: 1
record_id: "ee1d213f-96ac-4325-9a39-62e1098ef83e"
task_id: 156
task_key: "mcp-multi-session-persistence"
record_type: completion-report
run_id: "608e747e-e4c5-4e41-8244-7e9244f5271e"
created_at: "2026-08-29T14:46:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Completion report: multi-session MCP gateway persistence

## Delivered

- New Codex stdio MCP processes can renew via the same Keychain-backed gateway
  without invalidating already-running local Codex sessions.
- Gateway replacement and resume are serialized on the registered gateway row.
- Browser logout revokes the Account's active MCP gateways and their Agent
  tokens in the same transaction.
- Codex now launches the tokenless MCP binary from
  `C:\dev-bin\baley\baley-mcp.exe`; the registration contains only server URL
  and credential-store path.

## Evidence

- Disposable PostgreSQL MCP persistence integration: passed.
- Full Go suite: passed.
- Frontend suite: 93 tests passed; production build passed.
- Docker deployment completed and `/readyz` returned schema version 22 / ready.
- Independent review found and verified fixes for gateway replacement/resume
  serialization and logout revocation.

## Residual risk

The server-side concurrency invariant is enforced by row locking. A deterministic
multi-transaction interleaving test is worthwhile future hardening, but no
remaining release blocker was found.
