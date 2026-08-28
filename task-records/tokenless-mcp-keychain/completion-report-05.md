---
baley_record: 1
record_id: "eb81ab77-7b8c-4285-9e3c-905af171bf17"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: completion-report
run_id: "18b2c250-e49c-44ff-bf17-e079d3f7b9a2"
created_at: "2026-08-28T12:45:00Z"
created_by: "codex"
supersedes: null
---

# #149 completion report: tokenless MCP persistence remediation

## Delivered

The MCP client no longer persists or restores Workspace Agent credentials.
Only the registered gateway secret persists in the OS keychain; every fresh MCP
process renews an Operator-only Agent credential before making a Workspace call.
The token-bearing local PowerShell helpers are retired, and operations and
handoff documentation now describe the tokenless stdio path.

## Verification

- `go test ./cmd/baley-mcp` passed after exercising fresh-process renewal,
  keychain sanitization, legacy migration, and revoked-gateway recovery.
- `go test ./...` and `go vet ./...` passed in `server`.
- `npm test -- --run` passed (16 files, 87 tests), followed by successful
  `npm run typecheck` and `npm run build`.
- `codex mcp get baley --json` reported a stdio registration with only
  `BALEY_SERVER_URL` and `BALEY_MCP_CREDENTIAL_STORE`.
- A deployed Tailnet API smoke test reported a readable keychain-backed store,
  completed the normal signed-in Operator-only gateway binding, and then
  successfully read Task #149 at Workspace revision 798 without a token
  environment variable.

## Residual risk and assessment

The existing installed MCP executable could not be overwritten while active, so
the committed source and release artifact remain the deployment vehicle for the
next local MCP restart. The deployed API smoke verified the live enrollment and
tokenless request path; Task #149 can be reported implemented after the commit,
push, and record metadata attachment, but remains human-required and is not
confirmed by this report.
