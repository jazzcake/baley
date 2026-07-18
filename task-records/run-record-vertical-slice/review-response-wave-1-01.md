---
baley_record: 1
record_id: "fccd7466-8f3b-4bdb-b507-9c6a56821b8c"
task_id: null
task_key: "run-record-vertical-slice"
record_type: review-response
run_id: null
created_at: "2026-07-18T11:25:37+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Run/Record Vertical Slice Wave 1 리뷰 반영

## 반영 결과

독립 리뷰의 Medium 3건과 Low 2건을 모두 반영했다.

1. heartbeat가 이전 heartbeat 시각으로 역행하지 못하게 하고 기존 lease expiry에 양수 extension을 더해 단조 연장한다.
2. Run start planner가 Workspace lifecycle을 입력받아 active가 아닌 draft/closed Workspace를 거부한다.
3. 독립 Agent 리뷰 target을 제공할 때 존재, self-reference, Workspace, Task와 implementation kind를 검증한다. target 미보고는 계속 허용한다.
4. Workspace/Run/Record/Commit contract 값을 양방향 exact-set으로 검사하고 중복 literal도 거부한다.
5. heartbeat 후 stale terminal과 terminal 후 heartbeat를 직접 검증하는 순수 CAS 경쟁 테스트를 추가했다.

재리뷰에서 이전 5건이 모두 해소됐고 신규 High/Medium finding이 없음을 확인했다. 추가 Low finding인 contract 중복과 `run.terminal` 분류 drift도 exact-set 및 terminal set 검사로 반영했다.

## 검증

```text
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./internal/domain
```

모두 통과했다. PostgreSQL transaction과 MCP E2E는 후속 adapter 통합 범위다.
