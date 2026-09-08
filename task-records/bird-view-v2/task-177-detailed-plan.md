---
baley_record: 1
record_id: "17730000-0000-4000-8000-000000000177"
task_id: 177
run_id: "b561788a-4c5a-42df-b375-5069310de265"
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-06T09:30:00Z"
created_by: "codex"
registration_state: registered
---

# Task #177 detailed implementation plan

## Diagnosis

The PM correction identified three concrete divergences. The test helper still passed the removed `renderWorkspace` prop and made `npm run typecheck` fail; discarded separate-focus CSS remained in `src/styles.css`; and the focused state unmounted overview nodes and edges, then reintroduced them at full opacity during exit, so the reverse fade could not occur.

The instrumented transition boundary now identifies the first state mismatch across the user event, calculated target phase, React/application state, React Flow controller state, and rendered DOM. Acceptance is based on the rendered state sequence rather than on the presence of styling alone.

## Implementation

1. Retain development-only event, computed target, React/application-store, React Flow controller, and rendered-DOM traces at focus boundaries.
2. Keep a single React Flow mounted across `overview -> entering -> focused -> exiting-start -> exiting -> overview`, preserving the anchor Bird node's exact id and `birdViewNode` type throughout.
3. During entering, place the anchor with `screenToFlowPosition({ x: 32, y: 96 })` while retaining overview nodes and edges with fading classes. During focused, project only directly bound Tasks/Gates and their compact Phase subset, with truthful dependency and Gate condition/entry edges.
4. Reverse in two stages: mount the complete overview at opacity zero in `exiting-start`, advance on the next animation frame to `exiting`, return the anchor to its exact overview coordinate while the overview fades in, and then restore the exact saved viewport and overview state.
5. Bypass transition timers for reduced motion and prove every phase, projection invariant, coordinate, class, mount count, and viewport restoration with deterministic React Flow tests.
6. Remove only obsolete separate-focus selectors/keyframes while retaining the active same-canvas focus selectors, and run each required check independently with its own exit code.
7. Validate the projected subset against a real 1762x1039 Tailnet browser viewport. If the focus anchor is transformed into negative Flow coordinates, lay out the compact Phase/Task/Gate columns relative to that anchor, trace calculated and rendered bounds, and keep creation controls unavailable throughout the transition.

## Exclusions

No Workspace navigation/chrome, whole Workspace graph, invented dependency targets, Task-coordinate writes, server change, operating database mutation, firewall change, commit, merge, push, deploy, Baley lifecycle mutation, or Task confirmation.
