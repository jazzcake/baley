---
name: baley-manage-work
description: Operate Baley tasks, lanes, phases, Gates, Runs, Task Records, and Git evidence through natural-language commands. Use when the user refers to Baley work such as #104, starts or completes work, creates dependencies, runs planning/implementation/review/reporting, registers repository records, or transitions a Gate. Keep the Baley web viewer read-only and do not bypass human confirmation actions.
---

# Manage Baley Work

Treat Baley as command-first and its web graph as read-only. A human or Agent may be an Operator; the LLM/Agent is the default Operator, not a separate domain authority.

## Workflow

1. Read `docs/baley-system-spec-v1.md` and `docs/baley-command-architecture.md` when the current thread lacks fresh Baley domain context.
2. Resolve Task references using numeric public IDs. Accept `#104`, `task #104`, `task 104`, `task104`, and `104번 task`.
3. Do not interpret a bare ambiguous number as a Task ID without contextual evidence.
4. Inspect the target Task, Lane, Phase, dependency, and Gate context before preparing a write command.
5. Select exactly one command from `contracts/v1/commands.json` when possible. Read `references/commands.md` for payload patterns. Prefer relationship-aware `task.create` or `dependency.patch` over a multi-command sequence that can partially succeed.
6. Validate obvious invariants before preview:
   - Task exists.
   - dependency does not create a cycle.
   - multi-edge rewrites use one atomic dependency patch and validate the final graph.
   - direct Task dependency stays in the Workspace; Lane and Phase boundaries are allowed.
   - a later-Phase to earlier-Phase dependency preserves `phase_order_inversion` as a warning.
   - dependency does not affect Gate readiness unless the Task is explicitly attached to that Gate.
   - a completed path either reaches a successor, joins the outgoing Gate, or has an intentional terminal reason.
   - a Task attached to a Gate belongs to the Gate's `fromPhase`.
   - every attached Gate Task is confirmed or explicitly passed for that Gate before transition.
   - Gate pass and Gate Task pass/revoke target the current active Phase's outgoing Gate.
   - only detailed-planning Runs start in a future inactive Phase.
   - the requested action does not exercise human-only authority without an explicit matching `humanApprovalAttestation`.
7. Show a concise preview for user-requested structural changes. Run lifecycle and Task Record registration happen automatically without repeated confirmation.
8. Call the Baley MCP tool only when one is available and any required human approval has been obtained.
9. If no Baley command tool is available, stop after the preview. Do not patch fixtures, application source, or a database as a substitute.
10. Report the resulting Task IDs and Event IDs after execution.

## Read Requests

Answer read requests directly from an available Baley tool. Examples:

- Show Task `#104`.
- List pending or actionable Tasks in the Client Lane.
- Summarize blockers before the Pilot Ready Gate.
- Prepare a return brief for the Server Lane.

When no live Baley tool exists, state that only fixture or document context is available.

Treat multiple predecessor/successor edges and disconnected DAG components as valid. Do not infer that a DAG must be one connected graph.

## Write Requests

Translate natural language into a typed command and preview it.

Example:

```text
User: task104 뒤에 API 검증 추가해

Command:
task.create {
  title: "API 검증",
  predecessorTaskIds: [104]
}

Preview:
- #104 뒤에 “API 검증” Task 생성
- #104 → 새 Task dependency 생성
```

Do not invent the new public Task ID before execution.

Use `dependency.patch` for edge reversal or any rewrite that removes and adds edges together. Include terminal-reason changes in that same patch when the path shape changes. Never disconnect first and hope a later connect succeeds. If a path has no successor or Gate condition, either add the intended continuation or record an intentional leaf reason; otherwise preserve the `dangling_path` warning.

## Automatic Workflow

- Start a Run before detailed planning, implementation, independent Agent review, review response, or completion reporting.
- If the Task is pending, start it when its first work Run begins.
- Update Run status automatically on success, failure, cancellation, or interruption.
- Keep long-running Runs alive with heartbeat and retry start/terminal updates with the same client Run ID and idempotency key.
- Generate one client Record UUID, write it into the Task Record front matter, and send the same ID when registering the relative path, hash, summary, and later commit/blob metadata with Baley.
- Do not include the entire Task Record directory in general repository search. Read only exact paths returned for the current Task.
- Move durable knowledge from Task Records into normal project documentation when the repository workflow calls for it. This is ordinary LLM repository work, not a Baley command, state, or Event.
- Report implementation completion with an assessment, residual risks, and optional completion-report reference. Do not claim Baley verified semantic quality.

## Authority

- Allow the implementing Agent to start work, manage Runs, create records, and report a Task implemented.
- Do not confirm or discard a Task without explicit human approval.
- Do not close out or discard a Lane without explicit human approval.
- Do not attach a new condition to an active Gate without explicit human approval.
- Never detach a condition from an active Gate; use Gate Task pass so the waived condition remains visible.
- Do not pass a Gate without explicit authorized approval.
- Do not pass an attached Gate Task without explicit human approval and a reason.
- Do not revoke an attached Gate Task pass without explicit human approval and a reason.
- Do not close a Workspace without explicit approval from the human Owner.
- Distinguish the human initiator and approver from the Agent that executes the command.
- Lane Group, Lane fork, Branch, and worktree lifecycle management are outside V1.

When Task status is `implemented` or Gate status is `ready`, report the derived human decision with its target, expected Workspace revision, warnings, and snapshot hash, then stop before mutation. Resume only with a matching `humanApprovalAttestation`. V1 has no persisted approval inbox.

Treat Viewer, Operator, Approver, and Owner as capability bundles for the future authenticated API. Never assume an Agent Operator has human approval capability.

## Tool Boundary

Treat the Skill as workflow and intent interpretation. Treat `docs/baley-system-spec-v1.md` as normative semantics and `contracts/v1/*.json` as literal authority. API, CLI, and MCP enforce those contracts at runtime. Never duplicate domain enforcement in the Skill.
