---
baley_record: 1
record_id: "0d4bc747-3e73-43a0-a34a-7d3bff35c716"
task_id: 161
task_key: "task-confirm-button-ux"
record_type: review-response
run_id: "7d95fe5c-9a8e-4f14-946f-fb34f1ee4ea5"
created_at: "2026-09-02T11:35:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Task #161 리뷰 반영

## 반영 결과

- High: Task·public ID·Workspace revision·권한을 scope key로 묶고 generation이 다른 preview 응답을 폐기했다.
  최종 실행 직전 command arguments와 preview entity까지 현재 Task에 다시 결속했다.
- Medium: 가장 높은 evidence version 하나만 표시하고 verification, review, unresolved blocker를 함께 노출했다.
- Low: approval capability를 잃으면 scope를 무효화하고 열린 preview와 실행 버튼을 제거했다.
- Low: preview/grant/execute 실패 단계와 grant revoke 성공·실패를 구조화 trace로 추가했다.

## 추가 검증

- A preview가 B 선택 뒤 늦게 도착해도 무시되는 회귀 테스트
- grant 발급 중 Task가 바뀌면 revoke하고 execute하지 않는 회귀 테스트
- 최신 evidence fail 및 blocker 표시 테스트
- approval capability 회수 시 preview·버튼 제거 테스트
- 관련 30개 테스트, 전체 프런트엔드 103개 테스트, typecheck와 production build 통과
- 전체 Go 테스트 통과
