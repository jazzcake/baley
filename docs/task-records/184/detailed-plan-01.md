---
baley_record: 1
record_id: "1974dce4-5b32-42d1-9697-2953e88fa6ea"
task_id: 184
record_type: detailed-plan
run_id: "382e0060-ae5b-47d1-b707-6b6e30e4355c"
created_at: "2026-09-08T21:18:35+09:00"
created_by: "codex-worker-term_6464e5ee"
supersedes: null
status: in_progress
---

# Task #184 상세계획 — 과거 Task lifecycle Event journal backfill

## 목표와 완료 계약

Migration 25 상태의 운영 DB를 변경하지 않은 채, reviewed Task #183 HEAD
`7a15b5711912272e164f24f9574eb2fb3b2860a2`를 기준으로 migration 26의
`task_journal_entries`에 과거 Task lifecycle Event의 **명시된 내용만** 복원하는
forward-only backfill을 구현한다. 완료 시 같은 Event를 몇 번 처리해도 한 row만
존재하고, Event와 journal의 Workspace·command·actor·시각 결속이 보존되며,
기존 journal API/UI/MCP가 새 row를 별도 adapter 없이 읽어야 한다.

Backfill은 reason, assessment, before/after, Run kind처럼 source payload에 실제로
존재하는 값만 이름을 바꾸지 않고 복사한다. reason·assessment·proceedReason가 없는
Event에는 rationale, goal, alternatives, completion contract를 생성하지 않는다.
최소 사실 projection이 필요한 lifecycle stage에는 `_backfill.narrativeState =
"not_recorded"`만 기록하여 Event가 존재했다는 사실과 narrative 부재를 구분한다.

## 조사 결과와 기준 증거

- 현재 worktree는 clean한 `jazzcake/task-journal-rollout`, HEAD는 예상한
  `7a15b5711912272e164f24f9574eb2fb3b2860a2`, base는
  `origin/jazzcake/development-decision-log`이다.
- migration 26은 Workspace 복합 FK, `(workspace_id,event_id)` 및
  `(workspace_id,command_id)` unique, append-only UPDATE/DELETE/TRUNCATE trigger를
  이미 제공한다. 새 Event journal은 Event ID를 journal ID로 사용한다.
- Task #183의 운영 감사 snapshot은 migration 25 DB에서 Event executor 100%와
  Task lifecycle history를 확인했다. PM review 뒤 `local-dev-postgres`의 `baley` DB를
  `REPEATABLE READ, READ ONLY` transaction으로 다시 조회해 Task #183 자체를
  entity로 갖는 정확한 Event 6건을 확보했다: rev1327 `task.created`, rev1328
  `task.started`, rev1351 `task.implemented_reported`, rev1352
  `task.acceptance_evidence_reported`, rev1353 `task.confirmed`, rev1354
  `task.terminal_cleared`.
- #183 implemented payload에는 명시적 `assessment`가 있다. confirmed payload에는
  `taskId`만 있고 narrative는 없으며 initiated/executed/approved actor가 모두
  Workspace Owner이고 command-bound approval attestation이 있다. 따라서 confirmed
  journal의 narrative는 반드시 NULL이고 `_backfill.narrativeState = not_recorded`여야
  한다.
- `task.acceptance_evidence_reported`와 `task.terminal_cleared`는 현재 Task Journal
  lifecycle stage enum 밖이다. 특히 terminal-cleared Event에는 #184 successor를
  설명하는 `proceedReason`이 있지만 이번 migration에서 stage를 몰래 확장하지 않고
  skip하며 runbook 잔여위험으로 기록한다.
- 모든 운영 조회 transaction은 `ROLLBACK`했다. 운영 DB에는 DDL/DML/migration을
  실행하지 않았다.

## 구현 단계

1. Task #183의 실제 entity Event 6건 ID/type/payload/actor/time과 confirm approval
   attestation을 sanitized repository fixture로 고정한다.
   credential이나 비밀 값은 포함하지 않는다.
2. migration 27을 추가한다. 하나의 transaction 안에서 eligible lifecycle Event를
   Event ID 순서와 무관한 set projection으로 `INSERT ... SELECT`하고
   `ON CONFLICT (workspace_id,event_id) DO NOTHING`으로 재시작을 안전하게 한다.
   journal ID는 Event ID, occurred/recorded time은 원 Event `created_at`, actor는 원
   Event actor column을 그대로 사용한다.
3. lifecycle 별 explicit projection을 좁게 정의한다.
   - `task.created`: source Task snapshot 중 명시된 title/description/currentSummary/terminalReason
   - `run.started`: kind/clientRunId/sessionRef
   - `task.updated`: before/after
   - rework/block/unblock/discard: reason
   - implemented: assessment/proceedReason/warnings/acknowledgedWarningCodes
   - confirmed: proceedReason가 있으면 복사하고, 없으면 narrative 부재 메타만 기록
   이미 `payload.taskJournal`이 있는 #183 이후 Event와 이미 journal row가 있는
   Event는 건너뛴다.
4. migration 통합 테스트에서 #183 snapshot fixture를 migration 26 상태 DB에 넣고
   migration 27을 적용한다. implemented와 confirmed 조회, source provenance,
   approval attestation 결속, ordering/pagination, Workspace 격리, API read를 확인한다.
5. 같은 data migration을 다시 적용해 idempotency를 확인하고, 강제 insert 실패 뒤
   goose version·journal row가 모두 rollback되는지 확인한다. 기존 append-only,
   command transaction rollback, UI/MCP 테스트는 약화하지 않는다.
6. 제품 계약 문서와 운영 적용 runbook을 갱신한다. runbook은 pg_dump, disposable
   restore, migration/build/service 교체, health/Tailscale smoke, rollback 기준과 명령을
   포함하되 방화벽과 Tailscale serve rule 변경을 금지한다.

## 검증 계획

- `go test ./...` 및 `go vet ./...` (`server/`)
- `npm test -- --run` 및 `npm run build`
- migration 27 통합 테스트 단독 실행(실제 disposable PostgreSQL)
- `C:\dev-bin\baley\` 아래에 server/mcp executable을 build하고, 격리 disposable
  DB에서 migration과 read-only journal API smoke를 수행한다. `go run`은 사용하지
  않는다.
- 최종 diff·status·branch를 확인하고 새 commit을 origin에 push한다.

## 위험과 rollback 원칙

- append-only data migration은 down에서 과거 row를 삭제하지 않는다. rollback은
  이전 binary/service로 되돌리고 schema 26+27과 backfilled row는 보존한다.
- 예상 건수·provenance·API smoke가 disposable restore에서 맞지 않으면 운영 적용을
  중단한다. 운영 migration이나 service 교체는 이 구현 Run 범위 밖이다.
- migration transaction 실패 시 version 26과 0 backfill row로 원자 rollback돼야 한다.
  재실행은 같은 Event ID unique key로 수렴한다.

## 실행 기록(현재까지)

- `git status --short --branch`, `git log -5`: clean branch와 기준 HEAD 확인.
- `orca status --json`, `orca orchestration dispatch-show ...`: runtime ready 및 현재
  Dispatch authority 확인.
- `docker exec ... psql ... BEGIN ... READ ONLY ... ROLLBACK`: Task #121 예비 조사 뒤
  PM 지시에 따라 이를 fixture 기준에서 폐기하고, Task #183 entity Event 6건과
  confirm approval attestation을 다시 읽기 전용으로 확인; DB mutation 없음.
