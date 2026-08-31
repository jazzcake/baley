---
type: acceptance-policy-contract
status: pm-approved-implementation-contract
source_task: 118
implementation_task: 130
---

# Task Acceptance Policy Contract

## 목적

Task의 기술적 완료(`implemented`)와 업무 수용(`confirmed`)을 구분한다. 모든 Task
수용은 human-required이며 Agent evidence는 Task를 자동 확인하지 않는다. 이 P0 규칙은
기존 delegated acceptance 계약을 대체한다.

## Mode와 고정 규칙

| requested mode | 생성 시 처리 | effective mode |
| --- | --- | --- |
| `human_required` | 항상 허용 | `human_required` |
| `inherit` | Workspace default를 생성 시 한 번 resolve | `human_required` |

- Task는 생성 시 `effective_acceptance_mode`, `acceptance_policy_version`,
  `evidence_profile_id`를 하나의 immutable binding으로 저장한다. migration의 initial
  binding도 append-only record이며, 만들어진 Task의 current binding은 다시 바뀌지 않는다.
- 기존 Task migration은 `human_required`와 migration policy version으로 backfill한다.
- `task.acceptance_policy.change`는 사람 승인으로 future Task template만 바꾼다. pending,
  in_progress, implemented를 포함한 기존 Task의 binding은 절대 변경하지 않으며, 이후 새
  Task만 새 template binding을 resolve해 고정한다.
- migration은 기존 delegated assignment history를 보존하고 새 human-required assignment
  version을 append한 뒤 current Task binding과 Workspace default를 `human_required`로 바꾼다.
- 새 delegated policy 또는 Task 요청은 거부한다.
- 혼합 Task(기능 수용·디자인·sign-off 요소 포함)는 `human_required`가 우선한다.

## Mode 선택 기준

다음은 모두 `human_required`다.

- 기능 수용과 사용자 시각 검증
- 디자인·UX·copy·접근성의 최종 수용
- code-review/sign-off와 architecture/product decision
- 정책 완화, Task discard, Gate/Lane/Workspace 관련 판단

## Historical delegated assignments

Delegated rows remain append-only historical evidence. Migration appends a superseding
human-required assignment. Runtime resolution, policy changes and Task creation cannot select
delegated mode, and `task.evidence.report` never changes Task status.

## Typed data model

```text
Task
  requested_acceptance_mode: human_required | inherit
  effective_acceptance_mode: human_required
  acceptance_policy_version: string
  evidence_profile_id: string

TaskAcceptanceEvidence
  task_id, workspace_id
  completion_report_record_id
  verification_verdict: passed | failed | unavailable
  verification_reference: optional record/run/artifact reference
  independent_review_record_id
  review_verdict: pass | fail | unavailable
  unresolved_blocking_count: non-negative integer
  commit_reference_id: optional
  reported_by_actor_id, reported_at

EvidenceProfile
  id, version
  required: completion_report(1), verification(1+), independent_review(1)
  allowed_reference_kinds: task_record | run | commit_reference | artifact
  verification_pass_requires_nonempty_reference: true
  review_pass_requires_unresolved_blocking_count_zero: true
```

Evidence는 evidence UUID와 idempotency key를 가진 append-only version이다. 서버는
reference의 존재·Task/Workspace/Run 결속, profile cardinality, 허용 reference kind,
enum, non-empty passed verification reference와 blocking count를
검증한다. 서버는 테스트나 review 본문의 의미 품질을 판정하지 않는다.

## Commands와 Events

- `task.create`는 requested mode/evidence profile을 받고 effective mode/policy version을
  결과에 포함한다. `backlog.promote`도 같은 optional requested mode/profile을 받고
  pending Task, dependency, Backlog 전이와 assignment 동결을 한 transaction으로 처리한다.
  mode를 생략하면 `inherit`을 `human_required`로 resolve한다. delegated 선택은 거부한다.
- 사람 승인 `task.acceptance_policy.change`는 future Task template의 기본 mode/profile만
  auditably 변경한다. 생성된 Task의 `requested_acceptance_mode`,
  `effective_acceptance_mode`, `acceptance_policy_version`, `evidence_profile_id`는 절대
  다시 쓰지 않는다. 이후 새 Task만 당시 template binding을 resolve하여 고정한다.
- `task.report_implemented`는 최초 completion evidence를 기록한다.
- `task.evidence.report`는 implemented Task에 새 evidence version을 append하며 기존
  evidence를 삭제·수정하지 않는다. report/evidence command 뒤에는 서버가 current
  assignment와 가장 최근 유효 evidence version을 재평가한다.
- evidence 충족 여부와 무관하게 Task는 `implemented`에 남고 typed reason을 query에
  제공한다. 이후 evidence report로 다시 평가할 수 있지만 확인 상태로 전이하지 않는다.
- `task.confirm`은 browser-session approval grant를 요구하는 human-only command다.

## 바뀌지 않는 경계

acceptance mode는 Task `implemented → confirmed`에만 영향을 준다.

- Task discard: 사람 승인
- active Gate condition attach: 사람 승인; active detach: 금지
- future Gate attach/detach: 기존 Operator 규칙
- Gate Task pass/revoke 및 Gate pass: 사람 승인
- Lane close-out/discard: 사람 승인
- Workspace close: human Owner 승인

## #130 수용 기준

- schema/migration, command contract, Go domain/service, PostgreSQL, HTTP/CLI/MCP,
  Viewer projection이 effective mode와 typed evidence를 같은 의미로 노출한다.
- preview는 write-free이고 policy/evidence 변경은 revision·idempotency·audit을 지킨다.
- delegated 거부, evidence 비자동확인, existing-task migration, human-required grant,
  모든 Gate/Lane/Workspace 회귀를 integration/E2E로 검증한다.
