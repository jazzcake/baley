---
baley_record: 1
record_id: "eb90cb78-fdd7-4514-b540-f5fa92a5027f"
task_id: 117
task_key: "embedding-contract"
record_type: review-response
run_id: "fcb4949e-71cc-468b-a493-1708b7447be6"
created_at: "2026-07-26T19:10:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Embedding Contract 리뷰 반영

## 반영 결과

1. 기존 #121 범위를 바꾸지 않고 새 Enablement Task #130을 생성했다.
   #120 → #130 → #123 dependency로 Pilot 운영 키트 전에 acceptance policy 구현을
   완료하도록 topology를 보강했다.
2. `inherit`은 Task 생성 시 Workspace default로 resolve되고 effective mode와 policy
   version이 Task에 동결되도록 계약을 명시했다. existing Task는 human_required로
   보존하며 delegated 권한 부여/완화는 사람 승인만 허용한다.
3. auto-confirm은 자유 서술이 아니라 completion/report/review verdict와 reference를
   가진 typed evidence provenance로 판정하도록 #118/#130 계약을 명시했다. 서버는
   evidence의 결속과 enum을 검증하며 의미 품질을 스스로 판정하지 않는다.
4. acceptance mode가 Task confirmation 외 권한을 바꾸지 않음을 명시했다. Task discard,
   active Gate attach/pass/revoke/pass, future Gate operator 규칙, Lane close-out/discard,
   Workspace close의 기존 경계를 모두 열거했다.
5. #117 기준선, #118 executable policy, #119 runbook, #120 통합 review의 산출물
   경계를 분리했다.

## 변경 파일

- `docs/embedding-contract-draft.md`
- `docs/baley-adoption-task-manifest.md`

## 재검토 요청

독립 Agent가 네 blocking finding의 해소 여부를 재검토해야 한다.
