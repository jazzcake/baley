---
baley_record: 1
record_id: "f30e6153-4385-4974-9dc6-b8c62bca9a66"
task_id: 161
task_key: "task-confirm-button-ux"
record_type: detailed-plan
run_id: "55c9b7a8-d70f-4216-9d03-89d5141077ee"
created_at: "2026-09-02T11:25:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Task별 사람 확인 버튼과 승인 UX 계획

## 쉬운 설명

구현이 끝난 Task 상세 화면에 사람이 결과를 읽고 누를 수 있는 `Confirm task` 버튼을 추가한다.
사용자는 내부 Command JSON을 작성하지 않으며, 버튼 뒤에서는 기존의 안전한 1회 승인 절차가 그대로 실행된다.

## 왜 필요한가

현재 범용 Human approval 화면은 저수준 명령을 점검하는 진단 도구에 가깝다. 일반 사용자가 Task 확인을
위해 command name, UUID, revision을 직접 입력하면 이해하기 어렵고 오입력 가능성이 높다.

## 완료되면 무엇이 달라지는가

1. `implemented` Task의 Inspector에 요약, 구현 평가, 검증·리뷰 근거와 `Confirm task` 동작이 보인다.
2. 첫 클릭은 현재 Task와 Workspace revision으로 fresh preview를 만들고 사람이 확인할 결과를 표시한다.
3. 최종 클릭은 로그인한 브라우저 세션에서 단기·단발성 grant를 발급한 뒤 정확히 같은 명령을 한 번 실행한다.
4. 성공하면 Task 상태와 Workspace revision이 즉시 화면에 반영되고, 실패·경고·stale revision은 이해하기 쉬운 문구로 남는다.

## 범위·제외 사항

- 포함: React 상태/API 흐름 계측, Task 확인 전용 UI, warning acknowledgement, 오류·성공 상태,
  API 순서 회귀 테스트, production build, Docker 배포와 실제 브라우저 확인.
- 제외: 일괄 Task 확인, 다른 human-only 명령의 전용 UI, Agent/MCP 권한 확대, approval grant 계약 완화.
- 범용 JSON 패널은 고급 진단 경로로 유지하되 Task 확인의 기본 경로에서는 요구하지 않는다.

## 구현 순서

1. Inspector에 필요한 CSRF, Workspace membership, graph refresh 경계를 명시적으로 전달한다.
2. Task 상태가 `implemented`일 때만 표시되는 확인 컴포넌트를 만들고 클릭·계산 상태·React 상태·API 상태·DOM 상태를 개발용으로 추적한다.
3. fresh `task.confirm` preview를 생성하고, human approval required 이외의 blocking error와 warning acknowledgement를 처리한다.
4. 사람이 최종 버튼을 누르면 grant 발급과 exact command 실행을 순서대로 수행하고, 실패 시 미사용 grant를 revoke한다.
5. UI 노출 조건, preview/grant/execute 순서, approval 필드 비포함, stale/error, 성공 갱신을 테스트한다.
6. 독립 리뷰에서 보안 경계와 사용성을 확인하고 지적을 반영한 뒤 전체 검증·배포·실사용 확인을 기록한다.
