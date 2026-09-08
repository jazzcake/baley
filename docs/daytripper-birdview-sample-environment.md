---
type: operations
status: active
scope: daytripper-birdview-sample
---

# DayTripper Bird View sample environment

This environment is a disposable, repeatable, account-isolated Bird View sample built from a read-only snapshot of the operating DayTripper Workspace. It does not reuse the operating account, security history, Agent credentials, or the existing `baley_v2_test` Bird View demo.

## Fixed isolation boundary

| Purpose | Value |
| --- | --- |
| Read-only source | `local-dev-postgres` / `baley` |
| Expected source Workspace | `410f335e-ddb2-443f-be3c-7d1d18ccd534` / `DayTripper` |
| Parallel target | `baley-bird-view-v2-test` / `baley_daytripper_birdview_test` |
| Preserved database | `baley-bird-view-v2-test` / `baley_v2_test` |
| Target marker | database comment `BALEY_DISPOSABLE_DAYTRIPPER_BIRDVIEW_CLONE_V1` |
| Loopback Viewer / API | `127.0.0.1:5276` / `127.0.0.1:8182` |
| Tailnet Viewer / API | `https://jazzcake-home.tail87e929.ts.net:8449` / `https://jazzcake-home.tail87e929.ts.net:8450` |
| Review login | `daytripper-review` |
| Bird View | `17800000-0000-4000-8000-000000000178` |

The password is generated on every reseed and stored only in the gitignored `.tmp/daytripper-birdview/secrets/review_password` file. The Go executable is built under `C:\dev-bin\baley\daytripper-birdview`; the workflow never uses `go run`. Tailscale Serve adds only the dedicated HTTPS ports and no firewall rule.

## Authoritative source-table policy

The schema-25 source currently has 39 public tables. The snapshot exporter compares the entire sorted table inventory with the reviewed list and refuses to run if any table appears, disappears, or is renamed. It also requires the exact Workspace ID/name and uses one `REPEATABLE READ READ ONLY` transaction, so every included array and count comes from one stable database snapshot.

### Included and sanitized

| Table | Rows | Policy |
| --- | ---: | --- |
| `workspaces` | 1 | Copy Workspace identity, state, revision, and creation time. |
| `workspace_counters` | 1 | Copy all Task, Backlog, and Gate counters. |
| `phases` | 4 | Copy. |
| `lanes` | 4 | Copy. |
| `evidence_profiles` | 1 | Copy. |
| `workspace_acceptance_policies` | 1 | Copy; replace a non-null changer with the isolated review actor. |
| `tasks` | 60 | Copy all planning/lifecycle/acceptance fields. |
| `task_dependencies` | 52 | Copy. |
| `backlog_items` | 40 | Copy. |
| `gates` | 3 | Copy; replace a non-null passer with the isolated review actor. |
| `gate_tasks` | 14 | Copy; replace a non-null passer with the isolated review actor. |
| `gate_entry_tasks` | 0 | Included even though currently empty. |
| `repositories` | 1 | Copy the public repository and Task Record root reference. |
| `runs` | 146 | Copy execution history; replace the operator with the isolated review actor, clear `session_ref`, and replace every lease hash with an inert deterministic redaction. |
| `run_git_observations` | 0 | Included even though currently empty. |
| `commit_references` | 0 | Included even though currently empty. |
| `task_record_indexes` | 90 | Copy safe repository-relative indexes and hashes, not record file contents. |
| `task_acceptance_assignments` | 60 | Copy; replace a non-null approver with the isolated review actor. |
| `task_acceptance_evidence` | 3 | Copy; replace the reporter with the isolated review actor. |

The transformations happen inside the source-side `SELECT`. Original actor IDs, Run session references, and lease hashes therefore never enter the snapshot artifact.

### Explicitly excluded

| Tables | Reason and fidelity tradeoff |
| --- | --- |
| `accounts`, `account_credentials`, `account_external_identities`, `account_sessions`, `actors`, `workspace_memberships` | Identity, password hashes, login sessions, and membership provenance are private. One new local Owner account/membership replaces them. |
| `agent_tokens` | Contains Agent bearer-token hashes and issuance provenance. |
| `approval_grants`, `human_approval_attestations` | Contains human authorization grants and attestations. |
| `mcp_connection_requests`, `mcp_gateway_registrations` | Contains MCP device secrets, registrations, and identity linkage. |
| `auth_login_limits`, `oidc_authorization_flows`, `security_events` | Contains authentication/security state and private identity audit material. |
| `commands`, `events`, `mutation_attempts` | Historical command/audit rows carry initiator, executor, credential, approval, and arbitrary payload linkage. Omitting them removes the historical Event/audit timeline and means Workspace `observedAt` falls back to creation time, but does not affect graph, Task Inspector Run/Record evidence, acceptance evaluation, or Bird View focus behavior. |
| `goose_db_version` | Target schema provenance is produced by applying this worktree's migrations through version 27. |

Schema-27 `bird_view_*` tables do not exist at the source. They are target-only and the sample graph is created through authenticated `bird_view.*` application commands.

## Repeatable workflow

From this worktree:

```powershell
.\scripts\daytripper-birdview-environment.ps1 reseed
.\scripts\daytripper-birdview-environment.ps1 status
.\scripts\daytripper-birdview-environment.ps1 verify
```

`reseed` refuses to replace any existing target database without the exact disposable marker. It exports a stable sanitized snapshot, records its SHA-256 and counts, recreates only the fixed target database, applies migrations 1-27, imports within one transaction, checks count parity and explicit orphan queries, bootstraps a fresh local Owner through `baley-server account-bootstrap`, creates the Bird View through authenticated command HTTP, and starts dedicated API/Viewer processes. The workflow reads the target container's local bootstrap credential at runtime; no database credential is checked in or written to runtime state.

Use `stop` and `start` to control only the recorded `5276/8182` processes. PID, executable/worktree, and listener ownership are checked before stopping anything. The dedicated Tailscale routes remain configured across a stop and are reused only when they still point at the expected loopback targets.

Runtime evidence is gitignored:

- `.tmp/daytripper-birdview/clone-manifest.json`
- `.tmp/daytripper-birdview/verification.json`
- `.tmp/daytripper-birdview/runtime.json`
- `.tmp/daytripper-birdview/logs/`

## Bird View derivation

The five overview Nodes follow the four DayTripper phases and expose a useful drill-down:

1. `Intake and canonical foundations` binds the completed `intake` phase and G#1.
2. `Foundation Proof delivery` binds the active `foundation-proof` phase and G#2.
3. `PlaceMatch transition` binds Tasks #43–#52, the exact ten-Task/nine-dependency component discovered from Task #43, providing a realistic focused subset instead of the full 60-Task Workspace.
4. `Jeju Map v1` binds the planned `jeju-map-v1` phase and G#3.
5. `Curation and plan prototype` binds the final planned phase.

Four Bird View edges show Gate progression plus the focused PlaceMatch stream. The resulting aggregate is revision 15 with five Nodes, four edges, and 17 bindings.

## Verification contract

The importer fails its transaction on count drift, any explicit FK/orphan query, any source identity/security row, any unsanitized Run, or any unexpected actor. Post-seed verification requires schema 27; exactly one review Account, credential, and membership; zero external identities, Agent tokens, grants, attestations, MCP rows, OIDC flows, Workspace commands/events/mutation attempts; and only local bootstrap/login security Events owned by the review Account.

Authenticated API proof requires one visible Workspace, a five-Node/four-edge Bird View graph, and a ten-Task/nine-dependency PlaceMatch focus projection. A reseed also compares read-only source counts/revision and the preserved `baley_v2_test` schema/count/revision summary before and after the operation.
