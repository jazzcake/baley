---
baley_record: 1
record_id: "e7e60a24-80a3-401c-82d1-769001a64634"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: handoff
run_id: "cfc722c9-018c-4f1d-a660-ef3f82a537ae"
created_at: "2026-07-25T21:55:04+09:00"
created_by: "codex"
registration_state: registered
supersedes: "72c8771f-daa7-46f9-b2bf-04291a8b2272"
---

# 새 구현 세션 Handoff Prompt

아래 내용을 새 Codex 구현 세션의 첫 메시지로 그대로 전달한다.

---

`D:\Project_AI\baley`에서 Baley Task #121의 lane Backlog vertical slice를 끝까지 구현하세요.

반드시 먼저 다음 파일을 순서대로 전체 읽으세요.

1. `D:\Project_AI\baley\AGENTS.md`
2. `D:\Project_AI\baley\.agents\skills\baley-manage-work\SKILL.md`
3. `D:\Project_AI\baley\task-records\lane-backlog-vertical-slice\detailed-plan-02.md`
4. `D:\Project_AI\baley\docs\baley-lane-backlog.md`
5. `D:\Project_AI\baley\task-records\lane-backlog-vertical-slice\handoff-02.md`

목표:

- Phase를 가정하지 않는 lane별 `BacklogItem` 도메인을 추가합니다.
- create/update/move/reorder/discard/promote를 contracts, domain, application, PostgreSQL, HTTP, CLI, typed MCP에 수직으로 연결합니다.
- promote는 target Phase를 명시적으로 받고 기존 `task.create`의 dependency/topology/warning 계약을 재사용하여 pending Task를 원자적으로 생성합니다.
- 현재 mock 기반 Viewer Backlog rail/list/anchor를 live graph 데이터로 전환합니다.
- Task #121의 나머지 결과인 grouped approval과 phase-targeted Task creation도 기존 미커밋 구현을 보존하고 회귀 검증합니다.
- 구현, 전체 테스트, 독립 Agent raw-diff 리뷰, 수정, completion report, Baley implementation reporting까지 수행합니다.
- 마지막 사람 `task.confirm`만 pending으로 남기고 “완료로 확인할까요?”라고 묻습니다.

시작 상태:

- Workspace: `Baley Pilot`
- Workspace ID: `00000000-0000-4000-8000-000000000001`
- Task: `#121 Implement Operator approval and Task-intake path`
- Task UUID: `47c2d962-9008-49cb-9e41-2b063d0213e4`
- lane: `adoption`
- phase: `embedding-enablement`
- 이 handoff 작성 시 Task status는 `in_progress`입니다.
- 이 handoff 작성 전 관찰 revision은 189였지만, Task Record 등록으로 증가할 수 있으므로 숫자를 재사용하지 말고 fresh read하세요.
- planning Run `cfc722c9-018c-4f1d-a660-ef3f82a537ae`는 handoff 작성 세션에서 terminal 처리할 예정입니다. fresh read로 확인하고 terminal이면 새 `implementation` Run을 시작하세요.

Baley 운용:

1. `baley_task_get`과 `baley_workspace_graph`로 fresh state를 읽습니다.
2. active Run을 조회하고 stale/중복 Run을 만들지 않습니다.
3. Task #121에 `implementation` Run을 시작하고 장기 작업 동안 heartbeat를 유지합니다.
4. typed MCP가 현재 thread에 새 Backlog schema를 즉시 반영하지 못하는 것은 예상 가능한 schema-reload 경계입니다. 서버와 MCP stdio E2E까지 구현·검증한 뒤 실제 새 tool 호출만 새 thread가 필요하면 사용자에게 요청합니다.
5. 그 외에는 사용자에게 중간 승인을 요구하지 말고 진행합니다.
6. 구현 완료 후 completion report를 등록하고 `task.report_implemented`까지만 실행합니다. `task.confirm`은 실행하지 않습니다.

작업 트리 안전:

- 현재 worktree는 의도적으로 매우 dirty합니다. 모든 기존 수정/미추적 파일은 사용자 소유입니다.
- `git reset`, `git checkout --`, `git clean`, 광범위한 자동 rewrite를 사용하지 마세요.
- 수정 전 `git status --short`, migration 목록, 대상 파일 diff를 fresh 확인하세요.
- 특히 outcome-first/grouped approval, gate entry Task, Backlog UI prototype 변경을 삭제하거나 되돌리지 마세요.
- `00007_gate_entry_tasks.sql`이 이미 미추적 상태였으므로 Backlog migration 번호를 추측하지 말고 fresh 확인하세요.
- 파일 편집은 작은 `apply_patch`로 통합하세요.
- 현재 요청은 commit/push 권한을 새로 부여하지 않습니다. 검증된 working tree와 Baley evidence까지만 만들고, commit/push는 사용자가 새 세션에서 명시할 때 수행하세요.

고정 설계:

- BacklogItem은 Task와 별도이며 `phase_id`, dependencies, Gate, Run, Task Record, Task status를 갖지 않습니다.
- status는 `active | promoted | discarded`입니다.
- active만 lane-scoped position을 가집니다.
- 표시 ID는 `B#<publicId>`, counter는 Task counter와 독립입니다.
- Backlog 생성 때 Phase를 묻거나 추론하지 않습니다.
- promote 때 lane/title/description은 Backlog에서 복사하고 payload override를 허용하지 않습니다.
- promote는 기존 `domain.PlanTaskCreate` 의미를 재사용합니다.
- promote transaction은 Task/dependencies/Backlog transition/counter/events/revision을 원자적으로 처리합니다.
- promote가 Gate를 자동 attach하거나 entry Task를 변경하면 안 됩니다.
- typed promote execute schema는 `acknowledgedWarningCodes`를 노출해야 합니다.
- discard는 audited soft terminal transition이며 Task terminal reason을 만들지 않습니다.
- Viewer는 이번 slice에서 read-only이고 live active backlog만 rail에 표시합니다.
- lane palette는 UI-only 순환 palette이며 server에 저장하지 않습니다.

구현 순서는 상세 계획의 Wave A~F를 따르세요. 계획과 실제 코드가 충돌하면 임의로 설계를 바꾸지 말고, 기존 public contract와 데이터 무결성을 우선하여 최소 변경을 선택하고 completion report에 차이를 기록하세요.

필수 검증:

- touched Go files gofmt
- `go test -count=1 ./...`
- `go vet ./...`
- 실제 test DB를 사용하는 migration/integration tests
- MCP stdio E2E; 환경 누락에 의한 silent skip은 성공으로 세지 않음
- `npm test`
- `npm run build`
- Baley skill validator와 관련 contract assertions
- `git diff --check`
- 독립 Agent가 raw diff와 test output을 직접 리뷰
- blocking finding 수정 후 관련 suite와 전체 회귀 재실행

완료 판정:

- 상세 계획의 Acceptance criteria를 체크리스트로 검증합니다.
- #121 범위의 grouped approval, phase-targeted Task create, lane backlog promotion 셋 중 하나라도 검증되지 않으면 #121을 `implemented`로 보고하지 않습니다.
- 모두 충족되면 `task-records/lane-backlog-vertical-slice/completion-report-01.md`를 만들고 등록합니다.
- implementation/review/reporting Runs를 모두 terminal 처리합니다.
- Task #121을 `implemented`로 보고한 뒤 사용자에게 구현 결과, 테스트, 독립 리뷰를 짧게 요약하고 “완료로 확인할까요?”라고 질문합니다.

---
