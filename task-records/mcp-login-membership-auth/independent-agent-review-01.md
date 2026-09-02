---
baley_record: 1
record_id: "2e1a3f8a-df04-4721-b988-f43077108a92"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: independent-agent-review
run_id: "78609fc5-8298-4cb8-ae5b-d247e1b2ade5"
created_at: "2026-09-02T12:43:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #162 independent security review

## Final verdict

PASS after remediation. The reviewer found no actionable issue in the final
implementation at commit `3a6ad7c68253fdc667ff04291bfc60b11fd74128`.

## Findings and remediation

1. The first implementation could complete a remote login link when a user
   merely opened the page. This allowed device-link phishing across Gateways.
   The final flow requires an explicit `Connect local Gateway` click, verifies
   that the loopback start URL names a locally pending connection, and redeems
   a two-minute browser code only together with the matching device secret.
2. The initial Windows migration trusted a PID file too broadly. The installer
   now parses the PID safely and stops a process only when its exact executable
   name and allowed Baley path, standalone `serve-http` argument, and ownership
   of `127.0.0.1:8090` all match. Corrupt or reused PID files cannot kill an
   unrelated process.
3. Migration FK type, consumed-link UI, macOS loopback packaging, and wording
   findings were corrected and covered by focused tests.

## Security boundaries verified

- The browser link is bound to the initiating local Gateway and is single-use.
- Redeem atomically rechecks active membership and derives Agent-safe scopes
  from the current role.
- Viewer and Approver receive read-only Agent scope; Owner and Operator receive
  ordinary operation scopes. Human-only capabilities are never copied.
- Logout, membership removal, archive, replacement, and revoke invalidate the
  Gateway or derived credentials.
- The local transport listens only on `127.0.0.1`; Codex config contains no
  token or Authorization header.
- Legacy stdio sessions drain naturally while one validated loopback Gateway
  serves all new sessions.

## Independent verification

- Focused Go packages passed.
- Frontend authentication tests passed.
- PowerShell syntax and live process ownership were inspected.
- Repository scan found no active MCP Workspace approval endpoint, route,
  state, or documentation. Remaining approval code is the separate Task/Gate/
  Lane/Workspace-close human-only grant boundary or migration history.
