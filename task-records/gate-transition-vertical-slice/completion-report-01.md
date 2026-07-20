---
baley_record: 1
record_id: "6b7d348c-7fc1-4dc8-a904-81d2c5595e36"
task_id: 110
task_key: "gate-transition-vertical-slice"
record_type: completion-report
run_id: null
created_at: "2026-07-20T00:45:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Gate Transition Vertical Slice Completion Report

## Outcome

The Gate transition vertical slice is functionally complete. The Pilot Ready Gate is passed, Build is completed, Validate is active, and Task #110 was confirmed through a fresh MCP preview, explicit human approval, exact `dangling_path` acknowledgement, and MCP execute. The final Workspace revision is `11` and there are no remaining derived human decisions.

Baley records state transitions and evidence; it does not certify semantic implementation quality. The quality assessment below is based on automated tests, PostgreSQL/MCP integration evidence, independent review, and human acceptance.

## Implementation scope

- Domain and application behavior for Task confirmation, Gate readiness, Gate Task pass/revoke, Gate pass, Phase transition, warning acknowledgement, canonical command hashing, idempotency, revision binding, and approval attestation.
- PostgreSQL persistence for Workspace/Phase/Lane/Task/dependency/Gate/Run/Record/Git evidence, commands, Events, and approval attestations.
- HTTP query and preview/execute command surfaces.
- Typed MCP query and mutation tools, including the common execute envelope fields `acknowledgedWarningCodes` and `proceedReason`.
- Read-only React Viewer projections for Workspace graph, Task/Gate/Phase status, Runs, Record indexes, and Events.
- Run lease/heartbeat/terminal lifecycle, Task Record and Git index support, remaining graph commands, backup/restore, and independent fixture coverage accumulated in the working tree.

Migrations present in this slice are `00001_gate_transition_slice.sql` through `00006_lane_core_fields.sql`, including Run lifecycle, Record/Git indexes, Task core fields, audit/Record constraints, and Lane core fields.

## Warning acknowledgement adapter correction

`application.CommandEnvelope`, HTTP JSON decoding, MCP `executeEnvelope`, and MCP `executeEnv` now carry `acknowledgedWarningCodes` and `proceedReason` as common envelope fields. Both remain outside typed command arguments, the canonical command hash, and the request fingerprint. Warning-bearing `task.confirmed` Event evidence stores the evaluated warnings, exact acknowledgement set, and trimmed proceed reason.

Regression coverage proves that a dangling implemented Task previews with `dangling_path`; missing, unknown, or mismatched acknowledgement fails without Task, revision, command, approval, or Event mutation; and an exact acknowledged retry with the same idempotency key succeeds. Approval hash/revision/idempotency protections remain intact.

## Automated and integration evidence

- `go test -count=1 ./...`: passed.
- `go vet ./...`: passed.
- Real PostgreSQL integration on an isolated `baley_test` database: passed, including atomic warning failure and same-idempotency acknowledged retry.
- Actual MCP stdio E2E against an isolated loopback API: passed, including SDK schema decode, HTTP forwarding, `task.report_implemented`, confirm preview/execute, exact warning acknowledgement, approval hash binding, and final `confirmed` projection.
- Frontend tests: 13/13 passed.
- Production frontend build: passed with the pre-existing bundle chunk-size warning.
- Baley Skill validation: passed.
- `go test -race ./...`: not run because the Windows environment has `CGO_ENABLED=0`.

## Independent review and response

The superseding independent review is `independent-agent-review-02.md` (Record `5322d396-4da0-4e12-a851-33c1b20df6b0`). It found no High or Medium issues and reported two Low test-depth gaps. The superseding response is `review-response-02.md` (Record `5890513b-64ad-4e73-914a-3064ebc770b4`). Both Low findings were fixed: MCP stdio now executes the complete warning-bearing confirm flow, and PostgreSQL rollback assertions include Task status and approval-row absence. Focused PostgreSQL and MCP E2E checks passed again.

## Human acceptance and live Event evidence

Before #110 confirmation, live state was Workspace revision `10`, Validate active, Task #110 `implemented`, no active Run, and one derived `task.confirm` decision.

The fresh preview projected `implemented -> confirmed`, returned only `dangling_path`, and produced command hash `sha256:7c206a9bc3a29d58f9d8e42f6eec66313e22d52c4ba25ba76075dd971ef4f61a`. The human approver explicitly approved this transition and warning acknowledgement.

The successful command was `94caf1c2-0b28-4a75-9551-8a669a5697c0` and incremented the Workspace exactly once to revision `11`.

- `task.confirmed` Event `7d4679f0-c6a5-441e-9da9-6355775fd8c8` records warning `dangling_path`, exact acknowledgement `dangling_path`, and proceed reason: Task #110 is the intentional terminal user-acceptance Task in Validate and has no expected successor.
- `human_approval_attestation.recorded` Event `c4ab5277-7154-4116-b0fc-5210706e5049` separately records action `task_confirm`, entity `user-test`, approved command hash, expected revision `10`, approver `00000000-0000-4000-8000-000000000002`, and executing Operator `00000000-0000-4000-8000-000000000003`.
- Post-execute reads show Task #110 `confirmed`, Workspace revision `11`, Validate active, and an empty decision list.

The earlier failed executions produced no state change. The original no-ack adapter failure left Workspace revision `10` and Task #110 `implemented`. In the close-out thread, an execute sent to the stale default API was rejected as an unknown `proceedReason` field before command evaluation; subsequent reads still showed revision `10`, and the only new confirmation Events are the two successful revision-11 Events above.

## Viewer evidence

Earlier human visual acceptance verified stable viewport decorations, repeated drag behavior, Lane bands/cards, zoom and fit controls, polling/layout race protection, and the #110 inspector with one interrupted and one succeeded Run plus no indexed Task Records. After confirmation, the final server projection was verified through MCP. Browser automation was unavailable during final close-out because no in-app or extension browser backend was present, so a new visual screenshot was not substituted or claimed.

## Records and registration

The live Baley graph has no Repository or Task Record indexes. This report and the related plan, handoff, reviews, and responses therefore remain `pending-bootstrap`. They were not registered by editing fixtures, the database, or application state. Registration can occur only after a real Repository is registered through Baley and the Record command prerequisites exist.

## Operational status and residual risks

- The default API at `127.0.0.1:8080` is still an older privileged process that rejects `proceedReason`. The current process could not be stopped from this session (`Access is denied`). The approved confirmation was executed through the updated typed MCP server against a temporary loopback API on port 8081 using the same PostgreSQL database; the temporary server was stopped afterward. The default 8080 service must be restarted from current source with its existing stable `BALEY_LEASE_TOKEN_SECRET` before the next warning-bearing mutation.
- V1 HumanApprovalAttestation is protocol audit metadata, not authenticated proof of human identity. Deployment-level access protection remains required.
- The Go race detector was unavailable on this Windows/CGO configuration. PostgreSQL locking, CAS, idempotency, and rollback are covered by real integration tests but do not replace a race build on a supported environment.
- The frontend retains the existing large-chunk warning.
- The working tree intentionally contains broad tracked and untracked changes from multiple related slices. No reset, branch, worktree, commit, or push was performed, and attribution cannot be reconstructed from the dirty tree alone.

## Roadmap transition

Gate V2 evidence is complete: API/DB consistency, deterministic invariant enforcement, reconstructable Gate/approval Events, revision persistence, and independent fixture modeling are covered. The next slice is Phase 3 entry: register the real repository and Task Record root, bootstrap the pending Record indexes, complete CLI/operator integration, and exercise multi-repository Commit/Record evidence without expanding Baley into Branch/worktree lifecycle management.
