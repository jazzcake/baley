---
baley_record: 1
record_id: "0a70b0b1-4988-4b3f-9fc9-7348cb3a81fa"
task_id: 186
record_type: completion-report
run_id: "8e932014-ae7d-4a1a-ae7c-c02011eceef5"
created_at: "2026-09-10T14:38:06+09:00"
created_by: "codex"
supersedes: null
status: ready_for_registration
---

# Task #186 completion report

## Delivered

- Ordinary explicit conversational `task.confirm` executes through compact or full MCP without Viewer mutation clicks.
- The compact compatibility name `baley_command_execute_with_approval` remains advertised, the compact catalog remains exactly 15 tools, and no replacement alias is advertised.
- Typed decision evidence is bound to action, target, revision, canonical command hash, idempotency, source/reference, unique decision ID, linked Account/human Actor, gateway registration, and executing Agent.
- The server rejects missing, vague, negated English/Korean, malformed, stale, replayed, cross-target, cross-revision, and unauthorized-linked-member evidence. Callers cannot supply an approver identity.
- The initiating human, executing Agent, evidence row, human attestation, command/Event, and Task Journal projection are separately auditable.
- The Task Inspector is read-only for implemented Tasks; TaskConfirmation UI and its mutation tests/styles were removed.
- Ordinary DAG leaves emit no topology warning and require no `terminalReason`. `terminal_path_conflict`, cycle, relationship, Gate, revision, idempotency, warning, and authorization safeguards remain.
- Migration 28, schema readiness/image expectations, contracts, MCP catalogs, the work-management skill, and normative product/spec/architecture/operations documents are aligned.

## Verification evidence

- `go test ./... -count=1`: passed across 15 test-bearing packages. JSON accounting recorded 562 passing test/subtest events and 43 PostgreSQL-gated skips because `BALEY_TEST_DATABASE_URL` was not configured.
- `go vet ./...`: passed.
- Focused application, domain, and MCP tests: passed, including explicit and negated decision parsing, exact compact compatibility name/count/schema, bridge routing, and ordinary-leaf behavior.
- `npm test -- --run --reporter=dot`: 16 files and 100 tests passed, including the absence of TaskConfirmation mutation UI.
- `npm run build`: passed; 2,111 modules transformed. Vite retained the existing large-chunk warning (1,943.04 kB minified, 596.04 kB gzip).
- `scripts/test-task-journal-rollout.ps1`: 12/12 safety regressions passed with schema-28 expectations.
- `go build -trimpath`: server and MCP executables built successfully under `C:\dev-bin\baley\task-186\`.
- `git diff --check`: passed.

The disposable-PostgreSQL integration cases for migration 28 and the full linked-account conversational transaction were added but intentionally not executed in this run because no disposable database URL was configured and Task #186 prohibited starting or touching services/databases.

## Trust boundary and residual risks

Baley authenticates the Agent credential and linked Account/member, revalidates capability, and persists cryptographic bindings and actor provenance. It cannot independently authenticate the semantics of the external conversation transcript; the Agent is trusted only to transmit the explicit statement into the typed envelope. Conservative verb/target/scope and negation checks reduce accidental or ambiguous execution but are not a general natural-language proof system.

The database-backed transaction and migration tests should run in CI against a disposable PostgreSQL instance before rollout. The existing Viewer bundle-size warning remains unrelated to Task #186.

## Migration and rollout

Migration 28 creates `conversational_decision_evidence` and links each consumed row one-to-one with `human_approval_attestations`. API readiness and the server image now require schema 28. Deploy API, Viewer, and MCP from the same reviewed commit; migrate before replacing services; verify the compact catalog still has 15 tools and the compatibility-stable tool name; then canary missing/vague/negated/stale/replay/cross-target/unauthorized cases before one explicit conversational success.

Application rollback must remain schema-28 compatible. Migration 28 down removes the evidence link and table and is therefore not a routine rollback path. No migration, deployment, service, firewall, merge, push, operating database, or operating worktree change occurred in this implementation run.
