---
baley_record: 1
record_id: "835bcc2b-7bdc-418c-9ac3-e5604b5c11ae"
task_id: 117
task_key: "embedding-contract"
record_type: detailed-plan
run_id: "e9bae370-eb6d-4555-82e1-07832c3b90ef"
created_at: "2026-07-26T18:41:59+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Embedding Contract 판단 초안 계획

## 목적

Embedding Contract Phase의 #117~#120을 하나의 PM 판단 가능한 문서로 정리한다.
이번 Run은 새 자동 확인 정책을 구현하거나 #118~#120을 완료 처리하지 않는다.

## 산출물

- `docs/embedding-contract-draft.md`
  - #117 범위·비범위·성공 기준
  - #118 Task acceptance mode와 delegated auto-confirm 제안
  - #119 evidence·복원·Pilot 측정 계약
  - #120 Enablement 진입 checklist

## 핵심 판단 경계

- `delegated` Task만 evidence 조건 뒤 자동 `confirmed`가 가능하다.
- 기능 수용, 디자인/UX 수용, code-review/sign-off, 제품·보안 판단은
  `human_required`로 남긴다.
- Gate pass, Lane close-out, Workspace close는 Task acceptance mode와 무관하게
  사람 승인이다.
- PM의 네 가지 policy 결정을 받은 뒤에만 Enablement 구현 범위로 옮긴다.

## 다음 단계

PM이 초안의 판단 항목을 확정하면 #117을 계약 문서로 확정하고 #118/#119를 병렬
상세화한 뒤 #120에서 Enablement checklist를 검토한다.
