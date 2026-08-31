---
type: embedding-contract
status: pm-approved-baseline
workspace_id: "00000000-0000-4000-8000-000000000001"
phase_id: embedding-contract
tasks: [117, 118, 119, 120, 130]
created_at: "2026-07-26"
pm_decision: "confirmed in chat on 2026-07-26"
approval_authority_superseded_by: docs/task-acceptance-policy-contract.md
---

> Historical baseline: the delegated acceptance and auto-confirm sections below
> are superseded. All current Tasks are human-required and evidence reporting
> cannot confirm a Task.

# Embedding Contract

## 한 문장 목적

Baley가 작업을 대신 결정하는 도구가 아니라, **사람이 정한 수용 기준 안에서
Agent가 구현·검증·기록을 끝까지 수행하도록 하는 운영 계약**을 고정한다.

이 문서는 #117~#120의 PM 승인 기준선이다. 자동 확인 정책의 실제 server/MCP/Viewer
구현은 Enablement의 #130에서 수행한다. 그 전 V1 runtime은 기존처럼 모든 Task
`confirmed`에 사람 승인 진술을 요구한다.

## #117 — 범위와 성공 기준

### 이번 Adoption 범위

- 단일 Git repository를 하나의 Baley Workspace와 연결한다.
- LLM/Agent가 Task, Run, Task Record, Git evidence를 command-first로 운용한다.
- Viewer는 읽기 전용이다.
- lane별 Backlog는 Phase 미정 계획 수집함이며, 명시적인 Phase와 관계 의도 뒤에만
  정식 Task로 승격한다.
- Gate는 Phase 전환의 사람 결정으로 남긴다.

### 이번 범위 밖

- 여러 repository의 하나의 Task 공동 관리
- GitHub/GitLab, Slack/email, webhook 연동
- 조직 권한 관리, hosted deployment, release/deployment Gate
- Viewer에서 직접 Task를 편집하는 UI
- 일정 예측·자동 우선순위화·LLM의 무승인 제품 방향 변경

### 성공으로 보는 상태

1. 다른 Agent가 Task Record와 lane brief만 읽고 다음 작업을 재개할 수 있다.
2. Agent는 코드·테스트·독립 리뷰·완료보고까지 자동으로 수행할 수 있다.
3. Task의 확인 방식이 사전에 보이며, 사람 판단이 필요한 일은 조용히 확정되지 않는다.
4. Gate 통과, Lane 종료, Workspace 종료는 사람이 결과를 보고 결정한다.
5. Pilot에서 복원 시간, 잘못된 다음 작업 제안, evidence 불일치, 수동 정정 횟수를
   측정할 수 있다.

## #118 — Task 확인과 intake 계약

### 핵심 원칙

`implemented`는 Agent의 기술적 완료이고, `confirmed`는 수용 완료다. 앞으로는 모든
Task에 동일한 사람 확인을 강제하지 않고, **생성 시 정한 acceptance mode**로 처리한다.

| acceptance mode | 의미 | `implemented` 뒤 처리 | 대표 예시 |
| --- | --- | --- | --- |
| `delegated` | PM이 사전에 Agent 수용을 위임 | 증거 조건 충족 시 자동 `confirmed` | regression, migration, deterministic API/CLI, 문서 기계 검증 |
| `human_required` | 사람이 결과를 직접 판단 | `implemented`에서 확인 대기 | 기능 수용, UX/디자인, code-review/sign-off, 제품·보안·비용 판단 |
| `inherit` | Workspace 기본값 사용 | 기본 정책에 따름 | 명시하지 않은 기존 Task |

### 추천 기본 정책

- **기술 검증 Task**: `delegated`를 기본 후보로 둔다. 단, 테스트·빌드·독립 리뷰 등
  정의된 evidence 조건을 모두 통과해야 한다.
- **기능 수용, 디자인 수용, code-review/sign-off Task**: `human_required`로 만든다.
- **애매한 Task**: 안전하게 `human_required`가 기본이다. Agent는 자동 위임을 추측하지 않는다.
- **Gate pass**: Task가 `delegated`여도 항상 사람 승인이다. Task 확인과 Phase 전환은
  별개의 결정이다.

### delegated 자동 확인의 실행 계약

자동 확인은 LLM의 조용한 자기승인이 아니라, 저장된 policy에 의한 deterministic 전이다.

1. Task 생성 시 `inherit`은 그 시점의 Workspace default로 해석하고, 계산된
   `effectiveAcceptanceMode`와 policy version을 Task에 영구 저장한다. 이후 default가
   바뀌어도 기존 Task의 mode는 바뀌지 않는다.
2. 기존 Task는 migration 시 `human_required` effective mode로 보존한다.
3. `delegated` 설정 또는 mode 완화는 사람 approver의 명시 승인과 audit Event가 있어야
   한다. Agent는 더 보수적인 `human_required` 전환을 제안할 수 있지만 무단으로
   자동 확인 권한을 부여하지 않는다.
4. Task가 `implemented`여야 한다.
5. `effectiveAcceptanceMode=delegated`와 policy version이 Task에 저장돼 있어야 한다.
6. required evidence profile을 모두 충족해야 한다.
   - 구현 Task: completion report + test/build 결과 + independent review의 blocking 없음
   - 문서/계약 Task: 결정 문서 + 독립 리뷰 또는 PM이 정한 review evidence
7. blocking diagnostic, 미해결 review finding, 필요한 warning acknowledgement가 없어야 한다.
8. 서버가 `task.auto_confirmed` Event를 남긴다. Event에는 policy version, evidence Record IDs,
   commit/reference, 실행 Agent를 기록한다.

evidence profile은 Record 본문을 LLM이 자유롭게 요약한 문자열로 판정하지 않는다. #118은
`completionReportRecordId`, `verificationVerdict`, `independentReviewRecordId`,
`reviewVerdict`, `unresolvedBlockingCount`, 선택적 commit/blob reference를 typed index
fields로 정의한다. 서버는 이 필드의 존재·Workspace/Task/Run 결속·허용 enum·review
verdict를 기계 검증한다. 서버가 테스트나 리뷰의 의미 품질을 스스로 판정하는 것은 아니며,
`delegated` 정책은 PM이 이 감사 가능한 evidence provenance를 신뢰한다는 사전 위임이다.

자동 확인은 다음을 절대 하지 않는다. acceptance mode는 `implemented → confirmed`에만
영향을 주며 다른 capability나 승인 경계를 바꾸지 않는다.

- Task discard
- active Gate condition attach, Gate Task pass/revoke, Gate pass
- future Gate attach/detach의 일반 Operator 규칙 또는 active Gate detach 금지 규칙
- Lane close-out/discard, Workspace close
- `human_required` Task의 confirmation
- evidence가 부족하거나 review finding이 남은 Task의 상태 전이

### 필요한 제품 변경(Enablement에서 구현)

- Task create에 requested `acceptanceMode`, resolved `effectiveAcceptanceMode`,
  policy version과 선택적 `evidenceProfile` 추가
- 사람이 승인한 mode 변경 command와 audit Event 추가
- Workspace default acceptance policy와 policy version
- `task.report_implemented` 시 delegated eligibility 계산 및 atomic auto-confirm
- `task.auto_confirmed` audit Event와 Viewer 표시
- policy를 human_required로 바꾸는 권한/이력 규칙
- MCP/CLI/HTTP/Skill의 preview·reporting·record tests

### 확정된 PM 기준

- 기능 수용, 디자인/UX 수용, code-review/sign-off는 `human_required`다.
- 기술 구현·회귀 테스트·migration·문서 기계 검증은 사전 위임된 경우에만 `delegated`다.
- 분류가 애매한 Task와 기존 Task는 `inherit/human_required`로 안전하게 보존한다.
- delegated 구현 Task도 completion report, test/build, independent review의 blocking 없음이
  evidence profile로 필요하다.
- Gate pass, Lane close-out, Workspace close는 acceptance mode와 관계없이 사람 승인이다.

## Task intake와 Backlog 원칙

- Backlog item은 lane에만 속하고 Phase를 갖지 않는다.
- Task 승격 때만 사람이 target Phase와 dependency/terminal intent를 명시한다.
- Agent는 후보 Task를 제안할 수 있지만, predecessor/independent-root intent가 불명확하면
  Task를 발명하지 않고 Backlog 또는 계약 문서에 남긴다.
- promote는 Task·dependency·Backlog 상태·Event를 원자적으로 바꾸며 Gate를 자동 변경하지 않는다.

## #119 — 증거·복원·Pilot 측정 계약

### 남겨야 하는 증거

- 상세계획, handoff, 독립 리뷰, 리뷰 반영, 완료보고의 Task Record
- 해당 Record의 상대 경로, hash, repository, commit/blob reference
- Task/Run/Gate Event history
- 비권위적인 Git worktree observation

### 복원 절차

1. Workspace graph와 active Gate/Phase를 fresh-read한다.
2. Lane brief와 현재 Task의 최신 Task Record를 읽는다.
3. Task 상태, 마지막 Run, Record hash, Git observation의 불일치를 표시한다.
4. Agent는 확정된 evidence와 추정을 구분해 다음 행동 후보를 제시한다.
5. 사람 판단이 필요한 Gate·수용·폐기 결정은 다시 명시적으로 요청한다.

### Pilot에서 측정할 지표

| 지표 | 측정 방법 | 좋은 신호 |
| --- | --- | --- |
| 복원 시간 | 새 세션이 lane brief만으로 유효한 다음 행동을 내기까지의 시간 | 짧고 재현 가능 |
| 다음 Task 정확도 | 제안된 다음 행동이 PM 판단과 맞는 비율 | 높음 |
| evidence 불일치 | Task/Record/commit이 어긋난 건수와 해결 시간 | 적고 빨리 해소 |
| 수동 정정 | 사람이 Task 상태·관계를 수동으로 바로잡은 횟수 | 감소 |
| Gate 조율 비용 | Gate 승인에 필요한 왕복과 예상 밖 재preview 수 | 낮고 설명 가능 |

## #117~#120 산출물 경계

- #117은 이 PM 기준선, 범위/비범위, human gate 예외를 확정한다.
- #118은 #130이 구현할 executable policy/schema/event/API contract를 상세화한다.
- #119는 evidence·복원·측정 runbook과 실제 수집 protocol을 상세화한다.
- #120은 #118/#119의 독립 리뷰와 #130의 새 ownership을 합쳐 Enablement checklist를 확정한다.

## #120 — Enablement 진입 checklist

다음이 모두 충족되면 #120을 `implemented`로 보고하고, 이후 Gate 통과 여부를 사람에게
물어볼 수 있다.

- [ ] #117 범위/비범위와 성공 기준이 PM에게 승인됐다.
- [ ] #118 acceptance mode, 기본값, evidence profile, Gate 예외가 확정됐다.
- [ ] #119 evidence/복원/측정 방식이 하나의 runbook으로 재현 가능하다.
- [ ] Task create, report, confirm/auto-confirm, Backlog promote, Gate pass의 책임 경계가 모순 없다.
- [ ] 독립 Agent가 문서와 현재 contracts/spec/skill의 충돌을 검토했다.
- [ ] #121~#124, 기존 #129, 새 #130 Enablement 작업이 이 계약에 맞는지 재검증됐다.

## 결정 후 실행 순서

1. PM이 위 네 가지 #118 결정과 #117 범위를 확정한다.
2. #117을 계약 문서로 확정한다.
3. #118/#119를 병렬로 상세화하고 독립 리뷰를 받는다.
4. #120이 Enablement checklist와 이행 항목을 확정한다.
5. `Embedding Contract → Embedding Enablement` Gate는 #120 확인 뒤에만 사람 승인으로 통과한다.
