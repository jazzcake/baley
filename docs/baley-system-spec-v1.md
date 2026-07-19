---
type: spec
status: active
authority: normative
last_active: 2026-07-17
when_to_read: "Baley V1의 제품 경계, 데이터 모델, 외부 서버, Operator 운용, repository 기록과 Git 연동을 구현하거나 변경할 때"
affects:
  - docs/baley-product.md
  - docs/baley-command-architecture.md
  - docs/baley-roadmap.md
  - .agents/skills/baley-manage-work/SKILL.md
---

# Baley System Specification V1

## 1. 한 문장 정의

Baley는 **Task 상태 관리 머신, 관계형 업무 DB와 읽기 전용 Viewer**다. 사람 또는 Agent가 Operator가 될 수 있으며, 기본 Operator인 LLM/Agent가 Skill과 MCP를 통해 이를 운용한다.

Baley는 코드 repository, Git history, Operator의 업무 판단을 대체하지 않는다.

## 2. 제품 포지션

### 2.1 역할 분리

```text
사람
  목표, 최종 판단, 완료확인, Gate 승인

Operator
  계획, Task 운용, 관계 변경, Run과 Record 관리
  사람 또는 Agent가 될 수 있으며 LLM / Agent가 기본 Operator

LLM / Agent
  구현, 독립 Agent 리뷰, 리뷰 반영, 완료보고

Baley Server
  Task 상태, 관계, Run, 기록 index, 명령과 Event 저장
  구조적 규칙과 상태 전이 검증

Repository / Git
  코드, 프로젝트 문서, Task Record 원문과 파일 이력의 정본

Baley Web
  현재 상태, 관계, Run, Record index와 Git 증거를 읽는 Viewer
```

### 2.2 Baley가 강제하는 것

- entity ID와 Workspace 격리
- Task 상태 전이
- dependency 방향과 cycle 방지
- dependency patch의 최종 graph 원자성
- Phase 순서와 Gate 전이
- 사람 전용 판단 command의 승인 진술
- command 원자성, idempotency와 revision
- Run 및 Record metadata의 참조 무결성
- 모든 상태 변경의 Event 기록

### 2.3 Baley가 판정하지 않는 것

- 상세계획의 품질
- 구현이 실제 요구사항을 만족하는지
- 독립 Agent 리뷰가 충분히 깊었는지
- reviewer가 의미상 완전히 독립적인지
- 기술부채와 잔여 위험이 허용 가능한지

이 판단은 구현 주체와 사람이 수행한다. Baley는 판단, 근거, 우려와 예외를 기록하고 누락을 경고할 수 있지만 의미 품질을 이유로 Task 진행을 막지 않는다.

## 3. 정본과 저장 경계

### 3.1 문서와 계약의 권한

Baley V1은 정본을 다음 순서로 나눈다.

```text
제품 기획서                 제품 의도와 UX 원칙
  ↓
System Specification       규범적 도메인 의미와 불변식
  ↓
contracts/v1/*.json        command·상태·diagnostic·capability literal
  ↓
HTTP API / MCP             계약의 구현과 transport adapter
  ↓
Skill                      Operator workflow와 자연어 해석
```

아래 단계는 위 단계의 의미를 바꾸지 않는다. 정확한 command 이름, 상태값, diagnostic code와 capability 이름은 [`contracts/v1`](../contracts/v1/README.md)이 정본이며 이 문서에서는 목록을 복제하지 않는다.

### 3.2 데이터의 정본

| 정보 | 정본 |
| --- | --- |
| 코드와 프로젝트 문서 | Repository |
| 파일의 이전 버전 | Git history |
| Task Record 원문 | Repository의 설정된 record 경로 |
| Task, Phase, Lane, Gate | Baley DB |
| Task 및 Run 상태 | Baley DB |
| Record 경로·hash·commit index | Baley DB |
| Task 상태 변경과 승인 | Baley Event |
| commit과 diff | Git |

Baley Event는 Git log를 대체하지 않는다.

- Git은 파일이 어떻게 변했는지 보여준다.
- Baley Event는 Task 관계와 상태가 왜 변했는지 보여준다.
- Viewer는 두 이력을 연결해 보여줄 수 있다.

## 4. 배포 구조

Baley는 프로젝트 filesystem과 분리된 외부 서버로 운영한다.

```text
로컬 프로젝트 + LLM
├─ 코드와 Task Record 작성
├─ Git 작업
├─ 상대 경로와 hash 계산
└─ remote MCP/API 호출
          │
          ▼
외부 Baley Server
├─ Go API / Command Service
├─ MCP tools
├─ PostgreSQL
└─ React Viewer
```

외부 서버는 로컬 절대 경로를 저장하거나 로컬 worktree를 직접 읽지 않는다.

### 4.1 V1 접근 보호

V1에는 회원가입, 로그인 UI와 다중 사용자 인증을 구현하지 않는다.

- 단일 bootstrap human Actor를 사용한다.
- Agent Actor는 감사와 출처 표시를 위해 구별한다.
- 모든 workspace-scoped row는 처음부터 `workspace_id`를 가진다. Actor처럼 deployment 범위인 row는 예외다.
- 외부 서버는 Tailscale, VPN, reverse proxy 인증 또는 동등한 배포 계층으로 보호한다.
- 인증 token과 secret은 repository의 `baley.yaml`에 저장하지 않는다.

제품 인증과 membership enforcement는 후속 단계에서 추가한다. 따라서 V1의 사람 승인은 보안적으로 증명된 신원이 아니라, MCP가 전달한 **사람 승인 진술**을 protocol과 Event로 보존하는 수준이다.

## 5. 프로젝트 통합

### 5.1 baley.yaml

Baley 사용 프로젝트에는 repository에 안전하게 커밋할 수 있는 설정 파일을 둔다.

```yaml
version: 1
server: https://baley.example.com
workspace_id: "uuid"
repository_id: "uuid"

task_records:
  root: task-records
```

여러 repository를 사용하는 Workspace는 Task Record를 보관할 대표 repository를 지정한다.

```yaml
record_repository_id: "uuid"
```

### 5.2 기본 Record 구조

```text
task-records/
├─ README.md
├─ _templates/
│  ├─ detailed-plan.md
│  ├─ handoff.md
│  ├─ independent-agent-review.md
│  ├─ review-response.md
│  └─ completion-report.md
└─ task-104/
   ├─ detailed-plan-01.md
   ├─ handoff-01.md
   ├─ independent-agent-review-01.md
   ├─ review-response-01.md
   └─ completion-report-01.md
```

경로명은 프로젝트가 바꿀 수 있으며 Baley는 특정 디렉터리 이름을 강제하지 않는다.

### 5.3 검색 제외

Task Record는 Git이 추적하지만 일반 repository 검색에서는 제외한다.

`.rgignore` 예시:

```text
task-records/**
```

Agent 규칙:

- Task Record 전체를 일반 repository 검색에 포함하지 않는다.
- 현재 Task에 연결된 정확한 경로만 명시적으로 읽는다.
- `docs/`에는 여러 Task에 지속적으로 적용되는 프로젝트 지식만 둔다.

`.rgignore`는 모든 검색 도구를 통제하지 못하므로 IDE `search.exclude`, Agent 지침과 Baley Skill을 함께 사용한다.

## 6. 도메인 관계

```text
Workspace
├─ Repositories
├─ ordered Phases
├─ Lanes ───────────────┐
├─ Tasks                │
│  ├─ parent/child      │
│  ├─ Dependencies      │  # 복수 DAG component
│  ├─ Intentional leaves│
│  ├─ Runs              │
│  ├─ Record indexes ───┼─ Repository files
│  └─ Commit references │
├─ Gates                │
│  ├─ from Phase        │
│  ├─ to Phase          │
│  └─ attached Tasks ───┘
└─ Events
```

## 7. Workspace, Phase와 Lane

### 7.1 Workspace

Workspace는 데이터 격리, Phase 흐름과 repository 묶음의 최상위 단위다.

핵심 필드:

```text
id UUID
name
state: draft | active | closed
active_phase_id nullable
record_repository_id nullable
revision BIGINT
created_at, updated_at
```

V1의 active Workspace에는 active Phase가 정확히 하나다.

### 7.2 Phase

Phase는 특정 Lane에 속하지 않고 모든 Lane을 관통하는 공통 진행 구간이다.

```text
id UUID
workspace_id
name
position INTEGER
activated_at nullable
completed_at nullable
```

V1 Phase는 선형이며 `position` 순서가 있다.

- `phase.create`는 마지막 position 뒤에만 append하며 V1에서는 Phase reorder/delete를 지원하지 않는다.
- Workspace 활성화 시 첫 Phase가 active가 된다.
- Gate 통과 시 현재 Phase가 completed되고 다음 Phase가 active가 된다.
- 마지막 Phase는 `workspace.close`의 사람 승인 진술로 완료한다. 같은 transaction에서 마지막 Phase `completed_at`을 설정하고 `active_phase_id=NULL`, Workspace `state=closed`로 변경한다.
- 미래 Phase의 Task 생성과 `detailed_planning` Run만 허용하며 그 밖의 Run은 해당 Phase가 활성화될 때까지 거부한다.
- completed Phase에 새 Task를 만들거나 Task를 이동해 넣지 않는다.
- completed Phase에 이미 존재한 Gate 비연결 Task는 계속 진행할 수 있다.
- Phase 분기·병합과 rollback은 V1 범위가 아니다.

`workspace.close`는 active Run이 있으면 hard error다. confirmed/discarded가 아닌 잔여 Task와 active Lane은 warning으로 보여주지만 사람 승인 진술이 있으면 종료할 수 있다.

닫힌 Workspace는 read-only다. V1에는 reopen이 없으며 query와 export만 허용하고 이후 domain mutation과 새 Run 시작은 hard error로 거부한다.

### 7.3 Lane

Lane은 일정 기간 지속되는 독립적인 업무 전선이다. 전체 Phase를 관통하며 자체 목표, 요약과 Task graph를 가진다.

```text
id UUID
workspace_id
name
goal
summary
state: active | closed_out | discarded
created_at, updated_at
```

Lane Group과 Lane fork는 V1에 포함하지 않는다.

Lane `closed_out`과 `discarded`는 업무 전선의 전략적 종료이므로 사람 승인 진술이 필요하다. Operator는 종료를 preview하고 실행할 수 있지만 승인 주체가 될 수는 없다.

## 8. Task

### 8.1 식별자

Task는 내부 UUID와 Workspace 범위에서 유일한 숫자 public ID를 가진다.

```text
화면 표준: #104
자연어 허용: #104, task #104, task104, task 104, 104번 task
```

public ID는 변경하거나 재사용하지 않는다.

### 8.2 핵심 필드

```text
id UUID
workspace_id
public_id BIGINT
lane_id
phase_id
parent_task_id nullable
title
description
status
blocked_at nullable
blocker_reason nullable
current_summary
next_action
terminal_reason nullable
implemented_assessment nullable
created_at, updated_at
```

### 8.3 상태

사용자 표시와 내부 상태를 일치시킨다.

| 표시 | 내부값 | 의미 |
| --- | --- | --- |
| 대기 | `pending` | 아직 작업을 시작하지 않음 |
| 진행중 | `in_progress` | 상세계획·구현·리뷰·보고 중 |
| 구현완료 | `implemented` | 구현 주체가 요구에 따른 완료를 선언 |
| 완료확인 | `confirmed` | 사람이 결과를 확인하고 수용 |
| 폐기 | `discarded` | 더 진행하지 않기로 결정 |

기본 전이:

```text
pending → in_progress → implemented → confirmed
   └────────┴───────────────→ discarded
                    implemented → in_progress  # 리뷰 반영 또는 재작업
```

- 첫 작업 Run을 시작하면 Operator가 Task를 `in_progress`로 자동 전환한다.
- 구현 주체가 최소 assessment를 남기고 `implemented`를 선언한다.
- Baley는 구현 의미를 판정하지 않고 허용된 상태 전이와 기록 참조만 확인한다.
- 사람의 자연어 승인을 LLM이 명령으로 전달하면 `confirmed`가 된다.
- `confirmed`와 `discarded`는 V1 terminal 상태다.
- `blocked`는 생명주기 상태가 아니라 `blocked_at`과 `blocker_reason`으로 표현한다.

`task.block`은 업무상 blocker를 기록하는 metadata command다. 새 `implementation`과 `review_response` Run 및 `task.report_implemented`를 차단하지만, 상세계획·독립 Agent 리뷰·완료보고 Run과 일반 조회는 허용한다. 이미 실행 중인 Run은 자동 중단하지 않는다. `task.unblock`은 blocker를 명시적으로 해제하고 두 command 모두 사유와 Event를 남긴다. Gate readiness를 직접 바꾸지는 않지만 blocked Task가 `confirmed`까지 진행하지 못하므로 간접적으로 영향을 줄 수 있다.

`task.update`는 title, description, current summary와 next action만 바꾼다. 상태는 전용 lifecycle command로만 바꾸며 Lane, Phase와 parent 이동은 V1에서 지원하지 않는다. `task.create`는 선택적 `parentTaskId`, 복수 `predecessorTaskIds`, 복수 `successorTaskIds`와 `terminalReason`을 받아 Task 생성과 초기 관계를 한 transaction에서 검증한다. 의도적인 독립 경로 종료는 이후 별도 `task.set_terminal`로도 기록할 수 있다.

### 8.4 구현완료 판단

구현완료는 Baley가 계산하지 않는다. 구현 주체는 다음 최소 assessment를 반드시 함께 기록한다.

```text
결론: 요구사항에 따른 구현 완료
검증: test/build 결과
잔여 위험: 기술부채 또는 불확실성
후속 제안: 별도 Task 후보
완료보고 경로: task-records/task-104/completion-report-01.md
```

assessment는 구현 품질 인증이 아니라 상태 변경 이유, 수행한 검증과 알려진 우려를 남기는 최소 감사 정보다. 완료보고 Record는 선택 사항이며 없으면 warning이다.

상세계획, 독립 Agent 리뷰 또는 완료보고가 없으면 Baley는 warning을 반환하지만 상태 전이를 강제로 차단하지 않는다. 구현 주체는 warning code를 acknowledge하고 선택적 진행 사유를 남겨 진행할 수 있다.

## 9. Task 구조와 dependency

### 9.1 Parent/child

Parent/child는 Task의 구조적 분해를 나타낸다.

- V1 child는 parent와 같은 Workspace, Lane과 Phase에 속한다.
- self-parent와 hierarchy cycle을 금지한다.
- parent/child는 실행 차단 관계가 아니다.

### 9.2 Dependency

Dependency는 항상 선행 Task에서 후행 Task로 향하며 **후행 Task의 implementation/review-response Run 시작**을 차단한다. Task 자체의 상세계획, Handoff 준비와 일반 조회는 차단하지 않는다.

```text
id UUID
workspace_id
predecessor_task_id
successor_task_id
created_at
```

불변식:

- self-link와 중복 link 금지
- 같은 Workspace. Lane과 Phase는 달라도 됨
- 한 Task는 선행·후행 Task를 각각 0개 이상 가질 수 있음
- Workspace의 전체 dependency graph는 cycle이 없는 여러 disconnected component로 구성될 수 있음
- DAG는 acyclic을 의미하며 하나의 connected graph를 의미하지 않음
- predecessor가 `implemented` 또는 `confirmed`이면 dependency가 해소됨
- predecessor가 `discarded`이면 자동 해소되지 않음
- 뒤 Phase Task에서 앞 Phase Task로 향하는 dependency도 구조적으로 허용하되 `phase_order_inversion` warning을 반환함
- dependency가 Gate readiness에 영향을 주는 것은 해당 Task가 그 Gate에 명시적으로 attach된 경우뿐임

### 9.3 경로의 합류와 종료

Task의 진행 경로는 다음 중 하나 이상으로 이어진다.

```text
Task → 하나 이상의 후행 Task
Task → 현재 Phase의 outgoing Gate 조건
Task → intentional leaf
```

여러 branch가 본류로 합류할 때는 여러 predecessor를 가진 merge Task를 만든다. Gate에 합류하는 경로는 마지막 Task를 Gate 조건으로 attach한다.

후행 Task와 Gate 연결이 모두 없는 Task는 기본적으로 `dangling_path`다. 독립적이고 지엽적인 결과로 끝나는 것이 의도라면 `task.set_terminal { taskId, reason }`으로 `terminal_reason`을 남긴다. `terminal_reason`이 있는 Task에는 후행 dependency나 Gate 조건을 연결할 수 없으며, 새 경로를 연결할 때는 같은 `dependency.patch`의 terminal update로 사유를 지운다.

`dangling_path`는 graph를 무효로 만들지 않는다. Task를 아직 계획 중일 수 있으므로 생성 시에는 경고하지 않고 `task.report_implemented`, `task.confirm`, `lane.brief`와 `gate.status`에서 warning으로 반환한다. 사람 또는 Operator는 후행 Task 연결, Gate attach, intentional leaf 선언 중 하나로 해소하거나 warning을 acknowledge할 수 있다.

### 9.4 Atomic dependency patch

Edge 방향을 전환하거나 여러 선행·후행 관계를 재구성할 때 연결 해제와 생성을 별도 transaction으로 실행하지 않는다.

```text
dependency.patch {
  remove: [{ predecessorTaskId, successorTaskId }],
  add:    [{ predecessorTaskId, successorTaskId }],
  terminalUpdates: [{ taskId, terminalReason: string | null }]
}
```

- 기존 edge는 in-place로 방향을 바꾸지 않는다.
- 서버는 terminal update, remove와 add를 적용한 최종 Workspace graph에서 self-link, duplicate, cross-Workspace 연결, cycle과 terminal exclusivity를 검증한다.
- 최종 graph가 invalid이면 전체 patch를 rollback한다.
- `dependency.connect/disconnect`와 `task.set_terminal/clear_terminal`은 한 변경짜리 patch의 convenience command다.
- preview는 제거·추가 edge, 새 root/leaf, `dangling_path` 변화를 함께 보여준다.

미해소 predecessor가 있으면 코드를 바꿀 수 있는 `run.start(kind=implementation | review_response)`은 hard error다. 상세계획, 독립 Agent 리뷰와 완료보고 Run은 시작할 수 있다. dependency를 무시하려면 관계를 명시적으로 끊어야 하며 V1에는 숨은 override를 두지 않는다.

이미 implementation Run을 시작한 successor에 새 미해소 dependency를 연결하면 기존 Run은 자동 중단하지 않는다. command는 warning과 acknowledgement를 요구하고, 새 dependency는 이후 implementation 또는 review-response Run 시작부터 적용한다. Viewer는 `Task: 진행중 / 다음 구현 Run: dependency로 차단`을 구분해 표시한다.

implemented Task를 rework해 `in_progress`로 되돌리면 이미 시작된 후행 Task를 되돌리지 않는다. 후행 Task에는 predecessor rework advisory를 표시하고 새 implementation Run에는 현재 dependency를 다시 검사한다.

## 10. Gate와 Phase 전이

### 10.1 Gate 정의

Gate는 특정 Phase의 자식이나 종료 지점이 아니라 `fromPhase → toPhase` 전이 규칙이다.

```text
id UUID
workspace_id
slug
name
from_phase_id
to_phase_id
criteria_revision BIGINT
passed_at nullable
passed_by nullable
created_at, updated_at
```

V1에서는 인접한 Phase만 연결하며 각 Phase에는 outgoing Gate가 최대 하나다.
첫 Phase를 제외한 각 Phase의 incoming Gate도 최대 하나다. 아직 active가 아닌 미래 Phase와 Gate를 미리 구성할 수 있지만 전이 mutation은 현재 active Phase의 outgoing Gate에만 허용한다.

### 10.2 Gate–Task 연결

V1 Gate에 연결된 모든 Task는 전이 조건이다. `required`, `reference`, `unlocks` 타입은 사용하지 않는다.

```text
id UUID
workspace_id
gate_id
task_id
attached_at
passed_at nullable
passed_by nullable
pass_reason nullable
```

- 연결 Task는 Gate의 `fromPhase`에 속한다.
- Task가 `confirmed`이면 해당 연결은 충족된다.
- 해당 Gate에서 Task가 명시적으로 `passed`되어도 충족된다.
- `implemented`와 `discarded`는 자동으로 Gate 조건을 충족하지 않는다.
- Task pass는 Task 상태를 변경하지 않고 해당 Gate 연결에만 적용된다.
- passed Gate에는 Task attach/detach/pass/revoke를 허용하지 않는다.
- `gate.pass_task`와 `gate.revoke_task_pass`는 Gate의 `fromPhase`가 현재 active Phase일 때만 허용한다. HumanApprovalAttestation은 Gate–Task 연결의 `id`에 결속한다.
- `fromPhase`가 미래 Phase일 때는 Operator가 조건 Task를 attach/detach할 수 있다.
- `fromPhase`가 active가 되면 Gate 조건은 기본적으로 동결된다. 기존 Task detach는 금지하며 조건 면제는 `gate.pass_task`로 남긴다.
- active Gate에 새 조건 Task를 attach하려면 사람 승인 진술이 필요하다.
- intentional leaf Task를 Gate에 attach할 때는 `clearTerminalReason=true`를 같은 transaction에 포함해야 한다.
- 조건 변경마다 `criteria_revision`을 증가시키고 `gate.status`의 condition snapshot hash를 갱신한다.

### 10.3 Gate 상태

```text
unconfigured: 연결 Task가 0개
open: 하나 이상의 연결 Task가 미충족
ready: 모든 연결 Task가 confirmed 또는 passed
passed: 사람이 승인해 Phase 전이가 완료됨
```

`ready`는 별도 ApprovalRequest 없이 **Gate 통과 승인 대기**를 의미한다. `gate.status`는 `decisionRequired=gate.pass`, Workspace revision, Gate criteria revision과 condition snapshot hash를 반환한다.

Gate pass transaction:

1. `gate.from_phase_id == workspace.active_phase_id` 재검사
2. `fromPhase.completed_at IS NULL`과 `toPhase`가 position상 바로 다음 Phase인지 재검사
3. Gate ready 재검사
4. 사람 승인 확인
5. `fromPhase.completed_at` 설정
6. Workspace `active_phase_id`를 `toPhase`로 변경
7. Gate pass와 Phase 전이 Event 기록
8. Workspace revision 증가

`gate.passed` Event에는 통과 당시 연결 Task ID, 각 Task의 `confirmed` 또는 Gate-specific `passed` 근거, pass 사유, 사람 승인 진술 ID와 Workspace revision을 snapshot으로 기록한다. Gate reopen과 rollback이 없는 V1에서 이 Event가 역사적 근거를 보존한다.

Gate에 연결되지 않은 이전 Phase Task는 전이를 차단하지 않으며 전이 후에도 계속 수행할 수 있다.

Milestone 객체는 만들지 않는다. Gate pass Event가 달성과 전이 이력을 나타낸다.

## 11. Run

### 11.1 정의

Run은 한 Task를 위해 Operator가 수행한 하나의 제한된 작업 세션 또는 시도다. Task 상태를 대신하지 않는다. 사람도 Operator가 될 수 있지만 기본 Run 실행 주체는 LLM/Agent다.

사용자에게는 추상적인 Run 종류보다 구체적인 업무 이름을 표시한다.

```text
detailed_planning       상세계획
implementation          구현
independent_agent_review 독립 Agent 리뷰
review_response         리뷰 반영
completion_reporting    완료보고
```

### 11.2 필드

```text
id UUID
workspace_id
task_id
client_run_id UUID
kind
status: running | succeeded | failed | interrupted | cancelled
operator_actor_id
session_ref nullable
parent_run_id nullable
target_run_id nullable
lease_token_hash
heartbeat_at
lease_expires_at
version BIGINT
started_at
ended_at nullable
result_summary nullable
error_summary nullable
created_at
```

### 11.3 자동 갱신

Run은 Operator client와 Skill/MCP가 기본적으로 조용히 갱신한다.

```text
작업 시작 → run.start → running
작업 유지 → run.heartbeat
작업 성공 → run.succeed
작업 실패 → run.fail
작업 취소 → run.cancel
비정상 종료 감지 → interrupted
```

- 사람에게 매 Run 상태 변경을 확인받지 않는다.
- `run.start`는 client가 생성한 `client_run_id`를 요구한다. `UNIQUE(workspace_id, client_run_id)`를 적용하고, 동일 ID와 동일한 canonical start payload 재호출만 같은 Run을 반환한다. Task, kind, parent 또는 target이 다르면 conflict다.
- Task가 pending이면 `run.start` transaction 안에서 `in_progress`로 함께 전환한다. 별도 `task.start → run.start` 순서에 의존하지 않는다.
- 미래 inactive Phase에서는 `detailed_planning` Run만 허용한다. `implementation`, `independent_agent_review`, `review_response`, `completion_reporting`은 Task Phase가 현재 active이거나 이미 completed일 때만 시작할 수 있다.
- Run 시작 시 lease token을 반환하고 heartbeat가 lease를 연장한다.
- raw lease token은 DB, command 결과 또는 Event에 저장하지 않는다. 서버는 외부 secret과 Run ID의 HMAC으로 token을 결정적으로 재구성하여 같은 `client_run_id`/idempotency 재시도에 같은 token을 반환한다. `BALEY_LEASE_TOKEN_SECRET`이 없으면 서버는 시작을 거부하며 모든 server process에 같은 값을 안정적으로 주입해야 한다. secret rotation은 V1 비범위다.
- terminal 전이는 `running → succeeded | failed | interrupted | cancelled`다.
- heartbeat 만료 job과 정상 종료 command는 Run version CAS를 사용하며 하나만 terminal 전이에 성공한다.
- terminal Run의 같은 idempotent 종료 재호출은 기존 결과를 반환하고 다른 terminal 결과는 conflict다.
- manual correction은 예외이며 이전 값, 새 값과 사유를 Event로 남긴다.
- 네트워크 오류 시 client는 같은 `client_run_id`와 idempotency key로 재시도한다. 완전한 offline outbox는 V1 비범위다.

### 11.4 독립 Agent 리뷰

`independent_agent_review`는 명시적인 Run kind와 Task Record 종류다.

Baley는 reviewer의 자격이나 리뷰 품질을 인증하지 않는다. 같은 상위 세션에서 생성한 병렬 sub-agent도 허용한다.

Baley가 확인하는 것은 구조적 관계뿐이다.

- 대상 Task가 존재함
- 대상 implementation Run을 알고 있으면 `target_run_id`로 참조함
- 리뷰 Run과 대상 Run이 별도 기록임
- 리뷰 결과 Record가 연결될 수 있음

Agent, session과 parent Run metadata는 provenance로 표시하며 통과 조건으로 사용하지 않는다. Viewer는 “독립성 검증됨”이 아니라 “독립 Agent 리뷰로 기록됨”이라고 표시하고 provenance가 없으면 “출처 미보고”로 표시한다.

## 12. Task Record

### 12.1 사용자 용어

`Artifact`나 추상적인 `Document`를 제품 용어로 사용하지 않는다. 사용자에게는 구체적인 기록 이름을 사용한다.

```text
상세계획
Handoff
독립 Agent 리뷰
리뷰 반영
완료보고
```

내부에서는 이들을 `TaskRecord`로 묶어 index한다.

### 12.2 Repository 파일 metadata

Record 파일은 최소 front matter를 가진다.

```yaml
---
baley_record: 1
record_id: "uuid"
task_id: 104
record_type: independent-agent-review
run_id: "uuid"
created_at: "2026-07-17T12:00:00+09:00"
created_by: "review-agent"
supersedes: null
---
```

Record 원문과 version history는 Git이 보관한다. 수정할 때 기존 파일을 덮어쓰지 않고 새 `record_id`와 version 파일을 만든다.

### 12.3 Baley DB index

Baley DB에는 원문을 저장하지 않고 다음 index만 저장한다.

```text
id UUID
workspace_id
task_id
run_id nullable
record_type
repository_id
relative_path
working_tree_hash nullable
commit_sha nullable
blob_sha nullable
state: reported_uncommitted | committed_unverified | verified
short_summary
supersedes_record_id nullable
created_at
```

- 로컬 절대 경로는 저장하지 않는다.
- Repository마다 server-side `task_records_root`를 등록한다.
- 상대 경로는 `/` 기준으로 정규화하며 `..`, drive prefix, URI, NUL과 configured root 이탈은 hard error다.
- commit 전에는 상대 경로와 working tree hash를 등록한다.
- commit 후 commit SHA와 blob SHA를 연결한다.
- 서버는 Git provider 연동 전까지 파일 존재와 내용을 독자적으로 검증하지 못하므로 `reported_uncommitted` 또는 `committed_unverified`로 표시한다.
- 동일 record ID와 같은 payload/hash 재등록은 idempotent다. 다른 hash 또는 경로는 hard conflict이며 새 version 등록을 요구한다.
- `record_id`는 LLM client가 먼저 생성해 repository front matter와 `record.register`에 동일하게 넣는다. 서버는 별도 ID로 교체하지 않는다.
- 등록 이후 로컬 파일의 hash가 달라졌다는 관찰은 별도 integrity warning으로 표시한다.

### 12.4 Context 조회

기본 Task 조회는 Record 본문을 반환하지 않는다.

```text
Task #104
상태: 진행중
현재 요약: 리뷰 반영 중
관련 기록:
- 상세계획: task-records/task-104/detailed-plan-01.md
- 독립 Agent 리뷰: task-records/task-104/independent-agent-review-01.md
```

LLM은 현재 목적에 필요한 정확한 경로만 명시적으로 읽는다. 이전 version과 관련 없는 Task Record를 자동 context에 포함하지 않는다.

## 13. Git 관계

### 13.1 원칙

Baley는 Git 작업공간 관리자가 아니라 Task 결과와 Git 증거를 연결하는 시스템이다.

V1에서 Branch와 worktree를 정식 entity나 Task lifecycle로 관리하지 않는다.

### 13.2 Repository

```text
id UUID
workspace_id
name
remote_url
default_branch nullable
is_record_repository BOOLEAN
task_records_root nullable
created_at
```

### 13.3 CommitReference

Task 결과를 가리키는 content-addressed Git 식별자다. remote push 또는 provider 검증 전에는 지속 접근 가능성이 보장된 증거로 과장하지 않는다.

```text
id UUID
workspace_id
task_id
run_id nullable
repository_id
commit_sha
relation: base | produced | reviewed | superseded
verification_state: reported | remote_verified
created_at
```

### 13.4 RunGitObservation

진행 중 복귀를 돕는 선택적 관찰 metadata다.

```text
id UUID
workspace_id
run_id
repository_id
observed_at
head_commit_sha nullable
branch_hint nullable
worktree_label nullable
dirty nullable
```

- `branch_hint`는 최근 보고된 텍스트 정보일 뿐 현재 상태를 보장하지 않는다.
- worktree 절대 경로는 외부 서버에 저장하지 않는다.
- Branch 생성·삭제·rename, checkout과 worktree lifecycle은 Baley가 관리하지 않는다.
- Branch 또는 worktree 존재 여부로 Task 상태를 추론하지 않는다.

## 14. Actor, Operator와 사람 권한

V1은 단일 사용자지만 human과 Agent를 이력상 구분한다.

```text
Actor
- id
- kind: human | agent | system
- display_name
```

`Operator`는 Actor kind가 아니라 Baley를 조회·변경하는 역할이다. 사람과 Agent 모두 Operator가 될 수 있으며, LLM/Agent가 기본 Operator다.

Command와 Event에는 가능하면 다음 출처를 기록한다.

```text
initiated_by  요청을 시작한 사람 또는 자율 Agent
executed_by   API/MCP command를 실행한 Operator Actor
approved_by   사람 전용 판단을 승인한 human Actor
```

V1 사람 승인 대상:

- Task `confirmed`
- Task `discarded`
- Lane `closed_out`
- Lane `discarded`
- active Gate에 새 조건 Task attach
- Gate에 연결된 Task pass
- Gate에 연결된 Task pass 취소
- Gate pass
- Workspace close

Task `implemented`는 구현 주체의 선언이므로 사람 승인 대상이 아니다.

### 14.1 HumanApprovalAttestation

V1에는 인증된 별도 승인 채널이 없으므로, LLM이 전달한 사람의 승인 발화를 **승인 진술**로 기록한다.

```text
id UUID
workspace_id
action: task_confirm | task_discard | lane_close_out | lane_discard | gate_attach_task | gate_pass_task | gate_revoke_task_pass | gate_pass | workspace_close
entity_type
entity_id
workspace_revision
approved_by_actor_id
approved_command_hash
decision_snapshot_hash nullable
statement_hash nullable
conversation_ref nullable
approved_at
recorded_at
executed_command_id UNIQUE
```

- 승인 진술은 action, 대상 entity, canonical command payload hash와 Workspace revision에 결속된다. Gate 통과처럼 조건 snapshot이 있는 action은 snapshot hash에도 결속된다.
- `task_confirm/task_discard`는 Task ID, `lane_close_out/lane_discard`는 Lane ID, `gate_attach_task/gate_pass`는 Gate ID, `gate_pass_task/gate_revoke_task_pass`는 `gate_tasks.id`, `workspace_close`는 Workspace ID를 대상으로 사용한다.
- 승인 대상 command는 공통 mutation envelope에 `humanApprovalAttestation` payload를 포함한다.
- 서버는 mutation transaction 안에서 승인 진술과 실행 command를 1:1로 기록한다.
- 같은 idempotency key와 command hash의 재시도는 같은 결과를 반환할 수 있지만, 승인 진술을 다른 command·action·entity·revision에 재사용할 수 없다.
- V1은 만료되거나 대기 중인 승인 요청을 저장하지 않는다. 장기 비동기 승인 생명주기는 후속 `ApprovalRequest`의 책임이다.
- V1의 승인은 protocol audit이며 사람 신원을 보안적으로 증명하지 않는다. 실제 신뢰는 단일 사용자 배포 보호와 Skill 준수에 의존한다.

### 14.2 파생된 승인 대기

V1에는 별도 `ApprovalRequest` entity를 만들지 않는다. 상태 머신과 query 결과에서 사람이 내려야 할 결정을 파생한다.

```text
Task implemented
  → decisionRequired: task.confirm

Gate ready
  → decisionRequired: gate.pass

마지막 active Phase + active Run 없음
  → decisionAvailable: workspace.close
```

`task.get`, `gate.status`, `workspace.get`과 `decision.list`는 대상 action, 대상 ID, expected Workspace revision, 관련 criteria/condition snapshot hash와 warning을 반환한다. Viewer는 각각 “완료확인 대기”, “Gate 통과 승인 대기”, “Workspace 종료 가능”으로 표시한다.

Operator는 사람 전용 action에 도달하면 실행을 멈추고 이 snapshot을 사람에게 제시한다. 사람이 승인하면 같은 revision과 snapshot에 결속된 `humanApprovalAttestation`으로 mutation을 실행한다. 상태가 바뀌면 승인 진술은 stale conflict다. 여러 사람의 장기 비동기 결재함이 필요해질 때만 후속 버전에서 `ApprovalRequest`를 추가한다.

사람 전용 mutation을 승인 없이 preview하면 서버는 `human_approval_required`와 canonical command hash를 반환한다. 이는 pending row를 만들지 않는 일회성 decision preview다. active Gate 조건 attach처럼 파생 상태가 아닌 제안도 같은 방식으로 승인받는다.

### 14.3 향후 인증과 capability

사용자 권한은 단순한 상하 등급이 아니라 API capability와 Workspace membership으로 나눈다. Role은 capability bundle이다.

| Role | 주요 capability |
| --- | --- |
| `viewer` | Workspace, graph, Task, Gate, Run, Record와 Event 조회 |
| `operator` | 일반 Task·관계 mutation, Run, Record와 Git metadata 운용 |
| `approver` | Task 확인·폐기, Lane 종료, Gate 조건 추가·pass/revoke와 Gate 통과 승인 |
| `owner` | 모든 Workspace capability, membership·설정과 Workspace 종료 |

API capability 이름은 최소 다음과 같이 분리한다.

```text
workspace:read
workspace:operate
run:operate
record:operate
task:approve
lane:approve
gate:approve
workspace:close
workspace:admin
```

인증 도입 후 API token은 subject Actor, Actor kind, Workspace membership과 capability scope를 가진다. Agent token에는 approval capability를 부여하지 않고, 사람 인증 session만 approval scope를 발급한다. `workspace.close`는 human `owner`에게만 허용한다. 정확한 capability와 role bundle은 [`contracts/v1/capabilities.json`](../contracts/v1/capabilities.json)을 따른다.

V1에는 제품 인증과 membership enforcement가 없으므로 이 capability 모델은 API contract의 목표 경계이며 보안적으로 강제되지 않는다. 단일 bootstrap human Actor를 protocol상 Owner로 취급하며, 실제 보호는 배포 계층과 HumanApprovalAttestation protocol audit에 의존한다.

## 15. Command와 MCP

### 15.1 원칙

- UI에는 direct edit form을 두지 않는다.
- LLM이 자연어를 typed command로 바꾼다.
- 서버가 구조적 invariant와 상태 전이를 검증한다.
- 의미 품질과 workflow 완성도는 warning으로 반환한다.
- 성공한 domain mutation은 Event를 남긴다. 고빈도 operational write인 `run.heartbeat`만 예외다.
- 기존 Workspace를 바꾸는 domain mutation은 idempotency key와 expected Workspace revision을 사용한다.
- `project.bootstrap`은 `client_project_id`를 global idempotency key로 사용하고, `run.heartbeat`는 lease token과 expected Run version을 사용한다.

공통 mutation envelope:

```text
idempotencyKey
expectedWorkspaceRevision nullable
initiatedByActorId nullable
executedByActorId
acknowledgedWarningCodes []
proceedReason nullable
humanApprovalAttestation nullable
```

Warning acknowledgement와 진행 사유는 개별 command payload가 아니라 공통 envelope에 둔다. 각 command는 자신이 평가한 warning code만 수락하며 Event payload에 평가 결과를 기록한다.

정본 command catalog는 [`contracts/v1/commands.json`](../contracts/v1/commands.json)이다. 각 mutation은 `requiredCapability`를 가지며 V1에서는 이를 응답과 Event에 기록만 한다. 인증 도입 후 token scope와 Actor kind를 함께 검사한다. active Gate attach처럼 상태에 따라 권한이 달라지는 command는 transaction 안에서 현재 상태를 기준으로 capability를 다시 계산한다.

### 15.2 주요 query

정확한 query 이름은 [`contracts/v1/commands.json`](../contracts/v1/commands.json)의 `queries`를 따른다. 조회는 상태를 바꾸지 않으며 `workspace:read` capability를 요구한다.

### 15.3 주요 mutation

정확한 mutation 이름, capability와 사람 승인 요구는 [`contracts/v1/commands.json`](../contracts/v1/commands.json)의 `mutations`를 따른다.

`run.start`는 필요하면 Task 시작을 같은 transaction에서 처리한다. Run heartbeat/종료, Record 등록과 Git 관찰은 LLM이 정상 workflow 안에서 자동 수행한다. V1은 별도 `task.start` command를 두지 않으며 사람 작업도 적절한 kind의 Run으로 기록한다.

`project.bootstrap`은 client가 생성한 `client_project_id`로 Workspace와 첫 Repository 등록을 원자적으로 수행하고 재호출 시 같은 결과를 반환한다. 로컬 `baley.yaml`, Task Record template과 `.rgignore` 작성은 bootstrap 결과를 받은 로컬 CLI/LLM이 preview 후 수행한다.

`workspace.create`도 생성 전에는 잠글 Workspace row가 없으므로 client가 생성한 `client_workspace_id`를 global idempotency scope로 사용한다. command 예약, Workspace insert, `workspace.created` Event를 하나의 transaction으로 처리하며 성공 후 생성된 `workspace_id`를 command row에 연결한다. `workspace.activate`는 첫 Phase가 하나 이상 존재할 때만 허용하고 기존 Workspace lock/revision 규칙을 따른다.

`gate.attach_task`는 미래 Gate에서는 일반 Operator mutation이고 active Gate에서는 `gate_attach_task` 사람 승인 진술을 요구한다. `gate.detach_task`는 미래 Gate에서만 허용한다. `lane.close_out`과 `lane.discard`는 항상 사람 승인 진술을 요구한다.

### 15.4 Preview와 execute

모든 일반 mutation은 같은 command envelope를 사용한다.

```text
POST /v1/commands/preview
  상태를 쓰지 않고 현재 revision에서 command를 평가

POST /v1/commands/execute
  preview와 같은 command shape를 실행
```

preview는 최소한 `commandHash`, `expectedWorkspaceRevision`, `requiredCapability`, `projectedDiff`, `errors`, `warnings`, `advisories`와 필요한 경우 `decisionSnapshotHash`를 반환한다. execute는 idempotency key, expected revision, acknowledged warning code, canonical command hash와 선택적 승인 진술을 transaction 안에서 다시 검증한다. Preview 결과는 예약이나 잠금이 아니며 그 뒤 상태가 바뀌면 execute가 `stale_revision`을 반환한다.

Run heartbeat처럼 별도 동시성 키를 쓰는 operational command도 command catalog에는 포함하지만 매번 사용자 preview를 요구하지 않는다. 자연어로 요청한 구조 변경은 Skill이 preview를 보여주고, 자동 Run/Record lifecycle은 동일한 서버 validation을 거쳐 조용히 실행한다.

### 15.5 Diagnostic

정확한 error, warning과 advisory code는 [`contracts/v1/diagnostics.json`](../contracts/v1/diagnostics.json)이 정본이다. 아래 목록은 의미를 설명하는 예시이며 literal registry가 아니다.

Hard error 예시:

- 존재하지 않는 Task
- 잘못된 상태 전이
- dependency cycle
- invalid dependency patch의 최종 graph
- 다른 Workspace entity 연결
- ready가 아닌 Gate pass
- intentional leaf Task에 후행 dependency 또는 Gate 조건 연결
- active Gate의 Task detach
- 사람 승인 진술 없는 Task confirm/discard, Lane close-out/discard, active Gate Task attach, Gate Task pass/revoke, Gate pass 또는 Workspace close
- stale revision

Warning 예시:

- 상세계획 Record 없음
- 독립 Agent 리뷰 Record 없음
- 완료보고 Record 없음
- 후행 Task, Gate 연결과 terminal reason이 모두 없는 `dangling_path`
- 뒤 Phase Task가 앞 Phase Task를 막는 `phase_order_inversion`
- Branch observation이 오래됨
- 등록 Record의 현재 hash가 달라짐

Advisory 예시:

- 구현완료 assessment에 잔여 위험 존재
- predecessor Task 재작업
- commit reference가 remote에서 검증되지 않음
- 독립 Agent 리뷰 provenance 미보고

Warning과 advisory는 mutation을 막지 않는다. `task.report_implemented`처럼 warning을 평가하는 command는 `acknowledgedWarningCodes`와 선택적 `proceedReason`을 받으며 Event에 평가 결과와 구현 주체의 진행 결정을 기록한다. 잔여 위험을 솔직히 기록한 사실 자체는 warning이 아니라 advisory다.

## 16. Transaction과 Event

### 16.1 동시성

- 기존 Workspace write는 Workspace row를 `FOR UPDATE`로 잠근다.
- Workspace revision을 optimistic concurrency token으로 사용한다.
- dependency cycle 검사는 같은 transaction에서 수행한다.
- Gate pass와 Phase 전이는 하나의 transaction이다.
- Workspace close와 마지막 Phase 완료는 하나의 transaction이다.
- `run.start`와 pending Task의 자동 시작은 하나의 transaction이다.
- Run terminal 전이는 version CAS를 사용한다.
- idempotency key 재호출은 기존 결과를 반환한다.

초기 1인 사용 규모에서는 coarse Workspace lock의 단순성을 우선한다.

### 16.2 Event

Event는 append-only 감사 기록이며 전체 상태의 유일한 정본인 event sourcing으로 사용하지 않는다.

```text
id UUID
workspace_id
workspace_revision
command_id
event_type
entity_type
entity_id
initiated_by nullable
executed_by nullable
approved_by nullable
payload JSONB
occurred_at
```

도메인 Event 이름은 mutation 결과를 과거형으로 표현한다. V1의 mutation별 Event는 다음과 같다.

```text
project.bootstrapped
repository.registered
workspace.created
workspace.activated
workspace.closed
phase.created
phase.activated
phase.completed
lane.created
lane.updated
lane.closed_out
lane.discarded
task.created
task.updated
task.terminal_set
task.terminal_cleared
task.started
task.implemented_reported
task.confirmed
task.discarded
task.rework_started
task.blocked
task.unblocked
dependency.connected
dependency.disconnected
dependency.patched
gate.created
gate.task_attached
gate.task_detached
gate.task_passed
gate.task_pass_revoked
gate.passed
run.started
run.succeeded
run.failed
run.cancelled
run.interrupted
run.corrected
human_approval_attestation.recorded
record.registered
record.commit_attached
commit.attached
git.observed
```

`run.heartbeat`는 고빈도 lease 유지용 operational write이므로 domain Event를 매번 추가하지 않는다. 최신 `heartbeat_at`, `lease_expires_at`과 command/idempotency 기록으로 추적하고, 시작·terminal·manual correction만 append-only Event를 남긴다. 따라서 “성공한 mutation은 Event를 남긴다”는 원칙의 명시적 V1 예외는 `run.heartbeat` 하나다.

## 17. Viewer

Viewer는 상태를 읽고 탐색한다.

제공:

- Multi-lane, Lane Focus, Gate Focus
- Phase/Lane/Task/Gate 관계
- Task의 숫자 ID와 상태
- Task 경로의 후행 Task, Gate 합류, intentional leaf 또는 `dangling_path`
- Run 이력과 자동 갱신 상태
- Task Record 종류, 요약, repository 상대 경로
- 연결 commit과 remote link
- Baley Event와 Git 증거를 연결한 시간선
- 완료확인·Gate 통과·Workspace 종료 같은 파생된 사람 결정 대기

제외:

- Task 및 관계 direct edit form
- 상태 dropdown
- dependency drag-and-drop
- Branch/worktree 관리
- 의미 품질 판정 UI

V1 외부 서버는 private repository 원문을 읽지 않으므로 Record 본문을 직접 표시하지 않아도 된다.

## 18. baley init

프로젝트 통합은 CLI 또는 LLM command로 초기화한다.

```text
baley init
```

수행 항목:

1. 외부 서버에 Workspace와 Repository 등록
2. `baley.yaml` 생성
3. `task-records/`와 template 생성
4. `.rgignore` 규칙 추가
5. Agent 기록·검색 지침 설치
6. bootstrap human/Agent attribution 설정

서버 bootstrap은 `client_project_id`로 idempotent하다. 로컬 파일 적용도 생성 예정, 기존 유지와 충돌을 preview하고 기존 파일을 덮어쓰지 않는다. 서버 bootstrap 후 로컬 적용이 실패해도 같은 ID로 재실행해 이어갈 수 있다.

## 19. PostgreSQL V1 table

Core:

```text
actors
workspaces
repositories
phases
lanes
tasks
task_dependencies
gates
gate_tasks
runs
task_record_indexes
commit_references
run_git_observations
human_approval_attestations
commands
events
workspace_counters
```

`commands` 최소 필드:

```text
id UUID
workspace_id nullable            # project.bootstrap/workspace.create 성공 전에는 없음
idempotency_scope
idempotency_key
command_hash
expected_workspace_revision nullable
initiated_by_actor_id nullable
executed_by_actor_id
status: received | succeeded | rejected | failed
result_json nullable
error_code nullable
attempt_count
last_attempt_at
created_at, completed_at nullable
```

같은 idempotency key와 다른 `command_hash`는 conflict다. `succeeded`와 결정적 validation `rejected`는 같은 결과를 재사용한다. domain transaction이 commit되지 않은 transient `failed`는 같은 key와 payload로 재실행할 수 있으며 `attempt_count`를 증가시킨다.

`workspace_counters`는 Workspace별 다음 Task public ID를 row lock으로 발급한다. `project.bootstrap`과 `workspace.create`는 각각 `client_project_id`, `client_workspace_id` 기반 global idempotency scope를 사용한다.

주요 DB constraint:

- Workspace 범위 FK와 uniqueness
- `UNIQUE(workspace_id, task.public_id)`
- `UNIQUE(workspace_id, runs.client_run_id)`
- `UNIQUE(workspace_id, task_record_indexes.id)`
- `UNIQUE(workspace_id, gate_tasks.id)`
- `UNIQUE(workspace_id, phase.position)`
- self dependency 금지
- dependency 최종 graph cycle 금지
- `terminal_reason`과 outgoing dependency/Gate link의 동시 존재 금지
- Gate endpoint same Workspace
- Gate–Task와 Task Record reference same Workspace
- `gate_tasks`와 `run_git_observations`를 포함한 모든 workspace-scoped relation에 `workspace_id`
- `(workspace_id, id)` 또는 동등한 composite FK로 cross-Workspace 연결 방지
- positive Workspace revision과 Task public ID

cross-table 상태 전이, cycle, Phase 순서와 Gate readiness는 Go command service가 transaction 안에서 강제한다.

## 20. 구현 순서

### Step 1 — Domain core

- Task 상태 머신
- parent/dependency invariant
- disconnected DAG component, merge와 dangling-path projection
- atomic dependency patch
- Gate readiness와 Phase 전이
- Run 상태 머신
- Run client ID, lease, heartbeat와 terminal CAS
- hard error와 warning 분리

### Step 2 — PostgreSQL

- Core table migration
- Workspace revision과 Task ID counter
- command idempotency와 Event
- 실행 command와 1:1인 사람 승인 진술
- capability-ready command authorization boundary
- repository 상대 경로 및 hash index

### Step 3 — Go API와 MCP

- query/mutation command
- preview와 warning
- derived `decision.list`와 approval snapshot
- Run 자동 갱신 tool
- Record/Git metadata 등록

### Step 4 — Project integration

- `baley init`
- `baley.yaml`
- Task Record template
- `.rgignore` 및 Agent 지침

### Step 5 — Viewer migration

- fixture를 API projection으로 교체
- 새 Task 상태와 단일 Gate–Task link 적용
- Run, Record index와 Git 증거 표시

### Step 6 — External deployment

- Docker Compose
- PostgreSQL backup
- deployment-level access protection
- health check와 migration procedure

## 21. 필수 인수 테스트

### Task

- 숫자 public ID 발급과 재사용 금지
- Task 생성과 초기 predecessor/successor 관계의 원자성
- `pending → in_progress → implemented → confirmed`
- `implemented → in_progress` 재작업
- 폐기 전이와 terminal 상태 보호
- 첫 Run 시작 시 Task 자동 시작
- 완료보고 누락은 warning이고 구현완료 선언은 가능
- 구현완료 assessment 누락은 hard error
- Task confirm/discard의 사람 승인 진술 대상·command hash·revision·executed command 1:1 결속
- intentional leaf의 terminal reason 기록과 해제
- `dangling_path`가 생성은 막지 않고 구현완료·완료확인·brief에서 warning
- terminal Task에 outgoing dependency 또는 Gate condition 연결 거부

### Dependency

- self-link, duplicate와 cycle 거부
- 한 Task의 복수 predecessor와 복수 successor 허용
- Workspace 안의 disconnected DAG component 허용
- 여러 predecessor가 하나의 merge Task로 합류
- cross-lane dependency 허용
- cross-Phase dependency 허용
- 뒤 Phase에서 앞 Phase로 향하는 dependency에 `phase_order_inversion` warning
- Gate와 무관한 cross-Phase dependency가 Gate readiness에 영향 없음
- predecessor implemented/confirmed 시 해소
- discarded predecessor는 자동 해소하지 않음
- 미해소 dependency가 implementation/review-response Run 시작을 차단
- 상세계획 Run은 미해소 dependency가 있어도 시작 가능
- 미래 Phase의 상세계획만 허용하고 나머지 Run은 거부
- 진행 중 successor에 dependency 연결 시 warning과 acknowledgement 기록
- predecessor rework가 기존 후행 Run을 되돌리지 않고 새 implementation Run을 차단
- dependency 방향 전환 patch가 remove/add를 원자적으로 적용
- dependency patch의 최종 graph가 cycle이면 전체 rollback
- patch 실패 시 기존 edge가 유실되지 않음

### Gate

- fromPhase가 아닌 Task 연결 거부
- 연결 Task 0개인 Gate pass 거부
- confirmed/passed 혼합으로 모두 충족되면 ready
- implemented 또는 discarded Task만으로는 ready가 되지 않음
- Gate pass와 Phase 전이 원자성
- Gate 비연결 이전 Phase Task의 계속 진행
- Gate Task pass와 Gate pass의 사람 승인 진술 검증
- Gate Task pass/revoke 승인이 정확한 Gate–Task 연결 ID에 결속
- 미래 Gate 조건은 Operator가 attach/detach 가능
- active Gate detach 거부 및 `pass_task`로만 조건 면제
- active Gate 조건 attach의 사람 승인 진술 검증
- passed Gate attach/detach/pass/revoke 거부
- active fromPhase가 아닌 Gate의 Task pass/revoke와 Gate pass 거부
- 현재 Gate를 건너뛴 미래 Gate 선행 통과 거부
- Gate pass Event에 통과 당시 Task 해소 근거 snapshot
- ready Gate가 `decisionRequired=gate.pass`와 criteria/condition snapshot을 반환

### Decision과 권한

- implemented Task가 완료확인 대기로 파생
- ready Gate가 Gate 통과 승인 대기로 파생
- 마지막 Phase와 Run 조건에서 Workspace 종료 가능 결정 파생
- revision 또는 snapshot 변경 후 기존 humanApprovalAttestation 거부
- Lane close-out/discard의 사람 승인 진술 검증
- V1 protocol audit가 인증된 사람 신원 증명이 아님을 API metadata에 표시
- command catalog가 각 mutation의 `requiredCapability`를 반환
- active Gate attach가 transaction 현재 상태에서 `gate:approve`를 요구
- 인증 단계 acceptance: Agent token으로 approval API 거부, `workspace.close`는 human Owner만 허용

### Run과 Record

- Run 자동 시작·성공·실패·중단 Event
- `run.start`와 pending Task 시작의 원자성
- 동일 client_run_id 중복 시작은 같은 Run 반환
- 동일 client_run_id와 다른 Task/kind/parent/target payload는 conflict
- 같은 idempotency key와 다른 payload 거부
- heartbeat lease 연장과 서버 재시작 후 stale Run interruption
- `run.succeed`와 timeout interruption 경합에서 terminal 전이 하나만 성공
- manual correction 사유 필수
- 독립 Agent 리뷰에 다른 session 강제 없음
- Record 상대 경로와 hash 등록
- client가 생성한 record_id가 front matter와 DB index에 동일하게 유지
- 로컬 절대 경로 등록 거부
- `..`, drive prefix, URI와 configured root 이탈 거부
- 같은 record ID와 같은 hash 재등록은 idempotent
- 같은 record ID의 다른 hash는 hard conflict
- commit/blob 연결
- uncommitted와 committed-unverified 상태 표시

### Git

- Branch/worktree 없이 commit만 Task에 연결 가능
- branch hint가 Task 상태에 영향 없음
- worktree 절대 경로를 서버에 저장하지 않음

### Server

- 동일 `client_workspace_id`의 Workspace 생성 재시도는 같은 결과 반환
- 동일 global idempotency key와 다른 생성 payload는 거부
- stale revision 거부
- idempotent 재호출의 중복 mutation 없음
- mutation과 Event 원자성
- 서버 재시작 후 상태 유지
- 마지막 Phase에서 Workspace close가 `active_phase_id=NULL`과 completed를 원자적으로 기록
- `baley init` 부분 실패 후 같은 client_project_id로 재실행
- 배포 기본값이 보호되지 않은 public bind를 허용하지 않음

## 22. V1 비범위

- 회원가입과 로그인 UI
- 다중 사용자 membership enforcement
- 장기 비동기 결재함과 persisted ApprovalRequest
- 완전한 offline command outbox
- Branch/worktree lifecycle 관리
- Git provider를 통한 private repository 원문 조회
- Task Record 원문 서버 복제
- 리뷰 독립성 또는 구현 품질 인증
- 계획·리뷰·보고 존재 여부의 강제 workflow
- 복합 Gate 조건식
- Phase 분기·병합·rollback
- cross-Workspace dependency
- Lane Group과 Lane fork
- 범용 workflow automation engine

## 23. Legacy migration

Visual MVP의 아래 표현은 fixture 전용 legacy이며 서버 정본으로 사용하지 않는다.

```text
Task done/running/blocked/ready
Lane active/close-out/discard
Phase.order
Gate open/ready/passed/reopened
Gate required/reference/unlocks
Execution
GitBinding
```

서버 migration:

- Task fixture 상태를 V1 Task 상태로 명시적으로 매핑
- `blocked`는 Task 상태가 아니라 blocker metadata로 이동
- Lane `close-out`은 `closed_out`, `discard`는 `discarded`로 매핑
- `Phase.order`는 `Phase.position`으로 매핑
- Gate `reopened`는 제거하며 reopen과 Phase rollback을 지원하지 않음
- Gate `required` Task만 단일 Gate–Task link로 migration
- `reference`와 `unlocks` edge 제거
- 다음 Phase Task는 Phase 활성화로 통제
- `Execution`은 `Run`, 결과 Git 관계는 `CommitReference`로 대체

이 문서는 Baley V1 도메인 의미와 불변식의 규범적 정본이다. 정확한 command·상태·diagnostic·capability literal은 [`contracts/v1`](../contracts/v1/README.md)이 정본이다.
