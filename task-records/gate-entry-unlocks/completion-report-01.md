---
baley_record: 1
record_id: "e1e097ad-c9e5-481d-89f0-646eac73e77e"
task_id: 129
task_key: "gate-entry-unlocks"
record_type: completion-report
run_id: "037ac240-1589-4f07-a556-1e4a2872ede9"
created_at: "2026-07-27T00:07:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #129 completion report

Gate entry/unlock bindings are complete.

- Explicit to-Phase entry bindings are stored separately from from-Phase Gate conditions.
- With no explicit binding, all same-Phase DAG roots are projected deterministically and never persisted.
- Entry changes preserve Task/dependency/readiness boundaries and are rejected after Gate passage.
- Gate criteria revision, decision hash, pass preview, and pass Event evidence bind the resolved entry set and source.
- CLI, typed MCP attach/detach, HTTP graph data, Viewer unlock direction, contracts, and Operator documentation are aligned.
- migration 00013 safely removes legacy persisted automatic rows before enforcing explicit-only storage.
- the original detailed-plan metadata now targets Task #129 and is registered in Baley.

Validation: full Go tests and vet pass; the isolated PostgreSQL suite passes including migration upgrade and empty-target entry cases; frontend 10 files/39 tests and production build pass; final independent review is PASS with no findings.
