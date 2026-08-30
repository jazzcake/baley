---
baley_record: 1
record_id: "968a40a5-23af-41d5-9168-88ef914b9991"
task_id: 157
task_key: "tokenless-loopback-mcp-gateway"
record_type: independent-agent-review
run_id: "40d4033a-5a6f-4e60-ae85-00274a1bb56d"
created_at: "2026-08-30T16:15:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Independent review — tokenless loopback MCP Gateway

## Verdict

Approved after remediation.

## Findings and resolution

1. The initial automatic enrollment path allowed an active Viewer or Approver
   membership to mint an Operator Agent token. Enrollment now derives its
   Account from the existing gateway proof and requires the target human role
   to hold `workspace:operate` through the central authorization catalog.
   PostgreSQL integration coverage rejects Viewer and Approver roles and proves
   no Agent membership or token is created.
2. `localhost` was removed from the listener allow-list. Only literal
   `127.0.0.1` and `::1` bind addresses are accepted.
3. The initial exclusive-create lock could survive a forced process stop. It is
   now an OS-managed file lock (`LockFileEx` on Windows, `flock` elsewhere),
   with a Windows crash-owner regression test proving automatic release.
4. The installer now refuses to switch while legacy stdio processes exist,
   tracks the Gateway PID, stops the exact `serve-http` child on replacement,
   and requires a local MCP `initialize` response before changing Codex config.

## Verified evidence

- Independent reruns: `go test ./cmd/baley-mcp -count=1` and
  `go test ./internal/transport/httpapi -count=1` passed.
- The final local smoke test confirmed the scheduled Gateway listened only on
  `127.0.0.1:8090`, returned MCP initialize, used the Keychain-backed store,
  and opened the existing member Workspace without an `mcp-connect` approval.

## Residual scope

Loopback is an external-network denial boundary, not Windows-user isolation.
The transport is documented as same-machine trust only; shared or untrusted
Windows desktops must keep tokenless stdio until an OS-authenticated local MCP
transport is available.
