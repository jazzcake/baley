---
baley_record: 1
record_id: "bb688b5e-864d-4f51-890b-f5e9789c9865"
task_id: 195
record_type: detailed-plan
run_id: "7a1839c0-f0b1-4c24-ab64-98a33eac505f"
created_at: "2026-09-18T14:22:41+09:00"
created_by: "codex"
supersedes: null
status: completed
---

# Task #195 detailed plan — compound conversational Task confirmation

## Decision

Restore the seamless Task-confirmation goal without weakening the human-only boundary. An explicit prompt may contain several numbered Task decisions. For each `task.confirm`, Baley must verify that the exact target belongs to a confirmation group in the verbatim statement.

`decisionId` is a caller-generated, target-unique UUID for one evidence row. It is not a Codex thread, turn, or message identifier. `conversationRef` is a stable, non-secret, opaque audit reference; Baley records it but does not authenticate it against an external conversation provider.

## Implementation

1. Reproduce the reported statement and current `all_awaiting_confirmation` failure.
2. Add a closed comma-delimited grammar that assigns every listed `#<id>` to an explicit confirmation or discard action.
3. Accept `task.confirm` only when the current target is assigned to a confirmation action and the evidence uses `task` scope.
4. Preserve fail-closed behavior for unassigned, duplicate, unknown-action, question, conditional, wrong-target, and all-scope mismatches.
5. Add a field-level mismatch reason to the existing `decision_evidence_mismatch` response.
6. Align literal contracts, architecture/spec documentation, and the Baley work-management skill.

## Exclusions

No database migration, live Task confirmation/discard, service deployment, approval-capability relaxation, or external conversation-provider integration is included.
