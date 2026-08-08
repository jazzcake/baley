---
name: baley-manage-work
description: Operate Baley tasks, lanes, phases, Gates, Runs, Task Records, and Git evidence through natural-language commands. Use when the user refers to Baley work such as #104, starts or completes work, creates dependencies, runs planning/implementation/review/reporting, registers repository records, or transitions a Gate. Keep the Baley web viewer read-only and do not bypass human confirmation actions.
---

# Manage Baley Work

Treat Baley as command-first and its web graph as read-only. A human or Agent may be an Operator; the LLM/Agent is the default Operator, not a separate domain authority.

## Workflow

1. Read `docs/baley-system-spec-v1.md` and `docs/baley-command-architecture.md` when the current thread lacks fresh Baley domain context.
2. Resolve Task references using numeric public IDs. Accept `#104`, `task #104`, `task 104`, `task104`, and `104번 task`.
3. Resolve Gate references using `G#<publicId>` first for human-facing instructions while accepting the stable internal `gateId` and optional alias. Keep all three in evidence when available. Treat canonical `G#[1-9][0-9]*` as a public-reference-only namespace: never create it as an internal `gateId`, and never fall back to an internal ID or alias when that public reference is absent.
4. Do not interpret a bare ambiguous number as a Task ID without contextual evidence.
5. When the user provides a Viewer URL shaped as `/workspaces/<uuid>`, use that UUID
   as the target Workspace. If the first typed MCP call returns
   `workspace_connection_required`, show its `approvalUrl` and ask the signed-in
   Workspace Owner to approve the one-time Operator connection. After approval,
   retry the same MCP call in the same thread. Do not request a Workspace-specific
   env file, raw token, MCP registration, or new thread. This connection grants
   Operator capability only and never supplies human approval authority.
6. Inspect the target Task, Lane, Phase, dependency, and Gate context before preparing a write command.
7. Select exactly one command from `contracts/v1/commands.json` when possible. Read `references/commands.md` for payload patterns. Prefer relationship-aware `task.create` or `dependency.patch` over a multi-command sequence that can partially succeed.
8. Validate obvious invariants before preview:
   - Task exists.
   - dependency does not create a cycle.
   - multi-edge rewrites use one atomic dependency patch and validate the final graph.
   - direct Task dependency stays in the Workspace; Lane and Phase boundaries are allowed.
   - for an additional Task in an existing workflow, the proposed predecessor or independent-root intent is known. If the LLM cannot establish either from context, do not preview or create the Task; present a candidate and ask the human whether it is independent or follows a specific Task.
   - a later-Phase to earlier-Phase dependency preserves `phase_order_inversion` as a warning.
   - dependency does not affect Gate readiness unless the Task is explicitly attached to that Gate.
   - a completed path either reaches a successor, joins the outgoing Gate, or has an intentional terminal reason.
   - a Task attached to a Gate belongs to the Gate's `fromPhase`.
   - every attached Gate Task is confirmed or explicitly passed for that Gate before transition.
   - Gate pass and Gate Task pass/revoke target the current active Phase's outgoing Gate.
   - only detailed-planning Runs start in a future inactive Phase.
   - the requested action does not exercise human-only authority without an explicit matching `humanApprovalAttestation`.
9. For routine Operator mutations, treat the user's clear request as authorization: prepare the concise preview internally, execute immediately, and report the result without asking a confirmation question. This includes Task and Backlog create/update/move/reorder, normal dependency changes, Run lifecycle, and Task Record registration. Ask for a human decision only for the human-only authority boundaries below.
10. Call the Baley MCP tool only when one is available and any required human approval has been obtained.
11. If no Baley command tool is available, stop after the preview. Do not patch fixtures, application source, or a database as a substitute.
12. Report the resulting Task IDs and Event IDs after execution.

## Read Requests

Answer read requests directly from an available Baley tool. Examples:

- Show Task `#104`.
- List pending or actionable Tasks in the Client Lane.
- Summarize blockers before the Pilot Ready Gate.
- Prepare a return brief for the Server Lane.

When no live Baley tool exists, state that only fixture or document context is available.

Treat multiple predecessor/successor edges and disconnected DAG components as valid domain shapes. Their validity does not authorize the LLM to introduce a new disconnected component without explicit user intent.

## Write Requests

Translate natural language into a typed command. For routine Operator writes, create the required preview internally and execute seamlessly; do not expose it as a blocking approval step. Keep the preview as an audit/checkpoint artifact and report the resulting IDs and Events after execution.

For lane Backlog intake, do not ask for or infer a Phase. Resolve only the target
lane and use `backlog.create`; if the lane cannot be established from context,
ask for it. Backlog items are phase-free planning intake, use `B#<publicId>`,
and do not have Task dependencies, Gate conditions, Runs, Records, blockers, or
Task lifecycle status.

When the user asks to promote a Backlog item into a Task, establish the explicit
target Phase plus predecessor/successor or intentional-root/leaf intent before
previewing `backlog.promote`. Promotion copies lane, title, and description from
the Backlog item and does not accept overrides. It atomically creates the pending
Task and relationships through the same `task.create` topology/warning rules,
then marks the Backlog item promoted. Exact preview warning acknowledgement is
required on execute. Promotion never attaches a Gate condition or changes a
Gate entry Task; that is a separate command and approval boundary.

Use `backlog.update`, `backlog.move`, `backlog.reorder`, and `backlog.discard`
for active items. Discard is an audited soft terminal transition and does not
create a Task terminal reason. A lane with active Backlog items must move,
promote, or discard them before lane termination.

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

When adding a Task after work already exists, establish its predecessor or obtain an explicit user statement that it is an independent root. If neither is available, keep the proposed row in chat or a planning document and ask for the missing context; do not create a standalone Task merely to make the graph valid.

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
- After reporting the primary Task implemented, inspect related open Tasks whose acceptance outcome may have been satisfied or made unnecessary by the same work. Classify each one before proposing any human decision:
  - already `implemented`: re-check its assessment and commit, test/build, and review evidence against the Task acceptance outcome; include it only when the evidence is still sufficient, otherwise use `task.rework` and record the remaining work;
  - `pending` or `in_progress` and fully satisfied by the same commit, tests, build, and review evidence: record an explicit shared-evidence assessment and report it `implemented` through the normal Agent workflow before asking for confirmation;
  - no longer needed or superseded rather than implemented: propose `task.discard` with the real reason such as `superseded by #<id>`, never confirmation;
  - partially satisfied or uncertain: keep it open and update only the truthful remaining context.
- Never use human confirmation to bypass `pending`/`in_progress` → `implemented`, missing evidence, or another invalid state transition.
- A related Task already `confirmed` or `discarded` is terminal in V1. Create a new follow-up Task when new work is required instead of rewriting its outcome.

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

For every human-only action, create the fresh preview before asking for approval, but present a human decision brief rather than a transport dump. Lead with what was delivered, how it was verified, independent review results when available, and any residual risk that could change the decision. Keep Workspace revision, command hash, capability, and snapshot hash as internal audit-binding data unless the human asks for them or a stale/mismatch error requires explanation.

The human answers in the same conversation. After an unambiguous approval, execute immediately with a chat attestation bound to the fresh preview. The server derives `approvedByActorId` from the human who connected the current Agent credential and rechecks that person's current Workspace role. Never ask the human to open a separate approval panel, paste command JSON, issue a grant, copy a token, or repeat the same decision in another channel.

For one `task.confirm`, use this concise pattern:

```text
#<id>은 <delivered outcome>, <test/build verification>, <independent review result>를 완료했습니다. 완료로 확인할까요?
```

When several Tasks are already `implemented`, create a write-free baseline `task.confirm` preview for every target at the same starting Workspace revision, then explicitly enumerate their outcomes in one grouped decision brief. V1 grouped approval is limited to this homogeneous `task.confirm` set. Keep different actions separate: confirmation and discard must not be hidden in the same generic “complete all” question. Treat an unambiguous reply such as “yes”, “confirm”, “네”, or “확인합니다” to the immediately preceding single or explicitly enumerated grouped decision brief as the human approval statement.

Execute a grouped approval as an LLM-controlled sequence, not as a server batch mutation. For every command in the approved set:

1. create a fresh preview just in time;
2. require the first fresh preview revision to equal the retained group baseline revision and every later fresh preview revision to equal the immediately preceding successful command's resulting revision; then compare action, target, projected diff, required capability, all errors/warnings/advisories, and optional decision snapshot with that target's baseline preview, ignoring only the expected revision and command hash;
3. execute with a new command-specific `humanApprovalAttestation` bound to that preview's exact revision, command hash, and optional snapshot hash;
4. use the same non-empty approval `statementHash` and `conversationRef` on every command-specific attestation for correlation, never a prior command hash or attestation;
5. fresh-read before the next iteration.

The expected revision increase caused solely by an earlier successful command in that approved sequence does not require another human question. Re-preview internally and continue only when the revision progression and comparison fields remain equivalent. Any mismatch before the first command or between later commands means external or unexpected state changed: stop and ask again even when the projected diff appears unchanged. Do not let the Agent downgrade a changed diagnostic as “not material” on the human's behalf. Report each resulting status and Event IDs, including any partial progress if a later iteration stops.

Do not present routine topology diagnostics as though they were implementation-quality failures. In particular, acknowledge `dangling_path` only as a warning when confirmation is otherwise approved; never invent or approve a terminal reason to suppress it. Surface the diagnostic in the decision brief only when it materially changes the human decision.

Apply the same outcome-first approach to human-only Gate, Lane, `task.discard`, and Workspace decisions: describe the real-world effect and decision evidence first, while preserving exact revision/hash/snapshot binding internally. `task.rework` is an Agent Operator action and does not require human approval. V1 has no persisted approval inbox.

The server enforces each command's current revision, canonical hash, target, warnings, command-specific chat attestation, and the current capability of the human bound to the Agent credential. The Skill/Operator enforces the grouped baseline comparison, revision chain, and shared human statement. Do not claim the server provides an atomic approval bundle or independently verifies conversation semantics.

Treat Viewer, Operator, Approver, and Owner as capability bundles for the future authenticated API. Never assume an Agent Operator has human approval capability.

## Tool Boundary

Treat the Skill as workflow and intent interpretation. Treat `docs/baley-system-spec-v1.md` as normative semantics and `contracts/v1/*.json` as literal authority. API, CLI, and MCP enforce those contracts at runtime. Never duplicate domain enforcement in the Skill.
