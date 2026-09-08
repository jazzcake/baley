---
type: adr
status: accepted
authority: normative
last_active: 2026-09-05
affects:
  - docs/baley-system-spec-v1.md
  - docs/baley-command-architecture.md
  - contracts/v1/commands.json
  - contracts/v1/capabilities.json
  - contracts/v1/diagnostics.json
  - contracts/v1/states.json
---

# ADR 0003: Account-private Bird View planning graph

## Decision

Bird View is an account-private planning and understanding layer above Workspaces. It is not a Workspace child and it does not relax any Workspace command, Event, revision, ownership, Task dependency, Phase, or Gate invariant.

A Bird View is its own aggregate with an independent monotonically increasing revision, idempotent command ledger, and append-only Event ledger. Its Nodes are freely authored outcome, landmark, or concept clusters. A Node may exist without a Workspace, Phase, Gate, Task, or Backlog item. Baley does not introduce a Milestone entity: operational achievement remains represented by Gate pass Events, while a Bird View Node's `achieved` state is planning-layer annotation only.

Bird View Node edges are free-form, directed big-picture relationships. Self edges and duplicate directed endpoint pairs are rejected; cycles are allowed. They never become Task dependencies. Task dependencies remain strict, acyclic, Workspace-local relations.

## Overlay bindings

Node bindings are references only. Separate Phase, Gate, Task, and Backlog binding tables retain target type integrity and a state of `suggested`, `pinned`, or `excluded`. A binding never changes target ownership, lifecycle, status, dependency, Gate readiness, or Phase behavior and never creates Tasks.

`bird_view.overlay.replace` replaces only the Node's suggested bindings. Existing pinned and excluded bindings are preserved. `bird_view.binding.pin` and `bird_view.binding.exclude` upsert an explicit state. The command service validates every target against the calling account's current active membership before writing.

The Viewer omits excluded bindings. `bird_view.node.context`, intended for the existing MCP/LLM flow, may return accessible excluded references so an Operator does not suggest them again. Both surfaces suppress every binding whose Workspace membership is inactive or whose Workspace is no longer active. They do not return hidden Workspace IDs, entity IDs, counts, or existence diagnostics.

## Authorization

The owning `account_id` is the privacy boundary. Human browser sessions use their authenticated Account. MCP Agent credentials may access Bird View only when the token can be traced to the active human Account that connected or issued it; the token never derives human approval authority.

`bird_view:read` permits account-private queries and `bird_view:operate` permits ordinary planning mutations. Both are Agent-safe. Workspace overlay visibility additionally requires an active membership for the owning Account on every read and mutation. Revocation therefore removes the Workspace lane and its entities on the next statement/request without deleting the account-private Bird View graph.

Missing, inaccessible, and revoked Bird Views or overlay targets all produce the same `not_found` response at the transport boundary.

## Command and query contract

Queries:

- `bird_view.list`
- `bird_view.get`
- `bird_view.graph`
- `bird_view.node.focus`
- `bird_view.node.context`

Mutations:

- `bird_view.create`, `bird_view.update`, `bird_view.archive`
- `bird_view.node.create`, `bird_view.node.update`, `bird_view.node.delete`, `bird_view.node.achieve`, `bird_view.node.park`
- `bird_view.edge.connect`, `bird_view.edge.update`, `bird_view.edge.disconnect`
- `bird_view.binding.pin`, `bird_view.binding.exclude`
- `bird_view.overlay.replace`

Bird View mutations use the normal `idempotencyKey`, actor provenance, and preview/execute transport. Create has no expected revision. Every mutation of an existing Bird View requires `expectedBirdViewRevision`; stale values fail with `stale_bird_view_revision`. A successful mutation, its Event, revision increment, and command result commit atomically.

Archiving is soft and audited. Default list excludes archived views. An archived view remains directly readable by its owner but rejects further mutation.

## V2 canvas and focus projection

The original read-only Viewer and its separate focus route are rejected and superseded. The first level is a large, free-form canvas containing only Bird View Nodes and Bird View edges. A human may create, edit, drag, connect, relabel, and delete graph elements through the same durable command boundary exposed to MCP clients. Node coordinates are aggregate state and are retained across reloads.

Single click selects a Node for editing. Double-click enters focus without navigating away: the selected Node becomes the upper-left context header and the Baley Phase/Gate/Task Workspace graph is rendered beneath it. The lower graph preserves pan, zoom, selection, and the existing server-authored automatic layout; it adds safe human Task Inspector controls for `task.update` (title, description, current summary) and `task.move` (Phase membership) through the same preview/execute boundary used by MCP. Task nodes do not gain persisted XY coordinates: moving a Task means a durable Phase move, while visual placement remains automatic. Membership capability, active-Workspace, and terminal-Task guards remain authoritative, and failed mutations discard their stale preview without optimistically desynchronizing the graph. The first accessible bound Workspace is the current lower graph; additional Workspace bindings are compact external references only. Leaving focus remounts the Bird View canvas at its saved pan and zoom.

Semantic zoom has two levels only where it clarifies topology: overview keeps Node identity and status visible, while detail adds summaries and edge labels. Focus reuses the existing Workspace graph rather than maintaining a second layout or interaction system.

## Rejected alternatives

- Reusing the removed `feature/milestone-bird-view` implementation or either old clone database: prohibited because its Milestone-centered model conflicts with this contract.
- Making Bird View a Workspace child: prevents cross-Workspace understanding and couples its revision to unrelated Task operations.
- Storing model calls in the server: rejected; recommendation remains an MCP/LLM workflow over `bird_view.node.context` plus `bird_view.overlay.replace`.
- Translating Bird View edges into Task dependencies: rejected because it would weaken Workspace-local DAG invariants.
- A read-only dashboard, bespoke horizontal Workspace cards, or a separate focus route: rejected because humans and LLMs must share one editable graph model and the lower level must preserve the existing Baley graph experience.
