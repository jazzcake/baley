---
baley_record: 1
record_id: "17570000-0000-4000-8000-000000000175"
task_id: 175
task_key: "bird-view-v2"
record_type: completion-report
created_at: "2026-09-06T08:15:32Z"
created_by: "codex"
registration_state: pending
---

# Task #175 completion report

## Delivered

The isolated Viewer now exposes its existing password-auth boundary only when the development-only `VITE_BALEY_LOCAL_REVIEW_LOGIN=enabled` switch is present. The local review form uses the existing authentication service, preserves only safe Bird View and Workspace return targets, and remains absent in production or without explicit configuration. An authenticated `/login` render now preserves the requested safe return target instead of racing back to the Workspace list.

Bird View edges now match the existing Workspace graph exactly: both graph builders omit an explicit React Flow edge type and therefore use React Flow's default cubic bezier renderer. This applies to edges loaded from the API and edges created by a human connection gesture, with focused regression assertions for both paths.

## First-divergence evidence

A fresh logged-out Tailnet browser context captured the exact session and provider request URLs, configured and calculated auth mode, anonymous application auth state, API provider response, calculated provider list, and rendered login DOM before any functional change. The schema-27 isolated API returned an empty OIDC provider list because this review runtime is password-only; the Viewer had no explicit local-review login configuration, so React accurately preserved the empty list and the DOM rendered the no-provider alert. The first divergence was therefore the isolated Viewer/runtime review-mode contract, not routing, cookies, Tailnet, PostgreSQL, the API response, or stale React state.

## Verification

- Focused frontend suite: six files and 47 tests passed, covering authentication return handling, the explicit development review login, loaded Bird View edges, and newly connected Bird View edges.
- TypeScript project typecheck passed.
- Production Vite build passed. Bird View remains a separate 15.45 kB route chunk; the pre-existing manually split ELK Workspace-layout chunk still emits the known non-blocking size warning.
- `npm audit --audit-level=high` reported zero vulnerabilities.
- `git diff --check` passed; only existing line-ending conversion warnings were printed.
- No backend source changed for Task #175, so no Go test was required and no destructive integration suite was run.
- A fresh incognito Tailnet context reached the local review login from the direct Bird View URL, authenticated the review account, and returned to the requested graph. The canonical overview rendered exactly three nodes and two cubic paths.
- Trusted handle input created a temporary node and connected edge; reload retained four nodes, three edges, and cubic `C` path commands for every edge. A trusted double-click entered same-page focus and rendered the full four-Task lower Workspace graph.
- The temporary node and its edge were deleted through the product command boundary. A final reload restored exactly three nodes and two cubic edges; no database test or reseed followed.
- Final read-only checks show database `baley_v2_test`, schema 27 locally and through Tailnet, one Bird View, three nodes, two edges, one active review account, one active Workspace membership, and an API connection to that database.

## Run recovery

The initial Run lease expired before the first Baley heartbeat because its default lease was shorter than the diagnostic interval. Work resumed in Run `335c191a-8d8b-4eb5-a967-4c99383576e6`, which owns this completion evidence and is the Run reported succeeded.

## Review endpoint

Tailnet Viewer: `https://jazzcake-home.tail87e929.ts.net:8447/bird-views/17200000-0000-4000-8000-000000000172`

## Residual risk

The pre-existing ELK dependency remains a 1.44 MB manually split async chunk and continues to trigger Vite's size warning; it does not enlarge the initial application chunk and is outside Task #175. The local review login is intentionally a development-only runtime facility, and Tailnet review availability still depends on the isolated host and processes remaining online. Independent review and human confirmation remain outstanding by policy.
