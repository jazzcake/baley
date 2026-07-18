---
baley_record: 1
record_id: "360462cb-ee1b-4ada-8781-ad4220a19cb2"
task_id: null
task_key: "gate-transition-vertical-slice"
record_type: detailed-plan
run_id: null
created_at: "2026-07-18T00:53:37.4674351+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Gate Transition Vertical Slice 상세계획

## 목표

Baley의 첫 실제 운용 가능 vertical slice를 만든다. 사용자는 LLM에 자연어로 사람 승인 action을 요청하고, Agent는 Baley Skill과 MCP를 통해 preview와 execute를 수행하며, 사용자는 read-only Viewer에서 Task 확인, Gate 준비와 Phase 전환 결과를 확인할 수 있어야 한다.

```text
implemented Task
→ task.confirm preview
→ 사람 승인
→ task.confirm execute
→ Gate ready
→ gate.pass preview
→ 사람 승인
→ gate.pass execute
→ fromPhase completed + toPhase active
→ Viewer 갱신
```

이번 배치는 새 도메인 규칙 하나만 추가하는 내부 라이브러리 작업이 아니다. PostgreSQL, command service, HTTP, MCP adapter와 Viewer를 한 흐름으로 연결해 사람이 직접 검증할 수 있는 첫 제품 checkpoint를 만드는 것이 목적이다.

## 정본과 선행 결과

구현 중 판단 우선순위:

1. `docs/baley-system-spec-v1.md`
2. `contracts/v1/commands.json`
3. `contracts/v1/states.json`
4. `contracts/v1/diagnostics.json`
5. `contracts/v1/capabilities.json`
6. `docs/baley-command-architecture.md`
7. 이 상세계획

선행 구현:

- commit `201f541` — Baley V1 정본, 계약과 Domain Core Batch 2
- `task-records/domain-core-batch-2/completion-report-01.md`
- `server/internal/domain/`의 Task 상태 머신, WorkspaceGraph, atomic dependency patch와 Run 시작 정책

## 기술 기준

- HTTP: Go 표준 `net/http`
- PostgreSQL: [`pgx/v5`](https://github.com/jackc/pgx)의 `pgxpool`
- migration: [`pressly/goose`](https://pressly.github.io/goose/)의 순차 SQL migration
- MCP: [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)의 공식 Go SDK
- frontend: 기존 React/Vite/React Flow
- local orchestration: Docker Compose로 PostgreSQL만 실행하고 Go server와 Viewer는 개발 process로 실행

새 framework나 ORM은 추가하지 않는다. SQL과 transaction 경계를 명시적으로 유지하고 vertical slice에서 필요한 query만 hand-written repository로 구현한다.

## 사람 수동 테스트 결과물

완료 시 사용자는 다음을 할 수 있어야 한다.

1. 브라우저에서 API가 제공하는 Pilot Workspace를 연다.
2. Build Phase, Validate Phase와 Pilot Ready Gate를 확인한다.
3. Task가 `implemented`이고 “완료확인 대기”임을 본다.
4. LLM에 `task #101 완료확인`을 요청한다.
5. Agent가 preview를 보여주고 명시적 승인을 기다린다.
6. 사용자가 승인하면 Agent가 execute를 호출한다.
7. Viewer에서 Task가 `confirmed`로 바뀌고 Gate 조건 진행률이 갱신된다.
8. 모든 조건 충족 후 Gate가 `ready`지만 Phase는 아직 전환되지 않음을 본다.
9. `Pilot Ready Gate 통과`를 요청하고 preview 후 승인한다.
10. Build가 `completed`, Validate가 `active`, Gate가 `passed`로 바뀐 것을 본다.
11. 오래된 revision, 다른 command hash 또는 다른 Gate snapshot으로 execute하면 거부되는 것을 확인한다.

Viewer는 승인 버튼이나 상태 dropdown을 제공하지 않는다. 모든 mutation은 LLM → Skill → MCP → HTTP command service 경로를 사용한다.

## 전체 구조

```text
Codex + Baley Skill
        │ stdio MCP
        ▼
cmd/baley-mcp
        │ HTTP
        ▼
cmd/baley-server
├─ query handler
├─ command preview/execute handler
├─ application command service
├─ domain
└─ PostgreSQL repository
        │
        ▼
PostgreSQL

React Viewer ──GET──> cmd/baley-server
```

MCP adapter는 domain rule을 갖지 않는다. HTTP payload와 MCP input/output mapping만 담당한다. Viewer도 server projection을 읽을 뿐 mutation을 수행하지 않는다.

## 목표 서버 구조

```text
server/
├─ cmd/
│  ├─ baley-server/
│  │  └─ main.go
│  ├─ baley-mcp/
│  │  └─ main.go
│  └─ baley-dev-seed/
│     └─ main.go
├─ internal/
│  ├─ application/
│  │  ├─ command_service.go
│  │  ├─ command_types.go
│  │  ├─ command_hash.go
│  │  ├─ approval.go
│  │  └─ decisions.go
│  ├─ domain/
│  │  ├─ phase.go
│  │  ├─ gate.go
│  │  └─ 기존 domain core
│  ├─ persistence/postgres/
│  │  ├─ repository.go
│  │  ├─ query.go
│  │  └─ transaction.go
│  ├─ transport/httpapi/
│  │  ├─ router.go
│  │  ├─ commands.go
│  │  ├─ queries.go
│  │  └─ errors.go
│  └─ transport/mcpadapter/
│     ├─ server.go
│     └─ tools.go
├─ migrations/
│  └─ 00001_gate_transition_slice.sql
└─ integration/
   └─ gate_transition_test.go
```

패키지는 실제 책임 분리가 필요할 때만 나눈다. interface를 미리 대량 생성하거나 범용 framework를 만들지 않는다.

## Vertical slice 도메인 범위

### Phase

```go
type Phase struct {
    ID          string
    WorkspaceID string
    Position    int
    State       PhaseState // planned | active | completed
}
```

규칙:

- Workspace에는 active Phase가 최대 하나다.
- Gate pass 전에는 `toPhase`가 `planned`다.
- Gate pass transaction에서 `fromPhase`를 `completed`, `toPhase`를 `active`로 바꾼다.
- V1에는 rollback과 reopen이 없다.

### Gate

Gate status는 별도 mutable column보다 조건과 `passed_at`에서 projection한다.

```text
passed_at != null                     → passed
all explicit conditions satisfied     → ready
otherwise                             → open
```

규칙:

- 조건 Task가 0개면 ready가 아니다.
- Gate 조건 Task는 Gate의 `fromPhase` 소속이어야 한다.
- Task `confirmed` 또는 해당 Gate 연결의 explicit pass만 조건을 충족한다.
- `implemented`나 `discarded`는 자동 충족되지 않는다.
- dependency edge는 Gate 조건 집합을 바꾸지 않는다.
- `gate.pass`는 현재 active Phase의 outgoing Gate에만 허용한다.

### Gate Task pass

- `gate.pass_task`는 Task status를 변경하지 않는다.
- pass와 revoke는 정확한 Gate–Task 연결 ID, revision과 command hash에 결속한다.
- 사유가 필수다.
- passed Gate에서는 pass/revoke를 허용하지 않는다.

## 이번 slice의 command

Query:

```text
workspace.get
workspace.graph
task.get
gate.status
decision.list
event.list
```

Mutation:

```text
task.confirm
gate.pass_task
gate.revoke_task_pass
gate.pass
```

`task.report_implemented`는 이번 사람 테스트 fixture에서 Task를 미리 `implemented`로 seed하므로 제외한다. Task 생성, dependency patch, active Gate attach/detach와 Lane lifecycle도 이번 HTTP/MCP slice에서는 노출하지 않는다. 기존 domain core는 유지하고 다음 vertical slice에서 command로 연결한다.

## HTTP contract

### Query

```text
GET /healthz
GET /v1/workspaces/{workspaceId}
GET /v1/workspaces/{workspaceId}/graph
GET /v1/workspaces/{workspaceId}/tasks/{publicId}
GET /v1/workspaces/{workspaceId}/gates/{gateId}/status
GET /v1/workspaces/{workspaceId}/decisions
GET /v1/workspaces/{workspaceId}/events
```

### Mutation

```text
POST /v1/commands/preview
POST /v1/commands/execute
```

공통 request:

```json
{
  "name": "task.confirm",
  "arguments": { "workspaceId": "...", "taskId": 101 },
  "envelope": {
    "idempotencyKey": "...",
    "expectedWorkspaceRevision": 7,
    "initiatedByActorId": "...",
    "executedByActorId": "...",
    "acknowledgedWarningCodes": [],
    "humanApprovalAttestation": null
  }
}
```

Preview response:

```json
{
  "commandHash": "sha256:...",
  "expectedWorkspaceRevision": 7,
  "requiredCapability": "task:approve",
  "projectedDiff": {},
  "errors": [{ "code": "human_approval_required" }],
  "warnings": [],
  "advisories": [],
  "decisionSnapshotHash": "sha256:..."
}
```

Execute 성공 response는 새 Workspace revision, 발생 Event ID와 변경된 entity projection을 반환한다.

HTTP status는 transport 의미만 나타내고 domain code는 body에 유지한다.

- 200: preview 또는 idempotent success
- 400: malformed transport payload
- 404: Workspace/Task/Gate 없음
- 409: stale revision, idempotency conflict 또는 domain transition conflict
- 422: well-formed command의 domain validation error
- 500: 예상하지 못한 server failure

## Canonical command hash

승인 진술에서 가장 중요한 부분이므로 raw map을 임의 순회해 hash하지 않는다.

Hash 입력:

```text
contractVersion
commandName
workspaceId
typed arguments
expectedWorkspaceRevision
decisionSnapshotHash nullable
```

제외:

- idempotency key
- initiated/executed actor
- warning acknowledgement
- humanApprovalAttestation 자체

각 command를 typed Go struct로 decode하고 field 순서가 고정된 canonical DTO를 `encoding/json`으로 직렬화한 뒤 SHA-256을 계산한다. 알려지지 않은 argument field는 거부해 같은 의미의 command가 서로 다른 hash가 되는 것을 막는다.

Gate decision snapshot은 다음을 안정적으로 정렬해 hash한다.

```text
gateId
criteriaRevision
fromPhaseId
toPhaseId
sorted gateTask link IDs
각 link의 taskId, taskStatus, passedAt/passReason 존재 여부
workspaceRevision
```

## HumanApprovalAttestation

Preview는 사람 전용 command에 대해 `human_approval_required`, command hash와 decision snapshot hash를 반환한다. 상태를 저장하거나 ApprovalRequest row를 만들지 않는다.

사람이 preview를 확인하고 승인하면 execute request에 다음을 포함한다.

```text
approvedByActorId
approvedCommandHash
decisionSnapshotHash
statementHash optional
conversationRef optional
approvedAt optional
```

Execute 검증:

- approved actor가 bootstrap human Actor임
- action과 entity가 command에서 파생된 값과 일치
- command hash 일치
- Workspace revision 일치
- Gate action은 decision snapshot hash 일치
- 동일 attestation이 다른 executed command에 사용되지 않음

승인 진술과 성공한 executed command를 같은 transaction에서 1:1로 기록한다. V1에서는 인증된 신원 증명이 아니라 protocol audit라는 metadata를 응답에 표시한다.

## Command transaction

Execute의 순서:

1. request를 typed command로 decode
2. canonical command hash 계산
3. PostgreSQL transaction 시작
4. Workspace row `FOR UPDATE`
5. expected Workspace revision 확인
6. 같은 idempotency key의 기존 command 확인
7. 현재 Task, Phase, Gate와 Gate Task snapshot load
8. domain validation과 projected diff 재계산
9. warning acknowledgement 확인
10. HumanApprovalAttestation 검증
11. Task/Gate/Phase mutation 적용
12. Workspace revision 1 증가
13. command, attestation과 Event insert
14. commit

어느 단계든 실패하면 mutation, revision, attestation과 Event가 모두 rollback돼야 한다.

Preview는 write하지 않는다. read-only transaction에서 snapshot과 현재 revision을 읽고 같은 domain evaluation을 사용한다. Preview는 예약이 아니며 execute가 모든 조건을 다시 검사한다.

## Event

이번 slice에서 기록할 Event:

```text
task.confirmed
gate.task_passed
gate.task_pass_revoked
gate.passed
phase.completed
phase.activated
human_approval_attestation.recorded
```

Gate pass 한 번은 같은 command ID와 Workspace revision 아래 다음 Event를 남긴다.

```text
gate.passed
phase.completed
phase.activated
human_approval_attestation.recorded
```

`gate.passed` payload에는 통과 당시 각 조건의 confirmed/pass 근거 snapshot을 포함한다.

## PostgreSQL schema

이번 migration에 포함할 table:

```text
actors
workspaces
phases
lanes
tasks
task_dependencies
gates
gate_tasks
human_approval_attestations
commands
events
workspace_counters
```

필수 제약:

- 모든 workspace entity에 `workspace_id`
- `(workspace_id, id)` composite key 또는 동등한 FK
- `UNIQUE(workspace_id, task.public_id)`
- `UNIQUE(workspace_id, phase.position)`
- Workspace별 active Phase 최대 하나인 partial unique index
- Gate endpoint와 Gate Task의 same Workspace 보장
- Gate `from_phase.position < to_phase.position`
- Gate Task pass reason과 passed actor/time의 일관성 check
- HumanApprovalAttestation과 executed command 1:1 unique
- command idempotency scope/key unique
- positive Workspace revision과 public Task ID

cycle, 상태 전이와 Gate readiness처럼 cross-row 의미를 요구하는 규칙은 application transaction에서 검사한다.

Migration은 순차 번호 SQL로 작성하고 down migration도 제공한다. Application 시작 시 production에서 자동 migration하지 않는다. 개발 command와 명시적 migration command로 실행한다.

## Development seed

Production API에 seed endpoint를 두지 않는다. `cmd/baley-dev-seed`가 idempotent하게 다음 demo를 만든다.

```text
Workspace: Baley Pilot
Phase 1: Build      active
Phase 2: Validate   planned
Gate: Pilot Ready   Build → Validate

Gate conditions:
- #101 API 구현       implemented
- #104 Pilot UI       implemented
- #106 Asset 제작     implemented

Next Phase:
- #110 사용자 테스트  pending
```

Server 재시작이나 seed 재실행 시 중복 Workspace를 만들지 않는다. 고정 client seed ID를 사용한다.

## MCP adapter

첫 slice는 로컬 stdio MCP server로 구현한다. Adapter는 `BALEY_SERVER_URL`을 통해 HTTP API를 호출한다.

Query tools:

```text
baley_workspace_graph
baley_task_get
baley_gate_status
baley_decision_list
baley_event_list
```

Mutation tools:

```text
baley_task_confirm_preview
baley_task_confirm_execute
baley_gate_pass_task_preview
baley_gate_pass_task_execute
baley_gate_revoke_task_pass_preview
baley_gate_revoke_task_pass_execute
baley_gate_pass_preview
baley_gate_pass_execute
```

각 tool은 HTTP command name을 하드코딩하되 domain rule을 갖지 않는다. Preview와 execute를 별도 tool로 구분해 Agent가 사람 승인을 우회해 한 번에 실행하지 못하게 한다.

MCP tool output은 사람이 읽을 짧은 요약과 machine-readable structured result를 함께 반환한다.

## Viewer migration

기존 fixture import를 제거하고 `VITE_BALEY_API_URL`, `VITE_BALEY_WORKSPACE_ID`로 API를 조회한다.

필요한 상태:

- loading
- server unavailable
- empty Workspace
- loaded graph
- stale polling retry

`workspace.graph` projection에는 다음을 포함한다.

```text
workspace ID, name, revision, activePhaseId
phases and state
lanes and state
tasks with public ID, status and blocker
dependencies
gates with derived status
gateTask conditions and satisfaction reason
decisions
```

Viewer 표시:

- pending/in_progress/implemented/confirmed/discarded V1 상태
- implemented Task의 “완료확인 대기”
- Gate open/ready/passed
- ready Gate의 “통과 승인 대기”
- completed/active/planned Phase 구분
- 마지막 successful refresh revision

Viewer는 2초 polling과 window focus refresh로 mutation 결과를 반영한다. WebSocket/SSE는 비범위다.

## Local development 실행

목표 command:

```powershell
docker compose up -d postgres

Set-Location server
go run ./cmd/baley-server migrate up
go run ./cmd/baley-dev-seed
go run ./cmd/baley-server serve

Set-Location ..
npm run dev
```

MCP adapter는 별도 process로 실행한다.

```powershell
Set-Location server
$env:BALEY_SERVER_URL='http://127.0.0.1:8080'
go run ./cmd/baley-mcp
```

실제 CLI shape는 구현 중 Go flag parsing에 맞게 단순화할 수 있지만 migrate, seed와 serve가 명시적으로 분리돼야 한다.

## 구현 배치

### A. Domain 확장

- typed Phase와 Gate aggregate
- Gate readiness projection
- Task confirm
- Gate Task pass/revoke
- Gate pass와 Phase transition plan
- domain unit test

### B. PostgreSQL 기반

- Docker Compose PostgreSQL
- goose migration
- pgxpool connection
- transaction repository
- dev seed
- 실제 PostgreSQL integration test

### C. Command service

- typed command registry
- canonical command hash
- preview evaluation
- execute transaction
- revision/idempotency
- HumanApprovalAttestation
- Event creation

### D. HTTP

- query endpoint
- preview/execute endpoint
- structured domain error mapping
- health check와 CORS local configuration

### E. MCP

- official Go SDK stdio server
- query tools
- mutation preview/execute tools
- HTTP client error preservation

### F. Viewer

- API types와 fetch client
- server graph projection
- V1 status visual migration
- loading/error/refresh state
- decisions 표시

### G. End-to-end 검증

- migrate/seed/server/viewer 실행
- Task confirm preview/execute
- Gate ready 확인
- Gate pass preview/execute
- Phase 전환 확인
- stale approval와 idempotency test

각 배치는 앞 단계의 테스트가 통과해야 다음 단계로 간다. 중간에 범용 auth, notification이나 repository integration으로 범위를 확장하지 않는다.

## 자동 테스트

### Domain

- Gate 조건 0개는 open
- confirmed/pass 혼합으로 ready
- implemented/discarded는 미충족
- pass/revoke 사유와 상태 검증
- passed Gate mutation 거부
- 현재 active Phase가 아닌 Gate pass 거부
- Gate pass 결과의 Phase 상태 변화

### Command

- preview가 write하지 않음
- 승인 없는 execute 거부와 rollback
- command hash mismatch 거부
- decision snapshot mismatch 거부
- stale revision 거부
- 같은 idempotency key 같은 payload는 기존 결과
- 같은 key 다른 payload는 conflict
- Gate pass mutation/revision/Event 원자성
- attestation과 command 1:1

### PostgreSQL

- migration up/down/up
- FK와 unique constraint
- Workspace lock 경합에서 한 mutation만 성공
- server 재시작 후 상태 유지
- seed idempotency

### HTTP/MCP

- malformed payload와 domain error 분리
- MCP tool input schema
- MCP가 HTTP code와 structured diagnostic을 보존
- preview tool이 execute하지 않음

### Viewer

- API DTO mapping
- implemented/confirmed 상태 표시
- Gate ready와 decision 표시
- Phase completed/active/planned 표시
- loading/error state
- Lane Focus가 전체 graph와 dimming을 유지

### 회귀

- `go test ./...`
- `go vet ./...`
- PostgreSQL integration test
- `npm test -- --reporter=dot`
- `npm run build`
- Skill validation
- contracts JSON parsing

## 사람 인수 테스트

### 정상 경로

1. demo seed 후 Viewer를 연다.
2. #101, #104, #106이 implemented인지 확인한다.
3. 각 Task 완료확인을 요청하고 preview 내용을 확인한다.
4. 각 preview 후 명시적으로 승인한다.
5. Gate가 open에서 ready로 바뀌는지 확인한다.
6. ready 상태에서 Validate가 여전히 planned인지 확인한다.
7. Gate pass preview의 조건 snapshot을 확인하고 승인한다.
8. Gate passed, Build completed, Validate active를 확인한다.
9. Event timeline에서 Task confirm과 Gate/Phase Event를 확인한다.

### 실패 경로

- 승인 전 execute 거부
- preview 후 다른 Task 변경, 기존 approval stale 거부
- 다른 Gate snapshot hash 재사용 거부
- 같은 attestation 재사용 거부
- 조건 미충족 Gate pass 거부
- 같은 idempotency key와 다른 command 거부
- 서버 중단 시 Viewer가 기존 fixture로 위장하지 않고 오류 표시

## 비범위

- 회원가입, 로그인과 membership enforcement
- 원격 hosted MCP transport와 OAuth
- Task 생성/update/dependency patch의 HTTP/MCP 노출
- active Gate 조건 attach/detach UI
- Run lease와 heartbeat
- Task Record 등록과 Git metadata API
- Lane close-out/discard
- Workspace close
- notification, SSE와 WebSocket
- production deployment와 backup
- private repository 조회

## 위험과 대응

### Vertical slice 과대화

도메인, DB, API, MCP, Viewer가 모두 포함되므로 각 계층에서 이번 command만 구현한다. 범용 command framework를 먼저 완성하지 않는다.

### Approval protocol 오해

V1 attestation은 인증된 사람 신원 증명이 아니다. 응답과 Viewer에 protocol audit임을 명시하고 배포 계층 보호 전에는 외부 공개하지 않는다.

### Domain invariant 우회

PostgreSQL repository는 `WorkspaceGraph`와 command service를 통하지 않고 상태를 직접 변경하는 public method를 제공하지 않는다. exported map 기술부채는 command service 연결 전에 accessor로 캡슐화하는 것을 우선 검토한다.

### Race test 환경

CGO compiler가 없는 Windows에서는 `go test -race`가 실행되지 않을 수 있다. PostgreSQL lock integration test로 single-writer를 검증하고 race test 미실행은 완료보고에 남긴다.

### Viewer legacy drift

legacy fixture type과 V1 API type을 혼합하지 않는다. API migration 후 fixture는 별도 historical test data로만 유지하거나 제거 시점을 명시한다.

## 완료 조건

- PostgreSQL을 재시작해도 demo 상태가 유지된다.
- Task confirm은 사람 승인 진술 없이는 실행되지 않는다.
- 모든 Gate 조건 충족만으로 Phase가 자동 전환되지 않는다.
- Gate pass 승인 후 Gate와 두 Phase가 하나의 transaction으로 바뀐다.
- stale revision, command hash와 snapshot mismatch가 mutation 없이 거부된다.
- Event에서 승인 주체, 실행 주체와 Phase 전환 근거를 재구성할 수 있다.
- MCP가 HTTP command service를 우회하지 않는다.
- Viewer가 fixture가 아니라 server projection을 읽는다.
- 사용자가 LLM 명령과 Viewer만으로 정상/실패 인수 시나리오를 수행할 수 있다.
- 독립 Agent 리뷰의 차단 발견이 모두 반영되거나 명시적 잔여 위험으로 보고된다.

## Record와 리뷰

구현 시작 전 이 계획과 handoff를 정확한 경로로 읽는다. 구현 뒤 다음 Record를 새 UUID로 작성한다.

```text
task-records/gate-transition-vertical-slice/independent-agent-review-01.md
task-records/gate-transition-vertical-slice/review-response-01.md
task-records/gate-transition-vertical-slice/completion-report-01.md
```

사람 인수 테스트 결과는 완료보고에 별도 절로 기록한다. 자동 테스트 통과를 사람 UX 검증으로 대체하지 않는다.

## 다음 단계

이 slice가 통과하면 Domain Core Batch 2의 dependency/task command를 같은 command service와 persistence 경로로 확장하고, Run/Record/Git vertical slice로 이동한다.
