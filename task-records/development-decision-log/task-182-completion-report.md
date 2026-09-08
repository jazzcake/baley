---
baley_record: 1
record_id: "03f7fdb0-52bd-4d24-a0e1-0529f09b4db4"
task_id: 182
record_type: completion-report
run_id: "a0f97859-c040-48ea-a546-49cf73135233"
created_at: "2026-09-08T12:30:00+09:00"
created_by: "codex-operator"
supersedes: null
---

# Task #182 완료 보고 — 개발 의사결정 로그 운영 DB 감사

## 결과

현재 Baley API가 실제로 사용하는 운영 PostgreSQL을 Bird View 격리 DB와 구분한 뒤, 직접 read-only SQL로 schema·관계·row count·연결률·시간 분포와 대표 의사결정 사례를 조사했다. 확인된 사실과 설계상 추론을 분리한 감사 보고서와 sanitized 재현 SQL을 작성했다.

주요 결론은 현재 DB가 실행·상태·증거 timeline은 상당히 잘 보존하지만, decision ID, 검토 대안, 선택 근거, 예상·실제 결과 대응과 decision 간 supersedes/amends를 일관된 단위로 보존하지 않는다는 것이다.

## 산출물

- `docs/analysis/development-decision-log-db-audit.md`
- `docs/analysis/development-decision-log-readonly.sql`
- `task-records/development-decision-log/task-182-detailed-plan.md`
- 이 완료 보고

## 수치 근거 요약

감사 snapshot은 2026-09-08 11:55:35 KST, Workspace revision 1319다.

- Task 76, Run 277, Task Record index 191
- command 1,205, Event 1,237, mutation attempt 1,400
- commit reference 34, Run Git observation 4, typed acceptance evidence 6
- human approval attestation 44, approval grant 1
- 선언 FK의 Task/Run/Record/commit/evidence orphan 0
- Task 23개에 원래 create Event 없음, Run 14개·Record 31개에 원래 lifecycle Event 없음
- Task별 commit reference 27.6%, typed acceptance evidence 5.3%, current summary 53.9%, next action 7.9%
- Event의 전용 alternatives/rationale/expectedOutcome/actualOutcome/supersedes/amends key 0건

## 대표 사례

- Task #179: Bird View pause는 Task 상태 변경이 아니라 완료보고 Run summary, produced commit 2개와 Git observation에만 남았다. “PM decision”이라는 prose는 있으나 human approver actor link는 없다.
- Task #180/#181: 78개 catalog를 14개로 축소한 뒤 rework 복구 도구를 추가해 15개로 조정한 흐름은 복구된다. #181은 #180의 parent지만 정식 amends/supersedes relation은 없다.
- Task #121: 첫 implemented 후 confirmation 전에 mutation-attempt audit와 direct-write 방어를 추가하기로 한 유일한 rework reason, 재구현·리뷰·재보고·human confirm 흐름이 복구된다.

## 설계 제안

최소 모델은 다음 네 요소다.

1. 안정적인 Workspace-scoped `D#<n>` decision identity
2. 질문·맥락·대안·선택·근거·예상·실제 결과와 상태를 보존하는 append-only decision event
3. decision 간 `supersedes|amends` relation
4. Task/Run/Record/Event/command/commit/evidence link

자동화는 link와 candidate만 만들고 rationale이나 대안을 추정해 확정하지 않는다. 중요한 선택 직전과 pause/rework/discard/supersede 시점에는 사람 또는 Agent가 짧은 decision note를 직접 기록한다.

## 검증

- 모든 조사 SQL을 `BEGIN TRANSACTION ... READ ONLY`와 `SELECT`로 실행하고 명시적 `ROLLBACK`을 확인했다.
- sanitized SQL 파일을 운영 대상에서 snapshot 변수와 함께 처음부터 끝까지 재실행했다: PASS.
- 보고서의 필수 조사·설계 섹션 존재 검사: PASS.
- 산출물 내 credential-bearing PostgreSQL URI 검사: 0건.
- `git diff --check`: PASS.
- 서비스, 방화벽, Tailscale, 배포, DB schema/data/settings, 운영 branch를 변경하지 않았다.
- `go run`, commit, push를 실행하지 않았다.

## 잔여 위험과 한계

- Task current row는 event-sourced snapshot이 아니므로 사후 변경 뒤 당시 projection을 완전히 재현할 수 없다.
- 2026-07-26 recovery 이전의 원래 command/Event와 일부 internal identity는 DB에서 복구 불가능하다.
- Task Record 본문, chat·terminal 원문과 Git diff는 DB에 없으므로 설계 backfill은 source별 confidence와 human review가 필요하다.
- 독립 Agent review는 이번 세션에 요청되지 않았고 수행하지 않았다. 이는 분석 근거의 직접 SQL 재현성과 별개인 잔여 검토 항목이다.

## 후속 Task 후보

1. Decision log 계약·migration
2. Decision command와 사람 승인 권한
3. Task/Run/Record/Event/commit/evidence 자동 link
4. Decision search·timeline·session recovery read model
5. Sanitized backfill dry-run과 human review
6. Viewer Decision Inspector

Baley 사람 confirmation은 별도 human-only 결정이며 이 세션에서는 실행하지 않는다.
