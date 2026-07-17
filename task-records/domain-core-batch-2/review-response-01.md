---
baley_record: 1
record_id: "e55293e4-4853-41aa-b353-45f3d389071a"
task_id: null
task_key: "domain-core-batch-2"
record_type: review-response
run_id: null
created_at: "2026-07-17T23:04:06.8719617+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Domain Core Batch 2 리뷰 대응

## 리뷰 결론

초기 독립 리뷰는 변경 요청이었고, 보고된 3개 발견사항을 모두 반영했다. 동일 리뷰어의 재검토에서 세 항목이 모두 해소됐으며 새 correctness, invariant, atomicity 회귀는 발견되지 않았다.

## 발견사항별 대응

### 초기 그래프 오류 폐기

- `NewDependencyGraph`가 `(*DependencyGraph, Evaluation)`을 반환하도록 변경했다.
- 초기 그래프에 error가 있으면 graph 대신 `nil`을 반환한다.
- 누락 Task 참조, 중복 edge, cycle을 각각 검증하는 회귀 테스트를 추가했다.

### discarded Task의 blocker metadata 잔존

- blocked Task를 discard할 수 있는 lifecycle 의미는 유지했다.
- 성공한 discard 전이에서 `BlockedAt`과 `BlockerReason`을 함께 제거한다.
- terminal Task에 blocker metadata가 남지 않는 회귀 테스트를 추가했다.

### 계약 밖 Run kind 허용

- `EvaluateRunStart`가 계약에 선언된 Run kind인지 먼저 검증한다.
- 알 수 없는 kind는 계약에 이미 존재하는 `invalid_state_transition` hard error로 거부한다.
- unknown kind 회귀 테스트를 추가했다.

## 재검토

- 기존 3개 발견사항: 모두 해소
- 새 차단 발견사항: 없음
- 리뷰어 재실행 `go test ./...`: 통과
- 비차단 주석 불일치: 즉시 정정

명세를 완화하거나 발견사항을 숨기지 않았으며, 초기 리뷰 Record는 원문 그대로 보존했다.
