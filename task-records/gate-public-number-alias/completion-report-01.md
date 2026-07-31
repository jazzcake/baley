---
baley_record: 1
record_id: "88884c6a-95ac-44d4-85ae-c6ddb638b4e0"
task_id: 131
task_key: "gate-public-number-alias"
record_type: completion-report
run_id: "97197c80-9753-4557-9a08-3ecd2c31fa6b"
created_at: "2026-07-30T00:21:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #131 completion report

Gate public numbers and aliases are implemented end to end.

- Every Gate has a stable, positive public number scoped to its Workspace and is
  displayed as `G#<n>`.
- Existing Gates are backfilled deterministically by from-Phase position and
  internal Gate ID. The Workspace counter starts at `max(public_id)+1`.
- Public numbers are allocated inside the Workspace mutation transaction and
  are not reused after successful creation. Failed transactions do not consume
  a number.
- A Gate can have an optional canonical lowercase alias that is unique in its
  Workspace without changing its stable internal `gateId`.
- HTTP, CLI, typed MCP, and Viewer resolve internal `gateId`, `G#<n>`, and alias.
  Viewer cards, routes, Task relations, and Inspector prefer `G#<n>` while
  retaining alias and internal ID evidence.
- Canonical `G#[1-9][0-9]*` is reserved for public references. It cannot be a
  new internal Gate ID, and an unknown public reference never falls back to an
  internal ID or alias.
- Gate pass, attach, entry, capability, and human-approval semantics are
  unchanged.

Validation:

- full Go test suite and `go vet`;
- PostgreSQL migration/integration suite, including deterministic backfill,
  uniqueness, concurrent creation, stale retry, and rollback allocation;
- typed MCP schema and stdio E2E coverage;
- frontend 14 files / 56 tests and production build;
- Baley Operator skill validator;
- `git diff --check`;
- final independent Agent review PASS with no remaining findings.

Deployment note: migration 15 has been validated on the disposable test
database but is intentionally not applied to the live Pilot database in this
working-tree task. Stop the old server and MCP process, rebuild both binaries,
run migration 15, restart them, and open a fresh MCP thread so the additive
schema is loaded atomically.
