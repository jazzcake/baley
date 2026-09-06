---
baley_record: 1
record_id: "574c746c-16e9-4e44-b8e4-6de677385639"
task_id: 180
task_key: "mcp-tool-catalog"
record_type: detailed-plan
run_id: null
created_at: "2026-09-06T14:31:55Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #180 detailed plan: compact default MCP tool catalog

## Objective and measured baseline

Reduce the schema-token cost Codex pays for Baley's current 78-tool MCP
catalog while preserving all domain commands and the server's authorization
and validation boundaries. Measure the actual serialized `inputSchema` sum,
pin the baseline in tests, and require a deterministic default of no more than
15 tools and at least 75% fewer schema bytes.

## Design

1. Keep a compact, static default at `/mcp` with essential Workspace, Task,
   Lane, Backlog, Gate, and diagnostic reads.
2. Add an on-demand command catalog and three fixed bridges: preview, routine
   Operator execute, and human/conditional-approval execute. The bridge accepts
   typed command arguments and the existing envelope but cannot choose an HTTP
   path or tunnel an unknown command.
3. Classify every HTTP command deterministically from the literal command
   contract. Reject unknown commands, malformed inputs, wrong execution bridge,
   legacy approval authority, and missing required fresh browser grants before
   forwarding.
4. Keep capability, Workspace filtering, revision, idempotency, warnings,
   domain invariants, and grant validation in the HTTP command service. Preserve
   tokenless loopback credential and device linking unchanged.
5. Expose `/mcp/full` as an explicit compatibility and diagnostic opt-in that
   retains every legacy typed tool name, plus the new catalog and bridges.
   Report implementation version, catalog version, profile, and static-list
   mode through redacted diagnostics.

## Contract, workflow, and documentation work

- Add the catalog/profile contract to `contracts/v1/commands.json` and a test
  tying the implementation's command allow-list and approval classification to
  its literal policies.
- Update the active MCP operations and command architecture documentation with
  exact endpoints, catalog behavior, metrics, and opt-in instructions.
- Update the Baley management skill to use the compact bridges naturally and
  remove its stale client-authored approval-attestation guidance.
- Record implementation and verification evidence here without registering
  Baley Records or changing Task/Run state; the coordinator owns those actions.

## Verification and safety

- Test exact compact/full counts and serialized schema bytes, exact compact
  names, all 78 legacy names, diagnostics, command classification, malformed
  and unknown rejection, fixed endpoint forwarding, and browser-grant routing.
- Run focused and full Go tests, `go vet`, frontend tests/build, and build the
  executable only under `C:\dev-bin\baley` for isolated smoke checks.
- Do not use `go run`, touch an operating database, alter firewall rules,
  create/remove worktrees, install from a dirty non-reproducible revision, or
  disrupt the currently running MCP process. Do not commit or push.
