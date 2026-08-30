---
baley_record: 1
record_id: "f1c9bdc2-b152-4677-940d-ba3a1417ec9e"
task_id: 157
task_key: "tokenless-loopback-mcp-gateway"
record_type: detailed-plan
run_id: "1806f520-8b67-4a3a-9780-75a6596742ac"
created_at: "2026-08-30T15:30:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Tokenless single loopback MCP Gateway plan

## Objective

Replace one `baley-mcp` stdio process per Codex session with one per-user local
Gateway. It must serve MCP only on loopback, use the existing OS Keychain
device binding, and keep Codex configuration free of a gateway bearer token.

When the Gateway already proves that it is a registered device for a signed-in
Account, a second Workspace where that Account has active membership is enrolled
automatically. A browser `mcp-connect` decision remains only for the first device
enrolment and after an explicit security invalidation.

## Safety invariants

- No listener may bind outside `127.0.0.1` or `::1`; this work neither creates nor
  changes firewall rules.
- No `BALEY_MCP_GATEWAY_TOKEN` or plaintext Authorization header is stored in
  Codex configuration. The Gateway reads device material only through the OS
  Keychain-backed credential store.
- Auto-enrolment proves possession of an already active, Account-bound gateway
  registration, then rechecks target Workspace activity and active membership in
  one transaction. It never trusts only a gateway ID.
- First-device onboarding, logout, membership removal, archive, gateway replacement
  and suspected-compromise revoke remain hard invalidation boundaries.
- Gateway access remains Operator-only: Task confirmation, Gate changes and Gate
  pass human-only powers do not become automatic.

## Implementation and verification

1. Trace current connection requests, gateway registration persistence and the
   Streamable HTTP handler. Add a server endpoint that can derive the Account
   from an existing Keychain-proven registration and atomically enroll the target
   Workspace only when membership is active.
2. Extend the MCP credential flow to attempt this automatic enrollment before it
   creates a browser approval request. Preserve the approval flow as the first
   device/recovery fallback and audit auto enrollment without recording secrets.
3. Make `serve-http` a tokenless, strictly loopback-only Streamable HTTP Gateway;
   use a shared client safely across MCP sessions and reject unsafe bind addresses.
4. Add Windows user-session lifecycle and installer/config migration so Codex
   Desktop and CLI use the local HTTP endpoint without a plaintext bearer header.
   Existing stdio stores must migrate or roll back safely.
5. Add concurrent multi-session tests for cached and initial credential paths,
   auto-enrollment, forbidden membership, revoke/logout/replacement, loopback
   enforcement and concurrent Task/Run conflict behavior.
6. Build the executable in `C:\\dev-bin\\baley\\`, run focused and complete Go/
   frontend suites, deploy the service, smoke-test Codex Desktop and CLI, obtain
   an independent review, and register completion evidence.
