---
baley_record: 1
record_id: "252f83b0-f799-44cb-a3ed-b8d172d2b907"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: detailed-plan
run_id: "608cf824-64a6-41a1-95ac-49423cc8f260"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: "997a66ce-1a0c-4939-a851-e4a17146c2e4"
---

# #149 tokenless MCP and OS device-secret implementation plan

## Chosen transport and boundary

Codex Desktop and CLI will invoke `baley-mcp` through tokenless stdio. The
existing HTTP listener is retained only as an optional compatibility/diagnostic
path and will bind to loopback only; it will not be a public endpoint or require
users to copy a gateway token into Codex configuration. The server continues to
issue only the #142 Workspace-scoped Operator credential. No Task or Gate
human-only authority is added.

## Delivery slices

1. Introduce an OS keychain abstraction with a production provider and a
   test-only fake. Keep device/gateway secrets and cached agent credentials out
   of the JSON store; the store has only version, server routing metadata,
   gateway identity, key references, and migration state.
2. Replace the stdio credential-store encryption key derived from
   `BALEY_MCP_GATEWAY_TOKEN` with a random device secret stored in the OS
   keychain. Ensure a fresh stdio process can resume a registered #142 gateway
   with no token environment variable.
3. Implement one-way legacy-store migration: decrypt with the old token only
   when present, write the new keychain-backed format atomically, validate a
   gateway renewal, and retain a bounded rollback marker that can never revive a
   revoked registration. Redact every secret in failures and diagnostics.
4. Tighten the optional HTTP path to loopback, add explicit diagnostics for
   keychain availability, storage format, registration/revoke state, and
   rollback eligibility, and update Codex Desktop/CLI registration instructions
   to omit Authorization headers and `BALEY_MCP_GATEWAY_TOKEN`.
5. Add Go unit/integration coverage for tokenless resume, migration success and
   rollback, keychain failure, logout/revoke/member removal, and listener
   rejection off loopback. Run full Go/frontend validation, deploy, and test the
   Desktop/CLI route on the Pilot before completion reporting.

## Risks and rollback

- Native keychain availability differs by OS. Failure must be actionable and
  must not fall back to plaintext or an environment-derived secret.
- The legacy encrypted-store reader is temporary and is only available while a
  legacy token is supplied. Rollback restores no secret after a server-side
  revoke, logout, or membership removal.
- The Docker MCP compatibility service remains token-protected until its
  replacement is verified; users of Desktop/CLI are migrated to stdio first.
