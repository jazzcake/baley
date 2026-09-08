---
baley_record: 1
record_id: "17630000-0000-4000-8000-000000000176"
task_id: 176
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-06T08:26:38Z"
created_by: "codex"
registration_state: pending
---

# Task #176 detailed implementation plan

## Baseline evidence

The upper Bird View and lower Workspace graph both currently use React Flow's default dot `Background` on the same computed `rgb(248, 249, 252)` canvas color. The upper SVG pattern is a single dot at a 28 px gap (`patternTransform=translate(-15,-15)` at the captured viewport); the lower graph is the same dot grammar at a 24 px gap (`translate(-13,-13)`). The upper currently has no CSS background image or other planning-space identity, so the first visual divergence is absent at the canvas background boundary rather than being lost behind the React Flow viewport.

The baseline Tailnet DOM contained three Bird View nodes and two edges. A trusted double-click entered focus, where the lower `.graph-canvas` retained its existing dot Background and rendered five graph nodes. Source inspection confirms the lower graph is configured independently in `src/App.tsx` and the upper graph in `src/bird-view/BirdViewRoutes.tsx`.

## Chosen pattern

Use two upper-only React Flow `Lines` backgrounds with unique pattern IDs: a restrained minor drafting grid and a lower-contrast major planning grid at five times the spacing. This is a repository-native SVG grammar that follows React Flow pan and zoom transforms naturally, reads as planning paper rather than the Workspace dot field, and requires no bitmap, animation, layout change, or custom canvas controller.

## Work

1. Add named upper-only pattern configuration and render the two line layers inside the Bird View React Flow surface.
2. Scope subtle canvas and line colors to `.bird-v2-canvas`; do not change `.graph-canvas` or the lower Workspace `Background` configuration.
3. Extend development tracing so controller initialization and move completion report the upper pattern identity and rendered pattern layers.
4. Add focused assertions for the upper two-scale line identity, the lower dot grammar, retained default cubic edges, and DOM scoping.
5. Run focused tests, typecheck, build, npm audit, diff check, and source/runtime/database isolation checks.
6. In a fresh Tailnet browser context, verify computed pattern styles and trusted pan, zoom, node drag, edge visibility, reload, and double-click focus. Restore the canonical three-node/two-edge demo through product boundaries and finish the Baley Run/records without human confirmation.

## Non-goals

No lower Workspace graph change, cross-Workspace UI, bitmap asset, animation, operating repository/database/service mutation, firewall change, backend change, destructive database suite, commit, push, merge, deploy, or human confirmation.
