---
baley_record: 1
record_id: "17730000-0000-4000-8000-000000000178"
task_id: 177
run_id: "b561788a-4c5a-42df-b375-5069310de265"
task_key: "bird-view-v2"
record_type: completion-report
created_at: "2026-09-06T09:45:00Z"
created_by: "codex"
registration_state: registered
---

# Task #177 completion report

## Current implementation evidence

The Bird View focus boundary now retains one React Flow through every rerender and follows `overview -> entering -> focused -> exiting-start -> exiting -> overview`. The anchor Bird node retains the exact same id and `birdViewNode` type throughout; entering derives its anchor with `screenToFlowPosition({ x: 32, y: 96 })`, focused includes only the directly bound Task/Gate/compact Phase subset, and Workspace chrome plus unbound Tasks remain absent.

Exit is now truthful and reversible in the same canvas. `exiting-start` restores the complete overview at opacity zero, the next animation frame advances the fading nodes and edges to opacity one while the anchor returns to its exact overview coordinate, and completion restores the exact saved viewport and overview state; reduced motion reaches focused and overview with zero delay.

Development traces record the user event, calculated target phase, React/application-store state, React Flow controller nodes/edges/viewport, and rendered DOM at the relevant boundary. Obsolete `renderWorkspace` test wiring and discarded `.bird-v2-focus`, `.bird-v2-focus-canvas`, `.bird-focus-context`, `.bird-external-references`, and old focus keyframes were removed while the active same-canvas focus selectors were retained.

Tailnet browser inspection at 1762x1039 confirmed that the selected Bird View node keeps its existing visual component and moves to the upper-left, overview nodes and edges leave the focused canvas, and a directly bound node renders its compact BUILD/VALIDATE Phase markers, Task #110, Gate #1, and truthful Gate-to-Task edge. That inspection also exposed a clipped Gate card caused by fixed absolute projection coordinates. The correction now computes the Phase/Task/Gate columns relative to the transformed focus anchor, traces both calculated and rendered screen bounds, and hides the Node creation control from focus request through exit.

After interaction testing, the isolated review graph was restored through the authenticated product command endpoint to its canonical three-node layout at Bird View revision 76; no direct database write or operating-data mutation was used.

## Verification performed in this recovery session

- `npm run typecheck`: passed, exit 0.
- Focused `BirdViewRoutes` / `BirdViewNode` tests: passed; the final route suite contains 9 focus/overview tests, including the exact clipping-producing 1762x1039 transform.
- Full `npm test`: 22 files passed, 130 tests passed, exit 0.
- `npm run build`: passed, 2,118 modules transformed, exit 0; Vite emitted its existing large-chunk advisory.
- `npm audit --omit=dev`: 0 vulnerabilities, exit 0.
- `git diff --check`: passed, exit 0; Git emitted only line-ending conversion warnings for pre-existing modified files.

## Residual risks / incomplete operational evidence

The browser-visible behavior and the clipping-producing viewport transform are covered, with the final bounds regression proving a 32px desktop margin. The existing Vite large-chunk advisory remains nonblocking, and focused Task editing remains intentionally deferred because this pass changes navigation/projection rather than the Task mutation contract. Both Task Record indexes are registered as `reported_uncommitted`; Task confirmation remains human-only and is outside this implementation pass.
