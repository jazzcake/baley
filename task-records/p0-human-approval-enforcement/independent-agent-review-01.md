---
baley_record: 1
record_id: "d7ecf52d-694e-45ac-9ca5-58cf403e1b87"
task_id: 158
task_key: "p0-human-approval-enforcement"
record_type: independent-agent-review
run_id: "aa0cf69c-843c-4242-a76e-e85179ca5250"
created_at: "2026-08-31T17:53:00+09:00"
created_by: "codex-independent-review"
registration_state: pending
supersedes: null
reviewed_base_branch: "jazzcake/security-p0-approval"
reviewed_commit: "47ba26daeb63a5f8fa5e6a402ba1596d749dd3eb"
---

# Independent security review — Task #158

## Verdict

**PASS — no blocking findings.** Task #158 remains `implemented`; this review
does not confirm it. The approval boundary prevents an Agent bearer or a
caller-supplied legacy attestation from becoming a human approver, and every
current human-only command is bound to a fresh browser-session grant.

## Findings

| Severity | Finding | Paths / lines |
| --- | --- | --- |
| None | No security, correctness, or migration blocker found. | — |

## Adversarial review evidence

- Agent/body-attestation bypass: authenticated executions overwrite
  caller-supplied initiator/executor identity and reject legacy attestation
  authority before execution. Human-only commands require `approvalGrantId`;
  grant validation requires an authenticated principal. `server/internal/application/command_service.go:340-369`, `server/internal/persistence/postgres/repository.go:1449-1520`.
- Grant binding/replay/cross-workspace/staleness: validation locks the grant,
  verifies Workspace, action, entity, revision, command hash, decision snapshot,
  warning set, proceed-reason digest, status, expiry, issuing session, account,
  active membership, and current capability. Consumption is conditional on
  `status='active'` in the command transaction. `server/internal/persistence/postgres/repository.go:1459-1520`, `server/internal/persistence/postgres/repository.go:1331-1364`.
- CSRF/session/account/membership revocation: all non-safe browser mutations,
  including grant issue/revoke and execute, require allowed Origin plus valid
  CSRF double-submit verification. Session and membership revocation triggers
  revoke unused grants and append audit events; command-time rechecks cover
  concurrent revocation. `server/internal/transport/httpapi/router.go:739-795`,
  `server/migrations/00023_human_approval_grants.sql:41-96`.
- Human-only coverage: policy registry marks workspace close, lane terminal,
  Task confirm/discard/policy commands, active Gate attachment, Gate task
  pass/revoke, and Gate pass as approval-bound; the conditional active-Gate
  path sets `ForceHumanApproval`. `contracts/v1/commands.json:78-114`,
  `server/internal/application/command_service.go:1797-1817`,
  `server/internal/persistence/postgres/repository.go:829-850`.
- Delegated auto-confirm removal/migration: current domain resolution rejects
  delegated modes, evidence reporting remains non-confirming, and migration 23
  converts legacy policy/task rows then constrains all current modes to
  `human_required`. `server/internal/domain/acceptance.go:89-95`,
  `server/internal/application/command_service.go:1258-1326`,
  `server/migrations/00023_human_approval_grants.sql:98-133`.
- MCP/CLI/UI and audit: MCP exposes only a grant reference, CLI installs only
  that reference after approval, and the browser UI issues via its
  CSRF-protected session then executes once. Command, Event, attestation, and
  immutable security-event records are written in the same transaction.
  `server/cmd/baley-mcp/main.go:397-418,912-945`,
  `server/internal/cli/model.go:233-245`,
  `src/components/WorkspaceAccess.tsx:694-821`,
  `server/internal/persistence/postgres/repository.go:1305-1364`.

## Verification rerun

- `server`: `go test ./...` — PASS.
- Isolated PostgreSQL 17 container: migrated 1–23 using
  `C:\dev-bin\baley\baley-server-task158-review.exe migrate up`, then
  `BALEY_TEST_DATABASE_URL=postgres://…/baley_review_test?sslmode=disable go test ./integration -count=1 -timeout 10m` — PASS (16.818s).
- Frontend: `npm test -- --run` — PASS (16 files, 94 tests).
- Frontend production build: `npm run build` — PASS; existing Vite chunk-size
  warning only.
- `git diff --check 090a287b..47ba26daeb63a5f8fa5e6a402ba1596d749dd3eb` — PASS.

## Residual note

The completion report's absent operator-managed Google OIDC secret prevents
new Google sign-ins in that deployment. It is an operational availability
issue, not an approval-grant authorization bypass; current authenticated
sessions and enforcement remain protected.
