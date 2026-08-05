---
type: implementation-plan
status: draft-for-owner-review
scope: baley-orca-integration
last_active: 2026-08-05
related:
  - docs/baley-orca-integration-plan.md
  - docs/baley-system-spec-v1.md
  - docs/baley-command-architecture.md
  - contracts/v1
---

# Baley–Orca 연동 상세 구현 계획

## 1. 목표

Baley가 Lane·Task·Run·승인·증거의 장기 정본을 유지하고, Orca가 실제 코드 작업이
이루어지는 주 IDE가 되도록 연결한다. 첫 구현에서 반드시 성립해야 하는 사용자 경험은
다음 세 가지다.

1. Baley Task 하나에는 동시에 활성 Orca 작업이 최대 하나만 연결된다.
2. Orca 안의 Agent는 기존 Baley Skill과 MCP로 Task·Run·Record·Git evidence를 운용한다.
3. 사용자는 Baley Web Viewer의 Task Inspector에서 새 작업을 만들지 않고 연결된 Orca
   terminal로 즉시 이동한다.

Lane당 진행 중 Orca 작업 하나라는 개인 작업 방식도 지원한다. 다만 Baley의 일반 DAG와
병렬 Task 모델을 훼손하지 않도록 Task lock과 Lane WIP 정책을 분리한다.

## 2. 확정 설계 결정

### 2.1 정본과 역할

```text
Baley Workspace
└─ Lane                         장기 업무 전선
   └─ Baley Task               목표·관계·상태의 정본
      ├─ ExternalExecution     Orca 작업 컨테이너 연결과 시도 이력
      │  └─ Orca Worktree      실제 IDE 작업공간
      │     └─ Agent Terminal  Codex 등 실행 세션
      └─ Baley Run 0..N        계획·구현·리뷰·보고 시도
```

- Orca worktree는 Baley Task를 대체하지 않는다.
- Orca worktree는 Baley Run과 1:1로 고정하지 않는다.
- 하나의 Orca execution 아래 여러 종류의 Baley Run이 존재할 수 있다.
- Task 상태, dependency, Gate readiness와 사람 승인 경계는 기존 Baley 규칙을 따른다.
- Orca terminal 종료 또는 worktree 상태만으로 Task 상태를 추론하지 않는다.
- Viewer의 Orca 이동 action은 Baley mutation이 아닌 로컬 탐색 action이다.

### 2.2 Task lock과 Lane WIP

서버가 강제할 기본 불변식은 다음과 같다.

```text
하나의 Baley Task
→ provider=orca인 미종결 ExternalExecution 최대 1개
```

Lane WIP는 별도 정책이다.

- 기본 제품 동작은 같은 Lane의 다른 Task 실행을 허용한다.
- `external_execution_wip_limit=1`인 Lane에서는 다른 Task의 활성 실행이 있으면 진단한다.
- 첫 구현에서는 warning과 strict error 중 하나를 Workspace 또는 Lane 설정으로 선택할 수
  있도록 계약을 설계하되, UI 없이 설정하는 기능까지 MVP에 넣지는 않는다.
- 사용자의 로컬 Pilot 기본값은 strict `1`을 목표로 한다.

Task lock은 데이터 무결성이고 Lane WIP는 작업 방식이다. 둘을 같은 constraint로 구현하지
않는다.

### 2.3 Viewer에서 Orca로만 이동

첫 제품 흐름에는 `Baley Task를 Orca에서 열어줘` 같은 별도 Skill 명령을 넣지 않는다.
사용자는 이미 Orca를 주 IDE로 실행하고 있으며, 필요한 진입점은 다음이다.

```text
Baley Task Inspector
→ "Orca에서 계속하기"
→ local bridge
→ 연결된 worktree의 현재 terminal 확인
→ orca terminal switch
```

연결된 terminal이나 worktree가 없을 때 Viewer action이 새 작업을 자동 생성해서는 안 된다.
기존 실행, Git 결과와 연결 상태를 먼저 확인하도록 안내한다.

## 3. 범위와 비범위

### 3.1 MVP 범위

- Task와 Orca worktree의 안정적인 연결 저장
- Task별 활성 Orca execution 1개 lock
- 외부 시스템 생성 전 reserve와 생성 후 attach
- execution 관찰, 명시적 종결, lost 표시와 재연결
- 선택적인 `Run.external_execution_id` 연결
- Task/graph 조회 projection의 현재 execution 요약
- Viewer Task Inspector의 Orca 상태와 이동 버튼
- loopback local bridge를 통한 terminal 조회와 전환
- Baley MCP에서 external execution lifecycle 운용
- Event, revision, idempotency와 mutation attempt 감사
- Task lock, 복구와 bridge 보안 회귀 테스트

### 3.2 비범위

- Baley Server의 로컬 worktree 직접 관리
- Orca credential 또는 worktree 절대 경로 저장
- Orca 실행 종료에 따른 Task 자동 구현완료·확인
- heartbeat 만료에 따른 lock 자동 해제
- Viewer에서 임의 shell command 실행
- Orca worktree 삭제 또는 Git 변경 폐기
- Orca orchestration task와 Baley Task의 동일시
- 완전한 양방향 실시간 event 동기화
- 여러 IDE provider를 위한 범용 workflow engine

## 4. 도메인 모델

### 4.1 ExternalExecution

```text
ExternalExecution
- id UUID
- workspace_id UUID
- task_id UUID
- provider: orca
- external_id nullable
- provider_instance_id nullable
- host_id nullable
- status: creating | active | review | settled | lost
- attempt_number positive integer
- client_execution_id UUID
- context_snapshot_hash nullable
- last_terminal_handle nullable
- started_at
- last_observed_at nullable
- settled_at nullable
- settlement_reason nullable
- created_by_actor_id
- created_at, updated_at
```

필드 의미:

- `client_execution_id`는 reserve 재시도의 전역이 아닌 Workspace 범위 멱등 식별자다.
- `external_id`는 Orca가 반환한 안정적인 worktree ID다.
- `provider_instance_id`는 worktree 재생성·동명이인 구분을 위한 Orca instance ID다.
- `host_id`는 local 또는 remote Orca runtime을 식별하되 secret을 포함하지 않는다.
- `last_terminal_handle`은 최근 관찰값일 뿐 장기 식별자가 아니다.
- `context_snapshot_hash`는 Orca 시작 시 전달한 Task 맥락의 재현성을 돕는다.
- `attempt_number`는 Task·provider 범위에서 1부터 단조 증가하며 재사용하지 않는다.

### 4.2 상태 전이

```text
creating → active → review → settled
    │         │        │
    └─────────┴────────┴→ lost

lost → active       # 기존 worktree 재발견·재연결
lost → settled      # 조사 뒤 명시적 종결
```

규칙:

- `creating`, `active`, `review`, `lost`는 미종결이며 Task lock을 유지한다.
- `lost`는 새 실행 허가 상태가 아니라 조사 필요 상태다.
- `settled`만 다음 attempt 생성을 허용한다.
- `active → review`는 결과를 검토 중이며 중복 실행을 계속 막는다.
- `settled`는 terminal이고 V1에서는 reopen하지 않는다.
- 잘못 종결한 경우 기존 row를 되살리지 않고 감사 근거와 함께 새 attempt를 만든다.

### 4.3 종결 사유

```text
completed
abandoned
rejected
superseded
creation_failed
external_deleted_after_recovery
```

`completed`는 Orca 실행이 끝났다는 뜻이며 `task.report_implemented`를 자동 수행하지 않는다.
Task 구현완료에는 기존 assessment, test/build, Record와 warning 규칙을 그대로 적용한다.

### 4.4 Run 연결

`runs.external_execution_id`를 nullable FK로 추가한다.

- 연결된 Run과 execution은 같은 Workspace와 Task여야 한다.
- 외부 execution 없이 기존 Run을 시작할 수 있다.
- Orca 안에서 시작한 Run은 현재 active execution을 명시적으로 전달하는 것을 우선한다.
- 서버가 현재 execution을 자동 추론하는 기능은 모호성을 피하기 위해 후속으로 둔다.
- execution 종결은 연결된 running Run을 자동 종료하지 않는다.
- active Run이 남은 execution 종결에는 warning을 반환한다.

## 5. 데이터베이스 계획

### 5.1 Migration

새 migration은 다음을 포함한다.

1. `external_executions` table 생성
2. Workspace 범위 composite FK 구성
3. `runs.external_execution_id` nullable column과 composite FK 추가
4. Task·provider별 attempt counter uniqueness 추가
5. 미종결 Task lock partial unique index 추가
6. client execution id 멱등 uniqueness 추가
7. 상태·시간·settlement 일관성 CHECK 추가

핵심 index 예시:

```sql
CREATE UNIQUE INDEX external_executions_one_open_per_task_provider
ON external_executions (workspace_id, task_id, provider)
WHERE status IN ('creating', 'active', 'review', 'lost');

CREATE UNIQUE INDEX external_executions_client_id_unique
ON external_executions (workspace_id, client_execution_id);

CREATE UNIQUE INDEX external_executions_attempt_unique
ON external_executions (workspace_id, task_id, provider, attempt_number);
```

실제 SQL은 기존 migration의 Workspace composite FK와 naming convention을 따른다.

### 5.2 Transaction과 동시성

- reserve는 Workspace row lock과 expected revision 검사를 사용한다.
- 같은 Task에 대한 동시 reserve는 하나만 성공한다.
- 같은 `client_execution_id`와 동일 canonical payload 재호출은 기존 결과를 반환한다.
- 같은 ID와 다른 Task/provider/context는 idempotency conflict다.
- attach, observe, review, settle, lost, reconnect는 모두 Workspace revision을 증가시키고
  Event와 같은 transaction에서 commit한다.
- terminal focus 이동은 DB mutation이 아니므로 revision을 증가시키지 않는다.

## 6. 계약 변경

정확한 literal은 구현 전에 `contracts/v1`에 먼저 고정한다. 아래 이름은 계획 기준
제안이며 계약 리뷰에서 최종 확정한다.

### 6.1 Query

```text
external_execution.get
external_execution.list
external_execution.resolve_for_task
```

`resolve_for_task` 응답은 다음을 포함한다.

```text
taskId
execution nullable
navigationAvailability:
  available | worktree_only | different_host | lost | unavailable
observedAt nullable
```

Viewer가 terminal handle이나 경로를 신뢰해 직접 실행하지 않도록 navigation 세부 해석은
local bridge가 담당한다.

### 6.2 Mutation

```text
external_execution.reserve
external_execution.attach
external_execution.observe
external_execution.mark_review
external_execution.settle
external_execution.mark_lost
external_execution.reconnect
```

권장 capability:

- 조회: `workspace:read`
- lifecycle mutation: 기존 `workspace:operate`
- 별도 `external_execution:operate` 추가는 provider가 늘어 권한 분리가 필요해질 때 검토
- 모두 Operator action이며 사람 승인 진술을 요구하지 않음
- Task confirm, discard, Gate와 Workspace 권한은 기존 경계를 유지

### 6.3 Diagnostic

최소 diagnostic 후보:

```text
external_execution_already_active        error
external_execution_not_found             error
external_execution_payload_conflict      error
external_execution_invalid_transition     error
external_execution_task_mismatch          error
external_execution_provider_mismatch      error
external_execution_stale_observation      advisory
external_execution_lost                   warning
external_execution_has_running_runs       warning
lane_external_execution_wip_exceeded      warning 또는 strict error
orca_navigation_unavailable               advisory
orca_navigation_different_host            advisory
```

`lost`와 stale은 자동 lock 해제 근거가 아니다.

### 6.4 Event

```text
external_execution.reserved
external_execution.attached
external_execution.observed
external_execution.review_started
external_execution.settled
external_execution.marked_lost
external_execution.reconnected
```

`observed`가 고빈도 polling으로 사용되면 Event 양이 급증할 수 있다. MVP에서는 사용자 또는
workflow 경계의 관찰만 Event로 남기고, 후속 heartbeat성 관찰은 `run.heartbeat`처럼 명시적
operational write로 분리할지 성능 측정 뒤 결정한다.

## 7. Orca 연결 프로토콜

### 7.1 reserve–attach saga

Baley DB와 Orca runtime은 하나의 transaction을 공유할 수 없으므로 다음 순서를 사용한다.

```text
1. external_execution.reserve
   - Task lock 획득
   - status=creating
   - clientExecutionId와 context snapshot 기록

2. local bridge가 Orca CLI 호출
   - orca worktree create --agent codex --activate --json
   - Task 목표 전체를 복제하지 않고 Baley 참조와 최소 시작 맥락 전달

3. external_execution.attach
   - externalId, instanceId, hostId, 최근 terminal handle 기록
   - status=active
```

2단계 성공 후 3단계 응답이 유실되면 같은 `clientExecutionId`로 Orca 결과를 조회하고 attach를
재시도한다. 불일치가 발생하면 새 worktree를 자동 생성하지 않는다.

이번 합의의 핵심 사용 흐름은 기존 execution으로의 복귀이므로, worktree 생성 UI나 별도
사용자 Skill 문구는 MVP Viewer에 추가하지 않는다. reserve–attach는 bridge와 Agent workflow가
사용할 기반 계약으로 구현한다.

### 7.2 Orca 시작 context

Orca Agent에 전달하는 최소 context:

- Baley server와 Workspace ID
- Task public ID와 내부 Task ID
- external execution ID와 attempt number
- Task 제목, 현재 요약과 next action
- acceptance outcome 요약
- blocker/dependency 상태
- 정확한 관련 Task Record 상대 경로
- `Baley MCP로 Task를 fresh-read한 뒤 Run을 시작하라`는 지시
- 사람 전용 승인과 Gate 판단을 수행하지 말라는 경계

Task 설명 전체를 Orca metadata에 장기 복제하지 않는다. `context_snapshot_hash`와 시작
요약으로 당시 지시를 식별하고, 실제 작업 때 Baley MCP로 fresh-read한다.

### 7.3 Orca 관찰값

bridge가 읽을 수 있는 정보:

- worktree ID와 instance ID
- runtime/host ID
- terminal handle과 존재 여부
- head commit
- branch hint
- dirty 여부
- workspace status
- 마지막 관찰 시각

Baley에는 절대 경로를 보내지 않는다. Git 관련 값은 기존 `git.observe`, `commit.attach`와
중복 저장하지 않고 필요한 경우 해당 command를 함께 사용한다.

## 8. Local bridge

### 8.1 책임

local bridge는 Baley Viewer와 로컬 Orca CLI 사이의 제한된 adapter다.

```text
Viewer
→ POST http://127.0.0.1:<fixed-or-discovered-port>/v1/orca/focus
→ local bridge가 Baley execution을 fresh-read
→ orca worktree/terminal 조회
→ orca terminal switch
→ 결과 반환
```

bridge는 다음만 수행한다.

- execution ID에 연결된 provider/host/worktree 검증
- Orca runtime readiness 확인
- worktree에 속한 terminal 탐색
- `orca terminal switch --terminal <resolved handle>` 실행
- 구조화된 성공·복구 필요 결과 반환

### 8.2 보안 경계

- loopback에만 bind한다.
- 임의 command, executable, path 또는 terminal handle을 request에서 받지 않는다.
- request는 Baley `workspaceId`, `taskId`, `externalExecutionId`처럼 제한된 ID만 받는다.
- 허용된 Baley Viewer origin을 검사한다.
- 실행 전 Baley Server에서 Task–execution 관계를 다시 조회한다.
- provider가 `orca`가 아니거나 host가 현재 runtime과 다르면 거부한다.
- shell 문자열 조합 없이 고정 executable과 인자 배열을 사용한다.
- Baley session cookie나 Agent token을 URL, 로그 또는 Orca metadata에 노출하지 않는다.
- bridge 부재는 Viewer 전체 장애가 아니라 navigation unavailable 상태로 처리한다.

### 8.3 Focus 결과

```text
focused
worktree_available_no_terminal
different_host
execution_lost
orca_unavailable
bridge_unavailable
forbidden
```

- `focused`: 기존 terminal을 전면으로 전환
- `worktree_available_no_terminal`: 새 terminal을 자동 생성하지 않고 재개 안내
- `execution_lost`: 즉시 새 worktree를 만들지 않고 reconciliation 안내
- `different_host`: 대상 Orca host 정보를 표시하되 credential이나 주소는 노출하지 않음

## 9. Viewer 구현

### 9.1 Task Inspector projection

Task Inspector에 `Orca 작업` section을 추가한다.

```text
Orca 작업
상태: 진행 중
시도: 2
마지막 관찰: 4분 전
Worktree: rabbitfish
[Orca에서 계속하기]
```

표시 상태:

| 조건 | 표시 | action |
| --- | --- | --- |
| local active terminal | 진행 중 | Orca에서 계속하기 |
| worktree만 존재 | Terminal 없음 | Orca 작업공간 확인 |
| 다른 host | 다른 Orca에서 실행 중 | host 안내 |
| lost | 연결 확인 필요 | 복구 안내 |
| settled | 이전 Orca 시도 | action 없음 또는 이력 보기 |
| 연결 없음 | section 축약 | action 없음 |

### 9.2 Read-only 원칙

- focus 버튼은 Baley command를 실행하지 않는다.
- Task 상태, execution 상태와 관계를 Viewer에서 직접 수정하지 않는다.
- execution settle/reconnect 같은 변경은 Skill/MCP typed command로 수행한다.
- Viewer는 bridge 호출 결과를 표시하고 graph 정본을 다시 조회할 수 있다.

### 9.3 React 진단

focus 문제를 수정할 때 다음 경계를 development-only structured trace로 기록한다.

1. 사용자 click과 target execution ID
2. 계산된 navigation target과 버튼 상태
3. React의 선택 Task와 graph execution projection
4. bridge request/response 상태
5. Orca가 반환한 worktree/terminal 상태
6. 최종 DOM feedback과 focus 결과

예상과 실제가 처음 갈라지는 계층을 찾은 뒤 수정한다. 의존성 교체나 스타일 변경부터
시도하지 않는다.

## 10. 서버 구현 순서

### Wave A — 계약과 순수 도메인

1. System Spec에 ExternalExecution 정본 경계와 lock 의미 추가
2. Command Architecture에 reserve–attach, Viewer focus와 복구 흐름 추가
3. `contracts/v1/commands.json` query/mutation 추가
4. `states.json`, `diagnostics.json` literal 추가
5. ExternalExecution 상태 머신과 전이 단위 테스트
6. Task lock, Lane WIP evaluation과 Run 연결 규칙 단위 테스트

완료 기준:

- 문서와 계약 literal이 모순되지 않는다.
- Task lock과 lost 보존 의미가 순수 domain test로 고정된다.
- 사람 승인 action 목록이 변경되지 않는다.

### Wave B — PostgreSQL vertical slice

1. migration 작성
2. repository snapshot/projection 확장
3. reserve와 attach transaction 구현
4. observe/review/settle/lost/reconnect 구현
5. Run 선택적 FK와 조회 projection 연결
6. Event, idempotency와 revision 처리
7. migration up/down/up 및 concurrency integration test

완료 기준:

- 같은 Task의 동시 reserve 중 하나만 성공한다.
- attach 응답 유실 재시도가 중복 execution을 만들지 않는다.
- lost execution이 lock을 유지한다.
- settle 뒤에만 다음 attempt가 생성된다.

### Wave C — HTTP와 MCP

1. Workspace/Task별 execution query endpoint 추가
2. preview/execute command adapter 연결
3. MCP typed tool 추가
4. transport strict decode와 tenant/capability 검증
5. HTTP/MCP parity test

완료 기준:

- HTTP와 MCP가 같은 application service와 contract를 사용한다.
- Agent credential은 Operator 범위만 행사한다.
- external execution command로 사람 승인 action을 우회할 수 없다.

### Wave D — Viewer projection

1. API DTO와 domain model에 execution summary 추가
2. Task Inspector section 구현
3. navigation availability별 버튼과 안내 구현
4. bridge discovery와 실패 fallback 구현
5. development-only viewer trace 추가
6. component, navigation과 stale response test

완료 기준:

- 연결이 없는 기존 Workspace UI가 회귀하지 않는다.
- 버튼 클릭은 Baley mutation을 만들지 않는다.
- stale graph/bridge response가 다른 Task를 focus하지 않는다.

### Wave E — Local bridge와 Orca focus

1. loopback bridge skeleton과 health/capability endpoint
2. Baley execution fresh-read와 origin 검증
3. Orca runtime/worktree/terminal 조회
4. `orca terminal switch` 실행
5. terminal 없음, 다른 host, lost와 Orca 부재 처리
6. process argument injection과 credential leakage test
7. 실제 Orca 1.4.169 smoke test

완료 기준:

- Baley Task Inspector에서 기존 Orca terminal로 전환된다.
- request가 임의 shell command나 terminal handle을 주입할 수 없다.
- terminal/worktree 불일치 시 새 작업이 자동 생성되지 않는다.

### Wave F — Lane WIP와 Pilot

1. Lane/Workspace execution WIP policy 계약 확정
2. 사용자 Pilot Lane에 strict WIP 1 적용
3. 같은 Lane의 다른 Task 실행 시 preview 진단 검증
4. 중단·lost·review 중 lock 동작 검증
5. 실제 복귀 시간과 중복 생성 건수 측정

## 11. 실패·복구 매트릭스

| 상황 | 서버 상태 | 기본 대응 | 자동 새 실행 |
| --- | --- | --- | --- |
| reserve 성공, Orca 생성 실패 | creating | 같은 ID로 조사 후 `creation_failed` settle | 금지 |
| Orca 생성 성공, attach 응답 유실 | creating/active 불명 | 같은 client ID로 조회·attach 재시도 | 금지 |
| Task에 active execution 존재 | active/review/lost | 기존 연결 반환 | 금지 |
| terminal 종료, worktree 존재 | active | worktree 확인과 수동 재개 안내 | 금지 |
| worktree를 찾을 수 없음 | lost 후보 | mark_lost 후 Git/Orca 조사 | 금지 |
| 다른 host에서 실행 중 | active | 대상 host 안내 | 금지 |
| observe event 중복·역순 | 기존 상태 유지 | version/time 검증 후 idempotent 처리 | 해당 없음 |
| 검토 중 새 실행 요청 | review | Task lock error | 금지 |
| heartbeat/관찰 오래됨 | active | stale advisory | 금지 |
| settle 시 running Run 존재 | active/review | warning acknowledgement 요구 | 금지 |
| 잘못된 Task에 reconnect | lost | task mismatch error | 금지 |

복구는 기존 Orca worktree, execution ID와 Git 증거 확인을 우선한다. 시간 경과만으로
`settled`로 바꾸거나 partial unique lock을 해제하지 않는다.

## 12. 테스트 계획

### 12.1 Domain

- 모든 허용·금지 상태 전이
- creating/active/review/lost의 lock 유지
- settled 후 attempt number 증가
- Task/Workspace/provider mismatch 거부
- active Run이 있는 settle warning
- Lane WIP warning/strict mode

### 12.2 PostgreSQL integration

- 같은 Task 동시 reserve 경합
- 다른 Task와 다른 Lane의 독립 실행 허용
- client ID 동일 payload idempotency
- client ID 다른 payload conflict
- mutation과 Event 원자성
- stale Workspace revision 거부
- lost→reconnect와 lost→settled
- cross-Workspace FK 거부
- Run과 execution의 Task 불일치 거부
- migration up/down/up

통합 테스트는 기존 database safety guard를 사용하고 개발/운영 Baley DB에 연결하지 않는다.

### 12.3 HTTP/MCP

- query와 mutation schema parity
- strict unknown field 거부
- Workspace tenant 격리
- Viewer는 조회만 가능하고 Operator만 mutation 가능
- MCP retry가 execution을 중복 생성하지 않음
- 기존 Task/Run/Record tool 회귀 없음

### 12.4 Viewer

- execution 상태별 Inspector 표현
- 연결 없음 회귀
- Task 전환 중 stale bridge response 무시
- click→target→bridge→DOM structured trace
- bridge unavailable fallback
- 다른 host/lost/worktree-only 안내
- focus action이 Baley execute endpoint를 호출하지 않음

### 12.5 Local bridge

- loopback 외 bind 거부
- origin allowlist
- 임의 executable/path/terminal handle 입력 거부
- execution 관계 fresh-read
- 실제 worktree terminal resolve와 switch
- terminal 종료 및 worktree 소실
- Orca runtime unavailable
- secret, cookie와 로컬 절대 경로 로그 미노출

## 13. 관측과 운영

측정할 지표:

- Viewer focus 요청 성공률
- bridge unavailable/different host/lost 비율
- Task당 중복 reserve 거부 횟수
- creating 상태 체류 시간
- lost execution 수와 평균 복구 시간
- terminal handle 재해석 성공률
- Lane WIP 초과 시도 수
- Baley Task에서 Orca terminal 복귀까지 걸린 시간
- execution 결과 중 Run·commit·Record가 연결된 비율

로그에는 Workspace/Task/execution의 opaque ID와 결과 code만 남긴다. credential, Task 본문,
로컬 경로와 terminal 출력은 기록하지 않는다.

## 14. 배포와 호환성

- ExternalExecution이 없는 기존 Workspace projection은 그대로 동작해야 한다.
- 새 DB table과 nullable Run FK는 기존 데이터 backfill을 요구하지 않는다.
- Viewer는 bridge가 설치되지 않아도 읽기 기능 전체를 제공한다.
- Orca app version/capability를 bridge health 응답에서 확인한다.
- 현재 확인 기준 Orca 1.4.169의 `worktree current/list/show`, `terminal list/switch`,
  JSON 출력과 worktree/terminal 식별자를 baseline으로 삼는다.
- 공식 Orca deep link가 생기면 bridge를 제거하기보다 동일 focus contract의 adapter로 추가한다.
- 다른 IDE provider 지원은 `provider` 확장으로 가능하게 하되 MVP는 `orca`만 허용한다.

## 15. 수용 기준

### 데이터와 command

- [ ] Task 하나에 미종결 Orca execution이 두 개 생성되지 않는다.
- [ ] reserve와 attach가 외부 생성 부분 실패에서 idempotent하게 복구된다.
- [ ] lost 상태가 lock을 자동 해제하지 않는다.
- [ ] settle 뒤 새 attempt가 과거 이력을 보존하며 생성된다.
- [ ] execution lifecycle과 Event가 같은 Workspace revision transaction에 기록된다.
- [ ] 기존 Task/Run/Gate 승인 의미가 바뀌지 않는다.

### Orca와 MCP

- [ ] Orca Agent가 Baley MCP로 연결 Task를 fresh-read하고 Run을 운용한다.
- [ ] 한 Orca execution에 여러 Baley Run을 연결할 수 있다.
- [ ] worktree/terminal 관찰이 Task 상태나 완료 판단을 자동 변경하지 않는다.
- [ ] Orca 생성/연결 재시도가 고아 또는 중복 worktree를 만들지 않는다.

### Viewer와 bridge

- [ ] Task Inspector가 현재 Orca execution 상태를 표시한다.
- [ ] `Orca에서 계속하기`가 연결된 기존 terminal을 전면으로 전환한다.
- [ ] focus action은 Baley mutation을 만들지 않는다.
- [ ] terminal이 없거나 worktree가 소실되면 새 작업을 만들지 않고 복구 안내를 표시한다.
- [ ] bridge가 임의 command 실행과 credential/path 노출을 허용하지 않는다.
- [ ] bridge 부재가 Viewer의 일반 탐색을 방해하지 않는다.

### 개인 작업 방식

- [ ] Pilot Lane에 WIP 1 정책을 적용할 수 있다.
- [ ] 같은 Lane의 다른 Task 실행 시 명확한 진단을 제공한다.
- [ ] WIP 정책을 사용하지 않는 Workspace에서는 Baley의 병렬 Task 모델이 유지된다.

## 16. 구현 전 결정이 필요한 항목

아래 항목은 Wave A 계약 리뷰에서 확정한다.

1. `lost`를 미종결 lock 상태로 둘지 별도 `investigating` 상태를 추가할지
2. Lane WIP 정책을 Lane 필드로 둘지 Workspace 기본값과 Lane override로 둘지
3. `external_execution.observe`를 Event mutation으로 유지할 빈도 한계
4. bridge discovery를 고정 loopback port, well-known file 또는 Orca hook으로 할지
5. Orca worktree 표시 이름과 Task public ID naming convention
6. terminal이 없고 worktree만 있을 때 bridge가 terminal 생성 없이 Orca UI에서 worktree만
   활성화할 공식 명령이 있는지
7. `Run.external_execution_id`를 `run.start` 필수 명시로만 받을지 Agent 환경에서 안전하게
   자동 제안할지

이 결정들은 ExternalExecution의 핵심 정본과 Task lock을 바꾸지 않는다. 구현은 Wave A의
계약 및 인수 테스트가 합의된 뒤 migration부터 진행한다.

## 17. 권장 첫 vertical slice

전체 계획 중 가장 작은 유효 제품 단위는 다음이다.

```text
ExternalExecution reserve/attach/resolve
→ Task별 active lock
→ graph/Task 조회 projection
→ Task Inspector의 Orca 상태
→ local bridge의 기존 terminal resolve/switch
```

이 slice에서는 자동 상태 관찰, Lane WIP UI, 결과 증거 동기화를 미룬다. 먼저 “Baley에서
현재 Task를 보고 클릭 한 번으로 살아 있는 Orca 작업에 복귀한다”는 핵심 경험과 중복 방지
효과를 실제 사용으로 검증한다.
