---
baley_record: 1
record_id: "3fb52474-eb84-43bd-9865-7d60ee1588a1"
task_id: 116
task_key: "structural-typed-mcp"
record_type: independent-agent-review
run_id: "42623288-1a76-4289-8a84-02a2f622b91e"
created_at: "2026-07-22T00:26:48+09:00"
created_by: "independent-review-agent"
registration_state: pending
supersedes: null
---

# Structural Typed MCP Independent Review

## Final result

- High: 0
- Medium: 0
- Low: 0

The final review found no remaining defect in the Task #116 diff.

## Initial findings and disposition

1. Medium: active-Gate approval was not exercised through the new MCP tool. Resolved by an isolated PostgreSQL stdio E2E that creates an active-Phase fixture Task, verifies preview returns `gate:approve` and `human_approval_required`, verifies approval-less execute is rejected as `human_approval_mismatch`, and verifies execution with the fresh preview command hash and human actor.
2. Medium: structural Event checks only searched Event type strings. Resolved by binding every structural Event assertion to the exact command ID, entity ID, and Workspace revision.
3. Low: conditional approval forwarding included empty optional fields. Resolved with a shared attestation builder that omits absent optional fields and a unit assertion for omission.
4. Low: optional `initiatedByActorId` and attestation `approvedAt` were absent from the typed schema. Resolved by exposing both and asserting their optional schema boundary.

## Verification reviewed

- focused MCP handler and domain tests
- full Go test suite and vet
- isolated PostgreSQL MCP stdio E2E
- `git diff --check`

## Residual operational note

The real stdio/PostgreSQL E2E remains intentionally opt-in through `BALEY_MCP_E2E` and must use a reset disposable fixture database. The Task execution ran it explicitly and observed PASS; ordinary `go test ./...` skips that path when the environment is absent.
