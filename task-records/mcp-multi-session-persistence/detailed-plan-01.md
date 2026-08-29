---
baley_record: 1
record_id: "e9ff5764-eeba-440a-ba0d-efbb9c4679d7"
task_id: 156
task_key: "mcp-multi-session-persistence"
record_type: detailed-plan
run_id: "364d717f-0cea-4655-86ca-c3b848f2895e"
created_at: "2026-08-29T14:33:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Multi-session MCP gateway persistence plan

## Root cause

Every new stdio `baley-mcp` process resumed the shared registered gateway, and
the server revoked all previous Agent tokens for that registration. An already
running Codex session then received HTTP 401. The client correctly treats a
rejected token as potentially revoked, but incorrectly had to remove the
device credential and request browser approval again.

## Change

Gateway session resume issues a fresh process-local Agent token without
revoking tokens from other live MCP processes on the same registered device.
Gateway replacement, explicit/suspected-compromise revocation, logout,
membership changes, and Workspace archive keep their existing all-token
revocation behavior.

## Verification

1. Enroll a gateway and resume it from a second MCP process.
2. Prove both the old and newly issued token authenticate.
3. Prove replacement and revocation still reject the earlier tokens.
4. Run tokenless Keychain-store regression, full Go/frontend suites, and
   deployment smoke checks.
