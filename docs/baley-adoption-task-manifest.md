---
type: task-manifest
status: pm-approved-baseline
workspace_id: "00000000-0000-4000-8000-000000000001"
workspace_revision_observed: 182
last_validated_revision: 591
last_validated_at: "2026-08-03"
lane_id: adoption
phases:
  - embedding-contract
  - embedding-enablement
  - embedding-pilot
source_roadmap: docs/baley-roadmap.md
approved_at: "2026-07-22"
---

# Baley Adoption Task Manifest

## 1. Authority and scope

This manifest is the PM baseline for creating the work under the Adoption lane and the three Embedding phases. It maps the Operator workflow, collaboration approval boundary, lane-scoped Backlog intake, and Day Tripper pilot acceptance criteria onto `embedding-contract`, `embedding-enablement`, and `embedding-pilot`.

Repository scope does not determine Lane ownership. All Tasks use `laneId: adoption` so that the adoption outcome remains visible as one lane even when implementation changes server, CLI, Skill, or documentation code.

Multi-repository support is explicitly outside this Adoption slice. Existing single-repository CommitReference, Task Record, and Git observation behavior may be reused, but cross-repository orchestration and acceptance criteria are deferred to a later manifest.

The stable manifest keys below are planning identifiers. Baley public IDs are assigned by `task.create` and must be written back into this document after execution.

## 2. Entry condition from Validate

The condition for `embedding-contract-entry` is the existing Validate-phase Task **#116, 구조적 Baley 객체 typed MCP 도구**.

Rationale:

- Task #116 is in `validate`, which matches the Gate's `fromPhaseId`.
- It implements and verifies the typed structural MCP tools required to build and operate the Adoption structure.
- Task #116 already depends on the runtime-contract work in Task #111, so attaching both #111 and #116 would duplicate the effective prerequisite.
- It is not an intentional terminal Task. Once attached to the Gate, its current `dangling_path` warning should disappear on a fresh confirmation preview.

Required ordering:

1. In a thread where `gate.attach_task` is loaded, fresh-read the Workspace.
2. Preview attaching Task #116 to `embedding-contract-entry`.
3. Obtain explicit human approval for that exact preview and execute it.
4. Fresh-read and fresh-preview Task #116 confirmation.
5. Obtain explicit human approval and confirm Task #116.

## 3. Full Task manifest

| Key | phaseId | laneId | Title | Description | Predecessor intent | Successor intent / terminalReason | Gate condition |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | `embedding-contract` | `adoption` | Embedding 범위 및 성공 기준 계약 | Roadmap Phase 3~5를 Embedding 세 Phase에 대응시키고 제품 경계, 비목표, 승인 경계, 각 Gate의 검증 가능한 exit criteria를 하나의 계약으로 고정한다. | Task #116 | AC-02, AC-03 | No |
| AC-02 | `embedding-contract` | `adoption` | Operator 승인 및 Task intake 계약 | Outcome-first 단일·공동 승인, 순차 fresh-preview 실행, 특정 Phase의 Task 생성, lane별 BacklogItem과 정식 Task 승격 계약 및 capability 경계를 정의한다. | AC-01 | AC-04 | No |
| AC-03 | `embedding-contract` | `adoption` | 증거·복원 및 Pilot 측정 계약 | 단일 repository의 Task Record 원문/인덱스, commit evidence, 비권위적 worktree 관찰, lane brief 복원 규칙과 Pilot 측정 지표·수집 방법을 정의한다. | AC-01 | AC-04 | No |
| AC-04 | `embedding-contract` | `adoption` | Embedding Contract 기준선 리뷰 | AC-02와 AC-03을 통합하고 독립 Agent 리뷰를 거쳐 모순 없는 계약 기준선과 Enablement acceptance checklist를 확정한다. | AC-02, AC-03 | AE-01, AE-02 | Attach to `embedding-enablement-entry` |
| AE-01 | `embedding-enablement` | `adoption` | Operator 승인 및 Task intake 경로 구현 | UI 없이 outcome-first 공동 승인을 순차 처리하고, 특정 Phase에 Task를 생성하며, lane별 BacklogItem을 dependency와 terminal 의도를 포함한 정식 pending Task로 승격하는 typed MCP/CLI 경로를 완성한다. | AC-04 | AE-03 | No |
| AE-01C | `embedding-enablement` | `adoption` | Gate entry and unlock bindings | Gate condition과 별도로 to-Phase entry/unlock Task binding을 모델링하고 Viewer에 다음 작업 후보를 투영한다. 기존 graph에 등록된 추가 Enablement Task이며, #120에서 AE-03과의 successor 관계를 명시적으로 재검토한다. | AE-01 | successor topology review in AC-04 | No |
| AE-01D | `embedding-enablement` | `adoption` | Gate 공개 번호 및 별칭 구현 | Gate의 내부 `gateId`를 유지한 채 사람이 지시·참조할 안정적인 `G#<publicId>`와 선택적 별칭을 HTTP/CLI/MCP/Viewer에 제공하고 기존 Gate를 backfill한다. | AE-01C | AE-03 | No |
| AE-02 | `embedding-enablement` | `adoption` | Lane brief 및 증거 복원 경로 구현 | 상세계획, handoff, 독립 리뷰, 리뷰 반영, 완료보고 인덱스와 repository 불일치 탐지를 연결하고 며칠 뒤 lane brief만으로 맥락을 복원하는 경로를 구현한다. | AC-04 | AE-03 | No |
| AE-01B | `embedding-enablement` | `adoption` | 위임형 Task acceptance 정책 구현 | delegated/human_required/inherit mode, 생성 시 policy 동결, typed evidence verdict, auto-confirm audit Event와 projections를 구현한다. Gate·Lane·Workspace 승인 경계는 바꾸지 않는다. | AC-04 | AE-03 | No |
| AE-AUTH-01 | `embedding-enablement` | `adoption` | 로컬 계정 및 Session 인증 | ID+암호 기반 Account, Argon2id credential, Owner bootstrap, rate limit, hash-only Session과 CSRF 경계를 구현한다. | AE-01B | AE-03 | No |
| AE-AUTH-02 | `embedding-enablement` | `adoption` | Workspace membership 및 권한 강제 | Owner/참여자 membership, viewer/operator/approver/owner capability, last-Owner 보호와 모든 Workspace read/write의 default-deny 권한을 구현한다. | AE-01B | AE-03 | No |
| AE-AUTH-03 | `embedding-enablement` | `adoption` | 연결된 사람 승인 결속 및 Agent token | Workspace-scoped Agent token을 연결한 사람과 채팅 승인을 fresh preview에 결속하고, 별도 승인 token 없이 현재 membership/capability를 재검증한다. | AE-01B | AE-03 | No |
| AE-AUTH-04 | `embedding-enablement` | `adoption` | 계정 기반 Workspace 선택 및 멤버 관리 UI | 소속 Workspace 선택, Owner/참여자 표시, 멤버 관리, 전환 상태 격리와 권한별 Viewer UI를 구현한다. | AE-01B | AE-03 | No |
| AE-03 | `embedding-enablement` | `adoption` | Adoption Pilot 운영 키트 | 실제 pilot workspace를 안전하게 bootstrap할 Skill/runbook/template을 만들고 Task 생성, Record/Git 연결, 중단 복구, Gate 승인 경계를 재현 가능하게 만든다. | AE-01, AE-02, AE-01B | AE-04 | No |
| AE-04 | `embedding-enablement` | `adoption` | Embedding Enablement 수용 검증 | 격리된 단일-repository 시나리오에서 AC-04 checklist 전체를 E2E 검증하고 독립 Agent 리뷰와 잔여 위험을 완료 증거로 남긴다. | AE-03 | AP-01 | Attach to `embedding-pilot-entry` |
| AP-01 | `embedding-pilot` | `adoption` | Day Tripper Pilot 온보딩 | Day Tripper의 대표 lane 일부, lane별 Backlog, 특정 Phase Task와 실제 공유 Gate를 등록하고 기존 기록과 중복되지 않는 pilot baseline을 만든다. | AE-04 | AP-02 | No |
| AP-02 | `embedding-pilot` | `adoption` | 실사용 중단·복원 및 Gate 주기 실행 | 실제 작업을 Baley로 운영하면서 Run 중단과 수일 뒤 lane brief 복원, 단일-repository evidence 연결, Backlog 승격과 공유 Gate 조율을 최소 한 주기 수행한다. | AP-01 | AP-03 | No |
| AP-03 | `embedding-pilot` | `adoption` | Adoption 효과 측정 및 불일치 분석 | 복원 시간, 다음 Task 정확도, Git/Task 불일치, stale lane, 수동 상태 갱신 횟수, Gate 조율 비용을 계약된 방식으로 측정하고 baseline과 비교한다. | AP-02 | AP-04 | No |
| AP-04 | `embedding-pilot` | `adoption` | Baley 지속·일반화 결정 | Pilot evidence로 제품 지속 여부를 결정하고 일반화할 핵심 요구와 Day Tripper 특수 요구를 분리해 후속 backlog 또는 중단 결정을 기록한다. | AP-03 | `terminalReason`: Adoption pilot의 최종 제품 결정 Task이며 후속 실행은 결정 결과로 새 manifest에서 시작한다. | No |

## 3.1 Registered Task IDs

| Key | Public Task ID |
| --- | --- |
| AC-01 | #117 |
| AC-02 | #118 |
| AC-03 | #119 |
| AC-04 | #120 |
| AE-01 | #121 |
| AE-01C | #129 |
| AE-01D | #131 |
| AE-02 | #122 |
| AE-01B | #130 |
| AE-AUTH-01 | #135 |
| AE-AUTH-02 | #134 |
| AE-AUTH-03 | #133 |
| AE-AUTH-04 | #132 |
| AE-03 | #123 |
| AE-04 | #124 |
| AP-01 | #125 |
| AP-02 | #126 |
| AP-03 | #127 |
| AP-04 | #128 |

## 3.2 Task #121 implementation evidence

- grouped `task.confirm`은 동일 baseline과 순차 fresh-preview revision chain을
  유지하는 Operator protocol 및 server exact binding 테스트로 검증한다.
- phase-targeted `task.create`는 typed MCP stdio E2E에서 parent, predecessor,
  successor, warning acknowledgement와 Event 결속을 검증한다.
- lane `BacklogItem`은 Phase 없는 create/update/move/reorder/discard와 명시 Phase
  promote를 contracts, Go domain/application, PostgreSQL, HTTP/CLI, typed MCP와
  live Viewer에 연결한다.
- 상세 증거는
  `task-records/lane-backlog-vertical-slice/completion-report-01.md`에 기록한다.

`#120` is attached to `embedding-enablement-entry`, and `#124` is attached to `embedding-pilot-entry`. Both were attached while their from-Phases were inactive.

## 4. Dependency and Gate topology

```text
#116 --[embedding-contract-entry]--> AC-01
AC-01 --> AC-02 ----\
          AC-03 ----+--> AC-04 --[embedding-enablement-entry]--> AE-01 --\
                                                                  +--> AE-03 --> AE-04
                                                        AE-01B --/
                                                        AE-01C --> AE-01D -/
                                                        AE-02 ----/                |
                                                                                  |
                           [embedding-pilot-entry] <-------------------------------+
                                      |
                                      v
AP-01 --> AP-02 --> AP-03 --> AP-04 (terminal)
```

For each cross-Phase dependency, the matching Gate condition is the authoritative transition prerequisite: #116 for Contract entry, AC-04 for Enablement entry, and AE-04 for Pilot entry.

## 5. Creation and attachment rules

- Fresh-read the Workspace before the first preview and after every execute that changes its revision.
- Create Tasks in manifest order with typed `task.create` preview/execute, replacing manifest keys with assigned public IDs in later dependency requests.
- Do not create alternate or additional Tasks without updating this PM baseline first.
- Attach AC-04 to `embedding-enablement-entry` and AE-04 to `embedding-pilot-entry` after their public IDs exist. Their `fromPhaseId` values are not active at initial setup, so they remain Agent-operator changes unless the live preview says otherwise.
- Attaching #116 to active `embedding-contract-entry` is human-only and requires approval of the exact fresh preview. A PM statement prepared before the preview is not a substitute for that approval.
- Until AE-01B/#130 is implemented, all `implemented -> confirmed` transitions require
  their own fresh preview and explicit human approval. The future delegated policy changes
  only Task confirmation and never Gate/Lane/Workspace approval boundaries.

## 6. Current Pilot entry state at revision 591

- #124 `Embedding Enablement 수용 검증`: `confirmed`
- G#4 `embedding-pilot-entry`: `passed`
- active Phase: `embedding-pilot`
- #125 `Day Tripper Pilot 온보딩`: `pending`

위 상태는 2026-08-03 Workspace-scoped authenticated read로 재검증했다. 이전 revision 163의
#111/#114/#115/#116 확인 권고는 모두 종료된 과거 판단이므로 현재 실행 지침으로 사용하지
않는다.
