---
type: handoff
status: active
authority: derived
last_active: 2026-07-18
when_to_read: "Phase 2~4 환경 독립 선행 구현을 데스크탑에서 검증하거나 Run/Record vertical slice 통합을 재개할 때"
affects:
  - docs/baley-roadmap.md
  - docs/baley-phase2-4-prebuild-plan.md
  - server/internal/domain
  - server/internal/config
  - server/internal/gitmeta
  - server/internal/cli
  - server/internal/projectinit
  - server/internal/authz
  - server/internal/collab
---

# Phase 2~4 데스크탑 핸드오프

## 1. 목적과 현재 판정

이 문서는 환경 독립 모듈 선행 구현 Wave 1~7을 데스크탑으로 옮겨 실제 PostgreSQL, HTTP, MCP, Git repository와 Viewer에 연결하기 위한 복귀 지점이다.

2026-07-18 기준 판정:

- P2-01~P4-06의 순수 Go 모듈과 단위 테스트 작성 완료
- 각 Wave의 자체 `test`·`vet`·`race` 검증 완료
- 각 Wave의 독립 Agent 리뷰, finding 반영과 최종 재리뷰 완료
- Wave 7 최종 독립 리뷰 판정 `CLOSE`
- 전용 `baley_test`에서 migration up/down/up, PostgreSQL integration과 MCP stdio E2E skip 0건 검증 완료
- Viewer baseline과 Gate Focus API projection을 브라우저에서 검증하고 발견한 focus 전환 문제를 반영 완료
- Run persistence 첫 단위인 `run.start`를 PostgreSQL·HTTP·MCP에 연결 완료
- heartbeat/terminal CAS, Record, 실제 Git repository, CLI, project init과 Run/Record Viewer 연결은 후속 데스크탑 통합 대기

정본과 상세 증거:

1. [`baley-system-spec-v1.md`](baley-system-spec-v1.md) — 도메인 의미와 불변식
2. [`contracts/v1`](../contracts/v1) — 상태·command·diagnostic·capability literal
3. [`baley-command-architecture.md`](baley-command-architecture.md) — Operator, 승인과 adapter 경계
4. [`baley-phase2-4-prebuild-plan.md`](baley-phase2-4-prebuild-plan.md) — P2-01~P4-06 범위와 데스크탑 검증 큐
5. [`task-records/run-record-vertical-slice`](../task-records/run-record-vertical-slice) — Wave별 계획·독립 리뷰·리뷰 반영·완료보고

복사용 시작 프롬프트는 [`baley-phase2-4-desktop-handoff-prompt.md`](baley-phase2-4-desktop-handoff-prompt.md)에 분리했다.

## 2. 완료된 모듈

| Wave | 단위 | 구현 결과 | 주요 위치 |
|---|---|---|---|
| 1 | P2-01~03 | Run lifecycle, lease/heartbeat/CAS, start planner와 target Run 검증 | `server/internal/domain/run*.go` |
| 2 | P2-04~06 | Record path/identity/state, Repository, CommitReference, GitObservation | `server/internal/domain/record*.go`, `repository_git.go` |
| 3 | P2-07~09 | 구현완료 projection, 전체 mutation registry, Event/approval audit | `task_implemented_plan.go`, `mutation_*.go`, `event_audit.go` |
| 4 | P3-01~03 | strict `baley.yaml`, actionable selector, Lane brief | `server/internal/config`, `actionable_selector.go`, `lane_brief.go` |
| 5 | P3-04~07 | Record integrity, Git metadata port, CLI model, project init planner | `server/internal/domain/record_integrity.go`, `gitmeta`, `cli`, `projectinit` |
| 6 | P4-01~04 | capability/role, membership authz, 동적 승인, conflict disposition | `server/internal/authz`, `server/internal/collab/conflict.go` |
| 7 | P4-05~06 | 알림 후보 탐지와 provenance-bound audit timeline | `server/internal/collab/notifications.go`, `audit_visibility.go` |

계약 변경에는 Workspace/Run/Record/Git 상태와 `stale_run_version`, `run_lease_mismatch`, Workspace close warning 등이 포함된다. Go typed model과 literal contract의 양방향 exact-set 회귀 테스트가 있다.

## 3. 중요한 설계 경계

- Web은 read-only Viewer다. mutation은 Skill → typed MCP/HTTP command → Go service를 따른다.
- Agent는 Operator일 수 있지만 사람 승인 권한을 대신하지 않는다.
- Task confirm/discard, Lane 종료, active Gate 변경·pass와 Workspace close는 정본의 human approval 규칙을 유지한다.
- `AuthorizePlannedCommand`는 persistence가 Workspace row lock을 잡은 뒤 서버가 현재 snapshot에서 다시 만든 mutation plan에 적용해야 한다.
- Run heartbeat와 terminal 전이는 같은 Run version CAS를 사용한다.
- Record 원문은 repository에 있고 서버는 상대 경로, hash와 선택적 commit/blob metadata만 보관한다.
- 알림 projection은 Gate condition, Run과 Task를 같은 canonical snapshot/revision에서 조립해야 한다.
- audit projection은 executed command 단위로 actor provenance와 approval attestation을 동일하게 공급해야 하며 `TaskID–LaneID` scope를 보존해야 한다.
- 이번 선행 모듈을 우회하는 별도 adapter 규칙을 만들지 않는다.

## 4. 이 환경에서 통과한 검증

마지막 실행 결과:

```text
cd server
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
git diff --check
```

모두 통과했다. `server/internal/collab`의 마지막 statement coverage는 89.7%였다.

웹 baseline도 의존성을 `npm ci`로 복원한 뒤 다음 검증을 통과했다.

```text
npm test -- --reporter=dot — 3 files, 9 tests PASS
npm run typecheck — PASS
npm run build — PASS
```

Vite build에는 단일 JavaScript chunk가 500 kB를 넘는다는 비차단 warning이 있다. 현재 기능 실패는 아니며 Viewer 통합 시 route/feature 단위 code splitting 후보로 남긴다.

선행 환경에서 명시적으로 실행되지 않았던 두 integration test:

```text
TestGateTransitionAgainstPostgres — BALEY_TEST_DATABASE_URL 미설정으로 SKIP
TestMCPStdioListsAndCallsTools — BALEY_MCP_E2E 미설정으로 SKIP
```

2026-07-18 데스크탑 복귀 후 두 test를 각각 전용 `baley_test`와 격리 HTTP port에서 skip 없이 통과시켰다. Phase 2 최종 구현에서 Run lifecycle과 Record command/query를 더해 MCP catalog는 29개 tool이 됐다.

## 5. 데스크탑 최초 검증 절차

### 5.1 도구 확인

repository root에서 실행한다.

```bash
git pull --ff-only origin main
go version
node --version
npm --version
docker compose version
```

### 5.2 전용 PostgreSQL test DB

integration test는 대상 DB의 Baley table을 `TRUNCATE`한다. 개발용 `baley` DB가 아니라 폐기 가능한 `baley_test`만 사용한다.

```bash
docker compose up -d postgres
docker compose exec -T postgres pg_isready -U baley -d baley
docker compose exec -T postgres dropdb --if-exists -U baley baley_test
docker compose exec -T postgres createdb -U baley baley_test

cd server
export BALEY_DATABASE_URL='postgres://baley:baley@127.0.0.1:54329/baley_test?sslmode=disable'
export BALEY_TEST_DATABASE_URL="$BALEY_DATABASE_URL"
go run ./cmd/baley-server migrate up
go run ./cmd/baley-server migrate down
go run ./cmd/baley-server migrate up
```

`dropdb` 대상은 위의 `baley_test`로 고정한다. 다른 DB 이름으로 치환하지 않는다.

### 5.3 Go와 PostgreSQL

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
go test -v -count=1 ./integration -run TestGateTransitionAgainstPostgres
```

성공 조건은 PostgreSQL test가 skip 없이 통과하는 것이다.

### 5.4 MCP stdio E2E

첫 번째 터미널:

```bash
cd server
export BALEY_DATABASE_URL='postgres://baley:baley@127.0.0.1:54329/baley_test?sslmode=disable'
export BALEY_LEASE_TOKEN_SECRET='replace-with-a-stable-external-secret'
go run ./cmd/baley-server serve
```

두 번째 터미널:

```bash
cd server
BALEY_MCP_E2E=1 go test -v -count=1 ./integration -run TestMCPStdioListsAndCallsTools
```

성공 조건은 MCP test가 skip 없이 29개 tool을 확인하고 graph query, Run lifecycle과 Record 등록·조회 실제 호출을 완료하는 것이다.

### 5.5 Viewer baseline

repository root에서 실행한다.

```bash
npm ci
npm test -- --reporter=dot
npm run typecheck
npm run build
```

그다음 server와 `npm run dev`를 함께 실행해 Multi-lane, Lane Focus, Gate Focus, Task Inspector와 기존 사람 승인 시나리오를 확인한다.

## 6. 통합 구현 권장 순서

전체 선행 모듈을 한 번에 persistence로 옮기지 않는다. 각 단위에서 migration → repository → application transaction → HTTP contract → MCP → Viewer → integration test 순으로 닫고 다음 단위로 이동한다.

1. [x] Run schema와 Repository port
2. [x] `run.start` + pending Task 자동 시작 transaction
3. [ ] heartbeat/terminal version CAS와 lease timeout interruption
4. [ ] Task Record index와 Repository/Commit/Git observation schema
5. [ ] Record 등록·commit attach·remote verification command
6. [ ] Run/Record query와 HTTP/MCP adapter
7. [ ] `task.report_implemented`와 completion Record 연결
8. [ ] `baley.yaml`, actionable selector, Lane brief API/CLI 연결
9. [ ] capability/membership middleware와 locked-plan authorization
10. [ ] notification/audit read projection과 Viewer 연결

`run.start`의 raw lease token은 저장하지 않는다. `BALEY_LEASE_TOKEN_SECRET`과 Run ID의 HMAC으로 같은 token을 재구성하므로 중복 요청은 Run lease/version, Workspace revision과 Event를 바꾸지 않는다. 이 secret이 없으면 서버는 시작을 거부하며 모든 server process에 같은 값을 주입한다.

각 단계에서 [`baley-phase2-4-prebuild-plan.md`](baley-phase2-4-prebuild-plan.md)의 해당 P 단위 테스트와 데스크탑 확인 항목을 그대로 완료 조건으로 사용한다.

## 7. 아직 만들지 말아야 할 것

다음은 선행 계획에서 의도적으로 제외했다.

- OAuth/OIDC provider, 로그인·가입 UI
- access/refresh token과 cookie/CSRF 세부 정책
- 실제 알림 delivery channel
- remote MCP 인증
- 브라우저 시각 품질의 자동 합격 판정
- network/process restart E2E를 통과했다는 주장

필요한 port와 정책 입력까지만 유지하고, 별도 설계 결정 없이 특정 vendor나 protocol을 고정하지 않는다.

## 8. Task Record와 Baley bootstrap

현재 Wave 기록은 모두 `registration_state: pending-bootstrap`, `task_id: null`, `run_id: null`이다. live Baley command tool이 없었으므로 파일을 DB나 fixture에 대신 등록하지 않았다.

Baley Run/Record command가 실제로 연결된 뒤에는:

1. 실제 숫자 Task와 Run을 만든다.
2. 현재 Task에 해당하는 정확한 Record 파일만 읽는다.
3. 상대 경로와 hash를 등록한다.
4. 이 작업을 포함한 commit SHA와 Record blob SHA를 attach한다.
5. 등록 이후 기존 Record를 덮어쓰지 않고 새 version을 만든다.

## 9. 완료 보고 형식

데스크탑 검증 결과에는 다음을 남긴다.

```text
검증 단위:
사용한 commit:
실행 환경:
실행 명령:
통과/실패/skip:
발견된 finding과 반영:
PostgreSQL migration 결과:
HTTP/MCP contract 결과:
Viewer 인수 결과:
남은 위험과 다음 단위:
```
