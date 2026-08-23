---
baley_record: 1
record_id: "a65b4c8a-41a3-4bbc-a3a1-a16acf92a0b9"
task_id: 147
task_key: "task-phase-move"
record_type: handoff
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #147 implementation handoff

Implement `Task Phase 이동 명령 및 관계 보존` in the Baley repository.

## Product decision

`task.move` is a routine Operator mutation. It moves one existing non-terminal Task to a different Phase **without replacing it**. The Task keeps the same public ID and stable ID, Lane, status, dependencies, Task Records, Runs, Git evidence, acceptance/audit history, and description. Only its Phase membership changes.

The immediate operational goal is to use the finished command to move pending Tasks #126 and #127 to Phase 06 (`multi-user-production`). Preserve their existing `#126 -> #127` dependency exactly. Task #128 is discarded and must not be recreated or moved.

## Required implementation sequence

1. Inspect the current command contract, mutation planner/registry, persistence transaction, event audit, HTTP, MCP, CLI, and Task/Gate models. Follow existing command patterns rather than introducing a direct SQL or transport-specific write.
2. Before broad changes, add focused tests/traces at the command boundary for requested Task, source Phase, target Phase, expected revision, calculated mutation plan, persistence result, and returned Event. Keep any useful diagnostics development/test-only.
3. Add `task.move` to the literal contracts, capability/command registry, application service, persistence switch, audit allowlists, CLI mapping/help, HTTP, and MCP adapter.
4. Enforce these invariants atomically:
   - Task and target Phase are in the same Workspace; target differs from source.
   - Terminal Tasks and Tasks with an active Run cannot move.
   - only `phase_id` changes; public/stable ID, Lane, status, dependency rows, records, Runs, and Git links remain untouched.
   - ordinary cross-Phase dependencies remain. Return post-move phase-order-inversion warnings instead of editing edges.
   - reject a move that violates a Gate condition Task's `fromPhase` or an explicit Gate entry Task's `toPhase`. Never auto-rebind/detach/pass a Gate.
   - preserve standard CAS and idempotency behavior; failures leave no partial state.
5. Emit an auditable `task.moved` Event carrying Task ID/public ID plus source and target Phase IDs.
6. Test success, no-op/missing target, stale revision, idempotent retry, terminal and active-Run rejection, unchanged identity/relationships/evidence, Gate-boundary rejection, and dependency-warning behavior. Cover HTTP, MCP, CLI, and persistence/audit paths.
7. Run relevant focused tests followed by the full test/build suite. Fix failures; do not mark implementation complete with known regressions.
8. Register the plan/handoff and completion records, then use the new public command to move #126 and #127 to `multi-user-production`. Report resulting Event IDs and implementation evidence for #147. Do not human-confirm any Task.

## Acceptance

- A preview and execute succeed for a valid same-Workspace move and emit `task.moved`.
- The Task remains the same Task in every durable relation; no clone/discard workaround exists.
- Unsafe gate/active-run moves and stale/invalid inputs fail atomically.
- The command is equally reachable through CLI, HTTP, and MCP.
- #126 and #127 are actually in Phase 06 after verification, with `#126 -> #127` preserved.
- Full tests/build pass, and #147 receives a truthful implemented report with residual-risk notes.

## Non-goals

No bulk move, cross-Workspace move, Lane move, automatic dependency rewiring, Gate rebinding, or Viewer editing UI.
