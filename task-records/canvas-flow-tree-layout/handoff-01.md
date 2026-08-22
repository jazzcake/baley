---
baley_record: 1
record_id: "62e71b0c-3b8c-4526-92a9-bd12d44a1e61"
task_id: 140
task_key: "canvas-flow-tree-layout"
record_type: handoff
run_id: "77a6b079-8bce-4173-84f3-4f32c28851e4"
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #140 implementation handoff

Implement `Canvas Flow·Tree 레이아웃 전환` in the Baley Viewer.

The product decision is non-negotiable: **Lane is a first-class invariant in both modes.** Lanes must remain explicit swimlanes, every Task remains inside its own Lane, and Tree is only a compact dependency arrangement inside that structure. Do not replace Lane layout with a free-form global tree.

## Current architecture

- `src/graph/layout.ts` performs an async per-phase/per-Lane ELK layered layout. It currently uses `elk.direction = RIGHT`; Phase columns are arranged left-to-right and `lanePositions`/`laneHeights` produce horizontal Lane bands.
- `src/graph/projection.ts` owns collapsed-Phase summaries and dependency endpoint projection.
- `src/App.tsx` owns `collapsedPhaseIds`, guards requested vs rendered collapse layouts, calls `layoutGraph`, builds React Flow nodes/edges, and currently renders Gate links as direct brown/gray/green edges.
- `src/components/PhaseSummaryNode.tsx`, `LaneAnchorColumn.tsx`, and `BacklogRail.tsx` consume `GraphLayout` geometry.

## Required implementation sequence

1. Add a `LayoutMode` type and a Workspace-scoped localStorage helper. Default safely to `flow`.
2. Instrument mode selection and the async layout commit path before changing geometry. Log the event, requested/persisted mode, React state, ELK result summary, React Flow store state, and DOM counts in development only.
3. Thread the mode through `layoutGraph`. Keep the current Flow code path behaviorally identical.
4. Implement a Tree code path that retains phase columns and Lane bands but runs local same-Lane graphs with ELK `DOWN`. Preserve deterministic ordering. Expand Lane height from local content; do not move cards to a different Lane.
5. Add a graph projection for Gate collectors and a compact collector node type. Group by gate + source Lane + relation kind, split direct task-to-collector stubs from one collector-to-Gate trunk, and retain selection/focus dimming.
6. Add a channel-aware edge builder for cross-Lane dependencies. Prefer deterministic orthogonal paths/channels; never claim a line is a direct dependency when it represents Gate membership.
7. Make the mode switch resilient to collapse transitions and search selection. An async Flow result must not overwrite a newer Tree result.
8. Add tests before styling polish, then run the full Viewer checks and manually validate the DayTripper graph.

## Acceptance checks

- Flow remains default and visually unchanged.
- Tree shows `#30` above/centered over `#34`, `#32`, and `#33` when those cards share a Lane/Phase; `#39 → #40 → {#41,#42}` is similarly legible.
- Lanes stay obvious in Tree mode and no Task crosses a Lane boundary.
- G# condition relations are distinguishable from normal dependencies and are bundled rather than long canvas-crossing lines.
- Collapsed Phase summaries, selection, search, Gate focus, Backlog rail, and viewport controls still work.
- Typecheck, tests, and production build pass.

Do not change DayTripper graph data to make the layout look better. This is a Viewer-only projection and routing task.
