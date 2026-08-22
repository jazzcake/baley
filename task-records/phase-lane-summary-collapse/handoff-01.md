---
baley_record: 1
record_id: "ff4f3d9e-b2d1-4ac8-a969-95b3393b3a7f"
task_id: 138
task_key: "phase-lane-summary-collapse"
record_type: handoff
run_id: "5cc7f50d-135b-49d0-bf32-9069a1a37a1c"
created_at: "2026-08-20T10:30:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Handoff — 완료 Phase Lane 요약 접기 캔버스

Task #138을 구현하세요. 목표는 완료된 Phase가 Lane마다 요약 카드 하나 폭만 차지하게
접혀 캔버스 가로 폭을 줄이는 것입니다. 단일 Phase 카드로 합치지 마세요. Lane 행마다
summary card가 있어야 흐름이 보존됩니다.

먼저 `task-records/phase-lane-summary-collapse/detailed-plan-01.md`를 읽고 현재
`src/App.tsx`, `src/graph/layout.ts`, 관련 projection/layout 테스트를 검사하세요.

필수 동작:

1. `completed` Phase만 기본 접힘이며, active/planned Phase는 펼친 상태입니다.
2. 접힌 Phase에는 모든 Lane별 summary card 하나가 남습니다. Task가 없는 Lane은 `No work`
   상태를 표시합니다.
3. 접힌 내부 Task를 오가는 외부 Dependency는 요약 카드로 집계·재연결합니다. Lane 간
   연결은 방향이 보이는 점선/수량 표시로 남깁니다.
4. passed Gate도 compact 처리하되 open/ready Gate의 의미와 폭은 바꾸지 않습니다.
5. summary card와 Phase header toggle로 펼칠 수 있고, Task 검색·직접 URL·Inspector에서
   숨겨진 Task를 선택하면 자동으로 해당 Phase가 펼쳐집니다.
6. 접힘 상태는 Workspace별 Viewer localStorage에만 저장하고 서버 domain/API를 변경하지
   않습니다.

React/React Flow 관련 경로이므로 구현 전에 개발 전용 구조화 trace를 추가하세요. 사용자
event, 계산된 collapsed set, React state, React Flow store, 파생 node/edge 수, 렌더된 DOM
상태를 기록하고 첫 불일치 계층을 확인한 뒤 수정합니다.

관련 테스트를 추가한 뒤 `npm test`와 `npm run build`를 실행하세요. 기존 사용자의 변경은
보존하고, 범위 밖 파일은 수정하지 마세요.
