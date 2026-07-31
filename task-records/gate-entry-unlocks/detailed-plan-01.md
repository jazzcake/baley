---
baley_record: 1
record_id: "4b82164b-9bba-4b50-9d62-1d96f358b0ce"
task_id: 129
task_key: "gate-entry-unlocks"
record_type: detailed-plan
run_id: "f79e0929-01c3-43bc-8354-87ceea888106"
created_at: "2026-07-26T23:54:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Gate entry and unlock bindings — detailed plan

## Outcome

Represent the work a passed Gate makes available separately from its
from-phase completion conditions.  An Operator can bind one or more tasks in
the Gate's `toPhase` as entry/unlock tasks.  When no explicit binding exists,
the graph projects a deterministic to-phase DAG root as an automatic,
correctable entry choice.

## Design boundaries

- `gate_tasks` remains the only readiness/condition relationship and still
  requires a task from `fromPhase`.
- `gate_entry_tasks` contains only `toPhase` tasks.  It cannot make a task
  start, alter a dependency, or make a Gate ready.
- The pass preview and audit event snapshot the resolved entry set, including
  whether each entry was explicit or automatic.  The human approval for a
  pass is therefore bound to the next work that will be shown as unlocked.
- Automatic selection is a stable projection, not a hidden database write:
  select all target-phase tasks with no incoming same-phase dependency, ordered
  by public ID.  An explicit binding replaces the automatic fallback.

## Work plan

1. Add schema/domain/projection support plus attach/detach commands that
   validate target-phase membership.
2. Resolve explicit-or-automatic entries in the snapshot and include them in
   Gate-pass decision evidence and hashes.
3. Expose typed MCP commands and Viewer/API graph links with Gate → Task
   direction for `unlocks`.
4. Update normative docs and Operator guidance; add unit, integration, MCP,
   and viewer regression coverage.
