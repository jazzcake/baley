---
baley_record: 1
record_id: "adca0c2d-1467-4f82-bb5b-974aa1837a7f"
task_id: 117
task_key: "embedding-contract"
record_type: completion-report
run_id: "e53d1a20-20b2-447a-9e38-c9f4587ef723"
created_at: "2026-07-26T19:13:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: "5bd1b66f-0f75-47e0-b1a5-2f36fd55aeed"
---

# Embedding 범위 및 성공 기준 계약 완료보고 02

## 결론

Task #117의 PM 승인 Embedding Contract 기준선을 완료했다. 계약은 현재 V1의 사람
확인 규칙을 유지하면서, 향후 위임형 Task confirmation을 별도 Enablement Task #130으로
명확히 분리한다.

## 전달 결과

- `docs/embedding-contract-draft.md`를 PM-approved baseline으로 확정
- 단일-repository Adoption 범위, 비범위, 성공 기준 확정
- 기능 수용·디자인/UX·code-review/sign-off는 `human_required`, 순수 기계 검증 Task만
  사전 위임 시 `delegated`라는 분류 기준 확정
- inherit resolve 시점, effective mode/policy version 동결, 사람만 delegated 권한을
  부여·완화할 수 있는 규칙 확정
- typed evidence provenance의 서버 검증 범위와 의미 품질 비판정 경계 확정
- acceptance mode가 Gate/Lane/Workspace/Task discard 권한을 바꾸지 않음을 명시
- #130 `위임형 Task acceptance 정책 구현` 생성, 관계 `#120 → #130 → #123` 등록
- 기존 #129 Gate entry/unlock binding scope를 manifest와 #120 checklist에 반영

## 검증 및 독립 리뷰

- `git diff --check`: PASS
- 독립 Agent 리뷰 최초 Blocking 4건을 기록하고 review response로 모두 해결했다.
- 재검토 결과 #130 ownership, policy authority/inherit, typed evidence, Gate/Lane 예외에
  unresolved blocking finding이 없다.

## 다음 단계

- #118: #130을 위한 executable acceptance policy/schema/event/API contract 상세화
- #119: evidence·복원·Pilot 측정 runbook 상세화
- #120: #118/#119, #129, #130을 통합한 Enablement entry checklist 확정

## 확인 경계

#117은 운영·수용 계약을 확정하는 `human_required` Task다. 사람의 Task confirmation만
남아 있다.
