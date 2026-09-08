---
baley_record: 1
record_id: "712c186b-1fbf-4651-b5c7-ad60b49916e1"
task_id: 178
run_id: "473a6813-a975-49de-bc85-2426c9c9f273"
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-06T11:03:37Z"
created_by: "codex"
registration_state: registered
---

# Task #178 detailed plan

## Objective

Create a repeatable DayTripper-scale Bird View review environment from the operating Workspace without copying authentication, authorization, MCP, or private identity/audit material and without touching the existing `baley_v2_test` demo.

## Plan

1. Inventory every source schema-25 public table and FK, then classify Workspace graph/execution/evidence data for copy, sanitization, or explicit omission.
2. Export all allowed data from one exact-ID/name, repeatable-read, read-only source transaction and fail if the schema/table inventory changes.
3. Recreate only a fixed, database-comment-marked target in the isolated PostgreSQL container; migrate it through schema 27 and import the snapshot transactionally with count, orphan, and forbidden-row assertions.
4. Bootstrap a new local review Owner, then use authenticated Bird View commands to create a phase-derived overview and a focused ten-Task PlaceMatch subset.
5. Start dedicated loopback API/Viewer processes and add unused Tailnet HTTPS ports without changing firewall rules or existing routes.
6. Verify source/preserved-target summaries, schema/count/FK/security invariants, login/API reads, overview/focus rendering, frontend and Go test suites, typecheck, production build, and dependency audit. Perform no destructive database test after the final reseed and browser proof.

## Safety controls

- Source access is limited to `SELECT` inside `REPEATABLE READ READ ONLY` and schema inspection.
- The target database name and disposable comment are fixed and validated before replacement.
- Source identities, credentials, sessions, token/grant/attestation/MCP rows, and identity-bearing audit ledgers are never exported.
- Run session references and lease hashes are redacted in the source-side projection.
- The existing `baley_v2_test` database and `5275/8181` plus `8447/8448` demo remain independent.
- Go binaries are built only under `C:\dev-bin\baley`; no firewall or Git lifecycle mutation is performed.
