---
baley_record: 1
record_id: "ad8ca92a-9285-4972-85b2-97b28d75627e"
task_id: 147
task_key: "task-phase-move"
record_type: detailed-plan
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #147 detailed implementation plan

## Outcome

Add the routine Operator command `task.move`. It moves one non-terminal Task to another Phase in the same Workspace without creating a replacement Task or discarding the original. Its public ID, stable ID, Lane, status, description, dependencies, Task Records, Runs, Git evidence, acceptance state, and audit history remain attached to that one Task.

After the command is verified, move Task #126 and Task #127 from the Adoption Phase to Phase 06 (`multi-user-production`). Their existing `#126 -> #127` relationship must remain intact.

## 1. Establish the command contract

- Add `task.move` to `contracts/v1/commands.json`, capability definitions, CLI argument mapping, mutation-plan registry, HTTP command surface, and the MCP adapter.
- Accept a public Task reference and a target Phase reference/ID, plus the standard expected Workspace revision and idempotency envelope. Keep the command operator-only; it is not a human confirmation or a Gate decision.
- Add typed preview and execute payloads. Preview must expose source and target Phase, the Task public ID, and any diagnostics before a write occurs.
- Update command documentation and generated/help surfaces that enumerate supported command names.

## 2. Define and enforce domain invariants

- Resolve Task and target Phase inside the same Workspace under the existing Workspace transaction/lock and revision CAS rules.
- Reject a missing target, a no-op target, or terminal Task. Reject moving a Task that has an active Run so work context cannot silently change beneath an executing Run.
- Change only `tasks.phase_id`. Do not reallocate a public ID, clone a Task, create a new Task Record, rewrite a Run, or mutate its Lane/status/description/dependency rows.
- Preserve normal cross-Phase dependencies. Recompute and return `phase_order_inversion` warnings from the post-move graph rather than silently altering edges.
- Protect Gate invariants: reject a move that would leave a Gate condition outside that Gate's `fromPhase`, or an explicit Gate entry Task outside the Gate's `toPhase`. Do not auto-detach, auto-attach, pass, or rebind a Gate.
- Keep all rejection paths atomic: no Task row, Event, or Workspace revision changes on failure.

## 3. Persist an auditable move

- Implement the domain planner and persistence write using the established mutation-plan pattern.
- Emit a dedicated `task.moved` Event in the same transaction. Its audited payload must include Task ID/public ID, source Phase ID, and target Phase ID; update event-audit allowlists and visibility mappings.
- Ensure retries with the same idempotency key return the original successful result, while stale revisions and conflicting retries retain existing error semantics.
- Keep the command compatible with existing record registration and commit attachment: both pre-existing and future records must resolve to the same Task.

## 4. Expose the command consistently

- Add the HTTP command route/schema through the generic command service, MCP tool exposure, and CLI parsing/help so each transport reaches the same application command.
- Verify payload naming and public-reference resolution match existing Task mutation conventions; do not introduce a transport-only alternate mutation path.
- Provide a concise CLI example/documentation showing a preview followed by execute through the normal command envelope.

## 5. Tests

- Add domain/unit coverage for a successful move, source/target validation, no-op rejection, terminal Task rejection, active-Run rejection, stale revision, idempotent retry, and unchanged Task identity/Lane/status/records/Runs/dependencies.
- Add Gate-boundary cases for both attached condition Tasks and explicit entry Tasks; each invalid move must reject without partial mutation.
- Add dependency cases that demonstrate preservation and report a post-move phase-order inversion warning when applicable.
- Add integration coverage for HTTP, MCP, and CLI paths, including Event audit payload correctness and repository persistence.
- Run focused Go tests, the full Go test suite, and any Viewer/typecheck/build checks affected by command-contract generation or API typing.

## 6. Operational migration

- Start and maintain a Baley implementation Run for #147. Register this detailed plan and the handoff record with their repository hashes.
- After all acceptance tests pass, use the new command (not a direct database edit) to move #126 and #127 into `multi-user-production`, preserving `#126 -> #127`.
- Record the resulting Event IDs and any phase-order diagnostic. Produce a completion report and report #147 as implemented; do not human-confirm it.

## Non-goals

- No bulk move, cross-Workspace move, Lane move, automatic dependency rewiring, Gate rebinding, Task cloning, or direct Viewer mutation UI is part of this Task.
- Do not use a discard/recreate workaround for #126 or #127.
