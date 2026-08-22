---
baley_record: 1
record_id: "a9711717-18f8-4d74-993d-6360f2d5de00"
task_id: 140
task_key: "canvas-flow-tree-layout"
record_type: completion-report
created_at: "2026-08-22T04:58:55Z"
created_by: "codex"
registration_state: pending
supersedes_record_id: "4e8b8d74-c33d-4b12-bfc1-21ae701f72de"
---

# Task #140 completion report 03

## Outcome

The direct DAG edge model remains unchanged. This follow-up fixes the viewport boundary that made correctly rendered nodes appear missing and replaces Tree's public-ID row ordering with a custom parent-child locality layout; Tree does not use ELK.

On the first multi-Phase layout, the Viewer now focuses the active Phase entry. Collapsing a completed Phase focuses the active Phase entry and its incoming Gate; expanding a completed Phase focuses the expanded Phase exit and its outgoing Gate. The current zoom is preserved while the chosen anchor is positioned inside the visible canvas.

## Diagnosis

Development traces and live DOM inspection showed that React state, the React Flow store, and the rendered DOM already contained the expected nodes and direct edges. The first divergence was the React Flow viewport: it remained at the previous origin after layout geometry changed, leaving Phase 02 or G#1 outside the visible canvas.

## Delivered

- Added a reusable viewport-anchor calculation with validated dimensions, zoom, and clamped target fractions.
- Added Phase focus requests for initial active-Phase load, Phase collapse, and Phase expansion.
- Anchored entry Gates at 20% and exit Gates at 78% of the visible canvas width.
- Preserved the user's current zoom and synchronized React Flow controller, store, and rendered transform state.
- Retained development-only structured traces for the event, calculated target, React state, controller/store state, and rendered DOM geometry.
- Added focused unit coverage for viewport anchor placement and fraction clamping.
- Partitioned each Phase/Lane Tree into deterministic weakly connected component blocks.
- Reordered each depth through repeated parent/child barycenter sweeps and centered sparse layers inside their family block.
- Added Tree-only transitive reduction so a direct dependency is hidden when another visible Task path preserves the same reachability.
- Preserved all dependency data, the full Flow projection, Inspector direct relations, and every Gate relation.
- Recorded the durable layout, reduction, viewport, and invariant decisions in `docs/adr/0001-tree-dag-layout-and-visual-transitive-reduction.md`.

## Verification

- `npm run typecheck`: passed.
- `npm test`: passed, 16 files and 83 tests.
- Production Docker Viewer build: passed and deployed (existing Vite large-chunk warning only).
- Live DayTripper, Phase 01 collapsed then reloaded: 44 nodes; viewport `translate(-441px, 5.5px) scale(1)`; G#1 visible at x=273; active Phase 02 tasks #2, #7, and #30 visible at x=535.
- Live DayTripper, Phase 01 expanded: 55 nodes; viewport `translate(-1338.8px, 5.5px) scale(1)`; G#1 visible at x=1195, approximately 78% of the 1590px canvas width.
- Live DayTripper locality: #30 is centered at y=860 between #32/#33/#34 at y=726/860/994.
- Live DayTripper locality: #39 and #40 share y=1211; #41/#42 are centered around them at y=1144/1278.
- Live pre-fix DOM evidence confirmed #133?#136?#123 and #134?#136?#123 alongside the redundant #133/#134?#123 direct edges; the Tree projection now omits only the latter pair.

## Residual risk

Cross-Lane and direct Gate fan-out edges can still be long because Lane boundaries and the restored direct edge model are intentional invariants. Tree locality optimization is limited to Tasks sharing the same Phase and Lane.

No Workspace graph data was changed.
