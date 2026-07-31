---
baley_record: 1
record_id: "7b750bff-5242-49db-856d-32696ef09ace"
task_id: 124
task_key: "embedding-enablement-acceptance"
record_type: detailed-plan
run_id: "7087a0c1-5012-42f7-9438-0555bb4672ed"
created_at: "2026-07-30T15:27:41.6398576Z"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #124 detailed plan

## Goal

Validate the complete Embedding Enablement contract in a disposable
single-repository scenario, preserve measurement and review evidence, and prove
that delegated Task acceptance cannot cross human-only authority boundaries.

## Execution

1. Add one coherent PostgreSQL scenario that creates a delegated Task, starts
   and completes its Run, registers completion/review/PilotMeasurement records,
   reports typed evidence, and observes automatic Task confirmation.
2. In the same snapshot, assert that the active Phase, Gate, Lane, and Workspace
   remain unchanged and that a human-required Task stays implemented.
3. Assert append-only mutation-attempt evidence for the acceptance commands
   without storing request secrets.
4. Run the actual project-init CLI in a temporary existing project and verify
   guarded apply plus convergent rerun.
5. Execute migrations, focused PostgreSQL suites, full Go tests/vet, the
   PilotMeasurement validator, frontend tests, production build, and diff check.
6. Store a real PilotMeasurement sample and completion evidence, then obtain an
   independent Agent review with zero blocking findings.

## Approval boundary

Task #124 remains human-required. Its confirmation and the subsequent G#4 pass
are distinct human decisions; passing this acceptance suite does not perform
either action automatically.
