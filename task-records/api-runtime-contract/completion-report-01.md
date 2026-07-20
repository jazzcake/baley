---
baley_record: 1
record_id: "17ea70eb-46b5-4957-8a57-62f3eed87626"
task_id: 111
task_key: "api-runtime-contract"
record_type: completion-report
run_id: "4f7c4091-0431-487c-a312-e1bb4285f7fb"
created_at: "2026-07-21T00:31:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# API Runtime Contract Alignment Completion Report

## Outcome

Task #111 established a live current-source Baley API and warning-capable MCP contract, registered the repository and historical bootstrap evidence, and completed independent review. Human Task confirmation remains intentionally pending.

## Delivered

- Added typed `baley_task_create_preview` and `baley_task_create_execute` MCP tools.
- Added schema, forwarding, write-free preview, warning acknowledgement, idempotency, relationship, and Event-correlation coverage.
- Fixed execute serialization so empty optional `acknowledgedWarningCodes` and `proceedReason` fields are omitted.
- Created Task #111 through the official MCP preview/execute contract with `#110 → #111` and no warnings.
- Registered the `baley` repository with `task-records` as its Record root.
- Registered the Gate-transition and MCP-bootstrap Task Records without copying their bodies into Baley.
- Built the current source to `C:\tmp\baley-runtime\baley-server.exe`, bound it to loopback port 18080, and persisted user-level `BALEY_SERVER_URL` plus a non-repository lease secret.

## Verification

- `go test -count=1 ./...`: PASS
- `go vet ./...`: PASS
- focused MCP and integration tests: PASS
- frontend tests: 13/13 PASS
- frontend production build: PASS, with the existing large-chunk warning
- `git diff --check`: PASS, with line-ending notices only
- live Workspace read on current-source port 18080: PASS
- typed `task.create` preview and execute on current-source port 18080: PASS
- independent review: High 0, Medium 0, Low 0

## Runtime decision and residual risk

The pre-existing port 8080 listener is an elevated process that Windows refused to terminate even after exact listener PID, executable, and parent verification. It was preserved instead of being forcefully bypassed. The effective MCP runtime for new user processes is current-source port 18080 through the persisted user-level `BALEY_SERVER_URL`.

Clients that explicitly use port 8080 or do not inherit the user environment can still reach the legacy contract and a different lease-secret context. Cleanup remains an owning-launch-context operation. Independent review classified this split-routing risk as non-blocking for Task #111 because the current-source runtime is loopback-only, live, verified, and selected for new MCP processes.

## Baley evidence

- Task create Command: `c2e5c9d7-369f-4f69-95f1-08f13e4562b1`
- Task created Event: `30aaaa20-09b6-44f2-9630-cfd90ce03643`
- Implementation Run: `e589d695-ac21-40d5-9f9a-e9183977e872`
- Successful planning Run: `0a739e25-b3ea-482c-ba33-c76a1913d9f4`
- Corrected successful review Run: `939f12f7-a963-4287-a823-13cf203503c3`
- Successful review-response Run: `50d06c62-f932-4b45-97a7-0a97fa703564`
- Completion-reporting Run: `4f7c4091-0431-487c-a312-e1bb4285f7fb`

## Remaining authority boundary

The Agent may report Task #111 implemented but must not execute `task.confirm`. Confirmation is the only remaining human operation.
