---
baley_record: 1
record_id: "ba01b5db-ad40-4c7b-8d5d-0e6b1e53cc34"
task_id: 140
task_key: "canvas-flow-tree-layout"
record_type: detailed-plan
run_id: "77a6b079-8bce-4173-84f3-4f32c28851e4"
created_at: "2026-08-22T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# Task #140 detailed implementation plan

## Outcome

Keep the present Lane-first Flow canvas as the default and add a Workspace-persisted `Tree` mode. Lanes remain explicit horizontal swimlanes in both modes; no Task may leave its Lane. Tree mode makes direct Task dependencies readable as compact parent/child groups inside each Lane, while Gate membership is rendered as a separate, aggregated relation rather than a long dependency-looking line.

## 1. Establish the layout-mode contract

- Add `LayoutMode = "flow" | "tree"` in the graph layout module.
- Store it at `baley:layout-mode:<workspaceId>`; hydrate it only after the Workspace ID is known. Invalid/missing values fall back to `flow`.
- Add an accessible two-option control to the canvas context actions: `Flow` and `Tree`. `Flow` remains the initial and fallback selection.
- Selecting a mode recomputes layout and fits only when the user explicitly requests a reset; changing mode must not silently reset a user’s zoom/pan.
- Development trace points must capture: toggle event, requested mode, persisted value, application state, calculated layout mode, React Flow node/edge counts, and rendered DOM counts. Retain these traces until both modes are verified.

## 2. Preserve the Flow implementation exactly

- Extend `layoutGraph` and its call sites with the mode, but keep the current per-phase/per-Lane ELK layered `RIGHT` options for `flow`.
- Preserve current Phase collapse projection, Backlog gutter, Lane label placement, Gate corridor, selection/search expansion, and lane/gate focused views.
- Add a regression assertion that Flow mode produces the same positions and phase/lane geometry as the pre-mode fixture for representative graph data.

## 3. Add Lane-constrained Tree layout

- Keep the existing Phase columns and Lane bands as hard geometry constraints.
- For each visible `(phase, lane)` task set, use ELK layered layout with direction `DOWN`; only dependencies whose endpoints are in that exact set participate in the local tree.
- Use deterministic sibling ordering (existing task order, then public ID) so refreshes do not reshuffle siblings.
- Let ELK place each parent above its direct children; vertically center the local tree in the Lane band. Increase that Lane’s height from the maximum local tree content height, as the current code already does for Flow content.
- A Task with no local parent is a local root. Cross-Lane dependencies must not reposition either endpoint out of its Lane; their route is handled separately.
- Phase width in Tree mode derives from local tree width rather than dependency depth, so wide dependency chains grow downward instead of forcing a wide Phase/canvas.

## 4. Separate dependency routing from Gate-condition routing

- Keep direct `dependencies` as Task-to-Task edges. Introduce edge metadata for mode, source/target Lane, and channel index.
- Route cross-Lane Task dependencies through reserved Lane-boundary channels. Use orthogonal/smooth-step segments that leave a node via a boundary port and never intentionally pass through an unrelated Task rectangle.
- Project Gate links into Lane-level collector nodes: one collector per `(gate, source lane, link kind)` with count and satisfaction state.
- Render `Task → collector` as a short tinted relation; render one collector-to-Gate trunk per group. This makes `G#2 required` visible without suggesting the Task has a direct workflow successor at the distant Gate.
- Keep direct Gate focus behavior: selecting a Task or Gate highlights only its collector branch/trunk; other Gate relations dim. The Inspector remains the authoritative list of all Gate conditions.
- Phase-collapse projection must map hidden Tasks to their Phase summary before collector aggregation, preserving one aggregate relation rather than restoring every hidden line.

## 5. Wire Viewer state and controls

- Pass `layoutMode` into the async layout effect and include it in its invalidation key so an old Flow result cannot commit after a Tree selection.
- Keep the existing requested-vs-rendered collapse presentation guard; apply the same commit boundary to layout mode to prevent origin flashes during mode changes.
- Supply explicit source/target positions and edge types appropriate to each mode. Ensure nodes are still non-draggable and viewport/search/Inspector navigation uses the active layout positions.
- Add concise visual treatment for the mode control and collector chips without competing with Lane labels or Phase headings.

## 6. Verification

- Unit-test local Tree layout: Lane containment, parent-above-child ordering, deterministic sibling ordering, compact Phase width, and unchanged Flow geometry.
- Unit-test cross-Lane channel selection and reject routes whose segments intersect unrelated node rectangles in synthetic multi-Lane fixtures.
- Unit-test Gate collector aggregation: multiple same-Lane required links become one trunk with the right count; different Lane/kind groups remain distinct; collapsed summaries remain valid endpoints.
- Add Viewer tests for mode persistence per Workspace, accessible toggle state, search/selected hidden Task expansion, and switching modes while a Phase is collapsed.
- Run `npm run typecheck`, `npm test`, and `npm run build`.
- Manual DayTripper verification: inspect the `#30 → {#34,#32,#33}` subtree and `#39 → #40 → {#41,#42}` subtree in Tree mode; verify Lane boundaries remain clear and no Gate condition line traverses unrelated cards.

## Non-goals

- Do not alter Task dependencies, Gate conditions, Phase membership, Lane membership, or server-side graph semantics.
- Do not make Tree the default, add manual node dragging, or treat Gate condition links as workflow dependencies.
