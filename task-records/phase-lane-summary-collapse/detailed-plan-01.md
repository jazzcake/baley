---
baley_record: 1
record_id: "218dd62a-6c15-49b7-b115-49c62479ab9b"
task_id: 138
task_key: "phase-lane-summary-collapse"
record_type: detailed-plan
run_id: "5cc7f50d-135b-49d0-bf32-9069a1a37a1c"
created_at: "2026-08-20T10:30:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# 완료 Phase Lane 요약 접기 캔버스 — 상세 구현계획

## Outcome

완료된 Phase는 캔버스에서 여러 Task 폭이 아니라 **Lane마다 요약 노드 하나의 폭**만
차지한다. 각 Lane의 선행·후행 흐름은 그 요약 노드를 통과해 유지되며, 클릭하면 해당
Phase를 원래 Task 그래프로 펼칠 수 있다.

## 범위와 경계

- 자동 접기 대상은 `phase.state === "completed"`뿐이다. active/planned Phase와 passed
  여부가 아닌 Task 개수만으로 접지 않는다.
- 접힘은 Viewer 전용 상태다. Task, Dependency, Gate, Phase 도메인 데이터나 서버 API를
  변경하지 않는다.
- 모든 Lane에 compact 요약 노드를 둔다. 해당 Phase에 Task가 없는 Lane은 `No work`로
  표시해 Lane 행의 연속성을 유지한다.
- passed Gate는 인접 compact Phase 사이의 좁은 `passed` 표시로 축소한다. open/ready
  Gate는 기존 폭과 표현을 유지한다.

## 구현 순서

1. `WorkspaceViewer`에 `collapsedPhaseIds` 상태와 Workspace별 localStorage key를 추가한다.
   첫 로드에서는 completed Phase를 접고, 사용자의 펼침/접힘 선택을 저장한다.
2. `phasePresentation` 파생 모델을 만든다. 각 collapsed Phase × Lane 조합에 task 수,
   confirmed 수, in/out dependency 수, cross-lane dependency 수를 계산한다.
3. `layoutGraph`가 collapsed Phase를 입력으로 받아 일반 Phase에는 기존 ELK 배치를,
   collapsed Phase에는 summary-node 한 칸 폭과 Lane별 고정 위치를 반환하게 한다.
   passed Gate corridor도 compact 폭으로 계산한다.
4. React Flow node projection에서 숨겨진 Task 대신 `phase-summary:<phaseId>:<laneId>`
   노드를 만든다. 카드에는 Phase 이름, 상태, `N tasks`, 완료 수, `No work`를 표시한다.
5. Dependency projection은 숨겨진 endpoint를 대응 Lane summary node로 치환한다. 같은
   source/target 쌍은 하나로 합쳐 수량 배지를 보이고, Lane 간 dependency는 점선으로
   그려 흐름 방향을 보존한다.
6. Phase header에 펼침/접힘 버튼을 추가한다. summary card, 검색 결과, direct Task URL,
   Inspector에서 숨겨진 Task를 선택하면 해당 Phase를 먼저 펼친 뒤 기존 navigation을
   수행한다.
7. 접힌 Phase의 Gate corridor, phase overlay, keyboard focus 순서를 검증한다. summary
   card는 button semantics, `aria-expanded`, accessible name을 제공한다.

## 진단과 검증

구현 전후 개발 모드에서 다음을 `traceViewer`로 남긴다.

- toggle user event, 목표 collapsed ID 집합, 현재 React 상태
- 파생 summary node/aggregate edge 수와 endpoint 치환 결과
- React Flow store의 node/edge 수와 viewport
- requestAnimationFrame 이후 DOM의 phase width, summary-card 수, `aria-expanded`

테스트는 다음을 포함한다.

- completed Phase만 기본 접힘, active/planned Phase는 항상 전체 표시
- Lane별 summary node 하나와 `No work` card 생성
- intra-lane/cross-lane external dependency의 방향과 aggregate count 보존
- passed Gate의 compact 폭, open/ready Gate의 기존 폭 유지
- summary click, 검색, direct URL 선택이 Phase를 펼치고 Task를 선택
- localStorage 복원, keyboard 접근성, 전체 `npm test`와 `npm run build`

## 완료 기준

실제 다수 완료 Phase Workspace에서 가로 폭이 줄고, 모든 Lane의 연결 방향과 다음 active
Task가 식별 가능하며, 펼침·검색·URL navigation이 기존 동작을 깨지 않는다.
