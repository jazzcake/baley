---
baley_record: 1
record_id: "43f29f57-3558-4232-8730-cd7ddec80f5b"
task_id: null
task_key: "phase2-gate-v2"
record_type: detailed-plan
run_id: null
created_at: "2026-07-19T01:06:28.9147113+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Phase 2 Gate V2 완료 상세계획

## 목표

환경 독립 정책 모듈을 실제 PostgreSQL, HTTP, MCP와 read-only Viewer에 연결하고 Gate V2 인수 기준을 증거와 함께 닫는다. Gate 통과 mutation은 구현 완료와 분리하며 사용자의 최종 수동 인수와 명시적 승인 뒤에만 수행한다.

## 현재 기준선

- Git 기준선: `5bd3e1f`
- 현재 워킹트리: `run.start` vertical slice와 Viewer baseline 수정 완료, 미커밋
- 완료된 통합: Gate 전이, Task confirm, Gate Task pass/revoke, `run.start`
- 미완료 핵심: Run heartbeat/terminal, Record/Git index, Run/Record query, 나머지 graph command, backup/restart 인수

## 구현 단위

1. Run heartbeat, terminal version CAS, timeout interruption와 manual correction
2. Task Record, Repository, CommitReference, RunGitObservation migration과 transaction
3. Record/Git mutation 및 Run/Record query의 HTTP/MCP adapter
4. `task.report_implemented`와 completion Record/decision projection 연결
5. Task, dependency, Lane, Gate의 남은 mutation planner를 command service와 PostgreSQL에 연결
6. read-only Viewer에 Run/Record/Git evidence projection 추가
7. backup/restore, process restart, 독립 fixture와 전체 Gate V2 인수 테스트

## 단위 완료 규칙

각 단위는 migration → repository → application transaction → HTTP → MCP → Viewer(해당 시) → PostgreSQL integration → 독립 리뷰 순서로 닫는다. 순수 policy를 adapter에 복제하지 않고 `domain.PlanMutation` 또는 기존 domain planner를 transaction 안에서 호출한다.

## 완료 조건

- Gate V2 다섯 기준을 각각 실행 증거에 연결
- PostgreSQL migration up/down/up
- DB integration skip 0, HTTP contract와 MCP stdio E2E 통과
- terminal 경쟁, idempotency, rollback, cycle, 승인 결속 검증
- backup/restore 및 재시작 후 projection/revision 일관성
- 독립 Agent 리뷰의 High/Medium finding 0건
- 사용자에게 수동 인수 항목과 Gate 승인 preview 제공

## 사용자 책임

구현 중 별도 작업은 없다. 마지막에 Viewer/명령 시나리오를 수동 확인하고, 준비된 Gate V2 통과 여부를 명시적으로 승인한다.
