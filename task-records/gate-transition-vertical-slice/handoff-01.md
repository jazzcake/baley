---
baley_record: 1
record_id: "6fbffba3-26a4-4840-88d0-d786da4f112f"
task_id: null
task_key: "gate-transition-vertical-slice"
record_type: handoff
run_id: null
created_at: "2026-07-18T00:53:37.4674351+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Gate Transition Vertical Slice 구현 Handoff

당신은 Baley의 첫 실제 운용 가능 vertical slice를 구현하는 세션이다. 목표는 Task confirm, Gate ready, 사람 Gate pass 승인과 Phase 전환을 PostgreSQL·HTTP·MCP·Viewer까지 연결해 사용자가 자연어 명령과 read-only Viewer로 검증할 수 있게 만드는 것이다.

## 반드시 먼저 읽을 파일

`task-records/` 전체를 검색하지 말고 다음 exact path만 읽는다.

1. `task-records/gate-transition-vertical-slice/detailed-plan-01.md`
2. `task-records/domain-core-batch-2/completion-report-01.md`
3. `docs/baley-system-spec-v1.md`
   - §4 배포 구조
   - §7 Workspace/Phase/Lane
   - §8 Task
   - §10 Gate와 Phase 전이
   - §14 사람 판단과 capability
   - §15 Command와 MCP
   - §16 Transaction과 Event
   - §19 PostgreSQL V1 table
   - §21 필수 인수 테스트
4. `docs/baley-command-architecture.md`
5. `contracts/v1/commands.json`
6. `contracts/v1/states.json`
7. `contracts/v1/diagnostics.json`
8. `contracts/v1/capabilities.json`
9. `server/internal/domain/` 전체
10. 현재 Viewer 관련 `src/` 파일

정본 의미와 literal이 이 handoff와 충돌하면 System Spec과 `contracts/v1`이 우선이다. 충돌을 발견하면 임의로 완화하지 말고 위치, 영향과 수정 제안을 먼저 보고한다.

## 작업 전 안전 규칙

- `git branch --show-current`, `git status --short`, 최근 commit을 확인한다.
- 사용자의 명시적 지시가 없으므로 branch나 worktree를 만들지 않는다.
- 기존 변경은 사용자 작업으로 취급하고 보존한다.
- 범위 밖 파일을 정리하거나 되돌리지 않는다.
- 커밋과 push는 사용자가 명시적으로 요청하기 전까지 하지 않는다.
- destructive Git command를 사용하지 않는다.
- 외부 dependency는 공식 source와 현재 Go 호환성을 확인하고 version을 pin한다.

## 구현 결과

완료 시 다음 수동 흐름이 실제로 동작해야 한다.

```text
Viewer에서 implemented #101 확인
→ LLM에 완료확인 요청
→ MCP preview
→ 사람 승인
→ MCP execute
→ Viewer confirmed 반영
→ 모든 조건 충족 후 Gate ready
→ Gate pass preview
→ 사람 승인
→ Gate pass execute
→ Build completed / Validate active
```

Viewer에 상태 변경 버튼을 추가하지 않는다.

## 기술 결정

- Go 표준 `net/http`
- `pgx/v5`와 `pgxpool`
- `pressly/goose` sequential SQL migration
- 공식 `modelcontextprotocol/go-sdk`
- PostgreSQL은 Docker Compose
- MCP는 우선 local stdio adapter
- Viewer는 기존 React/Vite 유지
- ORM, 범용 DI framework, message broker를 추가하지 않는다.

## 구현 범위

### Domain

- Phase state와 Gate transition
- Gate readiness
- Task confirm
- Gate Task pass/revoke
- Gate pass transition plan

### Persistence

- actors/workspaces/phases/lanes/tasks/dependencies/gates/gate_tasks
- commands/events/attestations/workspace counters
- Workspace revision lock
- dev seed

### Application

- typed command decode
- canonical command hash
- preview
- execute
- warning acknowledgement
- HumanApprovalAttestation
- idempotency
- Event generation

### Transport

- health/query/preview/execute HTTP
- structured error mapping
- local CORS
- MCP query and mutation preview/execute tools

### Viewer

- fixture 대신 API graph
- V1 Task/Gate/Phase 상태
- decision 표시
- loading/error/polling
- 기존 Lane Focus 전체 graph와 dimming 보존

## 명시적 비범위

- auth/login/membership
- remote MCP OAuth
- Run lease/heartbeat
- Record/Git API
- Task create/update/dependency command 노출
- active Gate attach/detach
- Lane/Workspace close
- notification/SSE/WebSocket
- production deployment

비범위 기능을 stub으로 가짜 동작시키지 않는다.

## 구현 순서

### 1. 현재 contract audit

- 이번 command와 response에 필요한 literal이 contracts에 존재하는지 확인한다.
- 새 literal이 필요하면 contract와 System Spec 의미를 먼저 수정한다.
- domain code에 임의 문자열을 추가하지 않는다.

### 2. Domain 확장

- Phase/Gate typed model과 test
- Gate condition evaluation
- pass/revoke 규칙
- Gate pass가 생성하는 Phase transition result
- existing Domain Core regression

### 3. PostgreSQL

- Docker Compose
- goose migration up/down
- pgxpool
- transaction repository
- dev seed
- real PostgreSQL integration test

DB mutation은 application command service 전용 transaction callback 안에서만 노출한다.

### 4. Command service

- command request typed union
- unknown field 거부
- canonical hash
- preview/execute 공통 evaluation
- Workspace `FOR UPDATE`
- revision/idempotency
- approval attestation
- atomic state/Event write

Preview는 어떤 row도 만들지 않는다. Execute는 preview 결과를 신뢰하지 않고 다시 평가한다.

### 5. HTTP

- `GET /healthz`
- workspace/graph/task/gate/decision/event query
- `/v1/commands/preview`
- `/v1/commands/execute`
- graceful shutdown
- config validation

기본 bind는 외부 공개 주소가 아니라 loopback이어야 한다.

### 6. MCP

- official Go SDK stdio server
- exact command별 preview와 execute tool 분리
- server URL config
- structured result 보존
- HTTP transport error와 domain error 구분

MCP adapter에서 상태 전이 규칙을 재구현하지 않는다.

### 7. Viewer

- API client와 DTO
- legacy fixture import 제거
- server loading/error state
- polling/focus refresh
- V1 status visual mapping
- Gate decision과 Phase state 표시

서버 오류 시 fixture 화면을 성공한 것처럼 보여주지 않는다.

### 8. End-to-end

- migrate
- seed
- server
- MCP
- Viewer
- 정상/실패 사람 인수 시나리오

## 승인 protocol

사람 전용 mutation을 처음 요청받으면 preview까지만 수행한다. Preview 결과에서 다음을 사용자에게 보여준다.

- action과 대상
- expected Workspace revision
- projected diff
- warnings
- command hash
- decision snapshot hash

사용자가 그 내용을 본 뒤 명시적으로 승인해야 execute한다. 처음 요청 문장만으로 preview와 execute를 연속 호출하지 않는다.

Attestation은 다른 action, entity, revision, command hash나 snapshot에 재사용하지 않는다.

## DB transaction 필수 조건

- Workspace row lock 이후 revision 검사
- idempotency key와 command hash 함께 검사
- domain state를 transaction 안에서 load
- approval을 mutation 직전에 검증
- state/revision/command/attestation/Event 동시 commit
- 실패 시 전체 rollback

Gate pass는 최소 다음 네 Event를 같은 command에 연결한다.

```text
gate.passed
phase.completed
phase.activated
human_approval_attestation.recorded
```

## 자동 검증

필수:

```powershell
Set-Location server
$files = Get-ChildItem -Recurse -Filter *.go | ForEach-Object { $_.FullName }
gofmt -w $files
go test -count=1 ./...
go vet ./...

Set-Location ..
npm test -- --reporter=dot
npm run build
$env:PYTHONUTF8='1'
python C:\Users\jazzc\.codex\skills\.system\skill-creator\scripts\quick_validate.py .agents/skills/baley-manage-work
```

PostgreSQL integration test는 실제 container DB에 실행한다. DB mock 통과를 persistence 검증으로 대체하지 않는다.

가능한 환경에서는 `go test -race ./...`도 실행한다. Windows C compiler 부재로 실패하면 도구 제약과 대체 검증을 완료보고에 기록한다.

## 독립 리뷰

구현과 자동 검증 후 독립 Agent에게 다음 원자료만 제공한다.

- 이 상세계획
- 관련 System Spec 절
- `contracts/v1/*.json`
- raw diff
- migration SQL
- 테스트 출력
- 수동 인수 테스트 결과

예상 결론이나 의심하는 버그를 미리 알려주지 않는다. 특히 다음을 중점 검토하게 한다.

- 승인 재사용 가능성
- canonical hash 누락 field
- revision/idempotency race
- partial transaction commit
- cross-Workspace FK 누락
- Gate 조건 snapshot drift
- Viewer가 fixture로 fallback해 실패를 숨기는지
- MCP가 execute 권한을 우회하는지

결과를 다음 경로에 새 UUID로 작성한다.

```text
task-records/gate-transition-vertical-slice/independent-agent-review-01.md
```

발견사항 반영은 `review-response-01.md`, 최종 결과는 `completion-report-01.md`에 기록한다. 초기 리뷰를 덮어쓰지 않는다.

## 사람 인수 테스트

자동 테스트와 독립 리뷰가 끝나면 사용자에게 다음을 안내하고 직접 확인받는다.

1. Viewer URL
2. 초기 Workspace revision과 Gate 상태
3. 요청할 자연어 문장
4. preview에서 확인할 항목
5. 승인 문장
6. Viewer에서 기대할 변화
7. 의도적 stale approval 실패 시나리오

사람 인수 테스트가 완료되지 않았다면 구현완료는 보고할 수 있어도 이 vertical slice close-out은 권고하지 않는다.

## 완료보고

`completion-report-01.md`에 다음을 포함한다.

- 구현 범위와 비범위
- migration과 schema
- HTTP/MCP tool 목록
- 자동 테스트 결과
- 독립 리뷰와 반영
- 사람 인수 테스트 결과
- commit 전 working tree 상태
- 미실행 검증과 이유
- 잔여 보안/동시성/운영 위험
- 다음 vertical slice

Baley가 품질을 인증했다고 표현하지 않는다.

## 세션 완료 응답

사용자에게 결과부터 보고한다.

- 서버/Viewer/MCP 실행 상태
- 사람이 테스트할 URL과 정확한 명령
- 자동 테스트 결과
- 독립 리뷰 결과
- 아직 close-out하지 않은 이유가 있다면 명확한 blocker
- 작성한 Record exact path
- commit/push 여부
