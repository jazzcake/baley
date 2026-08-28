---
baley_record: 1
record_id: "5bd00236-1309-4c41-b4db-8815ad128b62"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: review-response
run_id: "18b2c250-e49c-44ff-bf17-e079d3f7b9a2"
created_at: "2026-08-28T12:45:00Z"
created_by: "codex"
supersedes: null
---

# #149 review response: fresh-process credential renewal

## Findings addressed

Independent review found that the local credential store still persisted a
short-lived Workspace Agent credential and reused it after an MCP process
restart. It also found legacy PowerShell helpers and the other-PC handoff still
described or produced a plaintext `BALEY_AGENT_TOKEN` environment file.

## Resolution

- The persistent credential model is now version 6 and contains only gateway
  registration material plus pending connection data in the OS keychain.
  Workspace Agent credentials are held only in the running MCP client's memory.
- A new MCP process renews its Agent credential through
  `/v1/mcp/gateway-sessions`; it cannot reuse the credential delivered to a
  previous process. Existing version-5 keychain payloads containing an
  `agentToken` field are rewritten without it before use.
- Revocation remains fail-closed: an unauthorized renewal drops the saved
  gateway registration and returns to the signed-in connection flow. Legacy
  migration validates the gateway but never writes its validation credential.
- The plaintext environment helper and local token-issuance helper are retired
  fail-closed, and the cross-machine handoff directs operators to tokenless
  stdio and gateway renewal instead.

## Regression evidence

Focused `go test ./cmd/baley-mcp` covers fresh-process renewal, keychain
payload sanitization, legacy migration, and revoked-gateway recovery. Full Go,
frontend, typecheck, and production build verification are recorded in the
completion report.
