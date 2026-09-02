---
type: architecture
status: active
authority: derived
last_active: 2026-07-17
when_to_read: "Baley의 Operator 명령, Skill, MCP, 자동 Run 갱신 또는 사람 승인 경계를 설계하거나 변경할 때"
affects:
  - docs/baley-system-spec-v1.md
  - docs/baley-product.md
  - docs/baley-roadmap.md
  - .agents/skills/baley-manage-work/SKILL.md
---

# Baley Command Architecture

## 1. 결정

Baley Web은 read-only Viewer다. 사람 또는 Agent가 Operator가 될 수 있고 LLM/Agent가 기본 Operator다. Operator는 Skill과 remote MCP tool을 통해 Task, 관계, Run과 Record index를 운용한다.

```text
사람 또는 Agent Operator의 workflow
→ Baley Skill: 의도와 대상 확인
→ typed MCP command
→ Go command service: 구조 검증
→ mutation + Event
→ Viewer 갱신
```

Operator는 Baley DB를 직접 수정하지 않는다. Task Record 원문은 로컬 repository에 직접 작성하고 Baley에는 상대 경로·hash·commit만 등록한다.

## 2. 자동 처리와 승인 처리

Operator가 정상 workflow에서 수행:

- Task 조회와 context 선택
- Task 시작
- Run 시작·성공·실패·중단 보고
- 상세계획, Handoff, 독립 Agent 리뷰, 리뷰 반영과 완료보고 작성
- Task Record 경로와 hash 등록
- commit과 Record blob 연결
- 구현완료 선언과 assessment 기록

사람 승인이 필요한 처리:

- Task 완료확인
- Task 폐기
- Lane close-out과 discard
- active Gate에 새 조건 Task attach
- Gate에 연결된 Task pass
- Gate에 연결된 Task pass 취소
- Gate pass와 Phase 전이
- Workspace close는 human Owner 승인

Run 상태 갱신과 Record 등록을 매번 사람에게 확인하지 않는다. manual correction은 예외이며 사유와 Event를 요구한다.

Agent credential은 인간 승인 권한을 파생하거나 위임하지 않는다. 사람 전용 command는 로그인한 Viewer 세션에서 fresh preview를 확인한 사람이 발급한 5분 single-use approval grant ID를 실행 envelope의 `approvalGrantId`로 참조한다. 서버는 grant를 Account, human Actor, browser session, Workspace, action, entity, revision, canonical command hash, decision snapshot, warning acknowledgement, proceed-reason digest와 결속하고 실행 transaction에서 원자적으로 소비한다. session/membership/capability가 철회되거나 grant가 만료·소비되면 재사용할 수 없다. enforced HTTP/MCP는 `humanApprovalAttestation`, `approvedByActorId` 같은 legacy body authority를 거부한다. grant ID는 secret이 아니며 plaintext secret, header, 환경 변수 또는 token 복사 경로를 두지 않는다.

## 3. Skill, MCP와 로컬 filesystem

- **System Spec**: 도메인 의미와 불변식의 규범적 정본이다.
- **`contracts/v1`**: command·상태·diagnostic·capability literal의 기계 판독 정본이다.
- **HTTP API**: 정본 계약을 구현하는 transport다.
- **MCP**: HTTP API를 1:1 typed tool로 노출하는 얇은 adapter다. 별도 domain rule을 갖지 않는다.
- **Skill**: Baley 용어, Task 참조, Operator workflow, preview, 자동 갱신과 승인 경계를 LLM/Agent에 가르친다. 서버 invariant를 복제하지 않는다.
- **로컬 LLM 도구**: 코드, Task Record와 Git을 조작한다.
- **Go Server**: 상태 전이, 관계 무결성, command transaction과 Event를 강제한다.
- **Viewer**: 서버 상태와 repository/Git reference를 표시한다.

외부 Baley Server는 로컬 파일을 읽거나 worktree를 관리하지 않는다.

## 4. Task ID

Task는 Workspace 범위에서 유일한 양의 정수 public ID를 가진다.

```text
#104
task #104
task104
task 104
104번 task
```

숫자만 등장해 Task 참조인지 불명확하면 LLM은 추측하지 않는다.

## 5. Tool surface

정확한 query와 mutation 이름, capability 및 승인 요구는 [`contracts/v1/commands.json`](../contracts/v1/commands.json)을 따른다. 이 문서는 도구를 어떤 흐름으로 사용하는지 설명하고 목록을 복제하지 않는다.

### 5.1 Graph mutation

한 Task는 여러 predecessor와 successor를 가질 수 있고 Workspace에는 disconnected DAG component가 여러 개 존재할 수 있다. Dependency는 Lane과 Phase 경계를 넘을 수 있다. 뒤 Phase에서 앞 Phase로 향하는 관계는 허용하되 `phase_order_inversion` warning을 반환한다. Edge 방향 전환이나 복수 관계 변경은 `dependency.patch`의 remove/add로 원자적으로 처리한다. 서버는 최종 Workspace graph를 검증하고 cycle이면 전체 mutation을 rollback한다.

Dependency와 Gate 조건은 별도 관계다. Cross-Phase dependency는 그 자체로 중간 Gate의 조건이 아니며, Gate readiness에는 해당 Gate에 명시적으로 attach된 Task만 반영한다.

Gate는 Workspace별 public number `G#<n>`를 가지며 내부 `gateId`는 안정 식별자로
유지한다. 선택적 alias를 둘 수 있고, Gate를 받는 HTTP/CLI/MCP 명령은 `gateId`,
`G#<n>`, alias를 같은 reference 입력으로 해석한다. Viewer는 `G#<n>`을 우선 표시하고
Inspector에서 alias와 내부 gateId를 함께 보여준다.
정규형 `G#[1-9][0-9]*`은 public reference 전용 namespace다. 새 내부 `gateId`로
사용할 수 없고, 존재하지 않는 `G#<n>`은 내부 ID나 alias로 fallback하지 않는다.

새 Task는 `task.create`의 `predecessorTaskIds`와 `successorTaskIds`로 초기 관계까지 같은 transaction에서 만든다. 두 집합 사이에 기존 direct edge가 있으면 새 Task가 그 route에 삽입된 것으로 해석하여 기존 edge를 제거하고 새 두 edge로 원자적으로 대체한다. Task를 먼저 만들고 나중에 연결하다 실패하는 부분 성공을 피한다.

후행 Task와 Gate 조건이 없는 경로는 `task.set_terminal`의 사유가 없으면 `dangling_path` warning이다. Operator는 후행 연결, Gate 합류 또는 intentional leaf 중 하나를 선택한다.

Lane Backlog는 Task와 분리된 Phase 미정 planning intake다. `backlog.create`,
`update`, `move`, `reorder`, `discard`는 active item을 lane 범위에서 운용한다.
`backlog.promote`만 명시적인 target Phase와 Task 관계 의도를 받아 기존
`task.create` planner를 재사용하고 pending Task 생성, dependency, Backlog terminal
전이, counter, Event와 Workspace revision을 한 transaction에서 처리한다. 승격은
Gate 조건 또는 Gate entry Task를 자동 변경하지 않는다.

`task.block`은 상태 전이가 아니라 blocker metadata 변경이다. 새 implementation/review-response Run과 구현완료 보고를 막지만, 상세계획·독립 Agent 리뷰·완료보고 Run은 허용하고 이미 실행 중인 Run은 자동 중단하지 않는다. 해제는 `task.unblock`으로 명시한다.

### 5.2 승인 대기와 active Gate

- Task `implemented`는 `decisionRequired=task.confirm`이다.
- Gate `ready`는 `decisionRequired=gate.pass`다.
- 마지막 active Phase에 active Run이 없으면 `decisionAvailable=workspace.close`다.
- 미래 Gate 조건은 Operator가 attach/detach한다.
- active Gate는 detach할 수 없고 조건 면제는 `gate.pass_task`로 기록한다.
- active Gate 조건 추가는 사람 승인 진술이 필요하다.
- Gate entry binding은 `toPhase` Task만 explicit attach/detach하며 Gate readiness나 dependency를 바꾸지 않는다.
- explicit entry가 없으면 `toPhase`의 same-Phase incoming dependency가 없는 DAG root를 public ID 순으로 read-only 투영한다.

Query는 action, target, expected Workspace revision과 condition snapshot hash를 반환한다. `task.confirm`은 implemented Task의 Inspector에서 사람이 구현 결과와 evidence를 읽고 `Confirm task` 버튼을 명시적으로 누르는 전용 흐름을 기본으로 한다. 버튼은 내부적으로 exact command의 fresh preview를 만들고 warning과 잔여 위험을 보여준 뒤, 같은 CSRF-protected browser session으로 command-specific single-use grant를 발급해 즉시 실행한다. 사용자는 일반 Task 확인을 위해 command JSON을 작성하지 않는다. 범용 Human approval panel의 JSON 입력은 다른 사람 전용 명령을 위한 고급 진단 fallback으로만 유지한다. 서버는 Agent bearer에서 승인 Actor를 파생하지 않는다. 하나의 grant는 하나의 command만 승인하므로 grouped `task.confirm` 실행은 지원하지 않는다.

Task 완료 확인은 Task별 Viewer approval로 수행한다. 여러 Task가 `implemented`여도 각각 fresh preview와 별도 browser grant가 필요하며, 앞 command의 revision 변화 뒤에는 다음 Task를 다시 preview해야 한다. `dangling_path` 같은 topology warning은 구현 품질 실패나 terminal 승인으로 표현하지 않으며, 사람이 exact warning을 확인하고 grant를 발급해야 한다.

주 Task 구현이 다른 Task에도 영향을 주면 LLM이 관련 열린 Task를 분류한다. 이미 `implemented`여도 assessment와 commit·test/build·독립 리뷰 증거가 acceptance를 실제로 충족하는지 다시 확인한 뒤 공동 확인 대상에 넣는다. 부족하면 Agent가 `task.rework`로 되돌린다. 같은 증거가 `pending` 또는 `in_progress` Task의 범위를 완전히 충족하면 공유 증거 assessment를 남기고 정상 workflow로 먼저 `implemented` 보고한 뒤 공동 확인한다. 실제 구현이 아니라 필요성이 사라진 Task는 완료가 아니라 `task.discard`로 제안하고, 대체된 경우 사유에 `superseded by #<id>`를 기록한다. 부분 충족 또는 불확실한 Task는 열린 상태를 유지하며, 이미 confirmed/discarded인 terminal Task에 새 일이 생기면 follow-up Task를 생성한다. 사람 승인은 이 분류나 상태 머신을 우회하지 않는다.

### 5.3 API capability 경계

향후 인증에서는 Role을 capability bundle으로 구현한다.

```text
viewer   → query
operator → 일반 graph mutation, Run, Record, Git metadata
approver → 사람 전용 Task/Lane/Gate 승인
owner    → membership, Workspace 설정과 close
```

Agent token에는 approval scope를 부여하지 않는다. 사람 전용 command는 approval grant를 발급한 human session의 현재 capability와 exact grant binding을 함께 검증하며 Agent credential의 creator/connector는 authority가 아니다. 정확한 bundle은 [`contracts/v1/capabilities.json`](../contracts/v1/capabilities.json)을 따른다.

### 5.4 Preview와 execute

일반 mutation은 동일한 command shape로 두 경로를 사용한다.

```text
POST /v1/commands/preview  → write 없이 diff와 진단 계산
POST /v1/commands/execute  → revision과 command hash를 재검증하고 실행
```

Preview는 command hash, expected Workspace revision, required capability, projected diff, error/warning/advisory와 선택적 decision snapshot hash를 반환한다. 사람 승인 command는 이 preview를 통해 `human_approval_required`와 결속 정보를 얻는다. 이 필드는 기본적으로 Agent가 audit 결속에 사용하며, 사람에게는 판단 가능한 outcome-first brief를 제공한다. Run heartbeat와 자동 Record 등록은 사용자에게 매번 preview를 보여주지 않지만 같은 서버 계약과 검증을 사용한다.

## 6. 자동 Run 예시

사용자:

```text
task #104 구현을 진행해
```

기본 Agent Operator workflow:

```text
1. task.get #104
2. run.start(kind=implementation, client_run_id=uuid)
3. 로컬 repository에서 구현
4. 완료보고 파일 작성
5. record.register(client_record_id, path, hash)
6. commit이 있으면 commit.attach
7. run.succeed(result_summary)
8. task.report_implemented(assessment, completion_record_id)
```

`run.start`가 Task가 pending일 때 같은 transaction에서 자동 시작한다. implementation과 review-response Run은 미해소 dependency가 있으면 거부되지만 상세계획, 독립 Agent 리뷰와 완료보고 Run은 시작할 수 있다. 상세계획과 독립 Agent 리뷰가 없어도 서버는 구현완료를 의미상 거부하지 않는다. 누락은 warning으로 반환하고 판단은 구현 주체가 기록한다.

Run은 lease token과 heartbeat를 사용한다. raw token은 영속화하지 않고 외부 secret과 Run ID의 HMAC으로 재구성한다. 같은 client run ID 재호출은 같은 Run과 같은 token을 반환하며 Run lease/version을 갱신하지 않는다. terminal 전이는 version CAS로 하나만 성공한다.

## 7. Record 등록

LLM이 로컬 파일을 작성한 후 등록한다.

```json
{
  "name": "record.register",
  "arguments": {
    "recordId": "client-generated-uuid",
    "taskId": 104,
    "runId": "uuid",
    "recordType": "completion-report",
    "repositoryId": "uuid",
    "relativePath": "task-records/task-104/completion-report-01.md",
    "workingTreeHash": "sha256:...",
    "shortSummary": "Pilot UI 구현과 테스트 결과"
  }
}
```

서버는 로컬 절대 경로를 받지 않는다. Git commit 후 같은 Record에 commit SHA와 blob SHA를 연결한다.

## 8. Hard error와 warning

Hard error는 구조 무결성과 권한 위반이고 warning은 진행 전 확인할 업무상 위험이며 advisory는 비차단 참고 정보다. 정확한 code는 [`contracts/v1/diagnostics.json`](../contracts/v1/diagnostics.json)을 따른다. 잔여 위험은 warning이 아니라 advisory다. Warning과 advisory는 command를 막지 않으며 적용 command는 평가 결과와 acknowledgement를 Event에 기록한다.

## 9. Command transaction

- mutation은 Workspace revision을 확인한다.
- 기존 Workspace write는 같은 Workspace row lock을 사용한다.
- idempotency key가 같은 재호출은 기존 결과를 반환한다.
- 성공한 domain mutation과 Event는 같은 transaction에 기록한다. `run.heartbeat`는 domain Event를 만들지 않는 operational write다.
- Gate pass와 두 Phase 상태 변경은 원자적이다.

## 10. UI 범위

Web에서 허용:

- Multi-lane, Lane Focus, Gate Focus
- Task/Gate 선택과 탐색
- Run과 Record index 확인
- commit과 Event 확인
- LLM command 진입점

Web에서 제외:

- direct edit form
- 상태 dropdown
- dependency drag-and-drop
- Gate pass button
- Branch/worktree 관리

UI 안의 command bar가 추가되더라도 동일한 Skill/MCP command 경로를 사용한다.
