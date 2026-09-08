---
baley_record: 1
record_id: "2ee3f487-7994-4a1d-aa03-88e8a519f770"
task_id: 183
record_type: detailed-plan
run_id: "6831827a-5433-4164-9952-d1c20c51a37d"
created_at: "2026-09-08T13:30:00+09:00"
created_by: "codex-pm"
supersedes: null
---

# Task #183 상세 계획 — Seamless Task Journal

## 목표와 제품 계약

사용자는 기존처럼 자연어로 Task를 만들고, 진행하고, 바꾸고, 완료하고, 확인한다. `baley-manage-work` skill은 그 발화에 이미 포함된 맥락만 조용히 추출하며, 별도 로그 입력을 요구하지 않고 이유·대안·결과를 만들어내지도 않는다. 서버는 기존 lifecycle command와 같은 원자성·CAS·idempotency·승인 경계 안에서 그 맥락을 append-only로 보존한다.

Task가 기본 정체성이다. 이번 구현에서는 별도 필수 Decision 엔터티나 D# workflow를 만들지 않는다. 향후 실제 데이터가 쌓인 뒤 반복되는 분류와 cross-Task 승격 필요성을 판단한다.

## 구현 순서

1. 기존 command, Event, transaction, migration, read API, MCP/skill 경계를 먼저 추적하고 가장 작은 호환 계약을 확정한다.
2. lifecycle 입력에 선택적 `contextNote`를 추가한다. 필드가 없을 때 기존 request·hash·preview·execute·응답 동작은 바뀌지 않아야 한다.
3. append-only `task_journal_entries` projection을 도입한다. 안정적인 envelope에는 workspace/task/event/command, lifecycle stage, narrative, schema version, actor provenance, occurred/recorded time을 두고 진화 가능한 구조화 세부 정보는 JSONB context에 둔다.
4. 기존 lifecycle Event와 journal 기록을 같은 DB transaction에 묶는다. 승인 대상 command에서는 contextNote도 preview command hash와 grant 검증 대상이어야 하며, retry는 중복 journal row를 만들지 않아야 한다.
5. Task별 timeline과 Workspace 범위 평가·복기 query를 위한 bounded read API를 구현한다. authorization, 정렬, pagination, workspace 격리를 기존 read 경계와 맞춘다.
6. `baley-manage-work` skill에 silent capture 규칙과 lifecycle별 추출 힌트를 반영한다. 명시되지 않은 rationale·대안·결과는 생성하지 않는다는 금지 규칙을 둔다.
7. confirm 단계에서 새 질문 없이 축적된 맥락을 짧게 확인할 수 있도록 기존 Inspector 흐름과 API의 최소 연결을 검토한다. UI 변경이 불필요하거나 과도하면 그 근거를 완료 보고에 기록한다.
8. Task #182 분석 산출물을 보존하고 구현·리뷰·완료 Record와 commit evidence를 같은 feature branch에 남긴다.

## lifecycle capture 기준

- create: 문제, 의제, 왜 지금 Task가 생겼는지, 초기 목표
- first progress/run: 바뀐 상황, 현재 목표, 완료 계약
- update/rework: 변경 이유, 수정된 목표, 폐기·수정된 판단
- block/unblock: 실제 blocker 또는 해소된 외부 조건
- implemented: 중간 변화, 고려·폐기한 대안, 수정한 판단, 실제 결과와 잔여 위험
- confirm/discard: 기존 기록의 간단한 요약과 사람 결정의 맥락

모든 항목은 선택적이다. 비어 있는 context를 형식적으로 적재하지 않는다.

## 검증 계약

- migration up/down 또는 저장소 표준에 맞는 rollback 검증과 schema version 검증
- context 없는 기존 lifecycle 전체 회귀
- context가 있는 create/progress/update/rework/block/unblock/implemented/confirm/discard의 저장·조회
- lifecycle mutation 실패 시 journal 미기록, 성공 retry 시 단일 기록
- preview/execute payload mismatch 및 승인 grant binding 검증
- initiated/executed actor provenance, workspace 격리, pagination/order 검증
- JSONB query/index가 평가·복기 use case를 지원하는지 SQL 또는 integration test로 증명
- 전체 Go tests, `go vet`, UI 관련 변경 시 UI tests와 production build
- `go run` 금지. 필요한 실행 파일은 `C:\dev-bin\baley`에만 빌드
- 운영 DB, 운영 service, firewall, Tailscale 설정, 운영 branch는 변경하지 않음

## 오케스트레이션과 리뷰

PM이 구현 계약과 경계를 소유한다. 시니어 개발 agent가 기존 `development-decision-log` worktree에서 구현·검증하고, 별도 리뷰 agent는 소스 변경 없이 독립적으로 correctness, migration 안전성, auth/CAS/idempotency, 승인 binding, queryability, skill의 비창작 규칙과 회귀를 검토한다. blocking 또는 material finding은 같은 시니어 agent에게 돌려보내고 독립 재리뷰가 통과할 때까지 Task를 implemented로 보고하지 않는다.

## 완료 경계

기능과 테스트, 완료 보고, 독립 리뷰, commit reference, typed acceptance evidence가 Baley에 연결되고 unresolved blocking finding이 0일 때만 implemented로 전환한다. feature branch push는 허용하되 merge·deploy·운영 DB migration은 하지 않는다. 최종 Task confirm은 signed-in Baley Viewer의 browser-bound 사람 승인으로만 수행한다.
