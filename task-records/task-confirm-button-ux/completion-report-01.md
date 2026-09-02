---
baley_record: 1
record_id: "b03e99f9-9da3-4d2c-a48d-8ed1b8b2bfcc"
task_id: 161
task_key: "task-confirm-button-ux"
record_type: completion-report
run_id: "ecf329b9-c1de-452a-8cc8-05c163316fff"
created_at: "2026-09-02T12:00:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #161 completion report

## Delivered

- Added an implemented-Task confirmation panel to the Task Inspector. An
  Owner or Approver first reviews a fresh server preview and then makes one
  explicit `Confirm task once` decision; other roles receive a clear notice.
- Preserved the P0 human boundary: the browser flow uses the signed-in human
  session, CSRF protection, a short-lived single-use approval grant, the exact
  command hash and current Workspace revision. The client never manufactures
  an Actor or exposes approval JSON as the normal workflow.
- Bound preview, grant, and execution to the currently selected Workspace,
  Task, revision, entity version, and capability. Target changes revoke an
  issued grant and block stale execution.
- Shows the implementation assessment, successful Runs, indexed Task Records,
  and only the latest acceptance-evidence version. Warnings require explicit
  acknowledgement and a reason.
- Added development-only structured traces at the event, calculated target,
  React/application state, approval-controller state, and rendered DOM
  boundaries. Also fixed the generic access-panel checkbox event snapshot.
- Updated the canonical Baley Skill and architecture/access documentation so
  the Task Inspector button is the default path and raw Command JSON is an
  advanced diagnostic fallback.

## Independent review

The first independent review found stale-target execution, old evidence
selection, capability-loss, and revoke-trace weaknesses. All findings were
fixed. The re-review passed with no blocking findings, including a race where
the selected Task changes while grant issuance is in flight.

## Verification

- Focused frontend/API integration set: 30 tests passed.
- Full frontend suite: 17 files, 103 tests passed.
- TypeScript typecheck and production build passed.
- Full Go suite, including PostgreSQL integration packages, passed.
- `git diff --check` passed.
- The Baley Codex plugin was validated and reinstalled as
  `0.1.0+codex.20260902023121`; canonical, source, and cache Skill hashes match.
- Docker Compose rebuilt and deployed the Viewer/API. API health is ready,
  Viewer returns HTTP 200, schema 23 remains active, and the configured Google
  OIDC provider plus 30-day idle/90-day absolute session policy were retained.
- The deployed Viewer asset `/assets/index-B5WIbrvG.js` contains both
  `Confirm task` and `Confirm task once`. The live Chrome check reached the
  Google-only login page; the signed-in final confirmation was deliberately not
  performed because it is the user's human decision.

## Git evidence

- `43f264e6fcacfc244f87a415bc21ac554aeda0f8` — implementation
- `265963ca920ecd581cb750de629c1c4a7022522c` — review findings fixed
- `815effcd633cc1f9ddfbb28f93558b80e08b243c` — independent review records

## Remaining boundary

There are no implementation blockers. Task #161 is reportable as implemented,
but must remain unconfirmed until a signed-in human uses the Task Inspector
button or issues an equivalent explicit human command.
