# Baley command usage patterns

Use [`contracts/v1/commands.json`](../../../../contracts/v1/commands.json) for exact command names, capability requirements and approval rules. Use [`contracts/v1/states.json`](../../../../contracts/v1/states.json) and [`contracts/v1/diagnostics.json`](../../../../contracts/v1/diagnostics.json) for exact state and diagnostic literals. This file only describes payload patterns that are easy to misuse.

## Relationship-aware Task creation

```text
task.create {
  laneId,
  phaseId,
  title,
  currentSummary,
  description: """
    Easy explanation
    ...

    Why it is needed
    ...

    What changes when it is complete
    ...

    Scope and exclusions
    ...
  """,
  parentTaskId?,
  predecessorTaskIds?: [],
  successorTaskIds?: [],
  terminalReason?
}
```

The Baley Skill requires `currentSummary` and all four description sections even
when the wire contract keeps those fields optional for compatibility. A content
change sends the complete human-facing set together:

```text
task.update {
  taskId,
  title,
  currentSummary,
  description: """
    Easy explanation
    ...

    Why it is needed
    ...

    What changes when it is complete
    ...

    Scope and exclusions
    ...
  """
}
```

A Task may have multiple predecessors and successors. Dependency edges may cross Lane and Phase boundaries inside one Workspace. They never become Gate conditions automatically.

For a Task added to an existing workstream, the Operator must identify an upstream Task or receive an explicit instruction that the Task starts an independent component. A disconnected component is domain-valid, but it is not an LLM default when context is missing.

## Lane Backlog intake and promotion

```text
backlog.create { workspaceId, backlogUuid, laneId, title, description? }
backlog.update { workspaceId, backlogPublicId, title?, description? }
backlog.move { workspaceId, backlogPublicId, targetLaneId }
backlog.reorder { workspaceId, laneId, orderedBacklogPublicIds }
backlog.discard { workspaceId, backlogPublicId, reason }
backlog.promote {
  workspaceId, backlogPublicId, taskUuid, phaseId,
  parentTaskId?, predecessorTaskIds?, successorTaskIds?, terminalReason?
}
```

Backlog creation is deliberately phase-free. `backlog.promote` requires an
explicit target Phase, copies lane/title/description, and reuses atomic
`task.create` dependency and warning semantics. It never changes a Gate. Because
promotion cannot set `currentSummary`, re-read the created Task and immediately
normalize its title, summary, and four-section description with `task.update`.
On stale revision, interruption, or a retryable failure, re-read, re-preview, and
retry idempotently. Do not start downstream work until normalization succeeds.

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
gate.create { gateId, alias?, name, fromPhaseId, toPhaseId }
gate.attach_task { gateId, taskId, clearTerminalReason? }
gate.pass_task { gateId, taskId, reason }
gate.revoke_task_pass { gateId, taskId, reason }
gate.pass { gateId }
```

Gate reference 필드는 내부 `gateId`, `G#<publicId>`, 선택적 alias를 받는다. 출력과
사람용 보고에서는 `G#<publicId>`를 우선 사용하고 내부 gateId를 감사·호환 식별자로
함께 유지한다.
정규형 `G#[1-9][0-9]*`은 public reference 전용이므로 내부 gateId로 생성하지 않는다.
존재하지 않는 public reference는 내부 ID나 alias로 fallback하지 않는다.

An attached Task must belong to the Gate's `fromPhase`. A Gate is ready only when every explicitly attached Task is confirmed or passed for that Gate. Cross-Phase Task dependencies do not alter this set.

## Gate entry and unlock projection

```text
gate.attach_entry_task { gateId, taskId }
gate.detach_entry_task { gateId, taskId }
```

An explicit entry Task must belong to the Gate's `toPhase`. Entry bindings do not start Tasks, change dependencies, or affect Gate readiness. When no explicit entry exists, Baley projects every same-Phase DAG root in the target Phase as an `automatic` entry ordered by Task public ID; automatic entries are never persisted. Entry bindings cannot be changed after the Gate passes.

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

Use `/v1/commands/preview` for a write-free evaluation and `/v1/commands/execute` for mutation. Bind each human approval attestation to the exact action, target, Workspace revision, command hash and optional decision snapshot hash. The attestation and command hash cannot be reused for another command.

One explicit human statement may approve a finite, enumerated set of `task.confirm` outcomes for Tasks that are already `implemented` and whose evidence has been re-checked. Before asking, retain a baseline preview for every target at the same Workspace revision. Execute the set sequentially with a fresh preview and a distinct attestation per command. The first fresh preview revision must equal the group baseline revision; each later fresh preview revision must equal the immediately preceding successful result revision. Every attestation must use the same non-empty `statementHash` and `conversationRef` to correlate the group. Ignore revision and command hash only after that progression check passes; request command/Workspace/Task, projected diff, capability, all errors/warnings/advisories, and optional decision snapshot must remain equivalent. Stop and ask again on any revision or comparison mismatch. Other human-only action types use separate decision briefs.

Human confirmation never bypasses Task lifecycle rules. A related `pending` or `in_progress` Task that the same implementation fully satisfies must first be reported `implemented` with a shared-evidence assessment. A Task made unnecessary rather than implemented must be proposed for `task.discard`, using a reason such as `superseded by #<id>` when applicable. Partial or uncertain pending/in-progress work stays open; an insufficient `implemented` Task returns through `task.rework`; terminal confirmed/discarded work requires a new follow-up Task.

The server validates each command-specific preview/execute binding. Group membership, baseline equivalence, and self-caused revision continuity are Operator protocol checks, not server-enforced atomic batch semantics.

When preview returns warnings, execute must send the exact warning-code set in `acknowledgedWarningCodes`; use `proceedReason` to preserve the Operator's reason in command Event evidence. Both belong to the envelope and are excluded from the canonical command hash.
