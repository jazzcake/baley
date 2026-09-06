---
baley_record: 1
record_id: "4b2745a0-8dbf-4f3c-a4d7-e3f4ab0cb5a8"
task_id: 180
task_key: "mcp-tool-catalog"
record_type: completion-report
run_id: null
created_at: "2026-09-06T14:42:17Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #180 completion report: compact default MCP tool catalog

## Outcome

The assigned independent top-level worktree started clean at
`86a89298ac9bca8a87a5ac0aa1c659139404ca28` on
`jazzcake/mcp-tool-catalog`; all changes below were made only there.

Baley's default Streamable HTTP MCP endpoint is now a deterministic compact
catalog with 14 tools. It keeps ten essential read surfaces and adds an
on-demand command catalog plus fixed preview, routine-execute, and approved-
execute bridges. The compatibility endpoint `/mcp/full` contains all 78 legacy
tool names plus the four new catalog/bridge tools.

No HTTP or domain command was deleted. The bridge allow-list contains the 49
commands accepted by `/v1/commands/preview` and `/v1/commands/execute`; the
three domain/bootstrap policies (`project.bootstrap`, `workspace.create`, and
`workspace.activate`) that are not accepted by those HTTP endpoints remain
untouched. The application, HTTP API, domain implementation, credential
store, device-linking flow, and operating database were not changed.

## Catalog measurements

Schema bytes are measured exactly as the sum of `json.Marshal(tool.InputSchema)`
lengths returned by `tools/list`. Tests pin all three profiles:

| Catalog | Tools | Serialized input-schema bytes | Change from legacy |
| --- | ---: | ---: | ---: |
| Previous legacy baseline | 78 | 37,800 | baseline |
| Compact default `/mcp` | 14 | 4,184 | 88.93% reduction |
| Full opt-in `/mcp/full` | 82 | 40,124 | legacy plus 4 bridges |

The compact result is below the 15-tool target and exceeds the required 75%
schema-byte reduction.

## Exact compact default tool list

The MCP SDK returns the unique names in deterministic ascending order:

1. `baley_backlog_get`
2. `baley_backlog_list`
3. `baley_command_catalog`
4. `baley_command_execute`
5. `baley_command_execute_with_approval`
6. `baley_command_preview`
7. `baley_gate_status`
8. `baley_lane_brief`
9. `baley_mcp_diagnostics`
10. `baley_phase_tasks`
11. `baley_task_acceptance_get`
12. `baley_task_get`
13. `baley_workspace_context`
14. `baley_workspace_get`

`baley_mcp_diagnostics` reports MCP implementation version `0.2.0`, catalog
version `1.0.0`, active/default profile, profile tool count and schema bytes,
the full-profile path, static-list mode, and command-discovery tool while
retaining the pre-existing redacted credential diagnostics.

## Command bridge and safety boundaries

- `baley_command_catalog` returns the 49 allowed command names, contract-derived
  human-approval policy, and the correctly annotated execution bridge only when
  requested.
- `baley_command_preview` forwards only to `/v1/commands/preview`.
- `baley_command_execute` accepts routine Operator commands only, rejects any
  approval grant, and forwards only to `/v1/commands/execute`.
- `baley_command_execute_with_approval` accepts human-only and conditional
  commands. It requires a fresh browser-issued grant locally for commands that
  are always human-only; conditional `gate.attach_task` remains server-decided
  because its grant requirement depends on current Phase state.
- Unknown or whitespace-variant command names, non-object inputs, missing
  Workspace/idempotency/actor values, legacy approval authority, and incorrect
  bridge classification are rejected before an HTTP request.
- The bridge preserves arguments and envelope values. Workspace-scoped token
  selection, capability filtering, revisions, idempotency, warning sets and
  reasons, command-specific decoding, domain invariants, canonical hashes, and
  browser-grant validation remain in the existing server path.

## Full-profile opt-in and client behavior

The routine registration stays `http://127.0.0.1:8090/mcp`. Diagnostics or rare
administration can explicitly add a separate profile:

```text
codex mcp add baley-full --url http://127.0.0.1:8090/mcp/full
```

The current official Codex manual documents an initial MCP tool catalog and
static per-server `enabled_tools`/`disabled_tools`; it does not document a
dynamic tool-list reload that this design can safely assume. The Go MCP SDK can
advertise list changes, but Baley does not mutate either endpoint after server
creation and does not depend on `listChanged` or client on-demand loading.

## Files changed

- `server/cmd/baley-mcp/catalog.go`: catalog profiles, 49-command allow-list,
  fixed bridges, local classification checks, and profile diagnostics.
- `server/cmd/baley-mcp/main.go`: preserved legacy builder and served compact
  `/mcp` plus full `/mcp/full` without changing tokenless local auth.
- `server/cmd/baley-mcp/catalog_test.go`: exact metrics/names, legacy parity,
  diagnostics, classification, rejection, forwarding, and contract tests.
- `server/cmd/baley-mcp/main_test.go` and `server/integration/mcp_test.go`:
  profile-aware annotations and executable endpoint coverage.
- `contracts/v1/commands.json` and `contracts/v1/README.md`: literal catalog,
  bridge, profile, and measurement contract.
- `docs/streamable-http-mcp-operations.md`,
  `docs/baley-command-architecture.md`, and
  `docs/mutation-attempt-audit.md`: operator behavior, opt-in instructions,
  static-client rationale, and full-only diagnostic read clarification.
- `.agents/skills/baley-manage-work/SKILL.md` and
  `.agents/skills/baley-manage-work/references/commands.md`: compact natural-
  language workflow and current browser-grant semantics.
- `task-records/mcp-tool-catalog/detailed-plan-01.md` and this completion
  report: pending Task #180 records.

## Verification

- Focused catalog/bridge/MCP tests: pass. Logged metrics are
  `legacy=78/37800`, `compact=14/4184`, `full=82/40124`, reduction `88.93%`.
- `go test ./... -count=1`: pass for every Go package.
- `go vet ./...`: pass.
- `npm test`: pass, 17 test files and 107 tests.
- `npm run build`: pass; Vite retains its existing large-chunk advisory.
- Skill validation with `PYTHONUTF8=1`: `Skill is valid!`.
- `git diff --check`: pass (only expected Windows line-ending notices).

The stripped Windows executable was built only at
`C:\dev-bin\baley\task-180-ctx_cc60c3745cf4\baley-mcp.exe` (9,759,232 bytes,
SHA-256 `2BC59E285979CEB9404FC9F97D3E51305BCC6AD098F3628C4A7516B16A95E2FE`).
Its isolated `diagnose` reported a configured but `not_created` credential store,
keychain support, and no legacy token. An executable-level Streamable HTTP test
started it on a random loopback port with an unused local upstream, verified
14 compact tools, 82 full tools, legacy-tool inclusion only in full, and then
terminated only that test-owned process.

## Install status and residual risks

The repository's Windows installer was inspected but intentionally not run.
It correctly refuses a dirty worktree before mutation, derives its immutable
release path from committed `HEAD`, and would therefore either reject these
uncommitted changes or install the old revision; later steps also replace the
port-8090 gateway. The existing `baley-mcp` process (PID 9080) was left running,
Codex configuration was not changed, and no firewall or database operation was
performed.

Remaining work and bounded risks:

- The coordinator must review and commit through its normal flow before the
  atomic installer can reproducibly install this revision, then register both
  pending Records, mark the Run successful, and call `task.report_implemented`.
- The full profile intentionally retains its high schema cost and should remain
  a short-lived explicit opt-in.
- Adding a future HTTP command requires adding a catalog descriptor; the
  uniqueness/count and literal-contract tests fail until that classification is
  updated.
- The isolated smoke did not mutate a live server or operating database, so
  deployed end-to-end command execution remains a post-commit release check.
- `npm ci` reported six dependency audit findings (one moderate and five high)
  in the existing lockfile dependency graph; no frontend dependency or source
  was changed by this task.

No commit or push was made.
