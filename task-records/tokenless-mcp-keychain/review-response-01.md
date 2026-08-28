---
baley_record: 1
record_id: "cd3774ec-04b8-4ac2-8cc7-f1ed40ef2fb5"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: review-response
run_id: "d5175dcd-1a1e-4149-bf17-e75f43041392"
created_at: "2026-08-28T12:10:00Z"
created_by: "codex"
supersedes: null
---

# #149 review response

## Response to persisted Agent credential reuse

`baley-mcp` now keeps issued Agent credentials only in the `client` process's
`sessionTokens` map. Both credential-store reads and writes remove historic
`AgentToken` values; a token-only legacy entry is discarded, while a gateway
secret remains in the keychain and must renew before the first request from a
new process.

`TestKeychainStoreResumesGatewayWithoutGatewayToken` injects a pre-fix
keychain payload containing an Agent credential and proves that a fresh client
renews through `/v1/mcp/gateway-sessions` instead. `TestRevokedGatewayInvalidatesCachedSessionAndRequiresReconnect` proves a rejected request clears the in-memory credential, rejects the gateway renewal, removes the local Workspace credential, and returns the normal signed-in reconnection path.

## Response to plaintext helper material

The environment helper now permits only `BALEY_SERVER_URL` and
`BALEY_MCP_CREDENTIAL_STORE`. The retired local Agent-token bootstrap body was
removed, its parser regression test is tokenless, and the cross-PC handoff now
instructs operators to use only the stdio registration and OS keychain.

## Review disposition

Both blocking findings are resolved in the repository and the focused tests
pass. The independent review's confirmed server-side revocation and
human-authority boundaries remain unchanged.
