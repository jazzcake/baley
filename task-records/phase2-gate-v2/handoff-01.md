---
baley_record: 1
record_id: "c976d5e0-7926-4ddb-94d6-416a2f021cf6"
task_id: null
task_key: "phase2-gate-v2"
record_type: handoff
run_id: null
created_at: "2026-07-19T01:06:28.9147113+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Phase 2 Gate V2 구현 Handoff

## 시작 지점

[`detailed-plan-01.md`](detailed-plan-01.md), `docs/baley-roadmap.md`의 Gate V2, `docs/baley-system-spec-v1.md` §20~21, `docs/baley-phase2-4-desktop-handoff.md`의 통합 큐를 읽는다.

현재 `main` HEAD는 `5bd3e1f`이고 별도 branch/worktree를 만들지 않는다. 기존 워킹트리의 `run.start`와 Viewer 변경을 보존한다. 개발 DB `baley`를 truncate하지 않고 `baley_test`만 integration test에 사용한다. 사용자의 기존 8080 server를 중지하지 않는다.

## 실행 순서

1. `run.heartbeat`와 terminal command를 version CAS로 연결한다.
2. Record/Git schema와 command/query를 연결한다.
3. `task.report_implemented`와 decision projection을 연결한다.
4. 나머지 graph mutation을 기존 domain registry 기반으로 연결한다.
5. Viewer projection과 Gate V2 인수 시나리오를 검증한다.
6. 각 단위의 독립 리뷰 finding을 반영한다.

## 금지

- fixture 직접 수정으로 server mutation을 대체하지 않는다.
- raw lease token이나 절대 worktree 경로를 저장하지 않는다.
- Agent가 Task confirm, Lane 종료, active Gate 변경, Gate pass 또는 Workspace close 승인을 대신하지 않는다.
- 단위 검증만으로 Phase 2 Gate를 통과 처리하지 않는다.
