---
baley_record: 1
task_id: 184
record_type: independent-review
reviewed_commit: "9ac96f48845ea0ea7537e82914db27a9552cac13"
created_at: "2026-09-10T02:10:00+09:00"
created_by: "codex-worker-term_cdcae52e"
verdict: CHANGES_REQUIRED
---

# Task #184 independent review 01

## Verdict

**CHANGES_REQUIRED**

Migration 26/27과 Task Journal의 application/API/MCP/Viewer 계약은 핵심 독립 테스트를 통과했다. 운영 schema 25에서 읽기 전용으로 재조회한 Task #183 Event 7건과 fixture의 Event ID, command 연계, payload, entity, revision, actor, approval, 시각도 일치했다. 그러나 rollout helper와 runbook의 restore 및 rollback 경계에 운영 데이터 손실 또는 API outage를 만들 수 있는 blocking finding이 있고, lifecycle canary 절차가 acceptance contract를 끝까지 검증하지 않는다.

## Findings

### F1 — BLOCKER — 기존 restore DB를 생성하지 못해도 삭제한다

- **File:line:** `scripts/task-journal-rollout.ps1:72-87`
- **Repro:** 허용 형식(`baley_task184_restore_YYYYMMDDHHMMSS_deadbeef`)의 DB를 미리 만들고 보존할 marker row를 넣은 뒤 같은 이름으로 `-Action VerifyRestore`를 실행한다. `createdb`는 이미 존재한다는 이유로 실패하지만 `finally`가 무조건 `dropdb --if-exists --force`를 실행해 이 호출이 생성하지 않은 DB까지 삭제한다.
- **Impact:** 오타, 재시도, 병렬 restore drill 또는 이름 재사용 시 기존 검증 DB와 그 안의 증거를 파괴한다. 이는 정확한 target 확인 후에도 소유권을 증명하지 못한 자원을 삭제하는 서버/DB 안전성 위반이다.
- **Fix condition:** 이 invocation이 DB 생성에 성공했음을 별도 상태로 기록하고 그 경우에만 정리한다. 생성 전 존재 확인에서 즉시 중단하고 기존 DB를 절대 삭제하지 않는 회귀 테스트를 추가한다.

### F2 — BLOCKER — schema 27에서 pre-rollout API 이미지로 rollback하면 readiness가 실패한다

- **File:line:** `scripts/task-journal-rollout.ps1:106-112`, `docs/task-journal-rollout-operations.md:67-75`, `server/cmd/baley-server/main.go:30`
- **Repro:** migration 27 적용 후 `Rollback`에 rollout 직전 API image ID를 전달한다. 현재 신규 API는 schema 27을 정확히 요구하고, 직전 commit `7a15b57`의 API는 schema 26을, 현재 운영 기준 image는 schema 25를 요구한다. helper는 DB를 27에 둔 채 이전 API를 기동하므로 `/readyz`가 version mismatch로 실패한다.
- **Impact:** service smoke 실패를 복구하려는 절차가 API를 unhealthy 상태로 만들어 Viewer와 MCP의 정상 경로까지 중단한다. `docker compose up` 성공만 확인하므로 helper 자체는 성공처럼 끝날 수 있다.
- **Fix condition:** schema 27 호환성이 입증된 rollback API만 허용하거나 API rollback을 금지하고 현재 schema-27 API를 유지한 채 Viewer/MCP만 되돌린다. rollback 후 container health, `/readyz`, `/versionz`를 검사해 실패 시 성공을 반환하지 않으며, runbook의 이미지/DB compatibility matrix를 실제 artifact SHA와 함께 명시한다.

### F3 — HIGH — restore drill이 백업의 동일성과 복원된 데이터 진실성을 검증하지 않는다

- **File:line:** `scripts/task-journal-rollout.ps1:68-83`, `docs/task-journal-rollout-operations.md:13-27`
- **Repro:** 원래 `backup.json`과 무관한 다른 schema-25 custom dump를 `-BackupFile`로 전달한다. `VerifyRestore`는 파일 SHA-256을 metadata와 비교하지 않고, 복원 후 schema version만 검사하며 table count는 출력만 한다. 따라서 데이터가 다른 유효한 schema-25 dump도 PASS처럼 종료된다.
- **Impact:** migration 직전 복구 가능성 gate가 잘못된 dump, 오래된 dump, 또는 내용이 다른 dump를 승인할 수 있다. RPO 경계와 #183/#184/Event/command/Workspace revision 증거가 복원됐다는 보장이 없다.
- **Fix condition:** backup metadata 경로를 명시적으로 입력받아 dump SHA-256을 복원 전에 대조하고, 복원 후 metadata의 모든 table count, Workspace revision, #183/#184 identity/status 및 고정 Event/command/approval IDs를 자동 비교한다. 하나라도 다르면 non-zero로 종료하고 migration을 막는 테스트를 추가한다.

### F4 — HIGH — 운영 lifecycle canary가 acceptance와 human approval 경계를 완주하지 않는다

- **File:line:** `docs/task-journal-rollout-operations.md:63-65` (대조: `docs/task-records/184/detailed-plan-02.md` §8.2, §10)
- **Repro:** runbook을 문서 그대로 수행하면 canary는 create와 run start 뒤 “new Event-backed row” 하나의 노출만 확인한다. acceptance가 요구하는 `task.report_implemented`, signed-in Viewer의 별도 human `task.confirm`, API/Viewer/MCP 간 전체 Event/Journal/command/provenance 일치, 동일 idempotency retry와 conflict/stale-grant 무기록 검증 절차가 없다.
- **Impact:** 신규 write path나 approval binding이 배포 artifact에서 깨져도 canary가 성공으로 기록될 수 있다. 특히 agent가 확인을 대신하지 않는 human approval 경계를 운영 단계에서 입증하지 못한다.
- **Fix condition:** canary 절차에 create → run.start → report_implemented → signed-in human confirm을 분리해 명시하고, 각 단계의 Event ID, Journal ID, command ID, actor/time/context를 HTTP·Viewer·MCP에서 대조한다. 동일 retry, changed-payload idempotency conflict, stale revision/grant의 zero-write 검사와 human-only confirm stop point를 포함한다.

## Verified evidence

- Reviewed full commit range `7a15b57..9ac96f4`; source code was not modified.
- Live PostgreSQL was queried read-only and remained at schema 25. Task #183 fixture matched the seven actual Events, including paired `task.started`/`run.started` from one command and the confirmation approval linkage.
- Disposable PostgreSQL 17 database: `TestMigration26TaskJournalUpDownAndAppendOnly`, both migration-27 tests and all six fail-closed subcases, `TestRunStartAgainstPostgres`, and all Task Journal lifecycle/atomicity/idempotency/pagination/workspace-isolation/approval tests passed. The exact disposable DB was removed afterward.
- `go test ./internal/application ./cmd/baley-mcp -count=1`: PASS.
- `go vet ./...`: PASS.
- `npm test -- --run`: PASS, 17 files / 108 tests.
- `npm run build`: PASS; existing large-chunk warning remained.
- `git diff --check 7a15b57..9ac96f4`: PASS.

The migration's deterministic projection, append-only triggers, transaction rollback, byte-equivalent idempotent replay, paired cursor query, Workspace scoping, approval provenance, and non-invention of historical narrative all passed the inspected and executed cases. No new mandatory user input, Decision object, or separate Journal write UI was introduced by the reviewed commit; the Viewer remains read-only and bounded to the newest page while HTTP/MCP retain cursor traversal.

## Remaining acceptance

After F1–F4 are corrected, independently rerun the focused database suite and add direct tests for restore ownership/non-deletion, metadata/hash/data comparison, and rollback readiness. Production backup/restore, immutable artifact verification, migration, API/Viewer/MCP/Tailscale smoke, full lifecycle canary, signed-in human confirmation, and post-deploy backup/restore remain operator-owned work and were intentionally not performed in this review.
