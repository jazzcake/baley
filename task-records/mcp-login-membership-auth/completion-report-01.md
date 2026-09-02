---
baley_record: 1
record_id: "dd505ee9-b576-46e5-a7d0-914881207935"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: completion-report
run_id: "a02c39b2-7868-4ad7-a466-cf5861408ce8"
created_at: "2026-09-02T12:47:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #162 completion report

## Outcome

MCP Workspace approval is removed from the active product model. A local
Gateway is linked once to an authenticated Account, and each request derives
Workspace access from that Account's current active membership and role.
Task/Gate/Lane/Workspace-close human-only grants remain enforced and were not
weakened.

## Implemented

- Replaced approve/reject connection APIs, database states, error payloads,
  routes, Viewer UI, credential metadata, Skill instructions, and current
  product/operations documentation with login-link terminology and behavior.
- Added explicit browser device-link intent, local pending-link verification,
  a two-minute one-time browser code, matching device-secret redemption, and an
  atomic membership/role recheck.
- Locked role mapping: Viewer/Approver are read-only Agents;
  Owner/Operator receive ordinary Agent-safe operation scopes; human-only
  capabilities never enter Agent tokens.
- Preserved immediate invalidation on logout, membership removal, archive,
  Gateway replacement, suspected compromise, and explicit revoke.
- Migrated schema to version 24 and credential metadata to the login-link model.
  Historical migration compatibility keeps old state names only while upgrading
  old databases; it is not reachable as active behavior.
- Replaced per-session stdio packaging with a single per-user loopback Gateway
  for Windows and macOS. Windows builds are revisioned under
  `C:\dev-bin\baley\`; Codex uses `http://127.0.0.1:8090/mcp` with no bearer
  environment variable or plaintext Authorization header.
- Removed the obsolete `scripts/run-baley-mcp.ps1` loader and stale pilot docs.

## Security review

Independent review initially found device-link phishing and unsafe PID-file
trust. Both were remediated. Final review of
`3a6ad7c68253fdc667ff04291bfc60b11fd74128` returned PASS with no actionable
finding. The review and response are recorded separately.

## Verification

- All Go packages and PostgreSQL integration tests passed.
- Frontend: 17 test files and 107 tests passed.
- TypeScript and Vite production build passed.
- PowerShell and macOS installer syntax checks passed.
- Windows installer left 16 existing stdio sessions to drain naturally, started
  exactly one validated `serve-http` Gateway on `127.0.0.1:8090`, and registered
  Codex with no token/header.
- A fresh Streamable HTTP session initialized, listed 78 tools, and completed a
  real `baley_workspace_get` request without Workspace reconnection.
- Docker API and Viewer are running; `/readyz` reports schema 24 and ready,
  Google is listed as an OIDC provider, and the deployed Viewer returns HTTP
  200.
- Targeted repository scans found no active MCP approval URL, endpoint, route,
  state, or Workspace-specific decision documentation.

## Evidence commits

- `9417d7ca4820819efcf56f2d06c92e77b1028395` — login/membership model.
- `f96a3b97fad1cc3411f760f91d381bf98a0554fb` — device-bound security fixes.
- `06b5db5d3dc6da0f64c80f41d83808fc21436df9` — legacy session drain.
- `3a6ad7c68253fdc667ff04291bfc60b11fd74128` — final installer and legacy cleanup.
- `415ab74f92e585f3128f41cf43ea18c6eb36e4bd` — review records.

## Status

Implementation, tests, deployment, live MCP use, documentation cleanup, and
independent review are complete. Task #162 is ready to report `implemented`.
Human confirmation remains a separate Viewer action.
