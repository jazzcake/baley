---
baley_record: 1
record_id: "17430000-0000-4000-8000-000000000174"
task_id: 174
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-05T12:28:00Z"
created_by: "codex"
registration_state: pending
---

# Task #174 detailed implementation plan

## Outcome

Align only the editable Bird View overview with the established Baley Workspace visual language and establish whether the reported unrelated source content exists at any Baley boundary.

## Initial boundary evidence

- The repository-wide search found none of the three user-provided identifiers in product source, fixtures, documentation, or Task Records.
- The isolated schema-27 PostgreSQL Bird View node, edge, Task, and Task Record content also contained no matches.
- A trusted browser click was traced through the API graph payload, calculated node and edge state, React state, React Flow controller state, and rendered DOM. Every layer contained only the isolated Bird View data; no Baley-layer divergence or unrelated source content was observed.
- The demo currently contains prior browser-test residue (four nodes and four edges). This is expected isolated test data, not the reported content, and will be reseeded after verification so final evidence starts from the canonical three-node/two-edge graph.

## Work

1. Retain the development-only boundary trace, including the API payload and post-commit DOM text.
2. Rework only the Bird View overview toolbar, node cards, Inspector, canvas hints, controls, focus states, and responsive rules using the existing Baley tokens and component conventions.
3. Preserve create/edit/delete/drag/connect, edge edit/delete, semantic zoom, selection, and double-click focus behavior without changing the lower Workspace graph.
4. Add focused component assertions for visual semantics and interaction regressions.
5. Run focused tests, typecheck, production build, npm audit, and diff checks; backend tests are unnecessary unless backend code changes.
6. Reseed the isolated demo only after tests, confirm schema/account/graph counts, then perform fresh Tailnet browser verification and capture retained traces.

## Non-goals

No lower Workspace redesign, cross-Workspace layout, backend/domain changes, operational database access, deployment, commit, push, merge, or human confirmation.
