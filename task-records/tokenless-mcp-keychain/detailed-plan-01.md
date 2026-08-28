---
baley_record: 1
record_id: "997a66ce-1a0c-4939-a851-e4a17146c2e4"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: detailed-plan
run_id: "7445423e-8496-4e5b-8673-7f6745ec91b9"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Tokenless local MCP and OS credential-store plan

## Boundary

This is planning only. #149 implementation follows #148 implementation and the
required G#6 human decision; no Phase 07 product change is included here.

## Design

- Prefer tokenless stdio for Codex Desktop and CLI. If a loopback transport is
  retained for a client surface, bind it strictly to loopback and authenticate the
  local process through an OS-protected device secret, never a copied environment
  variable or plaintext Codex Authorization header.
- Store the gateway/device credential in the native OS keychain (macOS Keychain,
  Windows Credential Manager, and Linux Secret Service where available). The
  persisted credential store contains routing metadata and opaque key references,
  not the secret or a derivation from `BALEY_MCP_GATEWAY_TOKEN`.
- Migrate legacy encrypted stores transactionally: read with the old local token,
  write the keychain-backed format, validate a #148 OAuth/device renewal, then
  remove the obsolete secret. Keep a time-bounded rollback record that cannot
  restore a revoked credential.
- Add explicit diagnostic commands that report registration, keychain availability,
  loopback binding, migration state, and revocation state without printing secrets.
- Reject public/listening interfaces and do not change the server's operator-only
  MCP scope or Task/Gate human-approval checks.

## Verification plan

Test migration/rollback/revoke on supported OSes; assert Codex CLI and Desktop
registrations contain no `BALEY_MCP_GATEWAY_TOKEN`; exercise Google and internal
OIDC device renewal; and prove external-host access cannot reach the local transport.
