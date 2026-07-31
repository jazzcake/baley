---
baley_record: 1
record_id: "fc487caa-3890-440c-8270-eb53bae551c6"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: detailed-plan
run_id: "cfc722c9-018c-4f1d-a660-ef3f82a537ae"
created_at: "2026-07-25T21:55:04+09:00"
created_by: "codex"
registration_state: registered
supersedes: "78adb737-ddeb-4e50-addf-c8a8867dc0f5"
---

# Lane Backlog Vertical Slice 상세 구현 계획

## 1. 목표

현재 Viewer에 mock으로 구현된 lane별 Backlog rail을 실제 Baley 도메인과 typed MCP에 연결한다.

완료 상태는 다음과 같다.

- 사용자는 Phase를 정하지 않고 lane에 Backlog item을 미리 등록할 수 있다.
- Backlog item은 Task와 별개의 계획 수집 항목이며 작업 상태나 승인 상태를 갖지 않는다.
- 계획이 구체화되면 사용자가 명시한 Phase로 Backlog item을 정식 pending Task로 원자적으로 승격한다.
- 승격 시 기존 `task.create`의 dependency, 실제 create-time warning, terminal reason 규칙을 그대로 적용한다.
- Viewer의 `[backlog][lane anchor][phase...]` 구조는 mock이 아니라 live Workspace 데이터로 동작한다.
- MCP, CLI, HTTP graph, persistence, contracts, docs, tests가 같은 모델과 용어를 사용한다.
- Task #121의 나머지 범위인 grouped approval과 phase-targeted Task creation도 현재 작업 트리의 구현을 기준으로 회귀 검증한다.
- 구현·테스트·독립 리뷰까지 완료한 뒤 Task #121은 `implemented`로 보고하고, 사람의 `confirmed` 전환만 남긴다.

## 2. 범위와 비범위

### 포함

1. `BacklogItem` 도메인 및 lifecycle
2. lane별 순서와 lane 간 이동
3. create, update, move, reorder, discard, promote 명령
4. typed MCP preview/execute 및 read tools
5. CLI 명령과 contract registry
6. PostgreSQL migration, projection, atomic persistence
7. HTTP graph/read model
8. Viewer live-data 연결과 기존 rail/list/anchor UX 유지
9. Operator skill, system spec, command architecture, backlog guide 정합화
10. 전체 자동 검증과 독립 Agent 리뷰

### 제외

- Backlog item에 Phase를 미리 배정하는 기능
- Backlog item 자체의 dependency, Gate condition, Run, Task Record, blocker, implemented/confirmed 상태
- 승격과 동시에 Gate condition 또는 Gate entry Task를 자동 연결하는 기능
- LLM이 Phase, dependency, terminal reason을 임의로 추론하여 무승인 승격하는 기능
- lane 색상 palette를 영속 도메인 속성으로 저장하는 기능
- drag-and-drop 보드, 다중 선택 편집, 검색·필터 고도화
- 기존 Task public ID 체계 변경

## 3. 핵심 도메인 결정

### 3.1 BacklogItem은 Task가 아니다

Task의 `phase_id`를 nullable로 바꾸지 않는다. 별도 aggregate/projection인 `BacklogItem`을 추가한다.

```text
BacklogItem
  id: UUID
  workspace_id: UUID
  public_id: integer
  lane_id: string
  title: string
  description: string
  status: active | promoted | discarded
  position: integer | null
  promoted_task_id: UUID | null
  discard_reason: string | null
  created_at: timestamp
  updated_at: timestamp
```

규칙:

- Phase 필드는 존재하지 않는다.
- 신규 항목은 `active`이며 lane의 마지막 position에 추가한다.
- active 상태의 lane에만 create/move target/promote할 수 있다.
- `active -> promoted`와 `active -> discarded`만 허용한다.
- `promoted`, `discarded`는 terminal이며 수정·이동·재정렬·재승격할 수 없다.
- active 항목만 position을 가진다.
- promoted 항목만 `promoted_task_id`를 가진다.
- discarded 항목만 non-empty `discard_reason`을 가진다.
- Task status와 혼동하지 않도록 UI 표시는 `B#123`, JSON은 integer `publicId: 123`을 사용한다.

Lane lifecycle 결합:

- active Backlog item이 하나라도 있으면 `lane.close_out`과 `lane.discard`를
  `lane_has_active_backlog`로 거부한다.
- Operator는 항목을 다른 active lane으로 move하거나 promote/discard한 뒤 lane을 종료한다.
- 따라서 terminal lane에 active Backlog item이 stranded 되는 상태를 허용하지 않는다.
- `workspace.close`에 active Backlog item이 남으면
  `workspace_close_residual_backlog` warning을 발생시키고 기존 exact acknowledgement
  경계를 적용한다.

### 3.2 독립 public counter

`workspace_counters`에 `next_backlog_public_id`를 추가한다.

- Task의 `next_task_public_id`와 독립적으로 증가한다.
- create execute에서만 증가한다.
- preview는 counter를 소비하지 않는다.
- promote는 Backlog counter를 바꾸지 않고 Task counter만 소비한다.
- concurrent create는 workspace row/counter locking 규칙을 기존 Task create와 동일하게 따른다.

### 3.3 lane별 순서

active 항목의 정렬 기준은 `(lane_id, position, public_id)`이다.

- create: 해당 lane의 마지막 position + 1
- move: 원래 lane에서 제거하고 target lane 마지막에 append
- reorder: 한 lane의 현재 active public ID 전체 순서를 입력받아 1..N으로 재기록
- move/discard/promote 뒤 원래 lane의 position을 연속된 1..N으로 compact
- reorder payload는 대상 lane의 현재 active set과 정확히 일치해야 한다.
- 누락, 중복, 다른 lane 항목, terminal 항목은 `backlog_order_mismatch`다.
- Workspace revision stale check로 동시 편집을 차단한다.

## 4. 명령 계약

모든 write 명령은 기존 명령과 같이 preview/execute를 제공하고 `workspace:operate` capability를 요구한다. Backlog는 계획 intake이므로 별도 human approval capability는 요구하지 않는다. 단, destructive delete 대신 audited soft discard만 제공한다.

### 4.1 `backlog.create`

command arguments:

```json
{
  "workspaceId": "<uuid>",
  "laneId": "adoption",
  "title": "사용자 인터뷰 정리",
  "description": "관찰 결과와 후보 요구사항을 정리한다."
}
```

command arguments의 `workspaceId`와 preview/execute envelope의 `idempotencyKey`,
`expectedWorkspaceRevision`, `acknowledgedWarningCodes`는 기존 mutation 공통
계약을 그대로 사용한다. envelope 필드는 command arguments에 중복 정의하지 않는다.

검증:

- workspace와 lane이 존재해야 한다.
- lane status가 active여야 한다.
- title은 기존 Task와 같이 trim 후 non-empty만 검증한다.
- description은 trim하고 empty를 허용한다. 이번 slice에서 Task에도 없는 임의 max
  length를 Backlog에만 추가하지 않는다.

결과:

- `backlog.created` event
- 생성된 Backlog public ID와 최종 lane/position
- execute 후 Workspace revision +1

### 4.2 `backlog.update`

입력은 Backlog public ID와 title/description patch를 받는다. Go decode model은
pointer/optional field를 사용하여 omitted와 explicit empty를 구분한다.

- active 항목만 가능하다.
- lane 변경이나 status 변경은 허용하지 않는다.
- empty patch는 `invalid_backlog_patch`로 거부한다.
- title의 explicit empty는 invalid이고 description의 explicit empty는 설명 clear로 허용한다.
- `backlog.updated` event에 before/after의 변경 필드만 기록한다.

### 4.3 `backlog.move`

입력은 Backlog public ID와 target lane ID를 받는다.

- active 항목만 가능하다.
- 동일 lane은 `backlog_lane_unchanged`로 거부한다.
- target lane은 active여야 한다.
- target lane 마지막에 append하고 두 lane의 active positions를 compact한다.
- `backlog.moved` event에 source/target lane과 position을 기록한다.

### 4.4 `backlog.reorder`

command arguments:

```json
{
  "laneId": "adoption",
  "orderedBacklogPublicIds": [4, 1, 8]
}
```

- 현재 lane의 active set 전체와 정확히 일치해야 한다.
- 순서가 동일하면 `backlog_order_unchanged`로 거부하여 불필요한 revision 증가를 막는다.
- empty lane의 `[]`도 동일한 no-op이므로 `backlog_order_unchanged`다.
- `backlog.reordered` event에는 ordered public IDs를 기록한다.

### 4.5 `backlog.discard`

입력은 Backlog public ID와 non-empty reason을 받는다.

- hard delete하지 않는다.
- status를 discarded로 바꾸고 position을 null로 만들며 원 lane을 compact한다.
- `backlog.discarded` event에 reason과 이전 lane/position을 기록한다.
- Task의 terminal reason이나 topology를 만들지 않는다.

### 4.6 `backlog.promote`

입력:

```json
{
  "workspaceId": "<uuid>",
  "backlogPublicId": 8,
  "taskUuid": "<caller-generated-uuid>",
  "phaseId": "embedding-enablement",
  "parentTaskId": null,
  "predecessorTaskIds": [],
  "successorTaskIds": [],
  "terminalReason": null
}
```

command arguments의 `workspaceId`와 공통 execute envelope의 `idempotencyKey`,
`expectedWorkspaceRevision`, `acknowledgedWarningCodes`를 사용한다.
`commandHash`는 서버가 canonical command로 계산·검증하는 audit 결과이며 일반
mutation execute의 caller input으로 추가하지 않는다.

결정:

- Task의 lane, title, description은 Backlog item에서 복사한다.
- promote payload에서 lane/title/description override를 받지 않는다. 변경이 필요하면 먼저 backlog update/move를 실행한다.
- Phase는 promote 시점에만 사람이/Operator가 명시한다.
- 새 Task status는 항상 `pending`이다.
- parent/dependencies/terminal reason은 기존 `task.create` 의미와 동일하다.
- `domain.PlanTaskCreate`를 재사용하여 phase/lane validation, DAG cycle,
  phase inversion 등 실제 create-time warning과 terminal reason 규칙을 이중 구현하지 않는다.
- preview는 새 Task public ID를 예약하지 않고 expected public ID만 보여준다.
- warning acknowledgement는 기존 exact-set binding을 그대로 사용한다.
- 신규 Task 생성은 현재 `task.create`와 마찬가지로 `dangling_path`를 발생시키지 않는다.
  `phase_order_inversion` 등 `PlanTaskCreate`가 실제로 내는 warning만 그대로 전달한다.
- `terminalReason`은 의도된 최종 leaf일 때만 받으며 임시 successor 부재나 warning
  억제를 위해 발명하지 않는다.

execute는 하나의 DB transaction에서 다음을 원자적으로 수행한다.

1. 기존 repository 규칙대로 Workspace row를 먼저 `FOR UPDATE`하고 revision을 재검증
2. active Backlog item과 counters를 그 다음 lock하고 재검증
3. pending Task insert
4. predecessor/successor dependency insert
5. Backlog item을 promoted로 전환하고 `promoted_task_id` 설정
6. 원 lane position compact
7. Task counter 증가
8. `task.created`와 `backlog.promoted` events 기록
9. Workspace revision 한 번 증가

어느 단계든 실패하면 Task, dependencies, Backlog status, counters, events가 모두 rollback되어야 한다.

`backlog.promoted` payload에는 Backlog UUID/public ID, 생성 Task UUID/public ID, lane, phase를 담는다. 자동 Gate attach 또는 Gate entry binding은 하지 않는다.

## 5. Event와 audit 계약

새 event type:

- `backlog.created`
- `backlog.updated`
- `backlog.moved`
- `backlog.reordered`
- `backlog.discarded`
- `backlog.promoted`

새 entity type은 `backlog_item`으로 통일한다.

작업:

- event audit allowlist와 payload validation 추가
- `backlog.promote`의 primary event는 `backlog.promoted`, secondary event는
  `task.created`로 고정한다.
- 기존 command당 primary event 1개 invariant를 유지하고 `secondaryAuditEvents`,
  audit importance totality, task/lane scope와 entity normalization에
  `task.created` secondary case를 명시적으로 추가한다.
- actor, command, envelope `idempotencyKey`, base/result revision 기록
- promote의 `task.created`와 `backlog.promoted`가 같은 command/audit correlation을 공유하도록 한다.
- collaboration/event read model에서 기존 event와 동일하게 노출한다.
- idempotency replay는 최초 response를 반환하고 event나 counter를 중복 생성하지 않는다.

## 6. 영속성 계획

### 6.1 Migration

현재 작업 트리에 `00007_gate_entry_tasks.sql`이 있으므로 구현 시작 시 migrations를 다시 나열한 뒤 다음 빈 번호를 사용한다. 현재 기준 후보는 `00008_lane_backlog.sql`이다.

테이블 `backlog_items`:

- PK `id`
- FK `workspace_id -> workspaces`
- FK `(workspace_id, lane_id) -> lanes`
- nullable composite FK `(workspace_id, promoted_task_id) -> tasks(workspace_id, id)`
- unique `(workspace_id, public_id)`
- partial unique `(workspace_id, promoted_task_id) WHERE promoted_task_id IS NOT NULL`
- status CHECK
- 상태별 position/promoted_task_id/discard_reason 일관성 CHECK
- `position > 0` CHECK
- active row partial unique `(workspace_id, lane_id, position) WHERE status = 'active'`
- active lane order와 promoted task lookup index

기존 Task public ID/counter와 같은 SQL/Go 타입인
`next_backlog_public_id INTEGER NOT NULL DEFAULT 1`을 추가하고 기존 row를 backfill한다.

down migration은 indexes/table/column을 역순으로 제거한다.

### 6.2 Snapshot과 projection

`server/internal/application/types.go`에 `BacklogItemProjection`과 `Snapshot.BacklogItems`, `Snapshot.NextBacklogPublicID`를 추가한다.

- command planning을 위해 active와 terminal 모두 Snapshot에 load한다.
- Workspace graph에는 active items만 싣고 Viewer payload의 무한 성장을 막는다.
- terminal provenance는 paginated backlog list/get에서 status와 promoted Task link로 제공한다.
- promoted Task public ID는 repository join 또는 projection map으로 제공한다.

### 6.3 Repository execute

기존 `MutationPlan`에 Backlog mutation fields를 명시적으로 추가한다. 범용 map으로 우회하지 않는다.

- create/update/move/reorder/discard persistence branch
- promote는 Task create branch와 Backlog transition을 같은 transaction에서 처리
- position unique 충돌을 피하기 위해 reorder/compact 시 임시 음수 offset 또는 deferrable-safe 2단계 update 사용
- execute 전 fresh snapshot/revision 검증
- preview 경로에는 DB write가 없어야 한다.

## 7. Application/domain 구현 순서

### Wave A — contract와 순수 domain

1. `contracts/v1/states.json`에 `backlogItem` values/terminal/transitions 추가
2. `contracts/v1/diagnostics.json`에
   `invalid_backlog_patch`, `backlog_lane_unchanged`,
   `backlog_order_mismatch`, `backlog_order_unchanged`,
   `invalid_backlog_filter`,
   `lane_has_active_backlog` errors와
   `workspace_close_residual_backlog` warning 추가
3. `contracts/v1/commands.json`에 6개 mutation과 `backlog.list|get` query metadata 추가
4. CLI `queryNames`와 `primaryArgument`에 list/get 추가
5. payload structs와 decode strictness 추가
6. Backlog lifecycle/validation pure functions 추가
7. mutation registry와 `MutationPlan` 확장
8. promote가 `PlanTaskCreate`를 호출하도록 composition
9. primary/secondary event audit schemas와 visibility 추가

예상 파일:

- `contracts/v1/commands.json`
- `server/internal/domain/mutation_plan.go`
- `server/internal/domain/event_audit.go`
- `server/internal/domain/*backlog*_test.go`
- `server/internal/application/types.go`
- `server/internal/application/command_service.go`

### Wave B — migration과 repository

1. migration up/down
2. Snapshot/query projection
3. Get/List queries
4. 6개 persistence branches
5. promote atomic transaction
6. counter, idempotency, rollback integration tests

예상 파일:

- `server/migrations/<next>_lane_backlog.sql`
- `server/internal/persistence/postgres/repository.go`
- `server/internal/persistence/postgres/*backlog*_test.go`

### Wave C — HTTP와 CLI

Graph DTO에 다음을 추가한다.

```json
{
  "backlogItems": [
    {
      "id": "<uuid>",
      "publicId": 8,
      "laneId": "adoption",
      "title": "...",
      "description": "...",
      "status": "active",
      "position": 1,
      "promotedTaskId": null,
      "promotedTaskPublicId": null
    }
  ]
}
```

read endpoints:

- `GET /v1/workspaces/{workspaceId}/backlog?laneId=&status=`
- `GET /v1/workspaces/{workspaceId}/backlog/{publicId}`

list는 cursor/limit pagination을 제공한다. unknown lane은 `not_found`, 지원하지 않는
status/filter/cursor는 `invalid_backlog_filter` diagnostic으로 400을 반환한다.

CLI:

- `baley backlog list|get|create|update|move|reorder|discard|promote`
- write 명령은 preview/execute envelope와 warning acknowledgement를 기존 명령처럼 지원

예상 파일:

- `server/internal/transport/httpapi/router.go`
- `server/internal/transport/httpapi/*backlog*_test.go`
- `server/internal/cli/model.go`
- `server/cmd/baley-cli/*`

### Wave D — typed MCP

read tools:

- `baley_backlog_list`
- `baley_backlog_get`

write tools:

- `baley_backlog_create_preview|execute`
- `baley_backlog_update_preview|execute`
- `baley_backlog_move_preview|execute`
- `baley_backlog_reorder_preview|execute`
- `baley_backlog_discard_preview|execute`
- `baley_backlog_promote_preview|execute`

요구사항:

- execute schemas에 공통 envelope의 `idempotencyKey`,
  `expectedWorkspaceRevision`, `acknowledgedWarningCodes`를 정확히 노출
- promote execute에 `acknowledgedWarningCodes`를 반드시 노출
- preview 응답의 semantic summary가 B# ID, lane, target Phase, 생성 Task 예상 ID, dependencies, terminal reason, warnings를 보여준다.
- adapter는 HTTP command 계약을 임의 변형하지 않는다.
- MCP stdio schema snapshot/E2E가 실제 tool exposure를 검증한다.
- schema 변경 후 이미 열린 Codex thread에서는 reload되지 않을 수 있음을 handoff/report에 명시한다.

예상 파일:

- `server/cmd/baley-mcp/main.go`
- `server/cmd/baley-mcp/*backlog*_test.go`
- 기존 `server/integration/mcp_test.go`의 `BALEY_MCP_E2E` harness

### Wave E — Viewer live-data 연결

현재 untracked UI prototype을 폐기하지 않고 live model로 전환하되, production
완료 시 `src/experiments` 밖의 backlog component 모듈로 승격한다.

1. frontend domain에 `BacklogItem` 추가
2. `GraphDTO.backlogItems` parse/map
3. `MOCK_ITEMS`, `8 MOCK`, `UI PROTOTYPE`, TEMP 명칭·주석·CSS marker 제거
4. 각 lane rail은 active items를 position 순으로 사용
5. compact rail은 처음 2개와 `+N MORE`를 표시
6. 확대 버튼은 전체 Backlog list로 전환
7. list는 lane별 그룹과 palette cycle을 유지
8. B# public ID와 `PHASE 미정`을 표시
9. 빈 lane은 조용한 empty state를 제공하되 lane anchor column 폭과 정렬은 유지
10. 4개보다 많은 lane에서도 palette 순환과 행 구분으로 식별 가능해야 한다.

중요:

- palette와 lane number는 presentation이다. server에 저장하지 않는다.
- 현재 `[backlog][lane anchor][phase...]` canvas geometry, ReactFlow mount, viewport restore, focus/inert 처리, Escape/back 동작을 회귀시키지 않는다.
- promoted/discarded 항목은 active rail에서 제외한다. provenance는 API/get에서 유지한다.
- Viewer는 read-only다. create/promote 버튼을 이번 slice에 넣지 않는다.

예상 파일:

- `src/domain/model.ts`
- `src/api/client.ts`
- `src/experiments/BacklogRail.tsx`
- `src/experiments/BacklogList.tsx`
- `src/experiments/LaneAnchorColumn.tsx`
- 관련 CSS/config/tests

### Wave F — Operator와 문서 정합화

다음 문서를 실제 schema와 동일하게 갱신한다.

- `.agents/skills/baley-manage-work/SKILL.md`
- `.agents/skills/baley-manage-work/references/commands.md`
- `docs/baley-command-architecture.md`
- `docs/baley-system-spec-v1.md`
- `docs/baley-lane-backlog.md`
- roadmap/task manifest의 #121 evidence

Operator 규칙:

- “Backlog에 넣어줘”는 Phase를 묻거나 추론하지 않고 lane만 확인한다.
- lane도 알 수 없을 때만 사용자에게 묻는다.
- “Task로 올려줘”는 target Phase와 dependency/successor intent가 없으면 확인한다.
- promote preview에 warning이 있으면 기존 exact warning acknowledgement 절차를 따른다.
- 승격과 Gate attach는 별도 명령·별도 승인 경계다.
- grouped confirmation UX와 서버 exact binding 계약을 혼동하지 않는다.

## 8. 기존 dirty worktree와 통합 규칙

현재 작업 트리에는 다음 미커밋 변경이 이미 있다.

- outcome-first/grouped approval 관련 skill/docs/contract/server 변경
- gate entry Task 관련 migration/server/tests
- lane Backlog rail/list/anchor UI prototype
- adoption manifest와 여러 Task Record

구현 세션은 이를 사용자 소유 변경으로 취급한다.

- reset, checkout, clean, 광범위한 formatter를 사용하지 않는다.
- 파일별 diff를 먼저 읽고 기존 의도를 보존한다.
- migration 번호와 command registry를 fresh 확인한다.
- 충돌 시 현재 변경을 삭제하고 재작성하지 말고 최소 patch로 통합한다.
- Task #121 완료 보고 전 grouped approval, phase-targeted Task create, backlog promote 세 결과를 모두 검증한다.

## 9. 테스트와 증거

### Domain/contract

- 각 lifecycle transition과 terminal immutability
- title/reason/position validation
- position compact/reorder invariant의 property-style test와 positive CHECK
- empty lane의 `[]` reorder가 `backlog_order_unchanged`인지 검증
- command literal/metadata/schema assertion
- event payload allowlist와 cross reference
- promote가 Task create warning/dependency 규칙을 그대로 사용

### PostgreSQL/integration

- migration up/down 및 constraints
- independent counters
- concurrent backlog create public ID uniqueness
- 같은 revision으로 시작한 concurrent create는 workspace-first lock에 따라 하나만
  성공하고 다른 하나는 `stale_revision`이어야 하며, loser가 fresh preview/retry하면
  다음 public ID로 성공해야 함
- 모든 Backlog mutation의 lock order가 workspace → backlog row/counter임을 검증
- move/reorder/compact correctness
- preview write-free
- execute revision +1
- stale revision rejection
- idempotency replay
- promote success와 rollback injection
- promoted/discarded 재변경 거부
- active backlog가 있는 lane 종료 거부와 workspace close residual warning
- dependency cycle/phase inversion과 실제 create-time warning
- promote가 Gate를 자동 변경하지 않음

### HTTP/CLI/MCP

- list/get lane/status filters
- graph includes active only; paginated list/get includes terminal status와 promoted Task link
- all typed tool schemas
- promote warning acknowledgement field
- stdio E2E preview/execute
- invalid UUID/public ID/unknown lane/phase/error mapping

### Viewer

- DTO mapping
- active-only lane grouping과 position ordering
- 0, 1, 2, 3+ items
- 5+ lanes palette cycle
- B# display와 mock marker 제거
- expand/back/Escape/focus/inert
- graph refresh 후 promoted item 제거
- lane anchor/phase canvas alignment snapshot 또는 DOM geometry assertion

### 필수 실행

환경에 맞춰 정확한 repo script를 우선 확인한 뒤 최소한 다음을 통과시킨다.

```text
gofmt on touched Go files
go test -count=1 ./...
go vet ./...
PostgreSQL migration/integration tests with configured test DB
MCP stdio E2E without a silent skip
npm test
npm run build
Baley skill validator
approval/backlog contract assertions
git diff --check
```

race test가 Windows/CGO 제약으로 실행 불가하면 일반 테스트 성공과 정확한 제약을 보고한다. E2E가 환경 누락으로 skip되면 통과로 간주하지 말고 별도 evidence gap으로 남긴다.

## 10. 독립 리뷰 기준

독립 Agent는 raw diff와 test output을 직접 확인한다.

Blocking finding:

- Backlog에 Phase가 저장됨
- promote가 Task create 규칙을 우회함
- preview가 write함
- promote가 부분 commit됨
- warning acknowledgement field가 typed schema에 없음
- active ordering/unique constraint가 동시성에서 깨짐
- mock 데이터가 production path에 남음
- 기존 dirty change를 유실함
- #121의 grouped approval 또는 phase-targeted Task path를 검증하지 않음

Non-blocking finding:

- 명명/문서 표현 개선
- 이번 slice 비범위의 UI 편의 기능

모든 blocking finding을 수정하고 재검증한 뒤에만 완료 보고한다.

## 11. 완료·보고 절차

1. implementation Run을 Task #121에 연결한다.
2. 구현 중 lease heartbeat를 유지한다.
3. 테스트와 독립 리뷰 증거를 Task Record completion report에 기록한다.
4. repository/working-tree evidence를 register한다.
5. implementation/review/reporting Runs를 terminal 처리한다.
6. #121의 세 결과가 모두 충족되면 `task.report_implemented`를 실행한다.
7. 사용자에게 구현 결과, 테스트, 독립 리뷰를 outcome-first로 요약하고 “완료로 확인할까요?”라고 한 번 묻는다.
8. 새 구현 세션은 `task.confirm`을 실행하지 않는다. 사람의 답변을 기다린다.

## 12. Acceptance criteria

- lane별 Backlog item을 Phase 없이 typed command로 생성·수정·이동·정렬·discard할 수 있다.
- Backlog public ID는 B#로 식별되고 Task #와 counter가 독립적이다.
- promote가 명시된 Phase에 pending Task를 원자적으로 만들며 기존 Task dependency/topology 계약을 재사용한다.
- promote 실패 시 어떤 부분 상태도 남지 않는다.
- Gate는 promote로 자동 변경되지 않는다.
- MCP read/write tools와 CLI/HTTP/contracts가 모두 같은 필드를 노출한다.
- Viewer가 live backlog data를 표시하며 mock data/labels가 없다.
- 5개 이상의 lane에서도 rail/anchor/palette가 안정적이다.
- grouped approval, phase-targeted Task create, lane backlog promotion이 함께 회귀 검증된다.
- 전체 테스트와 독립 리뷰에 unresolved blocking finding이 없다.
- Task #121은 `implemented`이며 사람 confirmation만 pending이다.
