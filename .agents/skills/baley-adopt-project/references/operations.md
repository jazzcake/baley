# Adoption pilot operations

## 1. Preflight

- Work in one Git repository and one Baley Workspace.
- Confirm `/healthz`, then use the authenticated session and Workspace list to
  verify membership. Health alone does not prove tenant access.
- Confirm the repository remote URL contains no user information or password.
- Confirm the Task Record root is relative, normalized, outside `.git`, and
  does not overlap `baley.yaml`, `.rgignore`, or `.baley-init-state.json`.

## 2. Workspace and repository binding

The current supported path is:

1. An authenticated Owner creates the Workspace from the Viewer Workspace
   menu. Creation atomically binds the creating account as Owner.
2. An Owner issues a Workspace-scoped Operator credential.
3. The Operator fresh-reads the Workspace and registers the Git repository
   using typed `repository.register`.
4. The installed `baley-project-init` CLI creates the local deterministic plan
   for `baley.yaml`, `.rgignore`, retry state, Task Record README, and all
   templates without overwriting user files.
5. After a rerun reports only `keep`, fresh-read the Workspace/repository
   binding and delete `.baley-init-state.json`.

`project.bootstrap` exists as a domain contract but is not exposed by the
current application/HTTP/MCP execution path. Use authenticated Workspace
creation, typed `repository.register`, and then run the CLI with
`bootstrapCompleted: true`. Do not use direct SQL as a substitute.

## 3. Secret handling

| Value | Allowed location | Forbidden location |
|---|---|---|
| Password | hidden login input | files, command JSON, logs |
| Agent token | process environment / Authorization header | `baley.yaml`, Records |
| Approval grant | one approved execute envelope | previews, Records, chat summary |
| Run lease token | process memory / heartbeat request | config, Records, logs |
| Lease-token server secret | server environment | repository and client |

An idempotency key is audit metadata and may be stored verbatim in the command
table. Never use a secret as an idempotency key.

## 4. Session recovery

1. Fresh-read the Workspace, lane brief, and Run list.
2. If the Run is still `running`, retry `run.start` with the same client Run ID
   and identical payload to recover the deterministic lease.
3. If it is expired or `interrupted`, do not rewrite it. Start a new Run and
   link it to the interrupted Run where supported.
4. Compare current Git head/worktree with registered Record and commit
   evidence. A mismatch is a reported state, not an automatic repair request.
5. Continue only after the next action and evidence binding are unambiguous.

## 5. Approval boundary

Delegated technical Tasks may auto-confirm only after typed verification and
independent-review evidence satisfy their immutable assignment. Human-required
Tasks stay implemented until an authenticated person confirms them. Gate pass,
active Gate condition changes, Lane close/discard, and Workspace close are
always separate human decisions based on a fresh preview.

## 6. Pilot measurement

Create one `pilot-measurement` Record per sample. Required fields are validated
by `scripts/validate_pilot_measurement.py`. `accepted_candidate_ids` must be a
subset of `candidate_ids`; all arrays are duplicate-free; timestamps are UTC
RFC 3339; and `workspace_revision` is the revision observed for the sample.

If a sample is corrected, create a new measurement Record. Put the Event IDs
that explain the correction in `correction_event_ids`; never replace the
registered original.
