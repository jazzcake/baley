---
baley_record: 1
record_id: "d79be8aa-bcd1-4edc-a3d0-4f0d89761e47"
task_id: 186
record_type: detailed-plan
run_id: "8e932014-ae7d-4a1a-ae7c-c02011eceef5"
created_at: "2026-09-10T14:38:06+09:00"
created_by: "codex"
supersedes: "c048f896-6933-48bc-8853-4cf64ce3b3eb"
status: implemented
---

# Task #186 detailed plan — conversational confirmation and ordinary DAG leaves

## Decision

Adopt a conservative linked-account conversational-attestation path for ordinary `task.confirm`. The authenticated MCP Agent may transmit an explicit current-conversation human decision, but it cannot nominate an approver. Baley derives the initiating human from the active MCP gateway link, revalidates that member's current `task:approve` capability, and records the Agent separately as executor.

The compatibility-stable compact tool remains `baley_command_execute_with_approval`. Its `task.confirm` branch accepts typed `decisionEvidence`; other human-only commands retain their applicable browser-session grant. The compact catalog remains exactly 15 tools.

## Contract and safeguards

Each decision binds a unique decision ID, conversation source/reference, explicit statement, exact-Task or all-awaiting scope, action, Task target, Workspace revision, canonical command hash, idempotency-key hash, linked Account/human Actor, gateway registration, and executing Agent. Evidence is consumed transactionally and cannot be replayed or reused across targets. Missing, vague, negated, malformed, stale, unauthorized, replayed, and cross-target evidence fails closed.

The server audits a typed declaration of the conversational statement; it does not independently authenticate the chat transcript's semantics. This is the narrowest trust boundary that permits seamless command-first confirmation without Viewer mutation clicks.

## Implementation sequence

1. Extend authenticated Agent provenance and the command envelope.
2. Validate evidence in application and persistence layers, then persist evidence and attestation linkage in migration 28.
3. Extend compact and full MCP while preserving the existing compact tool name/count.
4. Remove TaskConfirmation mutation UI and retain read-first inspection.
5. Remove `dangling_path` production and contract semantics while preserving graph conflicts and invariants.
6. Update contracts, skill, product/spec/architecture/operations documents, schema expectations, rollout checks, and Task Records.
7. Run focused and full Go, vet, rollout-script, Viewer, production-build, and executable-build verification; inspect and commit only Task #186 changes.

## Non-goals and safety

- No arbitrary `approvedByActorId` authority.
- No weakening of Gate, cycle, relationship, revision, idempotency, warning, or capability checks.
- No operating worktree/database/service, firewall, deployment, merge, push, or live Task/Run mutation.
- `terminalReason` remains optional descriptive metadata and `terminal_path_conflict` remains enforced.
