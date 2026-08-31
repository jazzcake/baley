---
baley_record: 1
record_id: "b83bd5c3-07e4-4cbc-aa43-2e0a6b482d37"
task_id: 158
task_key: "p0-human-approval-enforcement"
record_type: completion-report
run_id: "62fac858-ffae-42f5-a67a-defa774a6dd1"
created_at: "2026-08-31T15:13:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# P0 human-only approval enforcement completion report

## Outcome

Implemented the Task #158 security boundary on `origin/main` baseline
`090a287b`: an Agent bearer no longer derives an approval actor and legacy
body approval fields cannot authorize an enforced request. Every human-only
execution now requires an unexpired, unrevoked, single-use grant issued by an
authenticated human browser session after CSRF, active membership, capability,
fresh-preview, exact-command-hash, decision-snapshot, and Workspace-revision
checks.

The grant stores a reference identifier, not a reusable plaintext secret. It is
bound to the Workspace, account, actor, browser session, action/entity,
canonical command, decision snapshot, warning acknowledgements, proceed reason,
revision, and expiry. Consumption is atomic with the human-only command and is
audited. Logout, account-session revocation, membership deactivation, explicit
revocation, expiry detection, mismatch, and replay are rejected and audited.

Delegated acceptance is migrated to `human_required`; new delegated policies or
Tasks are rejected. Evidence reporting remains evidence-only and cannot confirm
a Task. Historical delegated and `task.auto_confirmed` constants remain only so
append-only historical events can still be read; no runtime path emits or
selects them.

## Implementation evidence

- Implementation commit: `8c6dd39404665fb31180e981225bd1ace855f15d`
- Migration: `server/migrations/00023_human_approval_grants.sql`
- Browser issue/revoke API: `POST`/`DELETE /v1/workspaces/{workspaceId}/approval-grants`
- Human approval UI: `src/components/WorkspaceAccess.tsx`
- Required MCP executable: `C:\dev-bin\baley\baley-mcp.exe`
  - SHA-256 `D413306DD6C0D50B02D5FB5CB2D1A72126F15E8F9B1E474637E04209EC5F8FDE`
- Validation server executable: `C:\dev-bin\baley\baley-server-task158.exe`
  - SHA-256 `132864952FA26C48A0D4CBEAD3268CFEA27D6009338F2C974FA9CBCD2C567F50`

## Verification

- `go test ./...`: pass.
- Isolated PostgreSQL 17, migrations 1 through 23, then
  `go test ./integration -count=1 -timeout 10m`: pass in 12.182s. The temporary
  container `baley-task158-pg-2fa0cc43` was removed after the run.
- The grant integration matrix covers all ten executable human-only actions and
  rejects Agent-without-grant, forged UUID, legacy body authority, stale,
  replayed, cross-Workspace, command/proceed mismatch, explicitly revoked,
  expired, session-revoked, and membership-deactivated grants. Positive
  authenticated-human-session issue/consume tests pass.
- `npm test -- --run`: 16 files and 94 tests pass.
- `npm run build`: pass; the pre-existing large-chunk warning remains.
- `git diff --check`: pass.
- Source scan: no `ApprovalActorID`/`approvalActorId` derivation remains.

## Deployment and real-use checks

- Docker image rebuilt from this worktree and deployed as Compose project
  `baley`.
- Migration 23 applied successfully; `/readyz` returned
  `{"schemaVersion":23,"status":"ready","version":"dev"}`.
- `baley-api-1` and `baley-viewer-1` are healthy/running; viewer returned HTTP
  200.
- An unauthenticated live grant issuance attempt returned HTTP 401 and wrote no
  grant.
- The external Chrome route reached the deployed login UI. No signed-in human
  browser session was available for a live positive grant. The external ignored
  Google OIDC client-secret file was also absent from this independent worktree,
  so the optional Google provider was deployed disabled as a unit rather than
  inventing or exposing a secret. Existing sessions and approval enforcement
  remain available; new Google sign-in requires the operator to restore the
  ignored mount and restart Compose.
- The first image restart exposed CRLF on the shell entrypoint; the Dockerfile
  now normalizes the copied script before execution. The first isolated database
  retry lacked pre-applied migrations; the final isolated run applied all 23
  migrations before the passing suite.

## Baley evidence

- Detailed-plan Record: `0c93ea77-1e62-45af-9d62-57c879dc8501`
- Detailed-plan Run: `ff8c50df-cbe0-4263-a851-77c482e29b21`
- Initial expired implementation Run: `95b40de7-9d99-4170-b33c-bedfc9d1b190`
- Completing implementation Run: `62fac858-ffae-42f5-a67a-defa774a6dd1`
- Completion Record: `b83bd5c3-07e4-4cbc-aa43-2e0a6b482d37`

No Task was confirmed, and no independent review was started or recorded, per
the Task instruction.

## Residual risks

- Restore `.tmp/local-pilot/secrets/google_oidc_client_secret` from the
  operator-managed secret source and redeploy to re-enable new Google logins.
- Expired grants are unusable immediately by timestamp validation; their
  persisted `expired` status and expiry audit are materialized on the next grant
  or command operation rather than by a background timer.
- Historical delegated/auto-confirm event validators remain for append-only
  audit compatibility and must not be reintroduced into current command paths.
- The viewer production bundle remains about 1.94 MB before gzip and Vite emits
  its existing chunk-size warning.
- Compose build output reports npm dependency audit debt; dependency remediation
  is outside this P0 authorization change.
- The container build does not stamp a Git SHA, so `/readyz` reports version
  `dev`; commit identity is supplied by Git/Task Record evidence instead.
