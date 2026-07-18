---
baley_record: 1
record_id: "9dc2f8f8-a9ea-4a02-9568-9b1bba539c96"
task_id: null
task_key: "run-record-vertical-slice"
record_type: completion-report
run_id: null
created_at: "2026-07-18T10:58:02+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Run/Record Vertical Slice Wave 1 완료보고

## 결과

Phase 2 선행 구현 계획의 Wave 1인 P2-01, P2-02와 P2-03 순수 domain 모듈을 구현하고 Go 단위 검증을 완료했다. PostgreSQL persistence, HTTP, MCP와 Viewer는 이번 Wave 범위에 포함하지 않았다.

## 구현

### P2-01 — Run lifecycle

- Run kind와 별개인 Run status typed model
- running에서 succeeded/failed/interrupted/cancelled로의 terminal 전이
- 같은 terminal payload 재호출의 idempotent 결과
- 다른 terminal 결과 또는 summary의 conflict
- result/error summary 정규화와 필수 조건
- 이전 값·새 값·사유를 반환하는 manual correction
- Run status와 `contracts/v1/states.json`의 drift test

### P2-02 — Run identity와 lease

- Workspace, Task, client Run ID, kind, parent와 target을 포함한 start identity
- 같은 client Run ID의 다른 identity를 idempotency conflict로 판정
- raw lease token을 보존하지 않는 SHA-256 hash
- constant-time lease token 비교
- heartbeat의 token, version CAS, expiry와 extension 검증
- lease boundary와 stale Run interruption
- heartbeat와 terminal mutation이 공유하는 Run version 증가 규칙

### P2-03 — Run start plan

- 기존 Task, Phase, blocker와 dependency policy를 결합한 순수 start planner
- pending Task를 in-progress로 자동 전환하는 계획
- `task.started`와 `run.started` Event 계획
- active/completed Phase에서 모든 Run kind 허용
- future planned Phase에서는 detailed planning만 허용
- terminal Task 시작 거부
- independent Agent review의 선택적 target Run provenance 보존

## 계약 변경

Run concurrency와 lease 오류를 Workspace revision 오류와 구분하기 위해 다음 error literal을 `contracts/v1/diagnostics.json`에 추가했다.

- `stale_run_version`
- `run_lease_mismatch`

domain diagnostic coverage test가 두 literal의 계약 존재를 확인한다.

## 자동 검증

통과:

```text
go test -count=1 ./internal/domain
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./internal/domain
go test -race -count=1 ./...
go test -count=1 -cover ./internal/domain
git diff --check
```

`go test ./...`와 전체 race 실행에서 모든 package가 compile되고 일반 테스트가 통과했다. Domain statement coverage는 89.1%다.

## 실행하지 않은 통합 검증

다음 기존 integration test는 환경변수가 없어 skip됐다.

```text
TestGateTransitionAgainstPostgres — BALEY_TEST_DATABASE_URL 미설정
TestMCPStdioListsAndCallsTools — BALEY_MCP_E2E 미설정
```

Wave 1 자체는 persistence와 transport를 만들지 않았으므로 다음 검증은 후속 desktop 통합 작업에서 수행한다.

- Run schema와 migration
- Workspace lock 안의 `run.start` + pending Task 자동 시작 transaction
- heartbeat와 terminal update의 PostgreSQL version CAS 경쟁
- 정상 종료와 lease timeout interruption 경쟁
- HTTP/MCP command shape와 structured error mapping

## Assessment

Wave 1의 환경 독립 domain 범위는 구현 및 단위 검증 완료다. Run lifecycle을 persistence에서 분리했으며, persistence adapter는 `RunStartPlan`과 Run transition 결과를 같은 transaction에 적용하면 된다.

독립 Agent 리뷰에서 High finding은 없었으나 Medium 3건과 Low 2건이 발견됐다. 따라서 이 보고의 완료 의미는 자체 단위 검증까지이며, 독립 리뷰 close-out은 아니다. 상세 findings는 `independent-agent-review-wave-1-01.md`에 보존한다.

## 잔여 위험

- terminal retry의 command-level idempotency key/fingerprint 검증은 아직 application/persistence에 연결되지 않았다.
- lease token 생성은 보안 random source를 가진 adapter 책임이며 이번 domain 모듈은 token hash와 비교만 수행한다.
- manual correction의 Event payload와 capability는 P2-09/application 연결에서 검증해야 한다.
- DB transaction과 실제 동시성 품질은 아직 검증되지 않았다.
