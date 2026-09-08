---
type: analysis
status: complete
task: 182
audited_at: 2026-09-08
source_of_evidence: direct-postgresql-read-only
snapshot_at_kst: 2026-09-08T11:55:35.522390+09:00
snapshot_workspace_revision: 1319
---

# Baley 개발 의사결정 로그 운영 DB 감사와 보전 설계

## 결론

운영 Baley DB는 **무엇을 실행했고 상태가 어떻게 바뀌었는지**는 상당히 잘 보존하지만, **왜 그 선택을 했는지, 무엇과 비교했고, 예상과 실제가 어떻게 달랐으며, 어느 판단을 대체·보완했는지**를 일관된 단위로 보존하지 않는다.

확인된 핵심 사실은 다음과 같다.

- public base table은 37개지만 `decision`, `decision_log` 또는 동등한 전용 table은 없다.
- 대상 Workspace에는 Task 76, Run 277, Task Record index 191, commit reference 34, typed acceptance evidence 6, command 1,205, Event 1,237, mutation attempt 1,400이 있었다.
- 선언된 Task/Run/Record/commit/evidence FK의 실제 orphan은 0건이다. 반면 과거 DB 재구성 때문에 Task 23개에는 원래 `task.created` Event가 없고, Run 14개와 Record 31개에는 원래 시작·등록 Event가 없다.
- Task의 description은 76/76에 있지만 current summary는 41/76, next action은 6/76, commit reference는 21/76 Task, typed acceptance evidence는 4/76 Task에만 있다.
- Event 전체에서 전용 `alternatives`, `rationale`, `expectedOutcome`, `actualOutcome`, `supersedes`, `amends` key는 0건이다. Record-level `supersedes_record_id`만 15/191건에 있다.
- Bird View 중단 결정은 Task #179의 상태·summary·assessment를 바꾸지 않고 completion-reporting Run summary, commit reference와 Git observation에만 추가됐다. “PM decision”이라는 문장은 남지만 actor 관계상 initiator/executor는 Agent이고 human approver는 없다.
- MCP catalog 축소 #180과 rework 도구 복구 #181은 제목·설명·assessment·parent 관계와 시간 순서로 변화 방향을 읽을 수 있지만, #181이 #180을 `amends`한다는 명시 관계는 없다.
- 오래된 판단 변경 Task #121은 `task.rework_started.reason`이 있어 가장 잘 복구되는 사례다. 다만 현재 status가 confirmed인데도 next action이 과거 실행 지시로 남아 있어 현재 projection의 의미적 stale 가능성을 보여준다.

따라서 최소 해법은 Task나 Event에 필드를 더 흩뿌리는 것이 아니라, Workspace 범위의 안정적인 `D#<n>` identity, append-only decision event, decision 간 `supersedes|amends`, 그리고 Task/Run/Record/Event/command/commit/evidence link를 도입하는 것이다. 자동화는 링크와 후보를 캡처하고, 질문·대안·선택·근거는 선택 직전에 사람 또는 Agent가 짧게 직접 기록해야 한다.

## 조사 경계와 재현성

### 사용자와 권한

사용자는 Baley의 제품 오너·PM·사람 승인자다. 이 세션은 Task #182 전담 조사 Operator로 수행했다. 사람 전용 confirm, discard, Gate/Lane/Workspace 결정은 실행하지 않았다.

### 자료원

이 보고서의 schema, row, count, 연결률, 사례와 사실 결론은 모두 현재 Baley API가 사용하는 PostgreSQL에 직접 접속해 얻었다. Baley MCP는 Task #182의 Run 시작·heartbeat와 Task Record index 등록에만 사용했고 분석 자료원으로 사용하지 않았다.

모든 성공 SQL 세션은 다음 형태였다.

```sql
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY;
SELECT ...;
ROLLBACK;
```

초기 schema 가정 오류가 난 세션은 SQL 오류 후 connection 종료로 자동 rollback됐고, 확인된 column type/name으로 다시 실행해 명시적 `ROLLBACK`을 확인했다. DDL, DML, migration, 설정 변경은 실행하지 않았다.

재현 SQL은 [`development-decision-log-readonly.sql`](development-decision-log-readonly.sql)에 있다. DB URL 없이 이미 신뢰된 `psql` session에서 다음처럼 변수만 넘긴다.

```powershell
psql -X -v ON_ERROR_STOP=1 `
  -v workspace_id='<workspace-id>' `
  -v snapshot_at='2026-09-08T02:55:35.52239Z' `
  -f docs/analysis/development-decision-log-readonly.sql
```

`tasks` 같은 current-state projection은 사후 변경을 시간여행할 수 없다. `snapshot_at` cutoff는 Event, command, Run 시작, Record, commit, evidence 같은 시간 column이 있는 row에 적용된다.

## 운영 DB 식별

### 확인된 사실

1. 원격 hosted-pilot 문서에 적힌 application/DB host를 먼저 조사했으나 두 호스트의 Baley service와 PostgreSQL service는 모두 inactive였다. 이를 현재 운영 대상으로 사용하지 않았다.
2. 현재 loopback `BALEY_SERVER_URL`의 실제 API container는 healthy 상태의 `baley-api-1`이었다.
3. API container의 DB URL은 값을 출력하지 않고 parsing했다. host는 main PostgreSQL container `local-dev-postgres`, database는 `baley`였다. credential 존재 여부만 확인했고 값은 출력하지 않았다.
4. Bird View 격리 대상은 별도 container `baley-bird-view-v2-test`, 별도 loopback port, 별도 volume, 별도 database `baley_v2_test`였다. API의 DB target이 아니었다.
5. main DB 직접 접속에서 `current_database() = baley`, `transaction_read_only = on`, 핵심 `workspaces/tasks/events` table 존재, 기대 Workspace와 Task #182 존재를 확인했다. bookkeeping 전 revision은 1317이었다.

### 감사 snapshot

Task #182 상세계획 Run 시작과 상세계획 Record 등록 후, 다음 시점을 수치 기준으로 고정했다.

| 항목 | 값 |
| --- | --- |
| KST 시각 | 2026-09-08 11:55:35.522390 |
| Workspace revision | 1319 |
| snapshot에 포함된 이번 Task row | in_progress Task #182, running 상세계획 Run 1개, 상세계획 Record 1개 |
| snapshot 이후 제외 | 첫 Run lease 만료 Event와 continuation Run/heartbeat, 완료 bookkeeping |

## 확인된 사실 1 — 실제 table과 관계

### 실제 table

public schema의 base table은 37개였다. 개발 의사결정 흔적과 직접 관련된 table은 다음과 같다.

| 계층 | 실제 table | 주요 필드와 의미 |
| --- | --- | --- |
| Actor | `actors` | `id`, `display_name`, `actor_type`; deployment 범위 identity |
| Task | `tasks` | title, description, status, blocker, parent, current summary, next action, terminal reason, implemented assessment, acceptance binding |
| Task 관계 | `task_dependencies` | `from_task_id → to_task_id`, 생성 시각 |
| Run | `runs` | kind/status, operator actor, session ref, parent/target Run, lease, 시작/종료, result/error summary |
| Task Record | `task_record_indexes` | Task/Run/repository, type, 상대 경로, tree/commit/blob hash, state, short summary, Record supersedes |
| acceptance | `task_acceptance_assignments` | mode/profile/policy version, reason/evidence reference, approver, assignment supersedes |
| acceptance evidence | `task_acceptance_evidence` | completion report, verification verdict/reference, independent review, blockers, optional commit, reporter/시각 |
| commit | `commit_references` | Task/Run/repository, SHA, relation, verification state, 생성 시각 |
| Git observation | `run_git_observations` | Run/repository, 관측 시각, head, branch hint, worktree label, dirty |
| command | `commands` | name/hash/fingerprint/revision, result JSON, initiator/executor, credential kind; 원래 argument 본문은 없음 |
| Event | `events` | command FK, revision, type, payload, 시각, 다형 entity type/id, initiator/executor/approver |
| 사람 승인 | `human_approval_attestations` | actor, command/snapshot hash, action/대상/revision, statement hash, conversation ref, approval grant, 시각 |
| browser grant | `approval_grants` | account/actor/session, action/대상/revision/hash/warning, status와 소비·취소·만료 시각 |
| mutation audit | `mutation_attempts` | source/outcome, 논리 entity, actor, argument/idempotency digest, command/event IDs, revision, diagnostics, duration |
| security audit | `security_events` | Account/Actor와 보안 Event type, entity, payload, 시각 |

전용 decision table이나 다음 의미를 정규화한 column은 없었다.

- decision ID와 decision status
- 질문과 결정 맥락
- 검토한 대안 집합과 대안별 장단점·탈락 사유
- 선택한 대안과 별도 rationale
- 예상 결과와 실제 결과의 대응
- decision-to-decision `supersedes|amends`

### 실제 FK 관계

확인된 핵심 FK는 다음과 같다.

```text
tasks
  -> workspaces, lanes, phases, evidence_profiles
  -> tasks(parent)

task_dependencies
  -> tasks(from), tasks(to)

runs
  -> tasks, actors(operator)
  -> runs(parent), runs(target)

task_record_indexes
  -> tasks, runs?, repositories
  -> task_record_indexes(supersedes)

commit_references
  -> tasks, runs?, repositories

run_git_observations
  -> runs, repositories

task_acceptance_evidence
  -> tasks, task_record_indexes(completion/review), commit_references?, actors

commands
  -> workspaces, actors(initiator/executor)

events
  -> workspaces, commands, actors(initiator/executor/approver)
  -X-> entity_type/entity_id 대상은 물리 FK 없음

human_approval_attestations
  -> workspaces, actors(approver), commands, approval_grants?
  UNIQUE(executed_command_id), UNIQUE(approval_grant_id)

mutation_attempts
  -X-> command_id, event_ids, actor IDs는 논리 link이며 물리 FK 없음
```

운영 schema는 규범 문서의 개념 예시와 일부 literal이 달랐다. ID 다수는 `uuid`가 아니라 `text`, Event 시각은 `occurred_at`이 아니라 `created_at`, actor column은 `executed_by`가 아니라 `executed_by_actor_id`였다. `tasks`에는 `created_at`이 없어 생성 시각을 Event에 의존한다. 반면 acceptance mode/profile과 typed evidence table은 문서의 초기 core보다 확장돼 있었다.

## 확인된 사실 2 — 수치, 연결률과 시간 분포

### 계층별 row와 기간

| 계층 | row | 최초 KST | 최종 KST |
| --- | ---: | --- | --- |
| Task | 76 | 2026-07-26 19:07:41 (확인 가능한 create Event 기준) | 2026-09-08 11:39:43 |
| command | 1,205 | 2026-07-26 16:07:34 | 2026-09-08 11:54:01 |
| Event | 1,237 | 2026-07-26 16:07:34 | 2026-09-08 11:54:01 |
| Run | 277 | 2026-07-25 11:00:00 | 2026-09-08 11:52:15 |
| Task Record index | 191 | 2026-07-26 16:06:41 | 2026-09-08 11:54:01 |
| acceptance assignment | 76 | 2026-07-28 00:08:00 | 2026-09-08 11:39:43 |
| typed acceptance evidence | 6 | 2026-08-28 21:59:54 | 2026-09-08 10:59:49 |
| commit reference | 34 | 2026-08-28 11:41:33 | 2026-09-08 11:32:40 |
| Run Git observation | 4 | 2026-09-07 16:53:07 | 2026-09-08 11:33:30 |
| human approval attestation | 44 | 2026-07-26 18:22:21 | 2026-09-02 11:17:18 |
| approval grant | 1 | 2026-09-02 11:17:18 | 동일 |
| mutation attempt | 1,400 | 2026-07-26 17:18:46 | 2026-09-08 11:54:01 |
| security Event | 37 | 2026-07-28 23:46:05 | 2026-09-05 11:55:37 |

Task 상태는 confirmed 42, implemented 24, pending 8, in_progress 1, discarded 1이었다. in_progress 1건은 이 감사 Task #182다.

Run은 상세계획 48, 구현 103, 독립 Agent 리뷰 58, 리뷰 반영 18, 완료보고 50이었다. snapshot 당시 succeeded 191, interrupted 85, running 1이었다. 82개 `run.heartbeat` command만 의도적으로 Event를 만들지 않았으며, heartbeat를 제외한 command 중 Event가 없는 row는 0건이었다.

Record 191건은 상세계획 56, 완료보고 57, Handoff 12, 독립 Agent 리뷰 46, 리뷰 반영 19, Pilot 측정 1이었다. 174건은 `committed_unverified`, 17건은 `reported_uncommitted`였다.

### Task 기준 연결률

| 복구 요소 | Task | 비율 |
| --- | ---: | ---: |
| description 비어 있지 않음 | 76/76 | 100.0% |
| current summary | 41/76 | 53.9% |
| next action | 6/76 | 7.9% |
| terminal reason | 20/76 | 26.3% |
| implemented assessment | 60/76 | 78.9% |
| `task.created` Event | 53/76 | 69.7% |
| 어떤 Task Event든 존재 | 70/76 | 92.1% |
| Run 1개 이상 | 60/76 | 78.9% |
| Record 1개 이상 | 52/76 | 68.4% |
| 상세계획 Record | 49/76 | 64.5% |
| 독립 Agent 리뷰 Record | 38/76 | 50.0% |
| 완료보고 Record | 51/76 | 67.1% |
| commit reference | 21/76 | 27.6% |
| typed acceptance evidence | 4/76 | 5.3% |

Record 자체의 연결 품질은 더 높았다.

| 항목 | row | 비율 |
| --- | ---: | ---: |
| Record → Run | 170/191 | 89.0% |
| Record → commit SHA/blob SHA | 174/191 | 91.1% |
| Record → superseded Record | 15/191 | 7.9% |
| commit reference → Run | 24/34 | 70.6% |
| commit remote verified | 0/34 | 0.0% |
| typed evidence → commit reference | 3/6 | 50.0% |
| acceptance assignment → superseded assignment | 0/76 | 0.0% |

Record supersedes 15건은 완료보고 4, 상세계획 3, Handoff 3, 독립 리뷰 3, 리뷰 반영 2였고 모두 같은 Task 안의 Record를 가리켰다. 이것은 문서 version 관계이지 개발 판단 간 관계는 아니다.

Run Git observation은 4개 Run, 2개 Task만 덮었다. Run coverage 1.4%, Task coverage 2.6%다.

### orphan과 missing history

FK 기반 orphan은 모두 0건이었다.

- Run → Task
- Record → Task 및 non-null Run
- commit reference → Task
- acceptance evidence → Task
- Event → command

Event `entity_type/entity_id`는 FK가 없는 다형 link다. Task, Run, Record, commit, attestation, Git observation, Phase, Gate Event는 모두 현재 row로 해석됐다. `backlog_item` Event는 내부 UUID가 아니라 public integer ID를 쓰고, `workspace_graph` Event는 실제 row ID가 아니라 literal aggregate key `workspace_graph`를 쓴다. UUID join 실패를 orphan으로 해석하면 안 된다.

역사 누락은 구조 orphan과 별개다.

- Task 6개(#101, #104, #106, #110, #112, #113)는 Task Event가 하나도 없다.
- Task 17개(#111, #114–#129)는 후속 Event는 있지만 `task.created`가 없다.
- Run 14개에는 `run.started`와 terminal Event가 모두 없다.
- Record 31개에는 `record.registered` Event가 없다.
- command 10개에는 mutation attempt가 없다. 반대 방향의 dangling command/event link는 0건이었다.

`recovery.reconstructed` Event는 2026-07-26 16:07 KST, revision 196에 발생했다. source revision은 194, physical recovery는 incomplete였고 source는 repository Task Records와 PM manifest였다. command result는 원래 command/Event stream이 없고, Task #112–#114 metadata가 부분 추론됐으며, Task #121 외 과거 internal UUID를 알 수 없다는 limitation과 Record 31개 재색인을 명시한다. 위 누락 수치와 정확히 같은 경계다.

### Actor와 승인·감사

Event executor는 1,237/1,237에 있었다: Agent 1,235, human 2. initiator는 Agent 801, human 49, null 387이었다. approver는 human 98 Event에 있고 나머지 1,139에는 없다. 승인 하나가 attestation과 domain/Phase Event 여러 개를 만들 수 있어 승인 Event 수는 attestation 44보다 많다.

Run operator 277/277은 Agent였다.

human approval attestation 44건 중 statement hash 42, conversation ref 43, approved_at 36, browser approval grant link 1이었다. action은 Task confirm 36, Gate pass 5, active Gate attach 2, Task discard 1이었다. approval grant 1건은 human session에서 발급되어 정상 소비·command 연결됐다. 이는 현재 browser-grant 방식 이전의 legacy attestation row가 함께 있음을 보여준다.

command credential provenance는 Agent token 709, historical null 495, human session 1이었다. 민감한 credential ID 원문은 조사 결과에 포함하지 않았다.

mutation attempt 1,400건은 succeeded 1,197, rejected 202, idempotent 1이었다. source는 command service 1,398과 database trigger 2였다. command ID 1,196, non-empty Event IDs 1,114, argument digest 1,398, request fingerprint 1,397, initiator 977, executor 1,398이었다. argument 원문 대신 digest를 남긴다는 점은 민감정보 방어에는 유리하지만 과거 command 선택 이유 복구에는 한계다.

### 주간 분포

| 주 시작(KST) | Event | Run | Task create Event | Record | commit | typed evidence | approval |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 2026-07-20 | 180 | 68 | 2 | 61 | 0 | 0 | 10 |
| 2026-07-27 | 258 | 45 | 5 | 28 | 0 | 0 | 12 |
| 2026-08-03 | 8 | 1 | 1 | 0 | 0 | 0 | 0 |
| 2026-08-10 | 3 | 1 | 0 | 0 | 0 | 0 | 0 |
| 2026-08-17 | 93 | 20 | 10 | 11 | 0 | 0 | 7 |
| 2026-08-24 | 329 | 69 | 10 | 43 | 6 | 2 | 14 |
| 2026-08-31 | 302 | 63 | 23 | 44 | 18 | 2 | 1 |
| 2026-09-07 | 64 | 10 | 2 | 4 | 10 | 2 | 0 |

초기 recovery 색인이 7월 20일 주의 Record/Run 수를 크게 만들고, commit reference와 typed evidence는 8월 24일 주부터 시작한다. 따라서 전체 기간의 “증거 연결률”을 단일 운영 습관으로 해석하면 안 된다.

## 확인된 사실 3 — 어떤 의사결정 의미를 복구할 수 있는가

| 의미 | 현재 source | 정량 근거 | 복구 수준 |
| --- | --- | --- | --- |
| 질문·맥락 | Task title/description | description 76/76 | 중간. Task 목적은 알지만 decision 단위가 아님 |
| 현재 요약 | Task current summary | 41/76 | 중간 이하. 절반가량 누락, stale 가능 |
| 다음 행동 | Task next action | 6/76 | 낮음 |
| 검토 대안 | 전용 source 없음 | Event key 0 | 매우 낮음. prose 추론만 가능 |
| 선택 | Task/Run 결과, 상태 전이 | 전용 field/ID 없음 | 중간 이하. 행동에서 역추론 |
| 근거 | assessment, 일부 reason/proceed reason | implemented Event assessment 57/57, proceed reason 27/57 | 중간. 구현 결론에는 강하고 선택 비교에는 약함 |
| 예상 결과 | description, terminal reason | 전용 Event key 0, terminal reason 20/76 | 낮음 |
| 실제 결과 | Task assessment, Run summary, evidence | Task assessment 60/76, succeeded Run summary 170, evidence 6 | 중간 이상. 여러 위치를 조합해야 함 |
| supersedes/amends | Record supersedes | Record 15, assignment 0, Event decision key 0 | 문서 version만 가능, decision 관계는 불가능 |
| actor·시각 | Event/Run/approval | executor 100%, initiator 68.7%, Run operator 100% | 실행 actor는 강함. 판단 actor는 불완전 |

국소 reason 보존은 다음과 같다.

- dependency patch: 6건 중 non-empty proceed reason 2건
- Task confirm: 36건 중 non-empty proceed reason 4건
- Task discard: 1/1 reason
- Task rework: 1/1 reason
- Run correction: 7/7 reason
- interrupted Run: snapshot의 92건 모두 error summary

전용 `alternatives`, `rationale`, `expectedOutcome`, `actualOutcome`, `supersedes`, `amends` JSON key는 1,237 Event 전체에서 0건이었다. assessment prose에 우연히 이런 내용이 포함될 수는 있지만 안정적인 query나 relation으로 사용할 수 없다.

## 확인된 사실 4 — DB row만으로 재구성한 대표 사례

### Bird View 중단과 Task #179

1. 2026-09-06 20:43 KST, revision 1234에 Task #179 “DayTripper Bird View Node–Task 의미 기반 재매칭”이 생성됐다.
2. 20:45 구현 Run이 시작되어 21:11 성공했다. Task는 revision 1240에서 implemented가 됐다. assessment는 5개 Node의 의미 기반 Task/Gate binding, 격리 DB reseed, API/UI/test 검증과 미연결 Task 24개를 범위 제외로 기록했다.
3. 2026-09-08 11:09 완료보고 Run은 Bird View 작업을 pause하면서 변경을 한 commit으로 묶어 push하고, Task #172–#179와 16개 Record에 연결하고, clean worktree를 제거했다고 기록했다. Task #179에는 produced commit과 Git observation이 연결됐다.
4. 11:31 두 번째 완료보고 Run은 “PM decision to pause Bird View”와 이유 두 가지—현재 효용 부족, 개발 의사결정 맥락·근거·대안·link·복구 보전의 우선순위—를 result summary에 기록했다. 별도 produced commit과 Git observation도 연결됐다.
5. 그러나 Task #179의 current row는 여전히 원래 재매칭 작업의 summary/assessment와 implemented 상태이며 `updated_at`도 9월 6일 21:11이다. 9월 8일 pause를 나타내는 Task Event, decision status, human approval attestation은 없다.
6. 두 pause 관련 Run의 operator와 `run.started` initiator는 모두 Agent, approver는 null이다. 따라서 DB는 “PM 결정”이라는 텍스트 주장을 보존하지만 그것을 사람 actor 권한과 결속하지 못한다.

복구 가능한 것: 중단 시점, 실행 주체, 두 개 commit, branch/worktree hint, 중단 이유가 적힌 Run summary.

복구 불가능한 것: 어떤 대안과 비교했는지, 사람이 정확히 언제 어떤 문장으로 결정했는지, 중단이 임시인지 폐기인지, 재개 조건, Task #179 원래 구현 결과와 pause 결정 사이의 정식 관계.

### MCP catalog 축소 Task #180

1. 2026-09-06 23:05 KST Task가 생성·시작됐다. 두 implementation Run이 lease expiry로 interrupted됐고, parent-linked 세 번째 Run이 23:22–23:50 성공했다.
2. assessment는 기본 catalog를 78 tools/37,800B에서 14 tools/4,184B로 줄여 88.93% 감소시키고 `/mcp/full`에 legacy 78개를 보존했다고 기록한다.
3. 상세계획·완료보고·독립 리뷰 Record 3개와 produced commit 1개가 있다. 세 Record는 Run link 없이 등록됐지만 모두 commit/blob가 연결됐다.
4. acceptance evidence는 2 version이다. 둘 다 verification passed, review pass, unresolved blocker 0이며 두 번째만 commit reference에 연결됐다.

현재 assessment는 “독립 worktree에 미커밋”이라고 말하지만 이후 commit reference가 생겼다. assessment가 append-only 역사 진술인지 current outcome인지 구분하는 field가 없어, 최신 사실은 commit/evidence row를 따로 읽어야 한다.

### MCP catalog 복구 Task #181

1. Task #181은 2026-09-07 15:54 생성됐고 DB parent가 #180이다. 두 Task 사이 dependency는 없다.
2. 한 번의 lease expiry 뒤 구현 Run이 성공했고 독립 Agent review도 성공했다. 16:31 implemented assessment는 compact profile에 정확한 rework preview/execute 2개를 복구해 15 tools/5,106B로 유지하고 full profile 84개를 보존했다고 기록한다.
3. 상세계획·완료보고·독립 리뷰 Record 3개, produced commit 1개, Git observation 2개, acceptance evidence 2 version이 있다. 두 번째 evidence는 commit에 연결됐다.
4. 후속 완료보고 Run은 #180 commit을 운영 worktree에 흡수한 상태, #181 local commit 저장, 이후 remote push를 차례로 기록한다.

#181이 #180을 보완한 것은 title, description, parent 관계, 수치 변화와 시간 순서로 강하게 추론할 수 있다. 그러나 DB에는 `amends #180` 또는 “왜 14개 선택을 바꿨는지”를 정식 decision relation으로 나타내는 row가 없다.

### 오래된 판단 변경 — Task #121

1. Task #121은 운영 DB 재구성 대상이라 `task.created` Event가 없다. 첫 보존 Event는 2026-07-26 16:10 KST revision 197의 implemented 보고다. 이 assessment는 원래 Event stream 손실과 일부 recovery 경계를 명시했다.
2. 16:58 revision 198에서 유일한 `task.rework_started`가 기록됐다. reason은 “사람 confirmation 전에 workspace-scoped mutation attempt audit logging과 direct-write 방어를 추가하기로 결정함”이었다.
3. 이어 상세계획, runtime 확인, 구현, 독립 리뷰, 리뷰 반영, 재리뷰가 반복됐다. 14개 Run에는 review finding과 해결 결과가 상세 summary로 남아 있다.
4. 23:42 revision 345에서 두 번째 implemented 보고가 append-only mutation audit, redaction, direct SQL/TRUNCATE 방어와 전체 검증을 기록했다.
5. 다음 날 revision 367에서 human-approved confirm이 기록됐다. attestation은 conversation ref와 statement hash를 보존하지만 승인 발화 원문은 보존하지 않는다.
6. Record 12개 중 7개가 앞 Record를 supersede한다. 별도 commit reference는 0개지만 Record index 자체의 commit/blob link는 있다.
7. 현재 Task status는 confirmed지만 next action은 여전히 “privileged server restart 후 확인”이라는 과거 지시다. 상태 전이 후 projection field를 의미상 정리하는 보장이 없음을 보여준다.

이 사례는 이유가 있는 rework Event가 있으면 판단 변경을 잘 복구할 수 있음을 보여준다. 동시에 원래 create Event, 최초 internal identity와 승인 발화 원문은 복구되지 않는다.

## 확인된 사실 5 — chat·terminal·worktree에만 남거나 DB에서 축약되는 정보

| 정보 | DB에 남는 것 | 사라지거나 외부 정본에만 있는 것 |
| --- | --- | --- |
| Task Record | type, path, short summary, hash, commit/blob, supersedes | 본문, 문서 안의 대안·논증·review 세부 finding |
| chat | 일부 approval conversation ref와 statement hash | 발화 원문, 질문·대안 논의, 조건부 동의, 뉘앙스 |
| terminal | Run result/error summary | 명령 transcript, 중간 실험, 관찰 순서, 버린 가설 |
| command | name/hash/fingerprint/revision/result | 원래 typed argument 본문과 사용자가 제시한 이유. mutation audit도 digest만 보존 |
| Git | SHA, relation, reported/verified state | commit message, diff, authoring 맥락, remote ref 이력; provider 검증은 0/34 |
| worktree | 관측된 head/branch hint/label/dirty | 절대 경로, 생성·전환·삭제 lifecycle, 현재 존재 여부; coverage는 Run 1.4% |
| 사람 결정 | attestation action/대상/hash/actor/시각 | 일반 product/PM 결정의 내용. 승인 command가 아닌 Bird View pause는 human actor link 없음 |

특히 Task #179는 중요한 product-priority 전환이 Run summary에는 들어갔지만 Task current state와 human approval audit에는 들어가지 않았다. 반대로 Task #180/#181의 실제 commit·push 결과는 나중 Run/commit row에 쌓였지만 최초 implemented assessment는 “미커밋” 상태로 남았다. 현재 구조는 시간선 조립은 가능하지만 하나의 decision thread로 질의할 수 없다.

## 설계상 추론 — 최소 decision-log 모델

이하 내용은 DB에서 직접 확인한 사실이 아니라, 위 gap을 해결하기 위한 설계 제안이다.

### 원칙

1. Decision은 Task와 별도 identity다. 한 Task에는 여러 결정이 있고 한 결정은 여러 Task에 영향을 줄 수 있다.
2. Event 전체를 decision으로 취급하지 않는다. Event는 실행 감사이고 Decision은 선택의 의미다.
3. 질문·대안·선택·근거는 사람이든 Agent든 **선택 직전**에 기록한다. 자동화는 관계와 결과를 붙이되 rationale을 발명하지 않는다.
4. 수정·철회·대체는 update/delete가 아니라 새 append-only entry와 명시 relation으로 표현한다.
5. `occurred_at`과 `recorded_at`을 모두 둬 사후 기록과 backfill을 구분한다.

### 최소 4개 table

#### `decisions` — 안정 identity

```text
workspace_id
id UUID
public_id BIGINT                 # 화면 표준 D#<n>
decision_kind                   # product | architecture | scope | safety | process | implementation
authority_required              # operator | approver | owner
created_by_actor_id
created_at
```

이 row는 생성 후 바꾸지 않는다. current status와 최신 내용은 append-only event에서 projection한다.

#### `decision_events` — append-only 의미 기록

```text
id UUID
workspace_id
decision_id
sequence BIGINT
event_type                      # proposed | accepted | rejected | outcome_observed |
                                # corrected | withdrawn | superseded
status_after                    # proposed | accepted | rejected | withdrawn | superseded
question
context
alternatives JSONB              # [{key, option, benefits, costs, rejected_reason}]
selected_alternative_key nullable
rationale nullable
expected_outcome JSONB nullable  # 설명, 확인 기준, 목표 시각
actual_outcome JSONB nullable    # verdict + 관찰; 여러 outcome Event 허용
change_reason nullable
corrects_event_id nullable
source                          # human | agent | automatic | backfill
backfill_confidence nullable     # high | medium | low
initiated_by_actor_id nullable
recorded_by_actor_id
decided_or_approved_by_actor_id nullable
occurred_at
recorded_at
workspace_revision nullable
sensitivity                    # internal | restricted
```

최소 유효성 규칙:

- `accepted`에는 질문, context, 선택, rationale, expected outcome이 필요하다.
- 대안이 하나뿐이면 빈 배열 대신 “왜 단일 viable path인지”를 기록한다.
- `outcome_observed`는 기존 expected outcome을 덮지 않고 `met|partially_met|not_met|unknown`을 추가한다.
- human-required decision의 `accepted/rejected/withdrawn/superseded`는 browser approval grant와 결속한다.

#### `decision_relations` — 판단 간 관계

```text
workspace_id
id UUID
from_decision_id
relation                       # supersedes | amends
to_decision_id
reason
recorded_by_actor_id
recorded_at
source
backfill_confidence nullable
```

- `supersedes`: 새 결정이 이전 결정을 대체한다. 이전 decision의 derived status를 superseded로 만든다.
- `amends`: 이전 결정은 유효하지만 일부 조건·범위·수치를 보완한다.
- relation을 잘못 기록하면 삭제하지 않고 correction Event로 무효화한다.

#### `decision_links` — 실행·증거 연결

```text
workspace_id
id UUID
decision_id
target_type                    # task | run | task_record | event | command |
                               # commit_reference | acceptance_evidence
target_id
role                           # context | trigger | implements | evidence | outcome
recorded_by_actor_id
recorded_at
source_event_id nullable
retracts_link_id nullable
```

target는 같은 Workspace여야 하고 server가 type별 FK 의미를 transaction에서 검증한다. 잘못 붙인 link는 삭제하지 않고 retract link로 남긴다.

### 상태, 수정·철회·대체

```text
proposed -> accepted -> superseded
         -> rejected
         -> withdrawn

accepted -> outcome_observed (0..n, status 유지)
accepted -> amended relation (원결정 유지)
accepted -> superseding decision (원결정 superseded)
```

본문 오타·사실 오류는 `corrected` Event가 정확한 이전 Event를 가리킨다. 철회는 `withdrawn`, 다른 판단으로 교체는 새 Decision + `supersedes`다. 과거 선택과 당시 근거는 그대로 남는다.

append-only와 법적 삭제·secret 사고는 구분한다. secret이나 raw credential은 처음부터 저장하지 않는다. 개인정보 삭제가 필요한 경우 제한 권한의 redaction 절차로 민감 payload를 암호화 키 폐기 또는 비식별화하고, decision ID·시각·redaction 사유만 non-sensitive tombstone Event로 남긴다.

## 설계상 추론 — 기록 습관과 자동 캡처

### 사람·Agent가 수동으로 기록할 시점

다음 순간에는 30초짜리 decision note를 필수 습관으로 둔다.

1. architecture, scope, security, dependency, 외부 비용 중 하나를 선택하기 직전
2. Task를 pause, rework, discard 또는 다른 Task로 대체하기 직전
3. 두 개 이상 viable 대안 중 하나를 버릴 때
4. 예상 결과가 빗나가 선택을 유지·수정·철회할 때
5. session 종료 전 아직 열린 판단이나 재개 조건이 있을 때

최소 입력은 `질문 / 대안 / 선택 / 근거 / 예상 결과 / 권한 주체`다. Task title/description이나 완료보고 prose로 대신하지 않는다.

### 자동 캡처

자동화는 다음을 한다.

- `task.rework`, `task.discard`, `dependency.patch`, Task/Lane/Gate/Workspace human decision, acceptance policy change가 `decisionId`를 받으면 같은 transaction에서 Event/command link를 추가한다.
- 중요한 command에 `decisionId`가 없으면 rationale을 추정해 생성하지 않고 **decision candidate** advisory만 만든다.
- linked Task의 Run 시작·성공·중단, Record 등록·supersede, commit attach, Git observation, acceptance evidence를 해당 Decision에 자동 link한다.
- expected outcome 기한이나 검증 기준에 도달하면 `outcome_observed` 작성 알림을 만든다.
- 완료보고 시 accepted Decision의 actual outcome 누락을 warning으로 보여주되 구현완료를 막지는 않는다.

Bird View pause라면 중단 선택 직전에 Decision을 만들고 #179, 두 completion Run, 두 commit, pause Record를 자동 연결했어야 한다. #181은 #180 Decision에 `amends` 관계를 추가하고 “14-tool 선택에서 rework recovery discoverability가 빠졌음”을 rationale로 기록했어야 한다.

## 설계상 추론 — 검색, timeline과 session recovery

### 검색

- full-text: question, context, alternatives, choice, rationale, expected/actual outcome
- facet: status, decision kind, authority, actor type, 시각, Task/Lane/Phase, source, backfill confidence, sensitivity
- relation traversal: supersedes/amends chain, implementing Task, evidence/commit
- index: `(workspace_id, public_id)`, `(workspace_id, status, recorded_at)`, link target, relation 양방향, full-text vector

### timeline

정렬 key는 `occurred_at`, `recorded_at`, decision sequence, Workspace revision을 함께 사용한다. 다음을 한 흐름으로 보인다.

```text
question -> alternatives -> accepted choice -> Task/Run execution
-> Record/commit/evidence -> actual outcome -> amendment/supersession
```

Event timeline은 그대로 유지하되 decision timeline의 실행 증거로 접는다. “실행이 여러 번 실패했다”와 “선택을 바꿨다”를 같은 것으로 보지 않는다.

### session recovery

`decision.recovery_brief`는 선택한 Task/Lane/Workspace에 대해 다음을 반환한다.

- 아직 accepted 상태인 최신 Decision과 superseded chain
- 질문, 선택, rationale, expected outcome, 마지막 actual outcome
- 연결된 active/last Run, 최신 Record, commit/evidence 상태
- 열린 outcome 확인, stale current summary/next action 경고
- backfill/자동 후보 중 아직 사람이 확인하지 않은 항목

이 기능이 있으면 Task #179 복귀 시 원래 Bird View 구현완료와 후속 pause·재개 조건을 분리해서 읽을 수 있다.

## 설계상 추론 — 권한, 민감정보, 보존·삭제

### 권한

기존 capability와 분리해 최소 다음을 둔다.

```text
decision:read
decision:write             # propose, technical outcome, link
decision:approve           # human-required accept/reject/withdraw/supersede
decision:read_restricted
decision:redact            # privacy/security incident 전용
```

- Agent는 reversible technical Decision을 operator authority로 accept할 수 있으나 product/scope/safety policy가 human-required이면 제안과 evidence만 기록한다.
- 제품 오너·PM·Approver/Owner의 최종 선택은 현재 browser-session single-use grant 방식에 결속한다.
- initiated, recorded, decided/approved actor를 분리한다. Agent가 “PM 결정”이라고 적는 prose는 human approval attribution이 아니다.
- 모든 link와 relation은 Workspace 경계를 강제한다.

### 민감정보

- raw chat, terminal transcript, DB URL, password, token, cookie, session secret, 개인 식별 정보를 자동 수집하지 않는다.
- 필요한 conversation evidence는 현재처럼 opaque ref/hash와 sanitized excerpt만 둔다.
- commit SHA는 허용하되 private remote URL·author email은 기본 응답에서 숨긴다.
- alternatives/rationale 입력에 secret detector와 size limit을 적용한다.
- `internal`은 일반 member read, `restricted`는 별도 capability와 audit를 요구한다.

### 보존·삭제

- accepted/superseded/withdrawn Decision과 link metadata는 Workspace 보존 기간 동안 장기 보존한다.
- 자동 decision candidate는 확정되지 않으면 정책 기반 단기 만료 대상으로 둔다.
- raw source는 처음부터 복제하지 않고 repository/Git/보안 저장소의 자체 retention을 따른다.
- Workspace close 후에는 read-only 보존을 기본으로 하고 export를 지원한다.
- 법적 삭제나 secret 유출은 일반 correction이 아니라 제한 권한 redaction workflow로 처리하며, 가능한 최소 비식별 audit tombstone만 남긴다.

## 설계상 추론 — 기존 데이터 backfill

| source | 가능한 backfill | confidence | 금지할 추론 |
| --- | --- | --- | --- |
| `task.rework_started.reason`, discard/gate human action | decision candidate와 actor/time/link | high | 기록되지 않은 대안 생성 |
| Task description/assessment + Run summary | 질문·맥락·선택·actual outcome 후보 | medium | prose를 human approval로 승격 |
| Record supersedes | 문서 version chain | high | 이를 decision supersedes로 자동 변환 |
| parent Task + 제목·시간 순서 | amendment 후보 | low/medium | #181→#180 관계를 확정 사실로 저장 |
| approval attestation | human action과 시각 | high | hash/ref에서 승인 문장 복원 |
| recovery.reconstructed 이전 row | 제한 표시가 붙은 candidate | low | 잃어버린 command/Event/UUID 발명 |

권장 backfill은 dry-run candidate 생성 → confidence/근거 표시 → 사람 batch review → append-only import 순서다. alternatives와 rationale가 없으면 `unknown`, 예상 결과가 없으면 `not_recorded`로 남겨야 한다.

우선 backfill 후보:

- Task #121 rework Decision: high confidence
- Bird View pause: 내용은 medium, actor authority는 human review 필요
- #181이 #180을 보완한 amendment: medium 이하, 사람 확인 필요
- 44개 승인 attestation: action Decision 후보는 high지만 제품 판단 맥락은 별도 보완 필요

23개 Task create Event, 14개 Run Event, 31개 Record Event의 원본은 recovery Event가 명시한 대로 복구할 수 없다. backfill이 이를 “원래 Event”처럼 위장하면 안 된다.

## 최소 구현 순서와 후속 Task 후보

이 Task에서는 구현하지 않는다. 다음은 별도 Task 후보와 순서다.

1. **Decision log 계약·migration**
   - `decisions`, `decision_events`, `decision_relations`, `decision_links`, Workspace counter와 invariant
   - append-only/correction/redaction acceptance test
2. **Decision command·권한 계약**
   - propose, accept/reject, record outcome, correct, withdraw, supersede/amend, link/retract
   - operator/human-required/Owner approval boundary와 browser grant 결속
3. **Task/Run/Record/Event/commit/evidence 자동 link**
   - command transaction hook, candidate advisory, no-rationale-invention rule
4. **Decision search·timeline·session recovery read model**
   - relation traversal, expected-vs-actual, stale summary/next-action 진단
5. **Sanitized backfill dry-run과 human review 도구**
   - confidence, source row, recovery boundary, import audit
6. **Viewer Decision Inspector와 recovery brief**
   - read-first UI, human approval surface, restricted-content masking

DB 구현 전에 즉시 적용할 최소 운영 습관은 상세계획·완료보고의 `decision journal`에 `질문 / 대안 / 선택 / 근거 / 예상 결과 / 관련 Task·Run·commit`을 시간순으로 쓰는 것이다. 단 이것은 임시 bridge이며 searchable cross-Task Decision identity를 대신하지 않는다.

## Decision journal — 이 조사에서 내린 판단

| 시각(KST) | 조사·설계 판단 | 근거 | 결과 |
| --- | --- | --- | --- |
| 2026-09-08 11:45 | hosted-pilot 문서의 원격 DB를 현재 운영 대상으로 가정하지 않는다. | 두 원격 service가 inactive였다. | 현재 실행 중인 loopback API의 DB target을 추적했다. |
| 2026-09-08 11:48 | `local-dev-postgres/baley`를 감사 대상으로 고정하고 Bird View DB를 제외한다. | API URL의 host/database 일치, Bird View의 별도 container/port/volume/database. | 이후 SQL을 main DB와 대상 Workspace에만 실행했다. |
| 2026-09-08 11:50 | 규범 문서보다 운영 catalog를 query 작성의 literal 근거로 쓴다. | 실제 ID type과 Event actor column명이 문서 예시와 달랐다. | `information_schema`와 `pg_constraint`를 먼저 읽었다. |
| 2026-09-08 11:55 | revision 1319와 고정 시각을 정량 snapshot으로 사용한다. | Task #182 plan bookkeeping 이후 일관된 수치 기준이 필요했다. | 이후 Run continuation은 수치에서 제외했다. |
| 2026-09-08 12:02 | FK orphan과 historical incompleteness를 분리한다. | FK orphan은 0이지만 recovery로 Task/Run/Record Event가 누락됐다. | “관계 무결성 양호, 역사 완전성 불완전”으로 결론냈다. |
| 2026-09-08 12:08 | backlog/workspace_graph Event를 orphan으로 단정하지 않는다. | backlog entity ID는 public integer, workspace graph는 aggregate literal이었다. | entity type별 ID 의미를 별도로 문서화했다. |
| 2026-09-08 12:14 | Task #121을 오래된 판단 변경 대표로 선택한다. | 전체 DB의 유일한 `task.rework_started`에 명시 reason과 후속 이력이 있었다. | 최초/변경/재구현/사람 confirm을 DB-only로 재구성했다. |
| 2026-09-08 12:20 | 최소 모델을 identity + append-only event + relation + link 네 table로 제한한다. | 현재 Event와 Task field를 더 늘리면 cross-Task 판단, supersedes/amends와 예상/실제 결과가 다시 분산된다. | current status는 event projection으로 두고 과거 판단을 덮어쓰지 않는다. |
| 2026-09-08 12:25 | 자동화는 link/candidate만 만들고 rationale·대안을 생성하지 않는다. | 현 DB의 prose에서 #181→#180 amendment는 추론 가능하지만 확정 사실은 아니다. | backfill과 실시간 캡처 모두 confidence와 human review를 요구한다. |

## 최종 판정

현재 DB만으로 Baley의 개발사를 **부분적으로** 설명할 수 있다. 실행 사실, 상태 전이, 시각, 대부분의 executor, Run 결과, Record/commit/evidence link는 강하다. 그러나 선택의 비교 구조와 인간 판단 provenance는 사례마다 prose에 우연히 남아 있고, Task/Run/Record를 가로지르는 안정적인 decision thread가 없다.

따라서 첫 구현 목표는 거대한 지식 시스템이 아니라 다음 한 문장이어야 한다.

> 중요한 선택마다 D# identity를 만들고, 질문·대안·선택·근거·예상 결과를 append-only로 기록한 뒤, 실제 Task/Run/Record/Event/commit/evidence와 결과를 자동으로 연결한다.
