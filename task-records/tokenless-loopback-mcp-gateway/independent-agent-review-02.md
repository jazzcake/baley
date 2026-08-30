---
baley_record: 1
record_id: "bb1a0e75-0585-4a7c-b208-5074c4900be3"
task_id: 157
task_key: "tokenless-loopback-mcp-gateway"
record_type: independent-agent-review
run_id: "19e38142-ea7c-4a77-84e5-a2c2d60fcb86"
created_at: "2026-08-30T17:10:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "968a40a5-23af-41d5-9168-88ef914b9991"
---

# Independent review: tokenless loopback MCP Gateway

## Verdict

Approved after remediation.

## Findings and resolution

1. Automatic enrollment derives its Account from an active, Keychain-proven
   source gateway and requires the target human membership to hold
   `workspace:operate` through the central authorization catalog. Viewer and
   Approver memberships cannot mint an Operator Agent credential.
2. The HTTP listener accepts only literal `127.0.0.1` or `::1`; `localhost`
   and every non-loopback bind are rejected.
3. Shared credential-store updates use OS-managed locks (`LockFileEx` on
   Windows and `flock` elsewhere). A Windows crash-owner regression proves the
   lock is released when its owner exits unexpectedly.
4. The Windows installer refuses a migration while legacy stdio clients are
   still live, tracks the exact Gateway PID, checks an MCP `initialize`
   response, then writes Codex's tokenless HTTP endpoint.

## Independent verification

- `go test ./cmd/baley-mcp -count=1` passed.
- `go test ./internal/transport/httpapi -count=1` passed.
- The scheduled Windows Gateway bound only to `127.0.0.1:8090`, returned MCP
  `initialize`, read the Keychain-backed credential store, and automatically
  opened an existing-member Workspace without `mcp-connect` approval.

## Residual scope

The loopback boundary denies external-network access; it does not isolate
different Windows users sharing the same desktop. The operational guide marks
this as same-machine trust only, with tokenless stdio retained for shared or
untrusted desktops until a per-user OS-authenticated local transport exists.
