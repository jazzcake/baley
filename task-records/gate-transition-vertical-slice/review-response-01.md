---
baley_record: 1
record_id: "172fc783-f750-43e5-bb4a-949918aa0aef"
task_id: 110
task_key: "gate-transition-vertical-slice"
record_type: review-response
run_id: null
created_at: "2026-07-18T01:28:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Gate Transition Vertical Slice 리뷰 반영

## 반영 결과

독립 Agent의 High 3건과 Medium 4건을 모두 반영했다.

1. typed request fingerprint를 command row에 저장하고 같은 idempotency key 재호출에서 비교한다. 다른 name/arguments/revision은 `idempotency_conflict`다.
2. Workspace row를 먼저 잠그고 idempotency row를 조회한다. 병렬 동일 요청은 같은 Command ID를 반환하며 한쪽만 실제 mutation을 수행한다.
3. Gate Task pass/revoke는 현재 active Phase의 outgoing Gate에만 허용한다.
4. Gate endpoint Phase별 uniqueness와 인접 Phase trigger를 migration에 추가하고 domain pass 평가에서도 position을 검사한다.
5. Task와 ready Gate query에 `decisionRequired`, expected revision과 decision snapshot hash를 제공한다.
6. `gate.passed` Event에 link ID, Task ID/status, explicit pass와 pass reason 존재 근거를 기록한다.
7. MCP에 `baley_workspace_get`을 추가해 query 6개, mutation preview/execute 8개, 총 14개 tool을 노출한다.

## 회귀 검증

- 다른 payload의 idempotency key 재사용 거부
- 동일 payload 병렬 execute의 단일 command/idempotent 결과
- 미래 Gate의 pass_task 거부
- ready Gate decision binding
- Gate pass Event condition evidence
- MCP 14개 tool 이름 집합 및 stdio graph 호출

## 재리뷰

독립 Agent 재감사에서 기존 7건이 모두 해소됐고 미해결 사항이나 신규 회귀가 없음을 확인했다.

## 잔여 위험

- HumanApprovalAttestation은 인증된 신원 증명이 아니라 로컬 단일 사용자용 protocol audit metadata다.
- Windows 환경에서 CGO가 비활성이라 `go test -race`는 실행하지 못했다. PostgreSQL 병렬 integration test로 idempotency lock 경로를 대신 검증했다.
- Viewer 번들은 현재 chunk size warning이 있으나 기능 차단 문제는 아니다.
- 연결 가능한 in-app browser가 없어 자동 시각 검증은 수행하지 못했다. HTTP 200, API projection과 프런트 test/build를 검증했고 사람 인수 테스트를 남겨 두었다.
