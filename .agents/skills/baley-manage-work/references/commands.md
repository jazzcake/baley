# Baley command usage patterns

Use [`contracts/v1/commands.json`](../../../../contracts/v1/commands.json) for exact command names, capability requirements and approval rules. Use [`contracts/v1/states.json`](../../../../contracts/v1/states.json) and [`contracts/v1/diagnostics.json`](../../../../contracts/v1/diagnostics.json) for exact state and diagnostic literals. This file only describes payload patterns that are easy to misuse.

## Relationship-aware Task creation

```text
task.create {
  laneId,
  phaseId,
  title,
  parentTaskId?,
  predecessorTaskIds?: [],
  successorTaskIds?: [],
  terminalReason?
}
```

A Task may have multiple predecessors and successors. Dependency edges may cross Lane and Phase boundaries inside one Workspace. They never become Gate conditions automatically.

## Atomic dependency rewrite

```text
dependency.patch {
  remove: [{ fromTaskId, toTaskId }],
  add: [{ fromTaskId, toTaskId }],
  terminalUpdates?: [{ taskId, terminalReason: string | null }]
}
```

Validate the final Workspace graph. Reject self-links, duplicates, cross-Workspace links and cycles. Preserve `phase_order_inversion` when an edge goes from a later Phase to an earlier Phase. A path without an outgoing Task dependency or explicit Gate condition needs an intentional terminal reason or retains `dangling_path`.

## Gate condition

```text
gate.attach_task { gateId, taskId, clearTerminalReason? }
gate.pass_task { gateId, taskId, reason }
gate.revoke_task_pass { gateId, taskId, reason }
gate.pass { gateId }
```

An attached Task must belong to the Gate's `fromPhase`. A Gate is ready only when every explicitly attached Task is confirmed or passed for that Gate. Cross-Phase Task dependencies do not alter this set.

## Automatic Run and Record lifecycle

```text
run.start { clientRunId, taskId, kind, parentRunId?, targetRunId?, sessionRef? }
record.register { recordId, taskId, runId?, recordType, repositoryId, relativePath, workingTreeHash?, shortSummary }
```

Generate client IDs once and reuse them for retries. Keep Runs alive with heartbeat and finish them automatically. Task Record bodies stay in the repository; send only relative path, hash, summary and optional commit/blob metadata.

## Mutation envelope

```text
{
  idempotencyKey,
  expectedWorkspaceRevision?,
  initiatedByActorId?,
  executedByActorId,
  acknowledgedWarningCodes?: [],
  proceedReason?,
  humanApprovalAttestation?
}
```

Use `/v1/commands/preview` for a write-free evaluation and `/v1/commands/execute` for mutation. Bind a human approval attestation to the exact action, target, Workspace revision, command hash and optional decision snapshot hash. It is not a persisted approval request and cannot be reused for another command.

When preview returns warnings, execute must send the exact warning-code set in `acknowledgedWarningCodes`; use `proceedReason` to preserve the Operator's reason in command Event evidence. Both belong to the envelope and are excluded from the canonical command hash.
