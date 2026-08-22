# ADR 0001: Tree DAG layout and visual transitive reduction

- Status: Accepted
- Date: 2026-08-22
- Scope: Viewer Canvas, Task #140

## Context

The Viewer supports Flow and Tree modes over the same Workspace graph. Flow is useful for seeing the complete modeled graph, but the first Tree implementation also rendered every Task in deterministic depth/public-ID rows and drew every direct dependency. That produced two readability problems:

1. Related parents and children could be far apart because same-depth ordering ignored graph families.
2. A direct edge such as #133 -> #123 remained visible even when #133 -> #136 -> #123 already preserved the same reachability.

The Workspace graph is domain data and may intentionally contain both the direct dependency and the longer path. Improving the diagram must not silently rewrite that data.

## Decision

Tree mode uses a custom, locality-first layered DAG projection. It does not use ELK. Flow mode keeps its existing ELK-based layout.

Tree mode also applies visual transitive reduction to projected Task dependencies. A direct Task edge is omitted when another visible Task path from the same source to the same target exists. This is a rendering decision only.

| Concern | Flow | Tree |
| --- | --- | --- |
| Node placement | Existing ELK layered layout | Custom phase/lane-aware locality layout |
| Direct Task dependencies | Render all | Hide transitively redundant edges |
| Inspector dependency data | Show original data | Show original data |
| Gate relations | Render all | Render all; never transitively reduce |
| Workspace graph mutation | None | None |

## Tree placement algorithm

The Tree layout preserves Phase columns and Lane bands as hard boundaries.

1. Within each Phase, Tasks without a same-Phase predecessor start at depth 0.
2. Each descendant is assigned max(parent depth) + 1.
3. Within each Phase/Lane, Tasks are partitioned into deterministic weakly connected components.
4. Layers are repeatedly swept left-to-right and right-to-left using the average rank of parents or children.
5. Each sparse layer is centered inside its component block. This places a parent near the center of its child block while retaining a fixed non-overlap gap.
6. Components and tie cases remain deterministic by public Task number and stable ID.

Cross-Lane dependencies do not move Tasks across Lane boundaries. Long cross-Lane lines are preferable to violating Lane ownership.

## Visual transitive reduction

Reduction runs after Phase-collapse endpoint projection, so it evaluates the graph that is actually visible. For each projected Task edge u -> v, Tree mode omits that edge only when v remains reachable from u through a path of two or more other visible Task edges.

For example:

    #133 -> #136 -> #123
    #133 ----------> #123   hidden in Tree

The direct #133 -> #123 dependency remains in Workspace data and remains visible in Flow and the Task Inspector. Gate edges are built separately and are outside the reduction input.

## Viewport behavior

Layout geometry changes can move the active Phase or its Gate outside the old viewport. The Viewer therefore anchors:

- initial multi-Phase load at the active Phase entry;
- Phase collapse at the active Phase entry/incoming Gate;
- Phase expansion at the expanded Phase exit/outgoing Gate.

The current zoom is preserved. Entry and exit anchors are placed at 20% and 78% of the visible canvas width respectively.

## Consequences

### Positive

- Parent/child families read as compact DAG branches rather than public-ID lists.
- Redundant long diagonals are removed without changing domain data.
- Flow remains a complete direct-edge view.
- Inspector and Gate semantics stay authoritative and unchanged.

### Trade-offs

- Tree and Flow intentionally show different Task edge counts.
- Cross-Lane and Gate fan-out edges may still be long.
- The reduction assumes the domain dependency graph is acyclic, as required by Baley's dependency contract.

## Verification contract

- Parent and child component blocks must be deterministic and non-overlapping.
- A parent with multiple same-Lane children should be centered on their block where geometry permits.
- Tree removes u -> v only when another visible Task path preserves reachability.
- Flow renders the complete projected dependency set.
- Inspector direct relations and all Gate edges remain unchanged.
- Phase collapse/expand, search focus, Lane focus, and viewport controls must continue to work.

## Rejected alternatives

- **ELK for Tree:** rejected because its output did not consistently prioritize the product's parent/child locality requirement inside fixed Phase/Lane boundaries.
- **Delete redundant Workspace dependencies:** rejected because a direct dependency can carry domain intent even when reachability is redundant.
- **Reduce Gate relations:** rejected because Gate membership and Task dependency are different semantics.

