# Bird View V2 implementation and verification handoff

Date: 2026-09-05 (Asia/Seoul)

## Adopted product model

The V1 read-only Viewer and separate focus presentation were rejected and replaced. V2 is an account-private, two-level graph: a free-form editable Bird View canvas at level 1, and the current Workspace's existing Phase/Gate/Task graph beneath the focused Bird View Node at level 2. Additional Workspace bindings remain small external references.

Bird View Nodes and edges share the durable preview/execute command boundary with future MCP/LLM callers. Manual Bird View coordinates persist in schema 27. The embedded Task graph remains auto-positioned, but an active member with `workspace:operate` can use the Task Inspector to preview and execute `task.update` for title/description/current summary and `task.move` for Phase membership. Confirmed/discarded Tasks and non-operating memberships are guarded in the UI and remain server-authoritative.

## Implementation

- Removed V1 route/layout behavior and replaced it with a large editable React Flow canvas.
- Added Bird View Node create/update/delete, coordinate persistence, edge connect/update/disconnect, revision-CAS, idempotency, audit Events, HTTP, MCP parity, and schema/readiness version 27.
- Delayed single-click selection so trusted double-click focus wins; same-page exit restores the previous viewport.
- Reused `WorkspaceViewer` below the focus header and retained pan, zoom, selection, Phase/Gate/Task projection, inspector, search, and canvas controls.
- Added human Task Inspector preview/execute controls with exact target/revision revalidation, warning handling, membership/terminal guards, and failed-command recovery without optimistic graph drift.
- Replaced `READ ONLY` with `TASK EDIT + PHASE MOVE · AUTO LAYOUT` or `VIEW ONLY · AUTO LAYOUT` according to membership capability.
- Fixed two browser-only first divergences found by structured tracing:
  - persisted edges existed in React state and the ReactFlow controller, but controller nodes had no initial `handleBounds`/`handles`, so EdgeWrapper returned null; fixed-size Bird nodes now declare deterministic left-target/right-source handles;
  - without external references, the lower Workspace occupied the grid's auto row and clipped/hid Task hit targets; `.bird-focus-lower` now always occupies flexible row 3.
- Split the Bird View route and major dependencies with the existing manual chunk policy. The pre-existing 1.44 MB ELK Workspace-layout chunk still triggers Vite's 500 kB warning; the limit was not raised and #173 deliberately did not broaden into an ELK refactor.

## Structured browser evidence

Development-only buffers `window.__BALEY_BIRD_VIEW_TRACE__` and `window.__BALEY_VIEWER_TRACE__` retain recent event-boundary records. The traces include user event, calculated target state, React state, ReactFlow or command-controller state, and rendered DOM state.

Tailnet Viewer: `https://jazzcake-home.tail87e929.ts.net:8447/bird-views/17200000-0000-4000-8000-000000000172`

Tailnet API: `https://jazzcake-home.tail87e929.ts.net:8448`

The runtime is isolated from the operational Viewer/API/database: Vite port 5275, API port 8181, disposable PostgreSQL container `baley-bird-view-v2-test` on loopback port 55433, and a separate authenticated Chrome profile.

Real Chrome interaction verification before the final test run proved:

- creation and editing of `Browser-created outcome`;
- drag persistence of the second Bird Node at `(725, 460)` after reload;
- two durable editable edges, both rendered immediately after reload;
- double-click focus with the selected Bird Node pinned as the upper-left context header;
- lower Workspace height 1169px in the flexible row and trusted pointer selection of Task #110 on `.task-title`;
- Task #110 current-summary update through preview/execute (revision 1→2);
- Task #110 Phase move Validate→Build through preview/execute (revision 2→3), with no XY fields;
- reload/refocus showing the saved summary and `Build Phase`;
- focus exit returning to the same route and prior `translate(0px, 0px) scale(1)` viewport.

The PostgreSQL integration suite intentionally resets its target database, so its final run invalidated that demo seed and browser session. After every destructive database test was complete, the isolated demo was reseeded through the current schema-27 server and durable Bird View command boundary. Direct database checks then showed one active `bird-v2-review` account, one active Owner membership, one Bird View, three Nodes, two edges, and the Task #110 binding. A fresh Tailnet navigation and server-backed reload showed both edges; CDP pointer input produced `isTrusted: true` for the Bird Node double-click and Task #110 click, entered focus, rendered the full four-Task Workspace graph in the 1169px flexible row, and displayed the human Task editor. No database test was run after that proof.

## Verification

- Focused frontend: Bird View route/layout and Task command editor tests pass (3 files, 10 tests).
- Full frontend: 21 test files and 119 tests pass after the last correction.
- TypeScript: `npm run typecheck` passes.
- Production build: route/dependency-split output builds successfully; chunks are approximately 112 kB app, 181 kB React, 206 kB React Flow, 14 kB Bird View, and the manually separated 1.44 MB ELK layout engine. Vite's pre-existing ELK chunk warning remains non-blocking and the warning limit was not raised.
- Dependency audit: `npm audit` reports 0 vulnerabilities.
- Backend: `go test -p 1 ./... -count=1` and `go vet ./...` pass.
- Disposable PostgreSQL: migration 27 fresh/down/up and Bird View integration tests pass as part of the full backend suite.
- Hygiene: `git diff --check` passes; work remains intentionally uncommitted.

## Security and correctness review

Account ownership remains the Bird View privacy boundary and lower Workspace reads/writes still require active membership. The browser never creates a second mutation transport: both Bird View edits and Task edits reach the same server command service, capability checks, revision CAS, idempotency ledger, and transactional Event write used by MCP. Task editing avoids human-approval commands entirely; confirmation/discard remain separate human-only boundaries.

Failed Bird View mutations roll back optimistic state and reload server state. Task edits are not optimistic: failed preview/execute clears the reviewed command, retains the user's draft for retry, and allows the polling graph to load the authoritative revision. No Task position storage, cross-Workspace Task mutation, operational database access, firewall change, or human confirmation was introduced.

## Baley tracking

- #169 Run `0cf7e80e-5fcb-48d2-be7f-8862f37306a3`: succeeded; V1 replaced by V2.
- #170 Run `94d370ea-69f7-4532-837a-fc327b83ccac`: succeeded; free-form editing and schema 27.
- #171 Run `83e77e2b-b6c3-4afc-9e29-e951e074f53d`: succeeded; same-page focus and viewport restoration.
- #172 original verification Run `985e5308-ae0b-45c5-88a3-46df801e56f1`: interrupted on lease expiry; recovery Run `a23fb89f-2a67-4c5a-bb24-0c0dc8aa82fc` succeeded and Task #172 was reported implemented at Workspace revision 1191.
- #173 original Run `1e4a2b99-3cba-4b35-b176-c9c1f78a6831`: interrupted on lease expiry; recovery Run `e32a1cb5-fbd2-4e52-b9c2-71b847b76358` succeeded and Task #173 was reported implemented at Workspace revision 1186.

Tasks #172 and #173 each retain the explicit `missing_independent_review_record` warning for the later human acceptance boundary. Their detailed-plan and completion-report records are registered as `reported_uncommitted`.

No Task was confirmed or discarded. Human acceptance remains outstanding by policy.

## Residual risks

- ELK remains a large manually separated Workspace-layout chunk and still emits Vite's size warning; reducing it is deferred outside #173.
- Browser proof covers desktop Chromium and trusted pointer input; touch-specific gesture ergonomics were not separately exercised.
- The Tailnet test runtime is intentionally ephemeral and should not be treated as a deployment.
