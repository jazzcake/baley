---
baley_record: 1
record_id: "669ad04a-6d09-4ac9-8ac9-0ce4de64c6ef"
task_id: 181
task_key: "mcp-tool-catalog-rework"
record_type: independent-agent-review
run_id: "660c4b1b-de48-44c5-babf-84eacaa80264"
created_at: "2026-09-07T07:27:21Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #181 independent review: MCP tool catalog rework

## Verdict

**Pass.** Zero unresolved blocking findings and zero material findings.

## Findings by severity

- Blocking: none.
- High: none.
- Medium: none.
- Low: none.

## Scope and evidence reviewed

I reviewed the complete tracked diff from base
`ca44415cd2776981b558755c60a11886a25296ec` and both pre-existing untracked Task
Records. The implementation changes are confined to the MCP catalog and typed
handlers, their tests, the literal command contract, matching operating docs,
and Baley skill guidance; no unrelated implementation change was found.

The exact `baley_task_rework_preview` and `baley_task_rework_execute` names are
present in compact and full profiles. Their generated schemas require exactly
the intended core inputs: `workspaceId`, `taskId`, `reason`,
`expectedWorkspaceRevision`, `executedByActorId`, and `idempotencyKey`;
`initiatedByActorId` is optional. Execute uses the existing routine mutation
envelope, preserving `acknowledgedWarningCodes` and `proceedReason`, while both
typed schemas exclude `approvalGrantId`.

Both handlers hard-code command `task.rework` and forward only to
`/v1/commands/preview` or `/v1/commands/execute`. They do not reproduce or
bypass the HTTP command service: existing server-side capability, revision,
idempotency, warning-set, domain, and approval-mismatch validation remains the
authority. In particular, the command service rejects an approval grant or
legacy approval attestation on routine commands, so a grant is neither required
nor accepted as authority for `task.rework`.

The domain and literal contracts agree that `task.rework` is
`workspace:operate` with `humanApproval: none`. Tool descriptions and guidance
state the `implemented` to `in_progress` transition and recording of the reason
in the `task.rework_started` Event. Catalog version `1.1.0` agrees across code,
contract, and operations documentation.

Compact removes only `baley_workspace_get` from the prior 14-name set, adds the
two exact rework names, and remains exactly 15 tools / 5,106 serialized schema
bytes, 86.49% below the pinned 78 tools / 37,800-byte baseline. Revision remains
available through `baley_workspace_context` and `baley_task_get`; the former
returns `Workspace.Revision` and the latter returns the Task projection's
`expectedWorkspaceRevision`.

Full remains all 78 legacy names plus two typed rework tools and four bridge
tools, exactly 84 tools / 41,201 serialized schema bytes. No typed block or
unblock tool name was added; `task.block` and `task.unblock` remain discoverable
and executable through the generic 49-command catalog/bridge.

## Verification

- `go test ./cmd/baley-mcp -count=1`: passed. This covers exact schemas and
  names, annotations, missing-reason rejection before HTTP, fixed forwarding,
  compact/full metrics and parity, contract agreement, and profile diagnostics.
- `go test ./internal/application ./internal/domain -count=1`: passed; relevant
  command-service and lifecycle/domain behavior remains green.
- `go vet ./cmd/baley-mcp ./integration`: passed.
- Fresh executable built under
  `C:\dev-bin\baley\task-181-independent-review-01\baley-mcp.exe`; SHA-256
  `D6845E3A6A259E5027A3DE08B1B06A81EC92701AF90AA702F01322B7BDCDF322`.
- With that executable,
  `go test ./integration -run '^TestMCPStreamableHTTPListsAndCallsTools$' -count=1 -v`
  in list-only E2E mode: passed, independently verifying `/mcp` and `/mcp/full`
  executable profile integration without contacting an operating Baley server.
- `contracts/v1/commands.json` parsed successfully as JSON.
- `quick_validate.py .agents/skills/baley-manage-work`: passed (`Skill is valid!`).
- `git diff --check ca44415cd2776981b558755c60a11886a25296ec`: passed; only
  informational Windows LF-to-CRLF notices were emitted.
- The implementation report's already-passed full `go test ./...`, UI test, and
  UI build evidence was inspected; no discrepancy or suspicious gap required a
  second full UI run.

## Residual risks

- The executable E2E was intentionally list-only, so it verified live profile
  exposure but did not mutate or query an operating Baley service. Typed payload
  forwarding is instead covered by the focused in-memory/upstream HTTP tests.
- The full profile intentionally remains larger than the legacy baseline and is
  suitable only as the documented compatibility/diagnostic opt-in.
- The review created only this pending Task Record in the repository. It did not
  register records, mutate Baley Task/Run state, access an operating database,
  alter firewall rules, install or replace an operating executable, restart a
  service, merge, commit, or push.
