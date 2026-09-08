---
baley_record: 1
record_id: "98a7c227-9207-473b-8f86-4e84f4bb3664"
task_id: 178
run_id: "473a6813-a975-49de-bc85-2426c9c9f273"
task_key: "bird-view-v2"
record_type: completion-report
created_at: "2026-09-06T11:25:15Z"
created_by: "codex"
registration_state: registered
---

# Task #178 completion report

## Outcome

Implemented and verified a repeatable, parallel DayTripper Bird View sample. The source Workspace is read through one guarded `REPEATABLE READ READ ONLY` snapshot; the sanitized clone runs in the existing isolated PostgreSQL container under a new database, account, processes, ports, and Tailnet routes. No commit, merge, push, Baley lifecycle transition, firewall change, or `go run` occurred.

## Files changed for Task #178

- `scripts/daytripper-birdview-source-snapshot.sql`
- `scripts/daytripper-birdview-import.sql`
- `scripts/daytripper-birdview-environment.ps1`
- `docs/daytripper-birdview-sample-environment.md`
- `task-records/bird-view-v2/task-178-detailed-plan.md`
- `task-records/bird-view-v2/task-178-completion-report.md`
- `src/bird-view/BirdViewRoutes.tsx`
- `src/bird-view/BirdViewRoutes.test.tsx`

The two Task Records are registered against the successful recovery Run. All pre-existing Bird View V2 changes in the dirty worktree were preserved.

## Copy policy and exact counts

The exporter guards the authoritative full schema-25 inventory of 39 public tables. It copied or sanitized these 19 Workspace graph/execution/evidence tables:

| Entity | Rows |
| --- | ---: |
| workspaces | 1 |
| workspace_counters | 1 |
| phases | 4 |
| lanes | 4 |
| evidence_profiles | 1 |
| workspace_acceptance_policies | 1 |
| tasks | 60 |
| task_dependencies | 52 |
| backlog_items | 40 |
| gates | 3 |
| gate_tasks | 14 |
| gate_entry_tasks | 0 |
| repositories | 1 |
| runs | 146 |
| run_git_observations | 0 |
| commit_references | 0 |
| task_record_indexes | 90 |
| task_acceptance_assignments | 60 |
| task_acceptance_evidence | 3 |

Every actor reference in copied rows is replaced with the isolated review actor, every Run session reference is cleared, and every Run lease hash becomes an inert deterministic redaction before it leaves the source query.

The source rows deliberately omitted from the Workspace include 2 memberships, 21 Agent tokens, 0 approval grants, 675 commands, 689 events, 29 human approval attestations, 0 MCP connection requests, 1 MCP gateway registration, 896 mutation attempts, and 29 security events. The identities referenced by that Workspace resolve to 2 actors and 1 account with 1 credential, 1 external identity, 92 sessions, and 3 OIDC authorization flows; all were omitted. The 2 global authentication-rate-limit rows cannot be safely attributed to one Workspace and were also omitted. The target contains zero forbidden source identity, token, approval, attestation, MCP, OIDC, command, event, or mutation-attempt rows.

## Environment and access

| Item | Value |
| --- | --- |
| Source, read-only | `local-dev-postgres` / `baley` |
| Target | `baley-bird-view-v2-test` / `baley_daytripper_birdview_test` |
| Disposable marker | `BALEY_DISPOSABLE_DAYTRIPPER_BIRDVIEW_CLONE_V1` |
| Preserved target | `baley-bird-view-v2-test` / `baley_v2_test` |
| Loopback Viewer / API | `http://127.0.0.1:5276` / `http://127.0.0.1:8182` |
| Tailnet Viewer | `https://jazzcake-home.tail87e929.ts.net:8449` |
| Tailnet API | `https://jazzcake-home.tail87e929.ts.net:8450` |
| Review login | `daytripper-review` |
| Password file | `.tmp/daytripper-birdview/secrets/review_password` (gitignored) |
| Bird View ID | `17800000-0000-4000-8000-000000000178` |
| Bird View revision | 15 |
| API process | PID 36452 on 8182 |
| Viewer process | PID 36896 on 5276 |
| Snapshot SHA-256 | `7503d6d3f4e1b8a2cc37b47c899324a90f1193162ff40e63eb4a0325b8a821e6` |

The password value is intentionally absent from this record and source control. It is reported only in the supervised `worker_done` message.

## Verification evidence

- Final source comparison: schema 25, Workspace revision 613, phases 4, lanes 4, tasks 60, dependencies 52, gates 3, gate tasks 14, backlog items 40, runs 146, Task Record indexes 90; identical before and after reseed.
- The operating database was accessed only by schema inspection and `SELECT`/read-only snapshot transactions. The workflow never accessed or wrote `D:\Project_AI\baley` source files.
- Preserved `baley_v2_test` comparison: schema 27, workspaces 2, Workspace revision sum 4, tasks 7, Bird Views 1; identical before and after reseed.
- Target: schema 27, one Workspace, one local account/credential/membership, copied counts exactly matching the snapshot, one Bird View with five Nodes, four edges, 17 bindings, and zero forbidden or unsanitized rows.
- Authenticated API: one visible Workspace, graph 5 Nodes/4 edges, PlaceMatch focus 10 Tasks/9 dependency edges.
- Browser proof over the Tailnet Viewer: overview rendered 5 Nodes/4 edges; focused Tasks #43-#52 rendered 14 cards and 10 total edges (9 dependencies plus the G#1 unlock edge), with zero overlapping cards.
- Frontend: all 22 test files and 130 tests passed; the focused rendering regression suite passed 9 tests; TypeScript typecheck and production build passed. The production build emitted only the existing ELK chunk-size warning.
- Dependency audit: `npm audit` passed with 0 vulnerabilities across 261 dependencies.
- Go: full `go test ./...` with database variables unset passed; `go vet ./...` passed. On fresh disposable databases, `TestBirdViewMigrationAndAccountPrivateFlow`, `TestValidateDisposableDatabaseConnection`, and `TestMigration27BackfillsDurableBirdViewPositions` passed, and the disposable databases were removed before the final reseed.
- PowerShell parser validation and final `git diff --check` passed.

## React defect diagnosed and fixed

Development-only structured traces captured the focus event, calculated projection, React state, React Flow controller state, and rendered DOM. The first divergence was `onNodesChange`: focus-card dimension events were being applied to the overview collection, creating a controlled rerender loop while the controller held 10 focus edges and the DOM rendered none. Focus projections now ignore those overview-persistence mutations; a regression assertion confirms `applyNodeChanges` is not called for a focused dimension event, and browser evidence then showed controller and DOM both rendering 10 edges.

## Residual risks

- The sample intentionally omits the historical command/event/audit timeline and private identity/security history; it is a graph, Run/Record, acceptance, and Bird View review fixture rather than a forensic clone.
- The source guard intentionally refuses future schema or table-inventory drift until the allowlist is reviewed.
- An initial combined database-test run exposed cross-test fixture pollution when migration 27 followed the Bird View integration fixture in the same database. Each test passed against its own fresh disposable database, which is the supported isolation contract, but shared-database test ordering remains a suite-harness risk.
- Tailnet access depends on the host's existing Tailscale service and current Serve configuration. No firewall rule was created or changed.

The final DayTripper processes and Tailnet routes remain running for review. No destructive database test was run after the final reseed and browser proof.
