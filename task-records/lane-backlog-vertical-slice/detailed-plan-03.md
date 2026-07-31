---
baley_record: 1
record_id: "5c43f0f1-f0f0-48e1-85b2-231d4a5971ac"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: detailed-plan
run_id: "8ca2a1e6-0918-4243-a947-d051c01bc2c7"
created_at: "2026-07-26T16:30:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "fc487caa-3890-440c-8270-eb53bae551c6"
---

# Workspace-scoped Mutation Attempt Audit 상세 계획

## 1. 목표

Baley의 모든 execute write 시도를 Workspace별 append-only audit로 남긴다.
기존 Command/Event가 성공한 domain mutation을 설명하는 역할은 유지하고,
새 로그는 성공 전에 거부되거나 rollback된 시도와 application을 우회한 직접
PostgreSQL write를 보완한다.

## 2. 저장소 결정

`mutation_attempts` PostgreSQL table을 사용한다.

- 기존 Workspace/Command/Event와 같은 운영 경계에서 조회할 수 있다.
- main mutation transaction과 분리해 validation reject와 rollback 이후에도 남긴다.
- 별도 SQLite/file lifecycle, backup, rotation, 동기화 실패 지점을 만들지 않는다.
- DB trigger를 같은 audit stream에 연결해 direct SQL도 구분해 남길 수 있다.

로그 자체는 상태 정본이나 Event sourcing에 사용하지 않는다.

## 3. 최소 모델

```text
MutationAttempt
  id UUID
  workspace_id text nullable
  command_name text
  source command_service | database_trigger
  outcome succeeded | rejected | failed | idempotent
  entity_type text nullable
  entity_id text nullable
  initiated_by_actor_id text nullable
  executed_by_actor_id text nullable
  idempotency_key_hash text nullable
  argument_digest text nullable
  request_fingerprint text nullable
  command_hash text nullable
  expected_workspace_revision bigint nullable
  observed_workspace_revision bigint nullable
  diagnostic_codes text[]
  command_id text nullable
  event_ids text[]
  duration_ms bigint
  occurred_at timestamptz
```

민감정보 및 고카디널리티 원문은 저장하지 않는다.

- raw arguments 없음
- raw lease token 없음
- 승인 발화/attestation 원문 없음
- idempotency key는 SHA-256만 저장
- 오류 메시지/SQL text 없음
- diagnostic code와 request/argument digest만 저장

## 4. Application 경로

1. `Service.Execute` 진입 시 secure attempt UUID와 시작 시각을 만든다.
2. raw arguments에서 `workspaceId`와 target 식별자를 best-effort로 추출한다.
3. repository execute transaction에 attempt ID를 transaction-local context로 전달한다.
4. execute가 반환된 뒤 별도 짧은 transaction으로 attempt를 append한다.
5. 결과 분류:
   - commit 성공: `succeeded`
   - 기존 결과 반환: `idempotent`
   - `CommandError`: `rejected`
   - infrastructure/internal error: `failed`
6. audit append 실패는 이미 결정된 domain 결과를 뒤집지 않고 structured server
   error log를 남긴다.

decode/envelope validation처럼 repository transaction 전 거부도 가능한 Workspace
ID를 추출해 기록한다.

## 5. Direct PostgreSQL 경로

task/relationship/Backlog/Gate/Run/Record 관련 table에 generic trigger를 둔다.

- application transaction은 `baley.mutation_attempt_id` local setting을 설정하므로
  trigger 중복 기록을 생략한다.
- context가 없는 row INSERT/UPDATE/DELETE는 `sql.<op>.<table>`로 기록한다.
- TRUNCATE는 실행 전 table에 존재하는 Workspace별로 기록한다.
- `mutation_attempts` 자체의 UPDATE/DELETE/TRUNCATE는 trigger로 거부한다.

Trigger audit는 같은 SQL transaction에 속한다. direct SQL이 rollback되면 실제
변경도 없으므로 audit도 rollback된다. commit된 우회 write만 durable하게 남긴다.

## 6. 조회

- HTTP:
  `GET /v1/workspaces/{workspaceId}/mutation-attempts`
- filters:
  `after`, `limit`, `outcome`, `commandName`
- 기본/최대 limit: 100/500
- 정렬: `(occurred_at, id)`
- typed MCP read tool:
  `baley_mutation_attempt_list`
- Viewer 편집 기능은 추가하지 않는다.

## 7. 구현 순서

1. contract literal과 migration `00009_mutation_attempts.sql`
2. application projection/recorder interface와 execute wrapper
3. PostgreSQL append/query와 transaction-local context
4. direct-write/append-only triggers
5. HTTP와 typed MCP read tool
6. system spec, command architecture, incident runbook 보정
7. 전용 DB integration 및 전체 회귀

## 8. Acceptance criteria

- [ ] 두 Workspace의 로그가 섞이지 않고 각각 조회된다.
- [ ] task.create 성공이 command/event ID와 함께 `succeeded`로 기록된다.
- [ ] stale revision, invalid transition과 malformed arguments가 `rejected`로 기록된다.
- [ ] idempotency 재호출이 `idempotent`로 기록된다.
- [ ] 주 mutation rollback 이후에도 application attempt가 남는다.
- [ ] raw payload, token, idempotency key 원문이 저장되지 않는다.
- [ ] application mutation은 DB trigger와 중복되지 않는다.
- [ ] direct SQL task write와 TRUNCATE가 `database_trigger` source로 기록된다.
- [ ] audit table UPDATE/DELETE/TRUNCATE가 거부된다.
- [ ] workspace-scoped HTTP/MCP query가 pagination/filter를 적용한다.
- [ ] production DB safety guard와 기존 Command/Event 계약이 회귀 통과한다.
