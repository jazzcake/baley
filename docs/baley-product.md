---
type: spec
status: active
authority: intent
last_active: 2026-07-17
when_to_read: "Baley의 제품 경계, 도메인 개념, UX 원칙을 확인하거나 변경할 때"
affects:
  - docs/baley-roadmap.md
  - docs/baley-visual-mvp-architecture.md
  - docs/baley-visual-grammar.md
  - docs/baley-command-architecture.md
  - docs/baley-system-spec-v1.md
---

# Baley 제품 기획서

## 1. 한 문장 정의

Baley는 여러 저장소와 사람·AI 작업을 장기간 기억하는 **lane 중심 업무 관리 시스템**이다. 각 lane의 진행은 하나 이상의 task dependency DAG component로 표현하며, 여러 lane은 공유 Gate를 통해 동기화될 수 있다.

## 2. 해결하려는 문제

AI와 개발하면 한 사람이 여러 층위의 업무를 동시에 진행하게 된다. 각 작업은 별도 세션, 브랜치, worktree, commit에서 실행되고 며칠 또는 몇 주 동안 중단되기도 한다. 풀타임으로 프로젝트를 추적하지 않으면 다음 맥락을 잊기 쉽다.

- 어떤 업무 전선이 열려 있는가
- 각 전선은 어디까지 왔고 무엇이 막혀 있는가
- 어떤 결정과 task가 현재 상태를 만들었는가
- 여러 업무 전선이 어디서 함께 준비되어야 하는가
- 다음에 실행할 수 있는 일은 무엇인가
- AI가 수행한 작업과 실제 Git 변경이 어떻게 연결되는가

기존 task manager는 issue, 담당자 또는 프로젝트를 중심축으로 삼는다. Baley는 지속되는 업무 흐름인 lane을 중심축으로 삼아, 중단 후에도 진행 맥락을 복원하는 것을 우선한다.

## 3. 제품 관점

Baley는 단순한 issue tracker나 AI 실행기가 아니다.

> 무엇을 해야 하는지 나열하기보다, 여러 업무 흐름이 지금 어떻게 전개되고 있는지를 사람과 AI가 함께 기억하고 운용한다.

Baley의 작업 주체는 `Operator`다. 사람과 Agent 모두 Operator가 될 수 있으며 LLM/Agent가 기본 Operator로서 task 생성·분해·진행·수집·보고와 Git 작업을 수행한다. 사람은 Operator가 될 수 있는 동시에 최종 판단, 승인과 권한을 맡는다.

프로젝트명은 Isaac Asimov의 로봇 소설에 등장하는 인간 형사 Elijah Baley에서 가져왔다. AI 자체보다 AI와 함께 일하며 판단하고 조율하는 인간의 관점을 나타낸다.

## 4. 핵심 원칙

1. **Lane first**: task나 담당자가 아니라 lane이 기본 탐색 및 회상 단위다.
2. **Graph is the interface**: DAG가 주 작업 화면이며 목록과 상세 패널은 이를 보조한다.
3. **Command over direct editing**: 사용자는 LLM에 의미를 명령하고 Skill과 command tool이 구조화된 변경을 preview·검증·실행한다. 그래프 UI는 상태를 읽고 탐색한다.
4. **Repository and Git first**: 코드, 프로젝트 문서와 Task Record 원문은 repository가 소유하고 Git history를 1급 이력으로 사용한다.
5. **Durable context**: Baley는 Task 상태, Run, Record index, Git 증거와 다음 행동을 연결한다.
6. **Human authority**: 구현완료는 구현 주체가 선언한다. Task 완료확인·폐기, Lane 종료, active Gate 조건 추가, Gate Task pass/revoke와 Gate 통과는 사람이 승인하고 Workspace 종료는 human Owner가 승인한다.
7. **Repository independent**: Baley는 Day Tripper와 독립된 제품이다. Day Tripper는 첫 파일럿일 뿐이다.
8. **Shared semantics**: 웹 UI, Go CLI와 AI skill이 동일한 API 및 도메인 규칙을 사용한다.

## 5. 도메인 모델

```text
Workspace
├─ Repositories
├─ Phases
├─ Lanes
│  ├─ Tasks
│  ├─ Dependency edges
│  ├─ Intentional leaves
│  ├─ Runs
│  ├─ Task Record indexes
│  └─ Commit references
├─ Gates
│  └─ Connected tasks
└─ Event history
```

### 5.1 Workspace

협업, 권한과 데이터 격리의 최상위 경계다. 1인 사용부터 2~3인 팀까지 지원한다.

### 5.2 Lane

일정 기간 독립적으로 진행되는 관심사 또는 업무 전선이다. 단순 label이나 Kanban swimlane이 아니며 자체 목표, 현재 요약, task DAG와 생명주기를 가진다.

예:

- client architecture
- sync engine
- art pipeline
- product specification
- pilot release

Lane 생명주기:

- `active`: 현재 진행 중
- `close-out`: 목표를 달성하고 결과와 맥락을 정리해 종료
- `discard`: 더 진행하지 않기로 결정하고 이유와 학습을 남김

Close-out과 discard는 삭제가 아니다. 둘 다 그래프, 결정과 Git 증거를 보존하고 기본 활성 화면에서만 제외한다.

Lane fork는 향후 후보이며 V1에는 포함하지 않는다. 도입한다면 task 복사가 아니라 원본 lane과 기준 Event를 참조하는 진행 맥락의 분기로 정의한다.

Gate가 없는 lane도 정상적인 lane이다.

Lane을 묶어 다룰 필요가 생기면 Module이 아니라 목적이 명시적인 `Lane Group` 개념을 검토한다. 초기 도메인 모델에는 별도 grouping 계층을 두지 않는다.

### 5.3 Phase

Phase는 특정 lane에 속하지 않고 전체 lane을 관통하는 공통 진행 구간이다. 각 lane은 서로 독립적으로 진행되지만 현재 어느 Phase에 있는지는 공유한다.

Gate는 특정 Phase에서 다음 Phase로 전이하기 위한 조건 집합과 승인 규칙이다. Gate는 어느 한 Phase의 종료 지점에 속하지 않고 `fromPhase`와 `toPhase`의 관계를 정의한다. 조건을 충족하고 승인되어 통과하면 현재 Phase가 끝나고 다음 Phase가 시작된다. Gate 조건에 직접 참여하지 않는 lane이나 task도 공통 Phase 안에 존재할 수 있으며, 이전 Phase에 남은 기존 Task는 전이 후에도 계속 실행할 수 있다. completed Phase에는 새 Task를 추가하지 않는다.

### 5.4 Task

Lane의 진행을 구성하는 실행 노드다.

최소 속성:

- 이름과 설명
- workspace 범위에서 유일한 숫자 public ID
- 소속 workspace, lane과 Phase
- 상태, blocker, 현재 요약과 다음 행동
- 부모·자식 관계
- 선행·후행 dependency
- 의도적인 독립 경로 종료 사유
- Run과 Task Record index
- 관련 repository와 commit
- 생성·변경·구현완료·완료확인 이력

부모·자식과 dependency는 분리한다.

- 부모·자식: 작업의 구조적 분해
- dependency: 실행 순서와 차단 관계

V1 상태:

- `pending` — 대기
- `in_progress` — 진행중
- `implemented` — 구현 주체가 구현완료를 선언
- `confirmed` — 사람이 완료확인
- `discarded` — 사람이 폐기 결정

`blocked`는 상태가 아니라 blocker metadata다. 구현완료의 의미 품질은 Baley가 판정하지 않으며 구현 주체의 assessment, 완료보고와 잔여 우려를 기록한다.

한 Task는 여러 선행·후행 Task를 가질 수 있다. Workspace의 dependency graph는 Lane과 Phase 경계를 넘을 수 있고, 하나로 연결될 필요 없이 여러 disconnected DAG component를 허용한다. 뒤 Phase에서 앞 Phase로 향하는 관계도 허용하되 진행 순서가 뒤집힐 수 있음을 경고한다. Task 경로는 후행 Task, outgoing Gate 조건 또는 사유가 있는 intentional leaf로 끝난다. 셋 중 어느 것도 없는 완료 단계의 Task는 `dangling_path` warning을 가진다.

### 5.5 Gate

Gate는 `fromPhase`에서 `toPhase`로 전이하기 위한 조건 집합과 승인 규칙이며, workspace 범위에서 lane들의 진행을 동기화한다. Gate는 특정 Phase에 소속되지 않고 두 Phase 사이의 방향성 있는 전이 관계를 가진다. 하나의 lane만 조건에 참여할 수도 있고 여러 lane이 참여할 수도 있다.

V1에서 Gate 조건은 별도의 조건식 언어를 사용하지 않는다. `fromPhase`의 Task를 Gate에 조건으로 연결하며, 연결된 모든 Task 조건이 해소되어야 Gate가 준비 상태가 된다.

각 Gate 연결은 다음 둘 중 하나로 충족된다.

- 연결된 Task가 `confirmed`됨: 사람이 완료확인
- 해당 Gate에 한해 Task가 명시적으로 `pass`됨: 사람이 사유를 남기고 예외적으로 통과시킴

`pass`는 Task 자체의 상태를 바꾸지 않고 해당 Gate와 Task의 연결만 해소한다. V1의 선형 Phase에서는 Task가 자신의 Phase에서 나가는 단일 Gate에만 연결될 수 있다. Task pass에는 승인자, 실행자, 사유와 시각을 Event로 기록한다.

미래 Phase의 Gate 조건은 Operator가 구성한다. `fromPhase`가 active가 되면 조건은 동결되며 기존 조건 detach는 금지한다. 조건을 면제하려면 관계를 삭제하지 않고 사람이 `pass_task`를 승인해야 한다. active Gate에 새 조건을 추가하는 것도 사람 승인을 요구한다.

따라서 V1의 Gate 준비 조건은 다음과 같다.

```text
gate.ready =
  Gate에 연결된 조건 Task가 하나 이상 존재하고
  모든 연결 Task가 (task.confirmed OR gate에서 task.passed)
```

같은 lane 또는 같은 Phase에 있다는 이유만으로 Task가 Gate 조건에 자동 참여하지는 않는다. Gate에 조건으로 명시적으로 연결된 Task만 평가한다.

`reference`는 Gate 조건이 아닌 일반적인 표시·문서 관계로 취급하며 V1 Gate 도메인 관계에서 제외한다. `unlocks`도 별도 Task 관계로 두지 않는다. Gate 통과가 `toPhase`를 활성화하고, 해당 Phase 안에서 각 Task의 dependency가 실제 실행 가능 여부를 결정한다.

Gate 상태 후보:

- `open`: 연결된 Task 중 하나 이상 미충족
- `ready`: 모든 Task가 confirmed 또는 passed, 전이 승인 대기
- `passed`: 승인되어 `toPhase`로 전이

Gate가 `ready`이고 권한자가 전이를 승인하면 하나의 원자적 처리로 Gate를 `passed` 상태로 만들고, `fromPhase`를 완료하며 `toPhase`를 활성화한다. 이 통과가 곧 마일스톤 달성의 의미를 가지며 Milestone을 별도 도메인 객체로 두지 않는다. Gate 통과와 Phase 전환 이력은 일반 Event로 기록한다.

수동 checklist, lane close-out, 외부 검증과 복합 논리식은 V1 Gate 조건에 포함하지 않는다. 실제 필요가 확인된 뒤 확장한다.

### 5.6 Repository, Task Record와 Git

Workspace 또는 lane을 단일 repository와 동일시하지 않는다. 한 lane과 한 task가 server, client, infrastructure, art 등 여러 repository를 참조할 수 있다.

코드, 프로젝트 문서와 Task Record 원문은 repository가 소유한다. Baley DB는 Task Record의 상대 경로, hash, commit과 짧은 요약만 index한다.

```text
docs/          → 지속되는 프로젝트 지식
task-records/  → 상세계획, Handoff, 독립 Agent 리뷰, 리뷰 반영, 완료보고
```

Task Record는 Git이 추적하되 일반 repository 검색에서는 제외한다. Branch와 worktree는 변화가 큰 진행 힌트이므로 정식 entity로 관리하지 않는다. Commit과 Record blob은 Task 결과의 지속되는 Git 증거다.

Task Record의 내용을 일반 프로젝트 문서로 정리하거나 승격하는 일은 repository를 작업하는 LLM의 책임이다. Baley는 이를 별도 기능, 상태나 Event로 관리하지 않는다.

### 5.7 Run과 Event

`Run`은 Operator가 Task를 위해 수행한 상세계획, 구현, 독립 Agent 리뷰, 리뷰 반영 또는 완료보고의 한 시도다. Skill과 MCP가 client Run ID, lease와 heartbeat로 Run 상태를 자동 갱신하며 사람의 직접 갱신은 예외다.

Baley는 독립 Agent 리뷰의 자격과 구현 품질을 인증하지 않고 Run provenance, 판단과 우려를 기록한다.

`Event`는 그래프와 상태가 어떻게 변했는지를 남기는 감사 기록이다. Gate pass, dependency 변경, Run과 Task 상태 전이를 재구성할 수 있어야 한다. Gate reopen과 Phase rollback은 V1에서 지원하지 않는다.

## 6. 그래프 규칙

- Task dependency는 방향성 비순환 그래프(DAG)다.
- DAG는 하나의 connected graph를 뜻하지 않으며 Workspace 안의 여러 component를 허용한다.
- Task는 복수 predecessor와 successor를 가질 수 있다.
- dependency는 Lane과 Phase 경계를 넘을 수 있으며 cross-Workspace 연결만 금지한다.
- 뒤 Phase에서 앞 Phase로 향하는 dependency는 허용하되 `phase_order_inversion` warning을 표시한다.
- dependency 방향 변경은 remove/add를 포함하는 atomic patch로 처리하고 서버가 최종 graph의 cycle 여부를 검증한다.
- 여러 branch는 merge Task 또는 outgoing Gate에서 본류로 합류할 수 있다.
- 후행 Task·Gate 연결·intentional leaf 사유가 모두 없으면 `dangling_path` warning이다.
- dependency는 상세계획·리뷰·보고가 아니라 후행 Task의 implementation/review-response Run 시작을 차단한다.
- Gate는 task dependency와 구별되는 타입이 있는 node다.
- Gate 조건에 연결되지 않은 task는 정상적인 독립 경로이며, 공통 Phase에는 속한다.
- Gate 통과는 `fromPhase`를 완료하고 `toPhase`를 활성화한다.
- `toPhase` Task의 실행 가능 여부는 개별 Gate link가 아니라 Phase 활성 상태와 Task dependency로 결정한다.
- 미래 Phase Task는 미리 상세계획할 수 있지만 그 밖의 Run은 Phase 활성화 전에는 시작할 수 없다.
- Phase는 전체 lane을 가로지르는 공통 구간으로 표현한다.
- Parent/child edge와 blocking dependency edge를 혼용하지 않는다.
- 화면 좌표는 정본 데이터가 아니다.

## 7. 주요 사용자 경험

### 7.1 Multi-lane View

여러 lane의 task 경로와 공유 Gate를 한 화면에서 읽는다. Gate 없는 lane, Gate와 무관한 task 경로도 함께 자연스럽게 보여야 한다.

주요 정보:

- lane 목표와 현재 상태
- active, blocked, stale 여부
- 공유 Gate 참여와 준비 상태
- 최근 활동과 다음 실행 가능 task
- lane close-out과 discard

### 7.2 Lane Focus View

전체 가로 흐름과 Phase 맥락을 유지한다. 선택한 lane의 Task는 lane header와 같은 강도로 표시하고, 다른 lane의 Task와 관계는 제거하거나 축약하지 않고 기본 화면보다 흐리게 표시한다.

### 7.3 Gate Focus View

여러 lane에서 Gate에 연결된 Task와 각 연결의 `confirmed/passed/unresolved` 상태를 보여준다. 함께 `fromPhase`, `toPhase`와 전이 준비 상태를 보여준다.

### 7.4 Task Inspector

선택한 task의 설명, 상태, dependency, Gate 관계, Run, Task Record index, commit과 Event를 표시한다.

### 7.5 명령과 그래프 갱신 원칙

사용자는 웹 UI에서 노드나 관계를 직접 편집하지 않는다. LLM에 자연어로 요청하고 Baley Skill이 command를 준비한다.

- Task는 숫자 public ID로 참조한다.
- 일반 write command는 preview를 지원한다. Task 완료확인·폐기, Lane close-out/discard, active Gate 조건 추가, Gate Task pass/revoke, Gate 통과와 Workspace 종료는 사람 승인 진술을 요구한다.
- Baley command tool이 cycle, Gate 방향과 권한을 검증한다.
- 적용 후 그래프는 자동 layout을 다시 계산한다.
- 웹 UI는 변경된 상태와 Event를 표시한다.

## 8. Operator 역할

Operator가 수행해야 할 핵심 동작:

- 대화와 문서에서 lane, Gate, task 생성 제안
- 큰 task를 자식 task DAG로 분해
- dependency atomic patch와 cycle 없는 변경
- dangling path를 후행 Task, Gate 합류 또는 intentional leaf로 정리
- 다음 실행 가능 task 조회
- 상세계획, Handoff, 독립 Agent 리뷰, 리뷰 반영과 완료보고 수행
- Run 상태의 자동 갱신
- Task Record를 repository에 작성하고 경로·hash 등록
- commit과 task 자동 연결
- blocker와 장기 정체 감지
- lane별 현재 상황 및 복귀 brief 작성
- Record hash와 commit index의 불일치 탐지
- 구현완료 선언과 잔여 위험·후속 의견 기록

Operator는 승인·권한 경계를 우회하지 않는다. LLM/Agent가 기본 Operator이며 Skill은 workflow와 의도 해석을 담당하고, API/CLI/MCP tool은 실제 검증과 변경을 담당한다.

## 9. 권한과 협업

V1은 단일 사용자로 시작하며 제품 인증과 다중 사용자 권한은 후속 단계로 미룬다.

- 사람·Agent Actor와 Operator 역할을 구별한다.
- 구현 주체는 Task의 구현완료를 선언할 수 있다.
- 사람만 Task 완료확인·폐기, Lane 종료, active Gate 조건 추가, Gate Task pass/revoke와 Gate 통과를 승인할 수 있다.
- Workspace 종료는 human Owner만 승인한다.
- Gate 통과에는 명시적인 승인 권한이 필요하다.
- 사람의 의도·승인 주체와 실제 MCP/API를 호출한 AI 실행 주체를 구별한다.
- 모든 권한 행위는 Event로 남긴다.
- 동시 변경 시 graph revision으로 충돌을 감지한다.

향후 권한은 단순 등급이 아니라 API capability와 Workspace membership으로 나눈다.

| Role | 책임 |
| --- | --- |
| Viewer | 읽기 전용 조회 |
| Operator | 일반 Task·관계 mutation, Run과 Record 운용 |
| Approver | 사람 전용 Task·Lane·Gate 판단 승인 |
| Owner | membership·설정과 Workspace 종료 |

인증 도입 후 Agent token에는 Operator scope만 부여하고 approval scope는 사람 인증 session에만 부여한다. V1의 HumanApprovalAttestation은 이 경계를 표현하는 protocol audit이며 인증된 사람 신원 증명은 아니다.

별도 결재 객체 없이 Task `implemented`는 완료확인 대기, Gate `ready`는 통과 승인 대기로 표시한다. `task.get`, `gate.status`, `workspace.get`과 `decision.list`가 action, revision과 snapshot을 반환한다. 장기 비동기 결재함이 실제로 필요해질 때만 ApprovalRequest를 도입한다.

외부 서버는 제품 인증이 추가되기 전까지 배포 계층에서 접근을 제한한다.

## 10. 기술 기준선

제품 런타임에 Python과 SQLite를 사용하지 않는다.

```text
Local LLM / MCP ─────┐
Web UI ──────────────┼── External Go API Server ── PostgreSQL
Git repositories ────┘
```

- 서버: Go 모듈형 모놀리스
- 저장소: Docker에서 운영하는 PostgreSQL
- API: 웹 UI, CLI와 skill이 공유하는 HTTP API
- DB 접근: SQL 중심, `pgx` 계열 우선 검토
- 배포: 외부 서버의 Docker Compose, 배포 계층 접근 보호
- CLI: Go
- AI 통합: API를 사용하는 Codex skill 및 기타 agent adapter
- 프론트엔드: React, TypeScript와 Vite
- 그래프 UI: React Flow와 ELK.js

서버가 DAG 무결성, Task/Gate 상태와 Event 기록을 강제한다. 로컬 LLM이 repository 파일과 Git을 조작하고 서버에는 상대 경로·hash·commit만 등록한다. 웹 클라이언트는 read-only Viewer다.

Graph DB와 microservice는 초기 범위에서 제외한다. PostgreSQL 관계 테이블과 recursive query로 시작한다.

## 11. Visual MVP 대표 시나리오

첫 프로토타입은 데이터 저장 없이 다음 고정 fixture만 렌더링한다.

```text
Server lane
  API 설계 → API 구현 ─────────┐

Client lane                    │
  화면 설계 → Pilot UI ────────┼→ Pilot Ready Gate → 사용자 테스트
       └→ 접근성 개선           │

Art lane                       │
  Asset 제작 ──────────────────┘

Research lane
  조사 → 실험 → 결과 정리 → close-out
  ※ Gate와 무관
```

이 시나리오는 다음을 한 번에 검증한다.

- Gate 없는 lane
- 여러 lane이 공유하는 단일 Gate
- Gate 조건 Task와 다음 Phase의 Task
- 같은 lane 안에서 Gate와 무관한 경로
- 병렬 dependency
- 완료, 진행, 대기와 blocker metadata 표시
- lane close-out

## 12. 초기 비범위

- 일정 추정과 자동 납기 계산
- 개인별 workload 최적화
- Kanban 보드 완성도
- billing과 대규모 조직 권한
- Graph DB
- microservice
- 모바일 앱
- 범용 workflow automation engine
- AI 모델 자체 호스팅
- Visual MVP 단계의 서버, DB, 로그인, 저장과 실제 Git 연동

## 13. 참고 제품과 차이

- **[Lanes](https://lanes.sh/)**: AI session, issue별 worktree와 병렬 실행에 강함. 장기 업무 전선인 lane과 cross-lane Gate가 중심은 아니다.
- **[Dagny](https://dagny.co/)**: interactive task DAG와 GitHub/MCP 연동이 가까움. lane 계층과 공유 Gate가 없다.
- **[ticks](https://ticks.sh/)**: Git-native task DAG를 병렬 wave로 실행한다. 중앙 서버, multi-repo workspace와 lane 중심 GUI가 없다.

Baley는 이들의 기능을 단순 결합하는 제품이 아니다. 차별점은 **장기 lane 기억, cross-lane Gate, multi-repository 실행 증거**다.

## 14. 미결정 사항

- Lane Group이 실제로 필요한지와 도입 조건
- Cross-workspace dependency 허용 여부
- 세부 Gate 승인 UI와 승인 만료 시간
- Lane fork 시 공유되는 event와 Git 기준점
- 완전한 offline Run outbox와 장기 단절 후 재전송 정책
- 알림 채널 및 stale lane 판정 기준
- 공개 제품명, 상표와 도메인 확인
