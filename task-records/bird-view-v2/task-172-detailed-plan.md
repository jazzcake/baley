---
baley_record: 1
record_id: "d67b1e72-57fb-455f-ae7e-70cd4cb35172"
task_id: 172
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-05T07:59:00Z"
created_by: "codex"
registration_state: pending
---

# Task #172 detailed plan

## Objective

Verify Bird View V2 across the frontend, backend, disposable PostgreSQL schema, and a separate Tailnet HTTPS runtime without touching the operational Baley service or database.

## Execution

1. Run focused and full frontend tests, TypeScript checking, production build, and dependency audit.
2. Run Go tests, Go vet, and migration/integration coverage only against a disposable PostgreSQL database.
3. Use development-only structured traces to locate the first ReactFlow/controller/DOM divergence for persisted edges and focus hit testing.
4. Browser-check create/edit/drag/connect/reload, same-page double-click focus, Task graph interaction, focus exit, and viewport restoration.
5. After destructive database tests finish, reseed the isolated demo and repeat a fresh Tailnet reload proof. Do not run database tests again after that proof.
6. Record security, authorization, revision/idempotency, regression, bundle-warning, and residual-risk findings without confirming or discarding any Task.

## Acceptance boundary

Completion requires green automated checks, schema readiness 27, a final isolated database containing the demo account and Bird View graph, and trusted Tailnet browser input reaching the lower Task editor. Human acceptance remains separate.
