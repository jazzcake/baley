---
type: implementation-session-handoff
status: ready
workspace_id: "00000000-0000-4000-8000-000000000001"
phase_id: embedding-enablement
source_plan: docs/embedding-enablement-implementation-plan.md
---

# Phase 4 구현 세션 Handoff Prompt

아래 내용을 새 구현 세션의 첫 메시지로 사용한다.

---

`D:\Project_AI\baley`의 Baley Phase 4, `embedding-enablement`를 끝까지 구현하세요.

먼저 다음 파일을 전부 읽고 정본으로 사용하세요.

- `D:\Project_AI\baley\docs\embedding-enablement-implementation-plan.md`
- `D:\Project_AI\baley\docs\embedding-enablement-entry-checklist.md`
- `D:\Project_AI\baley\docs\task-acceptance-policy-contract.md`
- `D:\Project_AI\baley\docs\evidence-recovery-pilot-runbook.md`
- `D:\Project_AI\baley\docs\baley-adoption-task-manifest.md`
- `D:\Project_AI\baley\.agents\skills\baley-manage-work\SKILL.md`

목표 Task는 #121, #122, #129, #130, #131, #123, #124입니다. 시작 시 live Baley
Workspace를 fresh-read하고 revision, active Phase, outgoing Gate, dependency, active Run,
Task Record를 기준으로 계획의 관찰값을 갱신하세요. fixture나 문서의 오래된 revision을 live
상태보다 우선하지 마세요.

중요한 현재 상태:

- Workspace: Baley Pilot
- Workspace ID: `00000000-0000-4000-8000-000000000001`
- 관찰 revision: 277
- active Phase: `embedding-enablement`
- #121과 #129는 in_progress이며 working tree에 상당한 기존 구현이 있습니다.
- #122, #123, #124, #130, #131은 pending입니다.
- #124는 `embedding-pilot-entry`의 Gate condition입니다.
- worktree는 매우 dirty합니다. 기존 변경은 사용자 작업이므로 reset, checkout, 삭제,
  광범위 formatting을 하지 말고 diff를 보존해 이어서 구현하세요.

실행 순서:

1. Wave A에서 #121, #122, #129, #130을 완성합니다.
2. 네 Task의 테스트·빌드·독립 리뷰·리뷰 반영·completion report를 끝낸 뒤 implemented로
   보고합니다.
3. 네 Task를 한 outcome-first grouped decision brief로 사람에게 확인받고, 승인 후 각 Task를
   fresh-preview → execute 순서로 confirm합니다.
4. #129 confirmation으로 열린 #131을 구현·검증·리뷰·보고하고 사람 확인을 받습니다.
5. 모든 predecessor confirmation 후 #123 운영 키트를 구현·실행 검증·리뷰하고 사람 확인을
   받습니다.
6. #123 confirmation 후 #124 격리 E2E를 실행하고 독립 리뷰·잔여 위험·completion evidence를
   남긴 뒤 사람 확인을 받습니다.
7. `embedding-pilot-entry`가 ready인지 fresh-read하고, Gate passage는 별도 decision brief로
   사람에게 요청합니다. 명시적 승인 전에는 통과시키지 마세요.

모든 현재 Phase 4 Task는 #130 이전에 생성됐으며 계약상 existing-task migration에서
`human_required`입니다. #130 구현이 이 Task들을 소급해 delegated로 바꾸면 안 됩니다.
Task confirmation, Gate pass, Gate Task pass/revoke, discard, Lane/Workspace 종료, active Gate
condition 변경을 Agent 권한으로 우회하지 마세요.

Task별 필수 사항:

- #121: 기존 completion-report-02와 live runtime을 대조하고 오래된 nextAction을 신뢰하지
  마세요. health, mutation-attempt endpoint, command_service/database_trigger audit를
  재확인하세요.
- #122: 기존 `server/internal/domain/lane_brief.go`를 재사용해 application/PostgreSQL/
  HTTP/CLI/typed MCP를 연결하고, active Run 우선·read-only recovery·정본 분리·mismatch
  classification을 구현하세요.
- #129: migration 00007과 기존 domain/MCP/Viewer diff를 검토해 누락만 보완하세요.
  `task-records/gate-entry-unlocks/detailed-plan-01.md`의 잘못된 `task_id: 130`은 #129로
  고치고 올바른 Run/Record metadata로 등록하세요.
- #130: assignment/evidence schema, existing human_required backfill, create/promote resolve,
  evidence report, monotonic escalation, atomic auto-confirm Event, HTTP/CLI/MCP/Viewer와
  authority regression을 구현하세요.
- #131: gateId는 내부 안정 식별자로 유지하고 Workspace 내 비재사용 public ID와 optional
  alias를 migration/backfill한 뒤 `G#<publicId>`를 모든 조회·Viewer에 투영하세요.
- #123: clean environment에서 실행 가능한 bootstrap/recovery/evidence/Gate runbook과
  PilotMeasurement template을 만드세요.
- #124: 전체 기능, recovery, evidence, delegated auto-confirm, human-only authority와 audit
  redaction을 격리 E2E로 검증하세요.

각 detailed planning, implementation, independent review, review response, completion reporting
전에 Task별 Baley Run을 시작하고 terminal 상태로 닫으세요. 각 Record는 하나의 UUID를
front matter와 MCP 등록에 동일하게 사용하세요. 이미 실행 중인 Run이나 존재하는 Record를
중복 생성하지 마세요.

검증 기본선:

- `go test ./...`
- `go vet ./...`
- `npm test -- --run`
- `npm run build`
- `git diff --check`
- focused PostgreSQL integration tests on an isolated DB

UI/React 문제가 생기면 사용자 event, 계산된 target state, React/store state, controller state,
rendered DOM state를 먼저 계측해 최초 divergence를 찾으세요. 추측성 dependency 변경이나
반복 CSS 조정부터 하지 마세요.

새 #130/#131 typed MCP tool을 서버에 추가한 뒤 현재 thread의 MCP schema에 보이지 않으면
서버 restart와 build 상태를 확인하세요. 그래도 schema가 reload되지 않을 때에만 durable
handoff를 갱신하고 사용자에게 새 thread를 요청하세요. privileged port 8080 프로세스 교체가
Access Denied로 막힐 때도 정확한 상태와 필요한 사람 동작만 요청하세요.

Task를 임의로 추가하거나 dependency/Gate condition을 바꾸지 마세요. 계획과 live graph가
충돌하면 안전한 read-only 진단을 모두 수행한 뒤 PM 판단이 필요한 차이만 사용자에게
보고하세요. 독립 Agent의 blocking finding은 수정·재검토하고, blocking finding이 남은
Task를 implemented/confirmed로 처리하지 마세요.

최종 보고에는 Task별 상태, 테스트·빌드, 독립 리뷰, Event/Record ID, 남은 risk,
Workspace revision과 다음 human decision을 간단히 정리하세요.

---
