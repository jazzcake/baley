---
baley_record: 1
record_id: "ac781e3d-47db-4928-925a-1b3b437b26b7"
task_id: 186
record_type: completion-report
run_id: "8e932014-ae7d-4a1a-ae7c-c02011eceef5"
created_at: "2026-09-10T14:38:06+09:00"
created_by: "codex"
supersedes: "0a70b0b1-4988-4b3f-9fc9-7348cb3a81fa"
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

## Independent review fixes

- **HIGH:** Replaced substring heuristics with a closed, full-string English/Korean decision grammar. It preserves direct exact-Task commands and practical all-awaiting commands, including counted Korean batch approval, while rejecting questions, conditions, hedges, exclusions, negations, scope mismatches, and cross-target references.
- **MEDIUM:** Made the rollout migrator safely resumable from schema 26 or 27 after Goose partially advances. It validates the contiguous migration chain, required Journal schema, Journal/Event/command provenance, and absence of partial migration-28 structures before a forward-only retry. Safety tests cover schema-26 resume, failure leaving schema 27 followed by successful retry, and invalid-chain refusal before the migrator starts.
- **LOW:** Corrected the account/workspace access contract and operations text: the Viewer is read-only for Task confirmation, conversational MCP execution is the ordinary path, and browser grants remain only for other explicitly applicable human-only commands.

Initial implementation commit: `150c2ef69aa824585b96e870f7abd1af18a22f7b`.

## Verification evidence

- `go test ./... -count=1 -parallel=1 -p=1` with a process-scoped `BALEY_TEST_DATABASE_URL`: passed against a fresh loopback-only PostgreSQL 17.5 database at schema 28. JSON accounting recorded 15 passed packages, 663 passed test/subtest events, one intentional `BALEY_MCP_E2E` skip, and zero failures; the two packages reported skipped contain no tests.
- Focused DB regression rerun: `TestLinkedAccountConversationalTaskConfirmationTrustBoundary` and `TestEmbeddingEnablementSingleRepositoryScenarioAgainstPostgres` passed 2/2 with zero skips or failures in a separate fresh PostgreSQL 17.5 container.
- `go vet ./...`: passed.
- Focused application, domain, and MCP tests: passed, including 47 positive/negative conversational grammar cases, exact compact compatibility name/count/schema, bridge routing, and ordinary-leaf behavior.
- `npm test -- --run --reporter=dot`: 16 files and 100 tests passed, including the absence of TaskConfirmation mutation UI.
- `npm run build`: passed; 2,111 modules transformed. Vite retained the existing large-chunk warning (1,943.04 kB minified, 596.04 kB gzip).
- `scripts/test-task-journal-rollout.ps1`: 15/15 safety regressions passed with schema-28 and partial-advance recovery expectations.
- `go build -trimpath`: server and MCP executables built successfully under `C:\dev-bin\baley\task-186\`.
- `git diff --check`: passed.

Both successful DB runs used unique container/database/user/credential identities, Docker `HEALTHCHECK`, a post-health stabilization interval, and `TestValidateDisposableDatabaseConnection` to prove the effective host URL before applying migrations to the disposable database. Each exact container was removed in `finally` and verified absent. Existing Baley databases, containers, and services were not modified.

## Trust boundary and residual risks

Baley authenticates the Agent credential and linked Account/member, revalidates capability, and persists cryptographic bindings and actor provenance. It cannot independently authenticate the semantics of the external conversation transcript; the Agent is trusted only to transmit the explicit statement into the typed envelope. Conservative verb/target/scope and negation checks reduce accidental or ambiguous execution but are not a general natural-language proof system.

The prior database-verification gap is closed locally. CI should continue running the same disposable PostgreSQL suite as regression coverage. The remaining known test skip requires the separate `BALEY_MCP_E2E` external transport harness and is unrelated to PostgreSQL or Task #186 behavior. The existing Viewer bundle-size warning also remains unrelated to Task #186.

## Migration and rollout

Migration 28 creates `conversational_decision_evidence` and links each consumed row one-to-one with `human_approval_attestations`. API readiness and the server image now require schema 28. Deploy API, Viewer, and MCP from the same reviewed commit; migrate before replacing services; verify the compact catalog still has 15 tools and the compatibility-stable tool name; then canary missing/vague/negated/stale/replay/cross-target/unauthorized cases before one explicit conversational success.

Application rollback must remain schema-28 compatible. Migration 28 down removes the evidence link and table and is therefore not a routine rollback path. No migration, deployment, service, firewall, merge, push, operating database, or operating worktree change occurred in this implementation run.
