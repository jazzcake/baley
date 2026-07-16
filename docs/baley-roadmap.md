---
type: roadmap
status: active
last_active: 2026-07-15
when_to_read: "Baley의 단계별 검증 순서, 현재 범위와 다음 Gate를 확인할 때"
affects:
  - docs/baley-product.md
  - docs/baley-visual-mvp-architecture.md
---

# Baley 제품 로드맵

## 1. 로드맵 목표

Baley의 초기 개발은 기능 수를 늘리는 방식이 아니라 가장 위험한 가정부터 검증한다.

검증 순서:

```text
시각 언어
→ 그래프 조작 의미
→ 서버 도메인 규칙
→ Git·AI 실행 연결
→ 소규모 협업
→ Day Tripper 파일럿
```

현재 위치는 **Phase 0: Visual MVP**다.

## 2. 단계 운영 원칙

- 각 Phase는 다음 단계 진입을 통제하는 검증 Gate를 가진다.
- Phase는 전체 작업 lane을 관통하는 공통 진행 구간이며, Gate 통과 후 다음 Phase로 전환한다.
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
2. **Lane Focus View** — 한 lane의 DAG와 외부 Gate 조건 요약
3. **Gate Focus View** — 여러 lane의 required/reference 조건과 unlock task
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
- Gate 이후 QA task가 잠김
- Client lane에 Gate와 무관한 접근성 경로 존재
- Research lane은 Gate 없이 close-out
- task 상태는 done, running, blocked, ready를 포함

### Gate V0 — 시각 언어 승인

- [ ] 설명 없이 lane별 진행 방향을 따라갈 수 있다.
- [ ] 여러 lane이 하나의 Gate에 참여한다는 점을 알아볼 수 있다.
- [ ] Gate required task와 Gate 이후 task가 구별된다.
- [ ] Gate와 무관한 lane 및 task 경로가 어색하지 않다.
- [ ] task dependency와 Gate 관계가 혼동되지 않는다.
- [ ] 전체 view에서 Lane/Gate focus로 자연스럽게 이동한다.
- [ ] 수동 좌표 없이 자동 배치가 충분히 안정적이다.
- [ ] 다음 단계의 편집 동작을 구체적으로 판단할 수 있다.

### 산출물

- 실행 가능한 프론트엔드 프로토타입
- 대표 fixture
- 화면별 캡처 또는 짧은 사용 기록
- 발견한 시각 문법 및 수정사항

## 4. Phase 1 — Graph Interaction Prototype

### 목적

사용자가 좌표가 아니라 의미 관계를 편집하는 조작 모델을 검증한다.

### 범위

브라우저 메모리에서만 동작하는 편집 프로토타입을 만든다.

- 선행·후속·병렬 task 생성
- 기존 task 간 dependency 연결·해제
- parent/child 분해 관계
- task를 Gate required/reference/unlocks로 연결
- cycle 및 잘못된 관계의 즉시 거부
- Gate open/ready/passed 상태 시뮬레이션
- undo 또는 변경 전 preview
- 관계 변경 후 자동 layout 재계산

### Gate V1 — 그래프 조작 승인

- [ ] 후속 task 생성과 기존 task 연결이 별도 설명 없이 가능하다.
- [ ] Parent/child와 blocking dependency가 구별된다.
- [ ] Gate에 task를 연결하는 세 가지 의미가 명확하다.
- [ ] 잘못된 cycle과 Gate 역방향 관계가 이해 가능한 방식으로 거부된다.
- [ ] 자동 재배치 후에도 사용자가 변경 위치를 추적할 수 있다.
- [ ] UI 조작에서 서버가 강제해야 할 불변 조건을 목록화했다.

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

- workspace와 membership
- repository registry
- lane lifecycle 및 fork lineage
- task와 parent/child
- dependency DAG
- Gate와 pass event
- authorization
- event log

### 범위

- 단일 사용자 또는 최소 인증
- fixture를 실제 API 데이터로 대체
- DAG cycle 방지
- Gate readiness 계산과 통과
- lane close-out/discard/fork
- optimistic concurrency 또는 graph revision 충돌 처리
- migration 및 기본 backup 절차

### Gate V2 — 도메인 코어 승인

- [ ] Phase 1의 주요 그래프 동작이 API와 DB에서 동일하게 보장된다.
- [ ] 서버가 cycle, 권한과 Gate 상태를 신뢰 가능한 방식으로 강제한다.
- [ ] lane fork와 Gate pass history를 event에서 재구성할 수 있다.
- [ ] 재시작 후 데이터와 graph revision이 일관된다.
- [ ] Day Tripper와 무관한 fixture로도 모델이 성립한다.

## 6. Phase 3 — Multi-repository Git 및 AI Integration

### 목적

Task 기록과 실제 개발 실행을 연결하고 AI가 Baley를 직접 운용하게 한다.

### 범위

- workspace에 여러 Git repository 등록
- task별 복수 GitBinding
- branch, worktree, commit과 PR 연결
- Go CLI
- Codex skill
- 실행 가능한 다음 task 조회
- AI execution 시작·진행·완료 기록
- lane brief 생성
- repository 상태와 Baley 기록의 불일치 탐지

CLI 개념 예시:

```text
baley lane list
baley lane brief <lane>
baley task ready --lane <lane>
baley task start <task>
baley task attach-git <task>
baley task finish <task>
baley gate status <gate>
```

### Gate V3 — 실행 연결 승인

- [ ] 하나의 task가 여러 repository의 Git 변경을 참조할 수 있다.
- [ ] AI가 UI 없이 CLI/API로 task 생명주기를 진행할 수 있다.
- [ ] Branch, worktree와 commit이 task의 실행 증거로 추적된다.
- [ ] 며칠 뒤 lane brief만으로 중단된 맥락을 복원할 수 있다.
- [ ] AI가 close/archive 권한을 우회하지 않는다.

## 7. Phase 4 — 2~3인 협업

### 목적

사람과 각자의 AI agent가 같은 workspace를 안전하게 공유한다.

### 범위

- 사용자 인증과 workspace membership
- task creator 기반 close/archive 권한
- Gate 승인 권한
- 동시 graph 변경 충돌 처리
- audit event
- stale lane, blocker와 Gate 준비 알림
- 생성자 퇴장 시 권한 승계 정책

### Gate V4 — 협업 승인

- [ ] 서로 다른 사용자가 동시에 다른 lane을 수정할 수 있다.
- [ ] 같은 graph revision 충돌이 데이터 손실 없이 처리된다.
- [ ] 생성자와 승인자 권한이 서버에서 강제된다.
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
| 큰 DAG가 읽기 어려움 | 전체 렌더링보다 focus, 축약과 Gate view를 우선 검증 |
| Lane이 단순 label로 퇴화 | 목표, brief, lifecycle과 자체 DAG를 제품 규칙으로 유지 |
| Gate가 단순 milestone label로 퇴화 | required/reference/unlocks 관계와 통과 조건을 명시적으로 유지 |
| AI 기록이 부정확함 | Git 증거, event history와 사람 승인으로 교차 검증 |
| 데이터 모델을 너무 일찍 고정 | Phase 0~1은 fixture와 메모리 상태로만 검증 |
| Day Tripper에 과적합 | 독립 fixture 및 다른 가상 프로젝트로 각 Gate에서 검증 |
| 기존 제품과 차별성이 약함 | 장기 lane, cross-lane Gate와 multi-repo 기억에 범위 집중 |
| 기능이 workflow engine으로 팽창 | 범용 실행 엔진, 일정 최적화와 대규모 조직 기능은 초기 제외 |

## 11. 현재 다음 행동

1. Phase 0 대표 fixture의 node, edge와 상태를 확정한다.
2. Multi-lane View의 첫 시각 구조를 구현한다.
3. Lane Focus와 Gate Focus를 같은 fixture에서 파생한다.
4. 실제 화면을 보고 Gate V0 체크리스트를 판정한다.
5. 통과 후에만 Phase 1 편집 동작을 상세 설계한다.

## 12. Close-out

- [ ] 완료 상태 (done / abandoned / superseded)
- [ ] 신규 작업 참조 필요? (yes → 유지, no → history 이동 검토)
- [ ] 이관 시 이동 경로:
