---
baley_record: 1
record_id: "f8f21359-47b0-4e6a-ad30-9a5775aaa801"
task_id: null
task_key: "run-record-vertical-slice"
record_type: detailed-plan
run_id: null
created_at: "2026-07-18T22:35:42.0125226+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Desktop Run Start Persistence 상세계획

## 목표

환경 독립 P2-01~03 Run 모델을 실제 PostgreSQL command transaction에 연결한다. 이번 단위는 `run.start` 하나만 닫고 heartbeat, terminal transition, Record, Run query와 Viewer 표시는 다음 단위로 남긴다.

```text
run.start
→ Workspace row lock 및 revision 재검사
→ 현재 Task/Phase/dependency/blocker snapshot에서 RunStartPlan 재계산
→ pending Task를 in_progress로 전환
→ Run insert
→ task.started + run.started Event
→ Workspace revision 증가
```

## 구현 범위

- `workspaces.state`와 `runs` migration
- Run projection 및 PostgreSQL snapshot load
- `run.start` strict argument decode, preview와 execute
- server-generated Run ID와 lease token, DB에는 token hash만 저장
- 동일 command idempotency key와 client Run ID 재시도 결과 재사용
- raw token은 저장하지 않고 외부 secret과 Run ID의 HMAC으로 재구성하며 재시도에서 lease/version을 갱신하지 않음
- 동일 client Run ID의 다른 canonical identity 거부
- generic HTTP preview/execute contract 연결
- `baley_run_start` MCP tool
- PostgreSQL transaction 및 MCP tool catalog integration test

## 비범위

- heartbeat, succeed/fail/cancel/interrupt/correct
- stale lease interruption job
- Run query, Viewer Run panel
- Task Record와 Git metadata
- 인증 및 remote MCP

## 완료 조건

- migration `up/down/up`
- pending Task와 Run insert, 두 Event 및 revision 증가가 하나의 transaction
- invalid future-Phase implementation Run은 어떤 row도 쓰지 않음
- 같은 idempotency key 재시도에 중복 Run/Event 없음
- PostgreSQL integration과 MCP E2E skip 0건
- 전체 Go test/vet/race 및 웹 회귀 검증 통과
- 독립 Agent 리뷰와 finding 반영
