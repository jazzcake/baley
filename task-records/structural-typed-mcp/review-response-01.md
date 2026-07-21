---
baley_record: 1
record_id: "537cd2b5-a876-4188-baa4-90f373975d1b"
task_id: 116
task_key: "structural-typed-mcp"
record_type: review-response
run_id: "bb5f9e29-cf12-497d-981b-150554088b1b"
created_at: "2026-07-22T00:27:03+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Structural Typed MCP Review Response

## Response

All two Medium and two Low findings from the initial independent review were accepted and resolved.

## Changes made

- Added active-Gate MCP E2E coverage for fresh approval preview, approval-less rejection, and matching approved execution.
- Replaced loose Event-type searches with command/entity/revision correlation for each structural mutation and the active attachment.
- Added absence-sensitive approval-attestation serialization.
- Added optional initiator attribution to preview/execute envelopes and optional approval timestamp to conditional attachment schema.
- Extended schema and forwarding assertions for the new fields.

## Re-verification

- `go test -count=1 ./...`: PASS
- `go vet ./...`: PASS
- isolated `BALEY_MCP_E2E=1` PostgreSQL stdio test: PASS
- final independent review: High 0, Medium 0, Low 0
