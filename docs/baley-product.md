---
type: spec
status: active
last_active: 2026-07-15
when_to_read: "Baley의 제품 경계, 도메인 개념, UX 원칙을 확인하거나 변경할 때"
affects:
  - docs/baley-roadmap.md
  - docs/baley-visual-mvp-architecture.md
  - docs/baley-visual-grammar.md
---

# Baley 제품 기획서

## 1. 한 문장 정의

Baley는 여러 저장소와 AI 작업을 장기간 기억하는 **lane 중심 업무 관리 시스템**이다. 각 lane의 진행은 task dependency DAG로 표현하며, 여러 lane은 공유 Gate를 통해 동기화될 수 있다.

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

AI는 부가 기능이 아니라 주요 작업 주체다. 사람은 목표, 판단, 승인과 권한을 맡고 AI는 task 생성·분해·진행·수집·보고와 Git 작업을 수행한다.

프로젝트명은 Isaac Asimov의 로봇 소설에 등장하는 인간 형사 Elijah Baley에서 가져왔다. AI 자체보다 AI와 함께 일하며 판단하고 조율하는 인간의 관점을 나타낸다.

## 4. 핵심 원칙

1. **Lane first**: task나 담당자가 아니라 lane이 기본 탐색 및 회상 단위다.
2. **Graph is the interface**: DAG가 주 작업 화면이며 목록과 상세 패널은 이를 보조한다.
3. **Meaning over coordinates**: 사용자는 노드 위치가 아니라 생성, dependency, Gate 관계를 편집한다. 배치는 자동 계산한다.
4. **Durable context**: 결과뿐 아니라 결정, blocker, AI 실행, Git 증거와 다음 행동을 남긴다.
5. **Human authority**: AI는 완료와 종료를 제안할 수 있지만 생성자에게 제한된 close/archive 권한을 대신 행사하지 않는다.
6. **Repository independent**: Baley는 Day Tripper와 독립된 제품이다. Day Tripper는 첫 파일럿일 뿐이다.
7. **Shared semantics**: 웹 UI, Go CLI와 AI skill이 동일한 API 및 도메인 규칙을 사용한다.

## 5. 도메인 모델

```text
Workspace
├─ Members
├─ Repositories
├─ Phases
├─ Lanes
│  ├─ Tasks
│  ├─ Dependency edges
│  ├─ AI executions
│  └─ Git bindings
├─ Gates
│  ├─ Required conditions
│  ├─ References
│  └─ Unlocked tasks
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
- `fork`: 특정 시점까지의 계보를 공유하고 이후 독립 lane으로 분기

Close-out과 discard는 삭제가 아니다. 둘 다 그래프, 결정과 Git 증거를 보존하고 기본 활성 화면에서만 제외한다.

Lane fork는 task 복사가 아니라 진행 맥락의 분기다. 원본 lane, 기준 event 또는 task를 계보로 참조하고 fork 이후부터 새로운 그래프를 만든다.

Gate가 없는 lane도 정상적인 lane이다.

Lane을 묶어 다룰 필요가 생기면 Module이 아니라 목적이 명시적인 `Lane Group` 개념을 검토한다. 초기 도메인 모델에는 별도 grouping 계층을 두지 않는다.

### 5.3 Phase

Phase는 특정 lane에 속하지 않고 전체 lane을 관통하는 공통 진행 구간이다. 각 lane은 서로 독립적으로 진행되지만 현재 어느 Phase에 있는지는 공유한다.

Gate는 현재 Phase에서 다음 Phase로 넘어가는 전환점이다. Gate의 조건을 충족하고 승인되어 통과하면 현재 Phase가 끝나고 다음 Phase가 시작된다. Gate 조건에 직접 참여하지 않는 lane이나 task도 공통 Phase 안에 존재할 수 있다.

### 5.4 Task

Lane의 진행을 구성하는 실행 노드다.

최소 속성:

- 이름과 설명
- 소속 workspace와 lane
- 생성자와 현재 실행 주체
- 상태와 blocker
- 부모·자식 관계
- 선행·후행 dependency
- 관련 repository, branch, worktree, commit, PR
- AI session과 실행 결과
- 검증 결과
- 최근 요약과 다음 행동
- 생성·변경·완료 이력

부모·자식과 dependency는 분리한다.

- 부모·자식: 작업의 구조적 분해
- dependency: 실행 순서와 차단 관계

초기 상태 후보:

- `draft`
- `ready`
- `running`
- `blocked`
- `review`
- `done`
- `archived`

`finish`와 `close`도 의미를 분리한다.

- `finish`: 실행 주체가 작업 완료를 보고
- `close`: 생성자가 결과를 수용
- `archive`: 생성자가 기본 화면에서 제외

### 5.5 Gate

Gate는 전체 lane에 공통으로 적용되는 Phase 사이의 전환점이며, workspace 범위에서 lane들의 진행을 동기화한다. 하나의 lane만 조건에 참여할 수도 있고 여러 lane이 참여할 수도 있다.

Task와 Gate의 기본 관계:

- `required`: task 완료가 Gate 통과에 필요
- `reference`: 관련 task지만 Gate를 차단하지 않음
- `unlocks`: Gate가 통과된 뒤 task 실행 가능

Lane과 Gate의 조건 참여 관계는 연결된 task로부터 파생한다. 같은 lane이라는 이유만으로 모든 task가 Gate의 통과 조건에 참여하지는 않지만, Phase 전환은 전체 lane에 공통으로 적용된다.

Gate 상태 후보:

- `open`: 필수 조건 미충족
- `ready`: 조건 충족, 승인 대기
- `passed`: 승인되어 후속 task 해제
- `reopened`: 통과 후 다시 열림

Gate의 조건을 충족하고 승인되어 `passed` 상태가 되면 현재 Phase를 완료하고 다음 Phase로 전환한다. 이 통과가 곧 마일스톤 달성의 의미를 가지며 Milestone을 별도 도메인 객체로 두지 않는다. Gate 통과와 Phase 전환 이력은 일반 Event로 기록한다.

향후 Gate 조건은 task 외에도 수동 checklist, lane close-out, 외부 검증을 받을 수 있다.

### 5.6 Repository와 GitBinding

Workspace 또는 lane을 단일 repository와 동일시하지 않는다. 한 lane과 한 task가 server, client, infrastructure, art 등 여러 repository를 참조할 수 있다.

Branch, worktree와 commit을 task의 단일 컬럼으로 두지 않고 `GitBinding` 또는 실행 기록으로 분리한다.

```text
Task
├─ server repository
│  ├─ branch
│  ├─ worktree
│  └─ commits
└─ client repository
   ├─ branch
   ├─ worktree
   └─ commits
```

Git 정보는 참고 링크가 아니라 task 실행의 증거다.

### 5.7 Execution과 Event

`Execution`은 사람 또는 AI가 task를 실제로 수행한 세션이다. 실행 주체, 도구, 시작·종료 시각, 입력, 결과, 검증과 GitBinding을 기록한다.

`Event`는 그래프와 상태가 어떻게 변했는지를 남기는 감사 기록이다. lane fork, Gate pass/reopen, dependency 변경과 권한 행위를 재구성할 수 있어야 한다.

## 6. 그래프 규칙

- Task dependency는 방향성 비순환 그래프(DAG)다.
- 새 dependency는 서버에서 cycle 여부를 검증한다.
- Gate는 task dependency와 구별되는 타입이 있는 node다.
- Gate 조건에 연결되지 않은 task는 정상적인 독립 경로이며, 공통 Phase에는 속한다.
- Gate 통과는 `unlocks`로 직접 연결된 task에만 영향을 준다.
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
- lane close-out, discard, fork 계보

### 7.2 Lane Focus View

한 lane의 DAG를 중심으로 보고, 연결된 외부 Gate 조건과 다른 lane의 기여는 축약해 보여준다.

### 7.3 Gate Focus View

여러 lane에서 들어오는 required/reference 조건과 Gate 통과 후 unlock되는 task를 보여준다.

### 7.4 Task Inspector

선택한 task의 설명, 구조 관계, dependency, Gate 관계, GitBinding, AI 실행, 검증과 event history를 표시한다.

### 7.5 그래프 편집 원칙

사용자는 노드를 자유 배치하지 않는다. 의미 있는 편집만 제공한다.

- 특정 task의 선행·후속·병렬 task 생성
- 기존 task 간 dependency 연결·해제
- parent task 아래 자식 task 생성
- 기존 task를 Gate의 required/reference로 연결
- Gate 이후 unlock task 생성 또는 연결
- dependency 이유 기록

자동 layout은 dependency 방향, lane과 Gate 관계를 기준으로 결정적으로 계산한다. 관계 변경 시 기존 상대 순서를 최대한 유지하고 변경된 node를 강조한다.

## 8. AI 역할

AI가 수행해야 할 핵심 동작:

- 대화와 문서에서 lane, Gate, task 생성 제안
- 큰 task를 자식 task DAG로 분해
- dependency 연결 및 cycle 없는 변경
- 다음 실행 가능 task 조회
- branch/worktree 생성 및 task 연결
- commit, PR과 task 자동 연결
- 진행 결과와 검증 기록
- blocker와 장기 정체 감지
- lane별 현재 상황 및 복귀 brief 작성
- 실제 repository 상태와 기록의 불일치 탐지
- close-out, discard, fork 제안

AI는 승인·권한 경계를 우회하지 않는다. 사람과 AI는 웹 UI, Go CLI 또는 Codex skill을 통해 같은 명령을 사용한다.

## 9. 권한과 협업

초기 협업 대상은 2~3인 개발팀이다.

- 사용자와 AI 실행 주체를 구별한다.
- task를 생성한 사람만 close/archive할 수 있다.
- AI는 finish 보고와 close 후보 제안까지만 할 수 있다.
- Gate 통과에는 명시적인 승인 권한이 필요하다.
- 모든 권한 행위는 Event로 남긴다.
- 동시 변경 시 graph revision으로 충돌을 감지한다.

세부 역할 체계는 파일럿 전 결정한다.

## 10. 기술 기준선

제품 런타임에 Python과 SQLite를 사용하지 않는다.

```text
Web UI ─────────────┐
Go CLI / AI Skill ──┼── Go API Server ── PostgreSQL
Git repositories ───┘
```

- 서버: Go 모듈형 모놀리스
- 저장소: Docker에서 운영하는 PostgreSQL
- API: 웹 UI, CLI와 skill이 공유하는 HTTP API
- DB 접근: SQL 중심, `pgx` 계열 우선 검토
- 배포: 초기에는 Docker Compose
- CLI: Go
- AI 통합: API를 사용하는 Codex skill 및 기타 agent adapter
- 프론트엔드: React, TypeScript와 Vite
- 그래프 UI: React Flow와 ELK.js

서버가 DAG 무결성, 권한, Gate 상태와 event 기록을 강제한다. 클라이언트는 graph layout, 선택, focus와 조작 preview를 담당한다.

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
- Gate required task와 unlock task
- 같은 lane 안에서 Gate와 무관한 경로
- 병렬 dependency
- 완료, 진행, 대기, blocked 상태
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
- Gate 통과 승인 모델
- Passed Gate 선행 조건 변경 정책
- Task creator 퇴장 시 close/archive 권한 승계
- Lane fork 시 공유되는 event와 Git 기준점
- AI execution의 표준 상태와 실패·재시도 모델
- 알림 채널 및 stale lane 판정 기준
- 공개 제품명, 상표와 도메인 확인
