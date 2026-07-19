---
baley_record: 1
record_id: "f53f3209-aed1-42f5-ab45-795327b6e48d"
task_id: null
task_key: "run-record-vertical-slice"
record_type: independent-agent-review
run_id: null
created_at: "2026-07-18T23:14:09.7947818+09:00"
created_by: "independent-review-agent"
registration_state: pending-bootstrap
supersedes: null
---

# Desktop Run Start 독립 Agent 리뷰

## 범위

`run.start`의 domain/application/PostgreSQL/HTTP/MCP 수직 단위와 Viewer baseline 수정, migration 및 통합 테스트를 독립적으로 검토했다.

## 발견 및 반영 확인

초기 리뷰에서는 다음 항목을 발견했다.

- 높음: raw lease token이 `commands.result`에 저장되어 secret 경계가 무너짐.
- 중간: `clientRunId` UUID, parent/target Run 관계, 새 Workspace 기본 상태 검증이 부족함.
- 중간: Gate Focus가 active Phase의 outgoing Gate가 아니라 배열상 첫 미통과 Gate를 선택함.

raw token 저장을 제거한 첫 수정은 응답 유실 시 재시도로 token을 복구하지 못해 Run이 고아가 되는 새 높은 문제를 만들었다. lease 회전 방식은 만료 Run 부활과 동시 응답 역전 가능성이 있어 채택하지 않았다.

최종 구현은 외부 secret과 Run ID의 HMAC으로 같은 token을 결정적으로 재구성한다. DB에는 hash만 저장하고 command 결과와 Event에는 raw token을 저장하지 않는다. 재시도는 Run lease/version, Workspace revision과 Event를 변경하지 않는다. UUID와 Run 관계 검증, Workspace 상태, Gate Focus 선택도 회귀 테스트로 고정했다.

마지막 재리뷰에서는 secret 누락 시 process별 임시 secret으로 시작하는 중간 문제를 발견했다. Repository가 DB 연결 전에 `BALEY_LEASE_TOKEN_SECRET` 누락을 거부하도록 바꾸고 fail-fast 테스트와 실행 문서를 추가한 뒤 다시 확인했다.

## 최종 판정

최종 재리뷰 판정은 별도 수정 요구가 없는 `CLOSE`다.
