---
type: plan
status: active
authority: sequence
last_active: 2026-07-18
when_to_read: "Phase 2부터 Phase 4까지 환경 독립 모듈을 선행 구현하고 데스크탑에서 통합 검증할 작업 순서를 정할 때"
affects:
  - docs/baley-roadmap.md
  - docs/baley-system-spec-v1.md
  - contracts/v1
  - server/internal
  - server/cmd
---

# Phase 2–4 독립 모듈 선행 구현 계획

## 1. 목적

Go·Docker·브라우저 통합 환경이 없는 작업 환경에서도 Phase 2부터 Phase 4까지 환경 의존성이 낮은 모듈과 테스트를 먼저 만든다. 각 단위는 데스크탑으로 옮겼을 때 독립적으로 컴파일·테스트하고, 검증이 끝난 순서대로 PostgreSQL·HTTP·MCP·Viewer에 연결할 수 있어야 한다.

이 계획은 로드맵 Phase를 건너뛰지 않는다. 뒤 Phase의 안정적인 순수 정책을 선행 구현하되, 앞 Phase Gate를 통과한 것으로 표시하거나 미결정된 인증·배포 설계를 확정하지 않는다.

## 2. 작업 원칙

각 단위는 다음 결과를 가진다.

1. System Spec과 `contracts/v1`에 근거한 입력·출력과 불변식
2. 외부 서비스에 의존하지 않는 작은 Go package 또는 TypeScript module
3. table-driven unit test와 실패 사례
4. PostgreSQL·HTTP·MCP·Git·브라우저가 없어도 판정 가능한 결과
5. 데스크탑에서 실행할 정확한 검증 명령과 통합 체크리스트

선행 구현에서 금지하는 것:

- PostgreSQL 동작을 in-memory fake 통과로 검증됐다고 간주
- MCP나 HTTP adapter에 domain rule 복제
- 실제 인증 방식이 정해지기 전에 token format, session, OAuth 구현
- Git branch/worktree를 Baley lifecycle entity로 승격
- Phase 4 작업을 이유로 V1 HumanApprovalAttestation 경계를 변경
- 실행하지 못한 테스트를 통과로 기록

## 3. 작업 흐름

```text
계약·테스트 벡터
→ 순수 domain/policy 모듈
→ application port와 fake 기반 orchestration test
→ 데스크탑 컴파일·unit test
→ PostgreSQL migration/integration
→ HTTP/MCP contract test
→ Viewer와 사람 인수 테스트
```

한 단위가 데스크탑 검증에서 실패하면 같은 단위를 수정한 뒤 다음 adapter 연결로 넘어간다. 여러 미검증 단위를 하나의 대형 migration이나 command service 변경으로 합치지 않는다.

## 4. Phase 2 선행 구현 단위

### P2-01 — Run 상태와 terminal 전이

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). PostgreSQL Run version CAS 통합 검증은 대기한다.

구현:

- Run kind/status typed model
- `running → succeeded | failed | interrupted | cancelled`
- terminal 재호출의 idempotent/conflict 판정
- result/error summary 필수 조건
- manual correction의 이전 값·새 값·사유 검증

단위 테스트:

- 허용·거부 상태 전이 전체 표
- 같은 terminal 결과 재시도
- 다른 terminal 결과 경쟁
- blank summary/reason 거부

데스크탑 확인:

- `go test`와 `go test -race`
- 이후 PostgreSQL Run version CAS 경쟁 테스트

### P2-02 — Run identity, lease와 heartbeat 정책

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). heartbeat/terminal 동시 DB 경쟁 검증은 대기한다.

구현:

- canonical start identity: Workspace, Task, kind, parent, target
- 같은 `client_run_id` payload 비교
- lease token hash와 노출 token 분리
- heartbeat version·만료 시각 계산
- stale Run interruption 판정

단위 테스트:

- 동일 ID/동일 payload idempotent
- 동일 ID/다른 Task·kind·parent·target conflict
- 잘못된 token과 stale version 거부
- lease 연장과 만료 경계
- heartbeat와 terminal 전이의 순수 CAS 결과

데스크탑 확인:

- 실제 clock을 쓰지 않는 deterministic test
- PostgreSQL에서 heartbeat/terminal 동시 경쟁

### P2-03 — Run 시작 계획

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). `run.start`와 Task 자동 시작의 단일 transaction 검증은 대기한다.

구현:

- 기존 `run_policy`와 Task/Phase/dependency/blocker를 결합한 start plan
- pending Task의 자동 `in_progress` 전이 계획
- `task.started`와 `run.started` Event 계획
- 미래 Phase의 `detailed_planning` 예외

단위 테스트:

- pending Task 자동 시작
- blocker와 미해소 predecessor 처리
- completed/current/future Phase별 모든 Run kind
- 독립 Agent 리뷰의 선택적 `target_run_id`

데스크탑 확인:

- `run.start`와 Task 시작이 같은 DB transaction인지 확인

### P2-04 — Task Record 경로 검증

상태: **단위 검증·독립 리뷰 완료** (2026-07-18). PostgreSQL 경로 index 통합 검증은 대기한다.

구현:

- `/` 기반 상대 경로 정규화
- configured `task_records_root` 내부 여부
- `..`, 절대 경로, Windows drive prefix, URI, NUL 거부
- root와 path의 platform-independent 비교

단위 테스트:

- Unix/Windows 입력 벡터
- encoded-looking 문자열과 separator 혼합
- root 자체와 root 밖 sibling
- Unicode 파일명

데스크탑 확인:

- OS별 path library 동작에 영향받지 않는지 확인

### P2-05 — Task Record identity와 상태

상태: **단위 검증·독립 리뷰 완료** (2026-07-18). composite FK와 transaction 통합 검증은 대기한다.

구현:

- client `record_id` 보존
- canonical 등록 payload와 idempotency 비교
- `reported_uncommitted → committed_unverified → verified` 정책
- 동일 ID/다른 path·hash conflict
- supersedes 관계 검증

단위 테스트:

- 같은 Record 재등록
- hash/path/repository/Task 변경 conflict
- commit/blob attach 전이
- 잘못된 supersedes와 self-reference

데스크탑 확인:

- composite FK와 unique constraint
- 등록과 Event의 transaction 원자성

### P2-06 — Repository, CommitReference와 GitObservation 모델

상태: **단위 검증·독립 리뷰 완료** (2026-07-18). 실제 Git repository와 persistence 통합 검증은 대기한다.

구현:

- Repository 등록값 검증
- commit SHA/blob SHA 형식과 relation 검증
- RunGitObservation 정규화
- branch/worktree hint가 Task 상태에 영향을 주지 않는 projection
- 절대 worktree 경로 저장 거부

단위 테스트:

- 한 Task의 multi-repository commit
- branch 없이 commit 연결
- reported/remote-verified 전이
- 허용되지 않는 경로·relation·SHA

데스크탑 확인:

- repository/Task/Run의 cross-Workspace FK

### P2-07 — 구현완료 보고와 결정 projection

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). Record/Task/Event transaction 통합 검증은 대기한다.

구현:

- `task.report_implemented` assessment 검증
- blocker hard error
- detailed plan/review/completion Record 누락 warning
- implemented Task의 `task.confirm` 결정 projection
- dangling path warning 결합

단위 테스트:

- assessment 누락
- Record 조합별 warning
- blocker와 상태 전이
- decision revision binding

데스크탑 확인:

- Record 등록, Task 상태, Event와 revision transaction

### P2-08 — 나머지 graph mutation planner

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). command service·PostgreSQL callback 연결은 대기한다.

구현 순서:

1. Task create/update/block/unblock/rework/discard
2. dependency connect/disconnect/patch
3. Lane create/update/close-out/discard
4. Gate create/attach/detach
5. Workspace/Phase lifecycle

각 planner는 projected diff, Event, warning과 required capability를 반환하고 persistence를 직접 호출하지 않는다.

단위 테스트:

- `contracts/v1` mutation catalog와 planner coverage 일치
- active Gate 조건 추가의 동적 capability
- human-only action의 attestation 요구
- atomic dependency patch와 terminal reason

데스크탑 확인:

- command service와 PostgreSQL callback에 단위별 연결

### P2-09 — Event 감사 projection

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). mutation/Event rollback 원자성 검증은 대기한다.

구현:

- Event type별 최소 evidence payload validator
- initiated/executed/approved actor 구분
- Gate pass condition snapshot 재구성
- Run heartbeat만 Event 예외인지 catalog 검사

단위 테스트:

- mutation catalog와 Event coverage
- human action의 attestation Event 동반
- 주요 상태 변화를 Event에서 설명할 수 있는지

데스크탑 확인:

- mutation과 Event의 rollback 원자성

## 5. Phase 3 선행 구현 단위

Phase 3 모듈은 Phase 2의 Run/Record/Repository 타입에 의존한다. 타입과 순수 정책은 미리 만들 수 있지만 실제 Git 및 서버 연결은 Phase 2 통합 후 수행한다.

### P3-01 — `baley.yaml` 설정 모델

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 실제 repository init/read round trip은 대기한다.

구현:

- version, server, Workspace/Repository ID
- record repository와 `task_records.root`
- secret/token 필드 금지
- 상대 경로 정규화

단위 테스트:

- 최소·multi-repository 설정
- unknown version과 누락 필드
- secret 유출 가능 필드와 잘못된 root

데스크탑 확인:

- 실제 repository의 init/read round trip

### P3-02 — 실행 가능 Task 선택기

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). API query contract 대조는 대기한다.

구현:

- 상태, blocker, dependency, Phase와 active Run을 이용한 actionable 판정
- “실행 가능”, “계획만 가능”, “사람 결정 대기”, “차단” reason code
- Lane 및 Workspace 필터

단위 테스트:

- 복수 predecessor와 disconnected component
- discarded predecessor
- future Phase 상세계획
- implemented/ready decision 대기

데스크탑 확인:

- API query 결과와 동일한 Task 집합인지 contract test

### P3-03 — Lane brief projection

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 실제 복귀 시나리오의 사람 유용성 검증은 대기한다.

구현:

- Lane 목표·현재 요약·열린 Task·blocker·다음 행동
- 최근 Run/Record/commit evidence
- Gate 참여와 사람 결정 대기
- stale 여부와 데이터 출처 표시

단위 테스트:

- 빈 Lane, 독립 DAG component, Gate 없는 Lane
- 여러 repository와 미검증 commit
- 오래된 Run/summary

데스크탑 확인:

- 며칠 뒤 복귀 시나리오의 사람 유용성 검증

### P3-04 — Repository/Record 불일치 탐지

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 실제 Git repository fixture 검증은 대기한다.

구현:

- registered hash와 observed hash 비교
- missing/unregistered/modified/uncommitted/commit-mismatch 진단
- 서버가 로컬 파일을 직접 읽지 않는 observation input 경계

단위 테스트:

- 동일 hash, 변경, 삭제, 새 Record
- commit/blob 정보 조합
- false verified 방지

데스크탑 확인:

- 임시 Git repository fixture를 이용한 integration test

### P3-05 — Git metadata 수집 포트

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 실제 Git command adapter 검증은 대기한다.

구현:

- Git command 실행을 숨기는 read-only port
- commit/blob/head/dirty observation DTO
- 절대 경로가 서버 payload로 넘어가지 않도록 redaction

단위 테스트:

- fake runner 출력 parsing
- detached HEAD, no branch, dirty tree
- command 실패와 부분 정보

데스크탑 확인:

- 실제 임시 Git repository에서 실행

### P3-06 — CLI command model

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 실제 서버 smoke test는 대기한다.

구현:

- lane/task/run/record/gate query와 mutation argument parser
- HTTP client port
- preview 출력과 사람 승인 전 execute 중단
- 자동 Run/Record lifecycle은 반복 승인 없이 실행

단위 테스트:

- parser golden test
- fake HTTP client request shape
- human-only command가 preview 후 멈추는지
- stale revision과 structured error 표시

데스크탑 확인:

- 실제 서버 smoke test

### P3-07 — Project init planner

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 임시 repository init E2E는 대기한다.

구현:

- `project.bootstrap` 요청 계획
- `baley.yaml`, Task Record template, 검색 제외 파일 생성 계획
- `client_project_id` 기반 재시도
- 부분 실패 복구 단계

단위 테스트:

- 생성 파일 manifest
- 기존 파일 충돌과 non-destructive merge
- 재실행 idempotency

데스크탑 확인:

- 임시 repository에서 init E2E

## 6. Phase 4 선행 구현 단위

Phase 4에서는 인증 provider나 token 형식을 구현하지 않는다. 아래는 `contracts/v1/capabilities.json`으로 이미 확정된 authorization 정책과 협업 진단만 선행한다.

### P4-01 — Capability catalog와 role resolver

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). HTTP middleware matrix는 대기한다.

구현:

- viewer/operator/approver/owner bundle
- capability lookup과 catalog validation
- Agent token forbidden capability 적용
- human/agent/system Actor kind 조건

단위 테스트:

- 모든 role/capability 조합
- Agent가 approval/admin capability를 얻지 못함
- owner의 Workspace close 조건
- contract literal drift 검출

데스크탑 확인:

- HTTP middleware 연결 후 endpoint matrix test

### P4-02 — Workspace membership authorization policy

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). persistence/middleware 연결은 대기한다.

구현:

- subject Actor, Workspace membership, role와 requested capability 입력
- allow/deny reason
- membership 없음·비활성 membership 거부
- cross-Workspace command 거부

단위 테스트:

- 2~3인 Workspace matrix
- 같은 Actor의 Workspace별 다른 role
- Agent operator와 human approver 분리

데스크탑 확인:

- 실제 인증 방식 결정 후 persistence/middleware 연결

### P4-03 — 동적 capability와 승인 권한 정책

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). Workspace lock 내부 연결은 대기한다.

구현:

- active Gate attach의 `gate:approve`
- Task/Lane/Gate/Workspace human-only action
- `workspace.close`의 human owner 전용 조건
- Agent 실행자와 human 승인자의 분리

단위 테스트:

- command × entity state × Actor kind × role matrix
- transaction 중 상태 변경 시 권한 재평가
- attestation subject와 authenticated subject 불일치

데스크탑 확인:

- Workspace lock 안에서 capability 재계산

### P4-04 — 협업 충돌 결과 모델

상태: **단위 검증·독립 재리뷰 완료** (2026-07-18). 병렬 PostgreSQL 검증은 대기한다.

구현:

- stale Workspace revision 응답
- current revision과 재조회 hint
- Run version conflict와 graph conflict 구분
- idempotency conflict의 안전한 표시

단위 테스트:

- 서로 다른 Lane의 같은 revision write
- 같은 graph mutation 경쟁
- 재시도 가능한 충돌과 재계획이 필요한 충돌

데스크탑 확인:

- 병렬 PostgreSQL integration test

### P4-05 — 협업 알림 후보 탐지

구현:

- stale Lane
- 장기 blocker
- ready Gate
- implemented Task 결정 대기
- lease 만료 Run

단위 테스트:

- clock 주입과 threshold 경계
- 중복 알림 억제용 stable fingerprint
- 상태 해소 후 알림 제거

데스크탑 확인:

- delivery channel 없이 query projection으로 먼저 검증

### P4-06 — Audit visibility projection

구현:

- 사람 요청, Agent 실행, 사람 승인 분리 표시
- important mutation 필터
- Workspace/Lane/Task별 audit timeline
- 승인 없는 Agent mutation과 승인 필요한 mutation의 표시 차이

단위 테스트:

- Actor provenance 누락 표시
- human approval evidence
- 여러 사용자의 interleaved Event ordering

데스크탑 확인:

- 실제 Event query와 Viewer projection 연결

## 7. 선행 구현하지 않을 항목

다음은 설계 결정 또는 실제 환경 검증 없이는 구현하지 않는다.

- 로그인·회원가입 UI
- OAuth/OIDC provider 선택
- access/refresh token format과 rotation
- session/cookie/CSRF 정책
- membership 초대·회수 UX
- 실제 알림 전달 채널
- remote MCP 인증
- PostgreSQL migration의 성공 판정
- backup/restore 성공 판정
- 브라우저 시각 품질 판정
- 네트워크 장애와 process restart E2E

이 항목들은 필요한 port와 정책 입력까지만 정의하고 adapter는 데스크탑 또는 배포 환경에서 구현한다.

## 8. 권장 구현 순서

```text
Wave 1  P2-01 → P2-02 → P2-03
Wave 2  P2-04 → P2-05 → P2-06
Wave 3  P2-07 → P2-08 → P2-09  [완료: 2026-07-18]
Wave 4  P3-01 → P3-02 → P3-03  [완료: 2026-07-18]
Wave 5  P3-04 → P3-05 → P3-06 → P3-07  [완료: 2026-07-18]
Wave 6  P4-01 → P4-02 → P4-03 → P4-04  [완료: 2026-07-18]
Wave 7  P4-05 → P4-06  [완료: 2026-07-18]
```

Wave 1~3은 Phase 2 통합의 직접 선행 작업이다. Wave 4~7은 앞 타입에 의존한다. 2026-07-18 기준 Wave 1~7의 환경 독립 구현·단위 검증·독립 리뷰는 모두 완료했으며, 실제 adapter와 인수 시나리오는 아래 데스크탑 검증 큐에서 순서대로 확인한다.

실행 환경 전환 시에는 [`baley-phase2-4-desktop-handoff.md`](baley-phase2-4-desktop-handoff.md)의 전용 test DB 절차와 [`baley-phase2-4-desktop-handoff-prompt.md`](baley-phase2-4-desktop-handoff-prompt.md)의 재개 프롬프트를 사용한다.

## 9. 단위별 완료 기록

각 단위 완료 시 다음 형식으로 남긴다.

```text
단위:
구현 파일:
테스트 파일:
정본 근거:
이 환경에서 실행한 검증:
실행하지 못한 검증:
데스크탑 명령:
통합 시 예상 연결점:
잔여 위험:
```

코드와 테스트를 작성했더라도 이 환경에서 Go compiler가 없으면 상태는 `작성 완료 / 실행 대기`다. 데스크탑에서 컴파일·단위 테스트가 통과하면 `단위 검증 완료`, PostgreSQL·HTTP·MCP까지 통과하면 `통합 검증 완료`로 구분한다.

## 10. 데스크탑 검증 큐

각 Wave마다 다음 순서로 확인한다.

1. `gofmt` 차이 없음
2. `go test -count=1 ./...`
3. `go vet ./...`
4. 가능한 경우 `go test -race ./...`
5. migration up/down/up
6. 실제 PostgreSQL integration test에서 skip 0건
7. HTTP contract test
8. MCP stdio E2E에서 skip 0건
9. `npm test -- --reporter=dot`, typecheck와 build
10. Viewer와 사람 승인 인수 시나리오

Phase Gate는 관련 Wave의 단위 검증만으로 통과시키지 않는다. 해당 Phase의 데이터 영속성, adapter와 사람 인수 시나리오까지 완료된 뒤 별도로 판단한다.
