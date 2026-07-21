---
baley_record: 1
record_id: "dbcae066-e09a-41a2-9695-33438ed1a6f6"
task_id: 116
task_key: "structural-typed-mcp"
record_type: detailed-plan
run_id: "65d5e486-b617-496b-b623-d4fe39610918"
created_at: "2026-07-22T00:07:20+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Structural Baley Object Typed MCP Plan

## Objective

Expose `phase.create`, `lane.create`, `gate.create`, and `gate.attach_task` as explicit typed MCP preview/execute pairs so a later thread can construct the agreed Adoption Lane, Embedding Contract/Enablement/Pilot Phases, Gate, and Tasks without generic command payloads or database bypasses.

## Contract design

- Every preview and execute tool forwards the same command arguments to `/v1/commands/preview` or `/v1/commands/execute`.
- Preview requires Workspace revision, idempotency key, and executing Agent actor, and remains write-free.
- Execute supports exact warning acknowledgement and proceed reason.
- `gate.attach_task` execute additionally exposes human-approval attestation fields because attachment to an active Gate is conditionally human-only; future-Gate attachment remains an Operator action and omits those fields.
- Empty optional fields are omitted from the wire payload where absence is semantically distinct from an empty value.

## Implementation and verification

1. Add typed Go inputs, argument builders, MCP registrations, and preview/execute handlers.
2. Extend stdio MCP schema checks for all eight tools.
3. Extend the real PostgreSQL MCP integration path to prove write-free previews, execute/idempotent retry behavior, projections, and Event correlation for the four structural commands.
4. Run focused integration tests, the complete Go test suite, vet, frontend tests/build, and `git diff --check`.
5. Obtain an independent Agent review, address findings, and record the response.
6. Produce a handoff with exact tool schemas and an ordered construction recipe for the next thread.

## Safety and authority

- Do not execute Task confirmations for #114 or #115.
- Do not use an active-Gate attachment without a fresh preview and explicit matching human approval.
- Do not mutate the live Workspace with the new structural tools during implementation testing; integration tests use their isolated fixture database.
- Leave Task #116 at `implemented`; human confirmation remains a separate decision.
