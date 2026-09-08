---
baley_record: 1
record_id: "f2e104f7-01a0-4647-9f4b-287dc82c2c31"
task_id: 181
task_key: "mcp-tool-catalog-rework"
record_type: completion-report
run_id: "40c51917-c73c-4cea-ac1d-ab4c176badea"
created_at: "2026-09-07T07:12:59Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #181 completion report: typed Task rework tools

## Outcome

Baley now exposes exact `baley_task_rework_preview` and
`baley_task_rework_execute` tools from both `/mcp` and `/mcp/full`. They forward
the existing `task.rework` command through the fixed preview and execute HTTP
paths and do not reproduce domain validation in the MCP process.

Both schemas require `workspaceId`, `taskId`, `reason`,
`expectedWorkspaceRevision`, `executedByActorId`, and `idempotencyKey`.
`initiatedByActorId` remains available when attribution is known, while execute
also preserves `acknowledgedWarningCodes` and `proceedReason` through the
established routine mutation envelope. Neither tool exposes an approval grant;
`task.rework` remains `workspace:operate` with no human approval.

Descriptions state the `implemented` to `in_progress` effect and that the reason
is recorded in `task.rework_started`. No typed `task.block` or `task.unblock`
tool was added.

## Catalog measurements

Schema bytes are the deterministic sum of each listed tool's JSON-serialized
`inputSchema`:

| Catalog | Tools | Serialized input-schema bytes | Notes |
| --- | ---: | ---: | --- |
| Legacy baseline | 78 | 37,800 | All former typed names retained |
| Compact `/mcp` | 15 | 5,106 | 86.49% below the legacy baseline |
| Full `/mcp/full` | 84 | 41,201 | 78 legacy + 2 rework + 4 bridge tools |

The compact profile removes only `baley_workspace_get`; normal compact callers
obtain revision/context from `baley_workspace_context` or `baley_task_get`.
`baley_workspace_get` remains in the full profile. Catalog version is `1.1.0`
and MCP implementation version is `0.3.0`.

## Verification

- Focused catalog/MCP tests passed, including exact names/counts/bytes, both
  profiles, all 78 legacy names, Operator annotations, required schema fields,
  missing-reason rejection before HTTP, fixed preview/execute paths, arguments,
  Actor attribution, revision, idempotency, warning acknowledgement, and proceed
  reason forwarding.
- `go test ./... -count=1`: passed for every Go package.
- `go vet ./...`: passed.
- `npm test -- --run`: 17 files and 107 tests passed.
- `npm run build`: passed; Vite emitted only the existing large-chunk advisory.
- `quick_validate.py .agents/skills/baley-manage-work` with UTF-8 mode: passed.
- `git diff --check`: passed; only expected Windows line-ending notices were
  printed.

The stripped Windows executable was built only at
`C:\dev-bin\baley\task-181-ctx_18a78bed7d32\baley-mcp.exe` (9,780,224 bytes,
SHA-256 `D6845E3A6A259E5027A3DE08B1B06A81EC92701AF90AA702F01322B7BDCDF322`).
It was not installed or substituted for the operating MCP.

## Files changed

- `server/cmd/baley-mcp/main.go`
- `server/cmd/baley-mcp/catalog.go`
- `server/cmd/baley-mcp/main_test.go`
- `server/cmd/baley-mcp/catalog_test.go`
- `server/integration/mcp_test.go`
- `contracts/v1/commands.json`
- `docs/baley-command-architecture.md`
- `docs/streamable-http-mcp-operations.md`
- `.agents/skills/baley-manage-work/SKILL.md`
- `.agents/skills/baley-manage-work/references/commands.md`
- `task-records/mcp-tool-catalog-rework/detailed-plan-01.md`
- `task-records/mcp-tool-catalog-rework/completion-report-01.md`

## Boundaries and residual risks

No commit, push, install, merge, deployment, operating database/data access,
firewall change, or operating-service stop/restart was performed. The
executable-level Streamable HTTP E2E test was not enabled because it requires a
configured Baley server; handler and in-memory MCP coverage verifies the new
catalog and forwarding without crossing that operating boundary.

`npm ci` reported the existing dependency audit state of one moderate and five
high vulnerabilities; no dependency manifest or lockfile was changed. The full
profile intentionally retains its high schema cost and should remain an
explicit diagnostic/compatibility opt-in. The coordinator still owns Task
Record registration and Baley Task/Run mutations.
