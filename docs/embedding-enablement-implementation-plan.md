---
type: phase-implementation-plan
status: ready-for-implementation
workspace_id: "00000000-0000-4000-8000-000000000001"
phase_id: embedding-enablement
workspace_revision_observed: 277
source_tasks: [121, 122, 123, 124, 129, 130, 131]
approval_authority_superseded_by: docs/task-acceptance-policy-contract.md
---

> Historical implementation plan: delegated acceptance and auto-confirm items
> are superseded by the P0 human-only approval contract. They are retained only
> as planning history and must not be implemented or used as authority.

# Embedding Enablement — Phase 4 상세 구현 계획

## 1. Phase 목표

Phase 4는 Phase 3에서 확정한 계약을 실제 Baley 실행 경로로 만드는 단계다. 완료 시에는 다음이
가능해야 한다.

- Operator가 Task 생성·공동 승인·lane Backlog 승격을 typed 경로로 운영한다.
- Gate condition과 다음 Phase에서 열리는 Task가 별도 관계로 표현된다.
- Gate를 안정적인 `gateId`와 사람이 읽기 쉬운 `G#<publicId>`로 함께 참조한다.
- Task Record·Git·Run 불일치를 탐지하고 lane brief로 작업을 복원한다.
- 미리 위임된 기술 Task는 typed evidence가 충분하면 자동 확인되고, 사람 판단 Task와
  Gate·Lane·Workspace 권한은 자동화되지 않는다.
- 이 기능들을 재현 가능한 Pilot 운영 키트와 격리 E2E 증거로 검증한다.

Phase 완료는 `embedding-pilot-entry` Gate를 자동 통과시키지 않는다. #124 확인으로 Gate가
ready가 된 뒤 별도의 fresh preview와 사람 승인이 필요하다.

## 2. 현재 Task와 human-confirm 경계

현재 Task는 모두 #130 배포 전에 생성됐다. #118 계약은 기존 Task migration을
`human_required`로 backfill하도록 정했으므로, 아래 일곱 Task는 구현·테스트·독립 리뷰가
통과해도 현재 Task 자체의 `implemented -> confirmed`에 사람 확인이 필요하다. #130은 이를
소급해 delegated로 완화하지 않는다.

| Task | 현재 상태 @ r277 | 간단한 결과 | human-confirm |
| --- | --- | --- | --- |
| #121 | in_progress | 공동 승인, phase-targeted Task 생성, lane Backlog와 승격 경로를 마감하고 운영 서버의 mutation audit 활성화를 재확인한다. | 필요 |
| #122 | pending | Task Record·Git·Run의 정본 경계를 지키며 불일치를 분류하고 lane brief 복원 경로를 완성한다. | 필요 |
| #129 | in_progress | Gate 조건과 별개인 to-Phase entry/unlock Task binding을 저장·조회·Viewer에 투영한다. | 필요 |
| #130 | pending | delegated/human_required 정책, typed evidence, auto-confirm Event와 전 transport projection을 구현한다. | 필요 |
| #131 | pending | 내부 `gateId`는 유지하면서 Workspace 내 비재사용 `G#<publicId>`와 선택적 별칭을 제공한다. | 필요 |
| #123 | pending | 위 기능을 새 Pilot workspace에서 반복 실행할 Skill·runbook·template으로 묶는다. | 필요 |
| #124 | pending | 격리 E2E, 독립 리뷰, 잔여 위험을 기록해 Phase 수용 증거를 만든다. | 필요 |

Task 확인 외에도 다음은 별도 사람 경계다.

- `task.acceptance_policy.change` 및 delegated → human_required escalation
- active Gate condition 변경, Gate Task pass/revoke, Gate pass
- Task discard, Lane close-out/discard, Workspace close

## 3. 실행 topology와 확인 지점

```text
                         #122 --------\
#120 -> #121 -----------> #123 -> #124 --[embedding-pilot-entry]--> #125
          |               /
          v              /
        #129 -> #131 ----/
                         \
               #130 -----/
```

실제 live graph의 직접 predecessor는 다음과 같다.

- #121, #122, #130은 #120 이후 시작한다.
- #129는 #121 이후, #131은 #129 이후다.
- #123은 #121, #122, #130, #131 이후다.
- #124는 #123 이후이며 `embedding-pilot-entry`의 조건이다.

현재 V1 dependency와 기존 Task의 `human_required` backfill 때문에 한 세션에서 코드를
연속 구현하더라도 사람 결정 지점은 남는다.

1. Wave A의 #121·#122·#129·#130을 함께 구현·검증·리뷰한 뒤 grouped Task confirmation
2. #131 구현·검증·리뷰 후 Task confirmation
3. #123 운영 키트 검증 후 Task confirmation
4. #124 E2E 수용 검증 후 Task confirmation
5. ready가 된 `embedding-pilot-entry` Gate pass

각 grouped confirmation은 같은 baseline revision의 write-free preview로 판단 brief를 만들고,
승인 후 각 Task를 fresh-preview → execute 순서로 처리한다. 서로 다른 action은 같은 승인
질문에 섞지 않는다.

## 4. 공통 착수 절차

1. Baley workspace, active Phase, outgoing Gate, #121/#122/#123/#124/#129/#130/#131과
   active/nonterminal Run을 fresh-read한다.
2. `git status --short`와 관련 diff를 보존한다. 현재 worktree는 #121과 #129의 기존 구현을
   포함해 매우 dirty하므로 reset, checkout, 덮어쓰기, 무관한 정리는 금지한다.
3. Task별로 새 Run을 시작하기 전에 이미 존재하는 Run·Record·completion evidence를 확인한다.
4. 기준 검증을 실행해 새 실패와 기존 실패를 구분한다.
   - `go test ./...`
   - `go vet ./...`
   - `npm test -- --run`
   - `npm run build`
   - `git diff --check`
5. PostgreSQL integration test는 격리 DB에서 실행한다. 운영 Workspace나 사용 중인 DB를
   destructive fixture로 사용하지 않는다.
6. implementation, independent review, review response, completion reporting마다 Task별 Run과
   Record를 남긴다.

## 5. Wave A — 기반 경로 병렬 완성

### 5.1 #121 Operator 승인 및 Task intake

기존 구현을 새로 만들지 말고 현재 completion evidence와 live runtime을 대조한다.

- `task-records/lane-backlog-vertical-slice/completion-report-02.md`의 결과와 live Task의
  오래된 `nextAction`을 비교한다.
- 운영 `GET /healthz`와
  `GET /v1/workspaces/{workspaceId}/mutation-attempts`를 확인한다.
- harmless 실제 write 한 건이 `source=command_service`로 감사되고, 직접 Task write가
  `source=database_trigger`로 구분되는지 검증한다.
- 공동 confirmation의 revision chain, phase-targeted `task.create`, phase-free BacklogItem,
  explicit-phase promotion의 contract/domain/HTTP/CLI/MCP/Viewer 회귀 테스트를 실행한다.
- stale `currentSummary`/`nextAction`을 그대로 완료 근거로 사용하지 말고 fresh assessment에
  실제 운영 활성화 상태를 적는다.
- 독립 리뷰 후 새 completion report가 필요하면 기존 report를 supersede하고
  `task.report_implemented`를 실행한다.

### 5.2 #122 Lane brief 및 증거 복원

기존 `server/internal/domain/lane_brief.go`를 재사용하고 transport와 mismatch 경로를 완성한다.

- application query가 active/nonterminal Run을 terminal history보다 우선한다.
- Task/Gate/Phase/Run은 Baley DB, Task Record 원문은 repository file, commit/blob/diff는 Git,
  worktree와 Agent summary는 비정본 observation으로 분리한다.
- record path/hash, commit reference와 Git observation을
  `aligned | stale | missing | unverified | reporting_pending`으로 분류한다.
- 복원은 read-only다. Task/Gate/Run 상태를 추정 mutation하지 않고 next-action 후보와 근거만
  반환한다.
- `lane.brief`를 application, PostgreSQL snapshot, HTTP, CLI, typed MCP에 연결한다.
- foreign workspace/repository evidence, stale source, active review/reporting Run, confirmed Task
  mismatch, disconnected DAG와 Gate participation을 테스트한다.

### 5.3 #129 Gate entry and unlock bindings

이미 존재하는 migration `00007_gate_entry_tasks.sql`, domain/application/MCP/Viewer 변경을
먼저 diff-review하고 누락만 보완한다.

- condition 관계는 from-Phase Task만, entry 관계는 to-Phase Task만 허용한다.
- entry binding은 Task를 자동 시작하거나 dependency/Gate readiness를 바꾸지 않는다.
- explicit binding이 없으면 to-Phase DAG root를 public ID 순으로 deterministic하게 투영한다.
- Gate pass snapshot/hash에는 resolved entry set과 explicit/automatic source를 포함한다.
- HTTP/CLI/MCP attach/detach와 Viewer의 Gate → Task `unlocks` 방향을 검증한다.
- `task-records/gate-entry-unlocks/detailed-plan-01.md`의 잘못된 `task_id: 130`을 #129로
  바로잡고 정식 Record metadata를 채운 뒤 등록한다.

### 5.4 #130 위임형 Task acceptance 정책

`docs/task-acceptance-policy-contract.md`를 실행 가능한 정본으로 사용한다.

- migration:
  - Task acceptance assignment와 EvidenceProfile version
  - append-only TaskAcceptanceEvidence
  - 기존 Task는 `human_required`로 backfill
- command/domain:
  - create/promote 시 requested mode를 resolve하고 assignment를 atomic하게 고정
  - `task.evidence.report`
  - human-approved `task.acceptance_policy.change`
  - human-approved monotonic `task.acceptance_mode.escalate`
- transaction:
  - implemented delegated Task의 current profile을 최신 valid evidence로 평가
  - 최초 eligible evaluation에서 Task confirm과 `task.auto_confirmed` Event를 한 transaction으로
    생성
  - evidence 부족/review fail은 implemented에 유지하고 typed reason 제공
- projection:
  - HTTP, CLI, typed MCP, Viewer에 mode, policy version, evidence verdict와 auto-confirm audit 표시
- authority regression:
  - Gate, Gate Task, Lane, Workspace, discard, active Gate condition은 자동 확인 대상이 아님
  - 기존 `task.confirm` exact hash/revision/warning attestation은 그대로 유지

### 5.5 Wave A 검증과 독립 리뷰

- 각 Task의 focused test를 먼저 실행하고 전체 Go/React/build 검증을 다시 실행한다.
- #121/#122/#129/#130을 서로 다른 관점으로 독립 리뷰한다.
  - domain invariant와 transaction
  - migration/backfill/rollback과 운영 DB 안전
  - HTTP/CLI/MCP contract parity
  - Viewer projection과 권한 표현
- blocking finding은 review-response Run에서 수정하고 재검토한다.
- 네 Task가 모두 implemented가 된 뒤 한 grouped human decision brief를 만든다.

## 6. Wave B — #131 Gate 공개 번호와 별칭

#129가 confirmed된 fresh graph에서 시작한다.

- `gates`에 Workspace-scoped positive public ID와 optional alias를 추가한다.
- 기존 Gate를 deterministic하게 backfill하고 삭제/종료 후 번호를 재사용하지 않는다.
- 내부 관계, Event entity ID, pass/attach command의 안정 식별자는 기존 `gateId`를 유지한다.
- 조회·목록·CLI parser·typed MCP·Viewer는 `G#<publicId>`, alias, `gateId`를 함께 제공한다.
- duplicate number/alias, cross-workspace lookup, migration retry, backfill order, legacy gateId
  호출 호환성을 테스트한다.
- 독립 리뷰 후 implemented 보고와 사람 confirmation을 받는다.

## 7. Wave C — #123 Adoption Pilot 운영 키트

#121/#122/#130/#131과 Account-bound Workspace access Tasks
#132/#133/#134/#135가 confirmed된 뒤 시작한다.

- 새 workspace bootstrap 절차, `baley.yaml`, Task Record template, Skill 사용 순서와 server
  activation check를 하나의 runbook으로 묶는다.
- Run interruption/lease expiry 복원, lane brief read-only recovery, Git/Record mismatch,
  Backlog 승격, human-required/delegated Task, Gate approval을 재현하는 스크립트·template을 만든다.
- `PilotMeasurement` template에 session/sample, candidate verdict, evidence ref, mismatch key,
  correction Event, conversation turn과 baseline/treatment를 포함한다.
- clean temporary workspace/DB에서 runbook을 처음부터 실행해 누락된 암묵적 전제와 secret
  노출이 없는지 검증한다.
- 독립 리뷰 후 implemented 보고와 사람 confirmation을 받는다.

## 8. Wave D — #124 Phase 수용 검증

#123 confirmed 후 격리 single-repository 시나리오를 실행한다.

1. workspace/lane/phase/Gate bootstrap
2. lane Backlog 생성·정렬·승격과 phase-targeted Task 생성
3. Gate condition과 entry/unlock Task의 분리 및 `G#` 조회
4. Run 중단과 fresh session 복원
5. Record/Git mismatch 주입과 read-only lane brief 분류
6. 새 delegated 기술 Task의 typed-evidence auto-confirm
7. human_required Task와 Gate/Lane/Workspace 권한의 비자동화
8. mutation-attempt audit와 secret redaction
9. PilotMeasurement Record 생성

E2E 결과에는 명령/Event/Run/Record/Git reference, 예상·실제 상태, 잔여 위험과 rollback 또는
후속 Task 제안을 남긴다. 독립 Agent가 전체 Phase contract와 authority regression을 검토하고
blocking finding이 0일 때만 #124를 implemented로 보고한다.

#124 confirmation 뒤 `embedding-pilot-entry`가 ready인지 fresh-read한다. Gate pass는 별도의
outcome-first decision brief와 사람 승인을 받아야 하며, 구현 세션이 임의로 실행하지 않는다.

## 9. 완료 기준

- #121, #122, #129, #130, #131, #123, #124가 모두 confirmed
- Task별 plan/review/review-response/completion evidence가 Baley index와 repository에 정합
- 전체 Go test/vet, React test/build, diff check 통과
- migration이 fresh DB와 업그레이드 DB 모두에서 검증되고 운영 DB destructive test가 없음
- HTTP/CLI/MCP/Viewer가 같은 contract와 authority boundary를 표현
- `embedding-pilot-entry`가 ready이며 아직 사람 결정 전에는 passed가 아님
