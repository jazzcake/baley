---
type: roadmap
status: active
authority: sequence
last_active: 2026-07-17
when_to_read: "Baley의 단계별 검증 순서, 현재 범위와 다음 Gate를 확인할 때"
affects:
  - docs/baley-product.md
  - docs/baley-visual-mvp-architecture.md
  - docs/baley-command-architecture.md
---

# Baley 제품 로드맵

## 1. 로드맵 목표

Baley의 초기 개발은 기능 수를 늘리는 방식이 아니라 가장 위험한 가정부터 검증한다.

검증 순서:

```text
시각 언어
→ Operator 명령 의미
→ 서버 도메인 규칙
→ Git·AI 실행 연결
→ 소규모 협업
→ Day Tripper 파일럿
```

현재 위치는 **Gate V2 close-out 완료 및 Phase 3 진입**이다. Persistent Core, Gate 전이, Run/Record, 나머지 graph command, backup·restore, 독립 fixture, Viewer 인수와 명시적 사람 승인 증거가 완료됐다. Task #110은 Workspace revision 11에서 `confirmed`이며, 다음 작업은 실제 Repository/Record bootstrap과 multi-repository Operator integration이다.

환경 독립 선행 구현은 Wave 7(P4-05~06)까지 단위 검증·독립 재리뷰를 완료했다. Phase 2~4의 선행 모듈은 모두 준비됐으며, PostgreSQL·MCP·실제 Git repository·API/브라우저 통합 판정은 데스크탑 검증 큐로 유지한다.

Phase 2~4의 환경 독립 모듈 선행 구현과 데스크탑 통합 검증 순서는 [`baley-phase2-4-prebuild-plan.md`](baley-phase2-4-prebuild-plan.md)를 따른다.

데스크탑 복귀 절차와 실행 프롬프트는 [`baley-phase2-4-desktop-handoff.md`](baley-phase2-4-desktop-handoff.md)에서 시작한다.

## 2. 단계 운영 원칙

- Gate는 특정 Phase에 속하지 않고 `fromPhase`에서 `toPhase`로의 진입을 통제하는 조건 집합과 승인 규칙이다.
- Phase는 전체 작업 lane을 관통하는 공통 진행 구간이며, 해당 전이 Gate를 통과한 뒤 다음 Phase로 전환한다.
- Gate 통과 전 다음 단계의 상세 기술 설계를 고정하지 않는다.
- Day Tripper 전용 요구를 일반 제품 모델에 직접 넣지 않는다.
- Visual MVP 결과가 기존 가정을 뒤집으면 문서와 로드맵을 수정한다.
- Go, PostgreSQL과 Docker Compose는 서버 구현 단계의 기준선이다.
- Python과 SQLite는 제품 런타임에서 제외한다.

## 3. Phase 0 — Visual MVP

### 목적

Lane, task DAG와 cross-lane Gate가 설명 없이도 읽히는지 검증한다.

### 범위

프론트엔드 코드 안에 고정된 하나의 fixture를 렌더링한다. 데이터 모델, API schema와 서버를 확정하지 않는다.

화면:

1. **Multi-lane View** — 여러 lane, 공유 Gate와 독립 경로
2. **Lane Focus View** — 전체 흐름을 유지하면서 한 lane을 강조하고 나머지를 dimming
3. **Gate Focus View** — 여러 lane의 조건 Task, 조건 해소 상태와 다음 Phase
4. **Task Inspector** — 선택 task의 최소 상세 정보

허용하는 상호작용:

- lane, task와 Gate 선택
- 전체/Lane/Gate view 전환
- 선택 node의 upstream/downstream 강조
- Gate 조건 펼치기
- 확대, 축소와 화면 이동

제외:

- task, lane, Gate 생성·수정·삭제
- dependency 연결과 drag-and-drop
- Gate 통과 처리
- 저장, 로그인, 권한
- Go 서버, PostgreSQL과 API
- 실제 Git·AI 연동

### 대표 fixture

- Server, Client, Art lane이 `Pilot Ready` Gate를 공유
- 다음 Phase의 QA task가 아직 비활성
- Client lane에 Gate와 무관한 접근성 경로 존재
- Research lane은 Gate 없이 close-out
- Visual fixture의 Task 상태, Lane `close-out/discard`, `Phase.order`, Gate `reopened`와 3종 Gate edge는 legacy이며 서버 전환 시 V1 상태·`Phase.position`·단일 Gate–Task link로 migration

### Gate V0 — 시각 언어 승인

- [x] 설명 없이 lane별 진행 방향을 따라갈 수 있다.
- [x] 여러 lane이 하나의 Gate에 참여한다는 점을 알아볼 수 있다.
- [x] Gate 조건 Task와 다음 Phase의 Task가 구별된다.
- [x] Gate와 무관한 lane 및 task 경로가 어색하지 않다.
- [x] task dependency와 Gate 관계가 혼동되지 않는다.
- [x] 전체 view에서 Lane/Gate focus로 자연스럽게 이동한다.
- [x] 수동 좌표 없이 자동 배치가 충분히 안정적이다.
- [x] 다음 단계의 command-only 운용 방향을 구체적으로 판단할 수 있다.

### 산출물

- 실행 가능한 프론트엔드 프로토타입
- 대표 fixture
- 화면별 캡처 또는 짧은 사용 기록
- 발견한 시각 문법 및 수정사항

## 4. Phase 1 — Operator Command Prototype

### 목적

사용자가 UI를 직접 편집하지 않고 Operator command로 Baley를 운용할 수 있는지 검증한다. LLM/Agent가 기본 Operator다.

### 범위

repo-scoped Baley Skill과 command contract를 만든다. UI는 read-only로 유지한다.

- 숫자 Task public ID와 자연어 참조
- 자연어 요청을 typed command로 변환
- Task/dependency/Gate command schema
- 복수 predecessor/successor와 disconnected DAG component
- atomic dependency patch와 cycle rollback
- Gate·후행 Task·intentional leaf가 없는 `dangling_path` warning
- Task의 pending/in_progress/implemented/confirmed/discarded 상태
- 자동 Run 상태 갱신과 repository Task Record 규약
- 외부 서버와 로컬 LLM의 책임 경계
- 변경 전 preview
- preview와 execute의 공통 command envelope
- cycle 및 잘못된 관계 검증
- Lane과 Phase를 넘는 Workspace DAG와 역방향 Phase warning
- 승인 필요 command 구분
- 파생된 완료확인·Gate 통과 승인 대기 query
- 향후 capability 기반 API 권한 경계
- `contracts/v1`의 command·상태·diagnostic·capability literal 정본
- 향후 Go API, CLI와 MCP가 공유할 command 의미 확정

### Gate V1 — Operator 명령 승인

- [x] `#104`, `task104`, `task 104`가 동일한 Task로 해석된다.
- [x] 모호한 Task 참조를 추측하지 않고 확인한다.
- [x] 선행·후속·병렬과 parent/child command가 구별된다.
- [x] 복수 predecessor/successor와 disconnected DAG component가 허용된다.
- [x] dependency 방향 전환이 atomic patch로 처리되고 cycle이면 전체 rollback된다.
- [x] dependency가 Lane과 Phase 경계를 넘을 수 있고 cycle만 절대 금지된다.
- [x] 뒤 Phase에서 앞 Phase로 향하는 dependency는 허용하되 warning을 반환한다.
- [x] cross-Phase dependency는 명시적 Gate–Task 연결 없이는 Gate readiness에 영향을 주지 않는다.
- [x] dangling path가 후행 Task, Gate 합류 또는 intentional leaf로 정리된다.
- [x] Gate에 연결된 모든 Task의 `confirmed` 또는 Gate 한정 `pass`가 구별된다.
- [x] `fromPhase` 밖의 Task를 Gate 조건으로 연결하는 것과 잘못된 Gate 전이 방향이 preview 단계에서 거부된다.
- [x] 모든 조건이 해소되기 전에는 Gate 전이가 거부된다.
- [x] Operator가 Task 완료확인·폐기, Lane 종료, active Gate 조건 추가, Gate Task pass/revoke, Gate 통과와 Workspace 종료의 사람 승인 진술을 우회하지 않는다.
- [x] Task implemented와 Gate ready가 사람 승인 대기로 조회된다.
- [x] Run 갱신과 Record 등록은 반복 승인 없이 자동 수행된다.
- [x] 계획·리뷰·보고 누락은 의미 품질 차단이 아니라 warning으로 처리된다.
- [x] Skill과 향후 API/MCP tool의 책임 경계가 명확하다.

## 5. Phase 2 — Persistent Core

### 목적

검증된 그래프 의미를 중앙 서버와 영속 저장소로 옮긴다.

### 기준 아키텍처

- Go 모듈형 모놀리스
- PostgreSQL
- Docker Compose
- 공용 HTTP API
- 웹 클라이언트
- event history와 graph revision

### 핵심 모듈

- workspace와 bootstrap Actor
- repository registry
- lane lifecycle
- task와 parent/child
- dependency DAG
- atomic dependency patch와 dangling-path projection
- Gate와 pass event
- Run과 Task Record index
- CommitReference와 선택적 Git observation
- event log

### 범위

- 외부 서버, 단일 사용자와 배포 계층 접근 보호
- fixture를 실제 API 데이터로 대체
- DAG cycle 방지
- Gate readiness 계산과 통과
- Task 상태 머신과 자동 Run 갱신
- Task Record 상대 경로·hash·commit index
- optimistic concurrency 또는 graph revision 충돌 처리
- migration 및 기본 backup 절차

### 현재 진행 상태

- [x] Workspace DAG, cross-Phase warning과 cycle 거부 domain core
- [x] Task 상태 머신, blocker와 atomic dependency patch domain core
- [x] Gate readiness, Task confirm, Gate Task pass/revoke와 Phase 전이
- [x] Gate 전이의 PostgreSQL·HTTP·MCP·Viewer vertical slice
- [x] Run schema, `run.start`, pending Task 자동 시작과 시작 Event의 PostgreSQL·HTTP·MCP 연결
- [x] Run heartbeat·terminal version CAS, lease timeout interruption과 종료 Event
- [x] Task Record 상대 경로·hash·commit index
- [x] Run/Record query·command와 Viewer read projection
- [x] 나머지 Task·dependency·Lane·Gate command의 API/DB 연결
- [x] backup·restore와 독립 fixture를 포함한 Gate V2 인수 검증

### Gate V2 — 도메인 코어 승인

- [x] Phase 1의 주요 그래프 동작이 API와 DB에서 동일하게 보장된다.
- [x] 서버가 cycle, 권한과 Gate 상태를 신뢰 가능한 방식으로 강제한다.
- [x] Gate pass history와 승인 근거를 event에서 재구성할 수 있다.
- [x] 재시작 후 데이터와 graph revision이 일관된다.
- [x] Day Tripper와 무관한 fixture로도 모델이 성립한다.

## 6. Phase 3 — Multi-repository Git 및 Operator Integration

### 목적

Task 기록과 실제 개발 실행을 연결하고 LLM/Agent Operator가 Baley를 직접 운용하게 한다.

### 범위

- workspace에 여러 Git repository 등록
- Task별 복수 repository와 commit 연결
- Branch/worktree는 선택적 Run 관찰 metadata로만 기록
- repository의 `task-records/` template과 검색 제외 규칙
- Go CLI
- Codex skill
- 실행 가능한 다음 task 조회
- Run 시작·성공·실패·중단 자동 기록
- 상세계획, Handoff, 독립 Agent 리뷰, 리뷰 반영과 완료보고 index
- lane brief 생성
- repository 상태와 Baley 기록의 불일치 탐지

CLI 개념 예시:

```text
baley lane list
baley lane brief <lane>
baley task list --status pending --lane <lane>
baley run start <task> --kind implementation
baley task attach-git <task>
baley task report-implemented <task>
baley gate status <gate>
```

### Gate V3 — 실행 연결 승인

- [ ] 하나의 task가 여러 repository의 Git 변경을 참조할 수 있다.
- [ ] Operator가 UI 없이 CLI/API로 task 생명주기를 진행할 수 있다.
- [ ] commit은 지속되는 Task 증거로, Branch/worktree는 최근 관찰 metadata로 구별된다.
- [ ] Task Record 원문은 repository에 남고 Baley에는 상대 경로·hash·commit만 저장된다.
- [ ] 며칠 뒤 lane brief만으로 중단된 맥락을 복원할 수 있다.
- [ ] Agent Operator가 Task 완료확인·폐기, Lane 종료, active Gate 조건 추가, Gate Task pass/revoke, Gate 통과와 Workspace 종료의 사람 승인 진술을 우회하지 않는다.

## 7. Phase 4 — 2~3인 협업

### 목적

사람과 각자의 AI agent가 같은 workspace를 안전하게 공유한다.

### 범위

- 사용자 인증과 workspace membership
- Viewer/Operator/Approver/Owner capability bundle
- Agent token과 human approval scope 분리
- Task 완료확인·폐기 승인 권한
- Lane close-out/discard 승인 권한
- active Gate 조건 추가 승인 권한
- Gate Task pass/revoke와 Gate 통과 승인 권한
- 동시 graph 변경 충돌 처리
- audit event
- stale lane, blocker와 Gate 준비 알림
- 사람 승인 권한 승계 정책

### Gate V4 — 협업 승인

- [ ] 서로 다른 사용자가 동시에 다른 lane을 수정할 수 있다.
- [ ] 같은 graph revision 충돌이 데이터 손실 없이 처리된다.
- [ ] Task 완료확인·폐기, Lane 종료, active Gate 조건 추가, Gate Task pass/revoke, Gate 통과와 Workspace 종료 승인자 권한이 서버에서 강제된다.
- [ ] Agent token으로 approval capability를 행사할 수 없다.
- [ ] 사람이 AI의 모든 중요 변경을 event history에서 확인할 수 있다.

## 8. Phase 5 — Day Tripper Pilot

### 목적

실제 장기·병렬 프로젝트에서 Baley가 기억 복원 비용을 줄이는지 검증한다.

### 파일럿 범위

- Day Tripper의 대표 lane 일부만 등록
- server/client/art 또는 spec 등 복수 repository 시나리오 적용
- 실제 공유 Gate 1개 이상 운용
- AI가 daily 또는 복귀 brief 생성
- 기존 문서·Git 운영과 중복 비용 측정

### 관찰 지표

- 중단 후 현재 상황을 복원하는 데 걸리는 시간
- AI가 다음 실행 가능 task를 정확히 찾는 비율
- Git 변경과 task 기록의 불일치 빈도
- stale lane 발견 빈도
- 사람이 수동으로 상태를 갱신해야 하는 횟수
- Gate가 실제 조율 비용을 줄였는지 여부

### Gate V5 — 제품 지속 여부 결정

- [ ] 사용자가 며칠 뒤 돌아와 lane brief로 업무를 재개할 수 있다.
- [ ] 기존 task manager보다 lane 상태 복원이 명확히 낫다.
- [ ] AI 자동 기록이 수동 관리 부담보다 큰 가치를 만든다.
- [ ] Cross-lane Gate가 실제 프로젝트 조율에 반복적으로 사용된다.
- [ ] 독립 제품으로 일반화할 핵심 요구가 Day Tripper 특수 요구와 구별된다.

## 9. 이후 후보

파일럿 근거가 있을 때만 검토한다.

- 외부 알림 및 Slack/메일 연동
- GitHub/GitLab 양방향 동기화
- lane 및 Gate template
- 검색과 장기 memory
- repository webhook
- 배포·release Gate
- 외부 도구 import
- 조직 단위 권한
- hosted deployment

## 10. 주요 위험과 대응

| 위험 | 초기 대응 |
|---|---|
| 큰 DAG가 읽기 어려움 | 전체 맥락을 유지하는 focus/dimming과 Gate view를 우선 검증 |
| Lane이 단순 label로 퇴화 | 목표, brief, lifecycle과 자체 DAG를 제품 규칙으로 유지 |
| Gate가 단순 milestone label로 퇴화 | 연결 Task의 confirmed/pass와 Phase 전이 규칙을 명시적으로 유지 |
| AI 기록이 부정확함 | Git 증거, event history와 사람 승인으로 교차 검증 |
| 데이터 모델을 너무 일찍 고정 | Phase 0은 fixture, Phase 1은 command contract로만 검증 |
| Day Tripper에 과적합 | 독립 fixture 및 다른 가상 프로젝트로 각 Gate에서 검증 |
| 기존 제품과 차별성이 약함 | 장기 lane, cross-lane Gate와 multi-repo 기억에 범위 집중 |
| 기능이 workflow engine으로 팽창 | 범용 실행 엔진, 일정 최적화와 대규모 조직 기능은 초기 제외 |

## 11. 현재 다음 행동

1. Task #111의 current-source runtime 계약과 독립 리뷰를 완료하고 사람 확인만 남긴다.
2. 권한이 다른 legacy 8080 listener는 소유 launch context에서 정리한다. 그 전까지 새 MCP process는 user-level `BALEY_SERVER_URL=http://127.0.0.1:18080`으로 current-source runtime을 사용한다.
3. 기존 Gate-transition Record index를 등록하고 이번 Task의 Git commit/blob evidence를 연결한다.
4. multi-repository CommitReference와 Record 증거를 위한 CLI/Operator 경로를 완성한다. Branch/worktree lifecycle은 외부 Git 도구에 두고 Baley에는 commit reference와 비권위적 observation만 기록한다.

## 12. Gate-transition close-out

- [x] 종료 의미: Task #110 `confirmed`, Pilot Ready Gate `passed`, Build `completed`, Validate `active`.
- [x] 사람 승인과 warning acknowledgement가 분리된 revision-11 Event에 보존됐다.
- [x] 대체 독립 리뷰와 리뷰 응답이 완료됐고 남은 High/Medium finding이 없다.
- [x] `task-records/gate-transition-vertical-slice/completion-report-01.md`를 작성했다.
- [x] 실제 Repository와 Task Record root를 Baley command로 등록했다.
- [ ] 권한이 다른 legacy 8080 process 정리는 owning launch context에서 대기 중이다. 새 MCP process의 current-source runtime은 18080으로 정렬했다.
