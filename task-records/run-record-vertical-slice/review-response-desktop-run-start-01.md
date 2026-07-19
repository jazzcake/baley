---
baley_record: 1
record_id: "968a006c-d2ec-40e1-9166-2a2bae71c3cd"
task_id: null
task_key: "run-record-vertical-slice"
record_type: review-response
run_id: null
created_at: "2026-07-18T23:14:09.7947818+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Desktop Run Start 리뷰 반영

## 반영 내용

- raw lease token을 `commands.result`에서 제거했다.
- `BALEY_LEASE_TOKEN_SECRET`과 Run ID의 HMAC으로 token을 재구성한다. 같은 command/client Run 재시도는 같은 token을 반환하며 어떤 상태도 갱신하지 않는다.
- stable secret이 없으면 Repository open을 거부해 재시작·다중 instance에서 사용할 수 없는 token을 발급하지 않는다.
- client Run ID를 domain과 PostgreSQL 양쪽에서 UUID로 제한했다.
- parent Run의 존재·동일 Task 관계와 independent review target의 존재·Workspace·Task·kind 관계를 검증한다.
- 새 Workspace 기본 상태를 `draft`로 바꾸고 demo seed는 명시적으로 `active`를 기록한다.
- Gate Focus 기본값을 active Phase에서 나가는 Gate로 계산한다.
- command별 Event 순서를 `command_event_index`로 명시해 PostgreSQL 반환 순서에 의존하지 않도록 했다.

## 추가 검증

- 같은 idempotency key 및 같은 `clientRunId`/다른 key 재시도에서 같은 token과 같은 Run을 반환한다.
- 만료 lease 재시도가 expiry/version을 되살리지 않는다.
- raw token이 Run row, command 결과와 Event에 존재하지 않는다.
- invalid UUID, parent/target 관계, future Phase policy 실패는 transaction을 남기지 않는다.
