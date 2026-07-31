---
type: evidence-recovery-runbook
status: pm-approved-implementation-contract
source_task: 119
---

# Evidence, Recovery, and Pilot Measurement Runbook

## 정본 분리와 충돌 처리

| 사실 종류 | 정본 | 불일치 시 처리 |
| --- | --- | --- |
| Task/Gate/Phase/Run 상태와 Event | Baley DB | DB를 기준으로 상태를 표시하고 잘못된 요약을 정정 |
| Task Record 원문 | repository file | path/hash를 재검사하고 index를 재등록 또는 새 Record 생성 |
| commit/blob/diff | Git repository | Git reference를 재검사하고 Baley reference를 stale/unverified로 표시 |
| worktree 관찰 | 비권위적 observation | 관찰 시각만 보존하며 DB·Git 사실을 덮어쓰지 않음 |
| Agent 요약 | 비권위적 설명 | 상위 정본과 맞도록 고침; 상태를 추측해 mutation하지 않음 |

## 최소 Record 세트

각 구현 Task는 가능한 범위에서 detailed plan, handoff, independent review, review
response, completion report를 남긴다. Baley index에는 record ID, Task/Run, repository,
relative path, working-tree hash, commit/blob reference와 short summary가 연결된다.

## 복원 절차

1. Workspace graph와 active Phase/Gate를 fresh-read한다.
2. 대상 lane의 open Task와 모든 active/nonterminal Run을 먼저 읽고, 마지막 terminal
   Run은 마지막 완료 근거로 별도 표시한다. active independent-review 또는
   completion-reporting Run은 terminal history보다 우선한다.
3. Record hash, referenced commit, Git observation을 비교해 `aligned`, `stale`,
   `missing`, `unverified`로 분류한다.
4. 다음 행동 후보는 confirmed dependency, active Gate, blocker, evidence freshness를
   근거로 제시한다.
5. 복원은 read-only이며 Task, Gate, Lane, Workspace, Run 상태를 바꾸지 않는다. Gate
   pass, `human_required` Task confirmation/discard, Lane/Workspace 종료는 후보가 아니라
   사람 판단으로 남긴다. 미래의 `delegated` Task auto-confirm은 복원 행동이 아니라
   #130의 policy와 typed-evidence transaction이다.

## 불일치 처리

| 상태 | Agent 행동 |
| --- | --- |
| Record path/hash missing | Task를 완료로 추정하지 않고 record 복구 또는 새 evidence 생성 |
| commit reference missing | working tree와 commit evidence를 분리해 보고 |
| completion record가 있고 independent review 또는 completion-reporting Run이 active/nonterminal | 정상 `reporting_pending`으로 분류하고 현재 Run을 기다린다. 중복 행동이나 mutation을 제안하지 않는다 |
| `reporting_pending` 밖의 Task status와 completion record 불일치 | 완료를 추정하지 않는다. active/terminal Run 순서를 조사하고 normal workflow 경로가 아닐 때만 rework 또는 구현보고 재검토를 제안한다. confirmed/discarded는 기존 outcome을 바꾸지 않고 follow-up Task만 제안한다 |
| stale worktree observation | 관찰 시각을 표시하고 live Git/Task evidence로 덮어쓰지 않음 |
| Gate condition 불일치 | Gate status fresh-read 후 사람 승인 경계를 재확인 |

## Pilot 측정 protocol

각 Pilot 측정은 append-only `PilotMeasurement` Record로 저장한다.

```text
PilotMeasurement
  measurement_id, workspace_id, lane_id, session_id, sample_id
  started_at, ended_at, workspace_revision, actor_id
  candidate_ids[], accepted_candidate_ids[], rejection_reasons[]
  evidence_reference_ids[], mismatch_keys[], correction_event_ids[]
  gate_id, conversation_ref, human_decision_turn_count
  baseline_or_treatment
```

| metric | 정의 | 수집 시점 |
| --- | --- | --- |
| recovery_time | session `started_at`부터 첫 candidate 생성 `ended_at`까지의 초 | Run 재개 |
| next_action_accuracy | accepted candidate 수 / 제시 candidate 수; 복수 후보는 각각 verdict 기록 | 복원 후 판단; defer/무응답은 별도 기록하고 분모에서 제외 |
| evidence_mismatch_count | `(reference_id, mismatch_kind)`별 기간 내 첫 발견 1건으로 dedup | 매 lane brief |
| manual_correction_count | 사람이 실행한 correction command Event ID 수 | Pilot 주기 종료 |
| gate_coordination_cost | 같은 `conversation_ref`의 사람 decision turn 수 | Gate마다 |

baseline은 Pilot 시작 전에 control sample로 한 번 기록한다. treatment sample은 같은
lane·기간 단위와 비교하며, 기간·Workspace revision·Actor·session/sample ID를 함께
남긴다. 대화 turn과 PM verdict는 V1 Event가 자동 보존하지 않으므로 Measurement Record에
명시적으로 기록한다. 수치는 제품 효과를 보조하는 신호이며 자동 우선순위 결정 입력은 아니다.

## #122와 #124 수용 기준

- #122는 이 runbook을 Task Record index, repository mismatch detection, lane brief query에
  연결한다.
- #124는 격리된 single-repository scenario에서 복원·증거·측정·Gate 판단을 E2E로 수행하고
  독립 review와 잔여 위험을 기록한다.
