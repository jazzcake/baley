---
baley_record: 1
record_id: "785a29d8-133f-465c-9af0-879ea0474d30"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: completion-report
created_at: "2026-07-26T14:42:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Lane Backlog Vertical Slice 완료 보고서

## Outcome

Task #121의 세 결과를 함께 구현·검증했다.

1. 기존 grouped approval과 exact warning acknowledgement 경계를 보존했다.
2. `task.create`와 Backlog promote가 명시적 target Phase로 pending Task를 만든다.
3. Phase-free lane Backlog를 contracts부터 Viewer까지 하나의 live vertical slice로 연결했다.

사람의 `task.confirm`은 실행하지 않았으며 구현 보고 후 confirmation만 pending으로 남긴다.

## 구현 범위

- `BacklogItem`의 `active -> promoted | discarded` lifecycle과 lane position 연산
- Task counter와 독립적인 `B#<publicId>` counter
- create/update/move/reorder/discard/promote preview/execute
- command별 strict application decoder와 typed MCP schema
- PostgreSQL migration, read projection, HTTP read API/graph, CLI model
- promote의 `domain.PlanTaskCreate` 재사용 및 단일 transaction
- active Backlog가 있는 lane 종료 차단과 Workspace close warning
- live active Backlog rail/list/lane anchor 및 5개 이상 lane palette
- Operator skill, system spec, command architecture, Backlog guide 동기화

## Acceptance criteria

- [x] Phase 없이 lane Backlog를 생성·수정·이동·정렬·discard할 수 있다.
- [x] Backlog public ID와 Task counter가 독립적이다.
- [x] promote가 명시된 Phase에 pending Task를 원자적으로 만든다.
- [x] promote가 기존 dependency/topology/warning/terminal 규칙을 재사용한다.
- [x] 실패 주입 시 Task, dependency, Backlog, counters, events, revision이 모두 rollback된다.
- [x] promote가 Gate 또는 Gate entry Task를 자동 변경하지 않는다.
- [x] MCP, CLI, HTTP, contracts가 같은 command/read model을 노출한다.
- [x] Viewer는 mock 없이 live active Backlog를 표시한다.
- [x] 5개 이상 lane의 rail/anchor/palette 회귀 테스트가 통과한다.
- [x] grouped approval, phase-targeted Task create, Backlog promotion을 함께 회귀 검증했다.
- [x] 독립 Agent 리뷰의 unresolved blocking finding이 0이다.

## 검증

- `go test -count=1 ./...`: PASS
- `go vet ./...`: PASS
- 전용 `baley_test_121` PostgreSQL DB migration 1→8: PASS
- 실제 DB rollback/concurrency/grouped integration suite: PASS
- 실제 server + MCP stdio E2E 55 tools, Backlog preview/create/promote/read: PASS
- `npm test -- --run`: 10 files, 38 tests PASS
- `npm run build`: PASS
- Baley skill validator: PASS
- contracts JSON parse/assertions: PASS
- `git diff --check`: PASS
- 독립 Agent raw-diff review: High 0 / Medium 0 / Low 0 / Blocking 0

## Run 및 잔여 경계

- planning Run `cfc722c9-018c-4f1d-a660-ef3f82a537ae`는 succeeded 상태다.
- implementation Run 시작을 fresh revision에서 시도했으나 Task의
  `embedding-enablement` Phase가 inactive여서 서버가 `phase_inactive`로 거부했다.
  실패한 요청은 Run을 생성하거나 Workspace를 변경하지 않았다.
- 이 lifecycle 제약을 우회하거나 구현/리뷰/reporting Run을 허위 생성하지 않았다.
  구현·독립 리뷰·보고 근거는 이 Task Record와 검증 출력에 남겼다.
- 현재 대화의 설치된 MCP schema는 런타임 hot reload를 하지 않는다. 새 14개
  Backlog 도구는 실제 최신 server와 stdio MCP E2E에서 검증했다.
- production build의 1.8 MB chunk-size 경고는 비차단 최적화 후속 항목이다.
- commit/push는 사용자 권한 범위가 아니므로 수행하지 않았다.

## Recovery addendum

독립 리뷰 중 test DB URL 오지정으로 운영 Baley DB가 손상됐다. 물리 복구가
불완전하여 repository Task Records와 승인된 Adoption manifest로 operational
state를 재구성했고, 최신 server에서 graph/Task/Gate/Record/Run 정합성을 검증한
뒤 운영 DB로 승격했다.

- 복구 원장: `docs/recovery/2026-07-26-baley-db-incident.md`
- 재구성 기준 revision: 194
- Record 복구 후 revision: 196
- 이 Task의 `task.report_implemented` 후 revision: 197
- original Command/Event stream 손실은 `recovery.reconstructed` Event에 명시
- 손상 DB는 `baley_damaged_20260726`로 보존
- production DB 이름을 거부하는 destructive integration test guard 추가
