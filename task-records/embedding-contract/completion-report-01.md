---
baley_record: 1
record_id: "5bd1b66f-0f75-47e0-b1a5-2f36fd55aeed"
task_id: 117
task_key: "embedding-contract"
record_type: completion-report
run_id: "fafcb3f4-17cb-4d95-b8e1-e21ea6a23778"
created_at: "2026-07-26T19:00:24+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Embedding 범위 및 성공 기준 계약 완료보고

## 전달 결과

- `docs/embedding-contract-draft.md`를 PM 확정 계약 기준선으로 전환했다.
- Adoption의 단일-repository 범위, 비범위, 성공 기준을 고정했다.
- Task acceptance mode를 `delegated`, `human_required`, `inherit`으로 정의했다.
- 기능 수용, 디자인/UX 수용, code-review/sign-off의 사람 확인 원칙과 Gate/Lane/Workspace의
  항상-사람-승인 원칙을 고정했다.
- 자동 확인은 #121 Enablement 구현에서 evidence profile과 audit Event까지 갖춰 적용하며,
  현재 V1 runtime의 사람 확인 규칙은 바꾸지 않는다고 명시했다.

## 검증

- 계약 문서의 범위, 정책, evidence, Gate 예외, #120 checklist가 상호 모순 없는지 검토했다.
- `git diff --check -- docs/embedding-contract-draft.md task-records/embedding-contract/detailed-plan-01.md`: PASS

## 잔여 작업

- #118: acceptance mode를 command/service/MCP/Viewer에 구현하는 Enablement 요구를 상세화한다.
- #119: evidence·복원·Pilot 측정 runbook을 독립 문서로 구체화한다.
- #120: #118/#119와 독립 리뷰 결과를 합쳐 Enablement entry checklist를 확정한다.

## 확인 경계

이 Task는 제품 운영 계약을 고정하는 human_required 성격의 Task다. 구현 완료 보고 뒤
사람의 `task.confirm`을 기다린다.
