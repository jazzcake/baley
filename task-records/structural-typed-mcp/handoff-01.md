---
baley_record: 1
record_id: "050f8a99-b6ca-49d9-8488-463d7572773d"
task_id: 116
task_key: "structural-typed-mcp"
record_type: handoff
run_id: "b157e804-d1cc-408c-b613-37e3852760e9"
created_at: "2026-07-22T00:26:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Adoption Structure Creation Handoff

## Resume boundary

Task #116 adds eight typed tools to the repository MCP server:

- `baley_phase_create_preview` / `baley_phase_create_execute`
- `baley_lane_create_preview` / `baley_lane_create_execute`
- `baley_gate_create_preview` / `baley_gate_create_execute`
- `baley_gate_attach_task_preview` / `baley_gate_attach_task_execute`

The current Codex thread loaded its MCP schema before these tools existed. Start the continuation only after the MCP server/schema is reloaded from the pushed source. Confirm all eight names are visible before any structural mutation. If they are absent, stop; do not substitute HTTP, SQL, fixture edits, or generic database writes.

## Typed input contracts

All preview tools require:

```text
expectedWorkspaceRevision: integer
idempotencyKey: string
executedByActorId: string
initiatedByActorId?: string
```

All execute tools add:

```text
acknowledgedWarningCodes?: string[]
proceedReason?: string
```

Command-specific fields:

```text
phase.create
  workspaceId: string
  phaseId: string
  name: string

lane.create
  workspaceId: string
  laneId: string
  name: string
  goal?: string
  summary?: string

gate.create
  workspaceId: string
  gateId: string
  name: string
  fromPhaseId: string
  toPhaseId: string

gate.attach_task
  workspaceId: string
  gateId: string
  taskId: integer
  clearTerminal?: boolean
```

`baley_gate_attach_task_execute` also exposes optional approval-attestation fields:

```text
approvedByActorId?: string
approvedCommandHash?: string
decisionSnapshotHash?: string
statementHash?: string
conversationRef?: string
approvedAt?: RFC 3339 timestamp
```

These fields are optional at schema level because a future Gate is an ordinary Operator mutation. When the Gate's `fromPhase` is active, preview returns `human_approval_required` and `requiredCapability=gate:approve`; execute then requires the exact fresh preview command hash and an explicit matching human approval. Approval-less active-Gate execute is rejected as `human_approval_mismatch`.

## Intended structural manifest

Use these stable IDs unless the continuation request explicitly supplies different agreed IDs:

```text
Lane
  id: adoption
  name: Adoption

Phases appended in order
  embedding-contract   / Embedding Contract
  embedding-enablement / Embedding Enablement
  embedding-pilot      / Embedding Pilot

Adjacent Gates required by V1
  embedding-contract-entry: Validate -> Embedding Contract
  embedding-enablement-entry: Embedding Contract -> Embedding Enablement
  embedding-pilot-entry: Embedding Enablement -> Embedding Pilot
```

V1 requires adjacent Phase Gates and permits only one outgoing Gate per Phase, so preview every Gate against the fresh graph before execute. The first Gate leaves the currently active Validate Phase. Attaching Task #116 or any other Validate Task to that active Gate is a human-only mutation. The other two Gates remain future Gates during initial construction, so their conditions can be attached by the Agent Operator.

## Exact continuation sequence

1. Read `workspace.get`, `workspace.graph`, Task #116, and `decision.list`. Preserve #114 and #115 confirmation as pending decisions.
2. Verify the eight new MCP tools are present after schema reload.
3. Preview and execute `lane.create` for `adoption`; refresh Workspace revision.
4. Preview and execute the three `phase.create` commands in Contract, Enablement, Pilot order, refreshing revision after each execute.
5. Preview and execute the three adjacent `gate.create` commands, refreshing revision after each execute.
6. Create every Task in the agreed Task manifest with the existing typed `baley_task_create_preview` / `baley_task_create_execute` tools. Preserve the agreed titles and descriptions exactly, assign `laneId=adoption`, assign the correct Embedding Phase, generate each `taskUuid` once, and create known predecessor/successor relationships atomically in `task.create` whenever possible.
7. Re-read the graph and map the returned public Task IDs to the agreed manifest. Do not guess public IDs before execute.
8. Attach each Gate condition only to a Task in that Gate's `fromPhase`. Use `clearTerminal=true` in the same command when an agreed condition Task currently has a terminal reason.
9. For the active Validate -> Embedding Contract Gate, issue a fresh `baley_gate_attach_task_preview`, show the target, revision, warnings, and command hash to the human, and stop until the human explicitly approves that exact attachment. Never reuse another approval or a stale preview.
10. After all permitted attachments, re-read Gate statuses and report Task IDs, command IDs, Event IDs, remaining human decisions, and any unconfigured/open Gate.

## Agreed Task manifest rule

The next thread must receive or recover the complete agreed Task manifest before Task creation. It must not invent missing titles, silently omit Tasks, or compress several agreed Tasks into one. Apply the manifest in Phase order and include cross-Task relationships in each `task.create` preview. If the manifest itself is absent, the structural Lane/Phase/Gate creation may proceed, but Task creation must pause for that manifest rather than guessing.

## Verification evidence

- Tool schema count increased from 31 to 39.
- Unit forwarding tests cover all eight handlers, initiator attribution, conditional approval forwarding, and omission of empty optional approval fields.
- Isolated PostgreSQL MCP stdio E2E covers write-free previews, execute, idempotent retry, command/entity/revision-bound Events, active-Gate approval preview, approval-less rejection, and approved execution.
- Full Go tests, vet, frontend 23/23 tests, production build, and diff check passed during Task #116.

## Authority boundary

- Do not confirm #114 or #115 without a fresh confirmation preview and explicit human approval.
- Do not confirm #116 automatically; `implemented` is the Agent's terminal state for this handoff.
- Do not attach a condition to the active Validate Gate without the exact fresh preview and explicit human approval.
