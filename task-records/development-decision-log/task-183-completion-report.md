---
baley_record: 1
record_id: "6cc9d827-24a1-4809-9717-730207f9add7"
task_id: 183
record_type: completion-report
run_id: "3239cb0a-a0b9-4c87-8322-bd51998fd153"
created_at: "2026-09-08T16:50:00+09:00"
created_by: "codex-operator"
supersedes: null
---

# Task #183 완료 보고 — Seamless Task Journal

## 결과

기존 Task 자연어 lifecycle에 선택적 `contextNote`를 추가하고, 그 명령이 만든 기존 Event를 source of truth로 삼는 append-only Task Journal projection을 구현했다. 별도 D# identity, Decision workflow, 입력 화면 또는 추가 확인 prompt는 만들지 않았다. `contextNote`가 없으면 종전 command JSON, canonical hash, idempotency fingerprint와 동작을 그대로 유지하며, 값이 있으면 preview·execute·approval grant가 같은 정규화된 입력에 결합된다.

사용자가 실제로 말한 문제, 지금 필요한 이유, 상황 변화, 목표, 완료 계약, 대안, 판단 수정, 결과만 `baley-manage-work` skill이 조용히 구조화하도록 계약했다. 누락된 rationale나 alternative는 생성하지 않고 빈 `contextNote`는 wire에서 생략한다.

## 구현 경계

- 공통 경로: typed lifecycle argument → canonical command/hash → plan → repository transaction → Event → Task Journal projection.
- Task 생성, backlog promotion adapter, 첫 Run 시작, update, rework, block, unblock, implemented report, confirm, discard를 지원한다.
- Task Journal 행은 Event와 command를 Workspace 복합 FK로 연결하며 Event 발생 시각과 actor/provenance를 보존한다.
- projection insert는 command, Event, Task/Run 상태 전이와 같은 PostgreSQL transaction 안에서 실행된다. CAS 실패, 승인 실패, validation 실패 또는 projection insert 실패는 Journal을 포함한 전체 변경을 rollback한다.
- 동일 idempotency key와 동일 입력은 기존 결과 및 같은 journal entry ID를 재사용하고, 같은 key에서 `contextNote`가 달라지면 충돌한다.
- 브라우저 승인 명령은 `contextNote`를 포함한 canonical command hash를 grant에 결합한다. grant 발급 후 내용을 바꾼 confirm/discard는 거부한다.

## 데이터 모델과 마이그레이션

`00026_task_journal.sql`은 schema version 26을 도입한다.

- 안정적인 envelope: identity, Workspace/Task/Event/command, lifecycle stage, narrative, `schema_version`, JSONB `context`, actor, occurred/recorded time.
- lifecycle stage check와 non-empty narrative, JSON object, positive version, time ordering 제약.
- Workspace/Event 및 Workspace/command unique/FK 제약으로 cross-Workspace provenance를 차단한다.
- Workspace 및 Task별 `(recorded_at, id)` 내림차순 keyset 조회 인덱스와 JSONB GIN 인덱스.
- UPDATE/DELETE 금지 row trigger와 비어 있지 않은 table의 TRUNCATE 금지 statement trigger.
- down migration은 projection과 전용 trigger/function 및 추가 복합 unique constraint를 제거한다.

배포 순서는 migration 26 적용 → readiness가 expected schema 26을 확인 → 새 서버 시작이다. 테이블과 unique constraint 생성은 대상 규모에 따라 DDL lock을 잡을 수 있으므로 운영 migration 창에서 먼저 적용해야 한다. 이번 실행에서는 운영 DB에 migration을 적용하지 않았다.

## 읽기 계약

- HTTP Task query: `GET /v1/workspaces/{workspaceId}/tasks/{publicId}/journal`
- HTTP Workspace query: `GET /v1/workspaces/{workspaceId}/task-journal`, 선택적 `taskId`
- cursor: `(recordedAt, id)` pair, 기본 50개, 최대 100개, 내림차순 deterministic order.
- 두 route 모두 기존 Workspace authentication/authorization을 통과하며 다른 Workspace의 Task/provenance는 반환하지 않는다.
- MCP read-only tool `baley_task_journal`을 compact/full profile에 제공한다.
- 기존 full profile lifecycle tool의 개별 typed schema는 호환성과 catalog 크기를 위해 확장하지 않았다. 두 profile 모두의 generic bridge가 `additionalProperties`로 선택적 `contextNote`를 전달하고, canonical contract와 skill이 사용법을 정의한다.

## Viewer와 진단

기존 Task Inspector의 confirmation 영역 바로 앞에 최신 Task Journal 50개를 읽기 전용으로 표시한다. 별도 form, Decision page 또는 workflow는 추가하지 않았다. React 경계에는 development-only structured trace를 남겨 user event, 계산된 대상 Task/revision, React state, request/controller 상태, store commit/failure, 최종 DOM render를 연결했으며 Task 전환 시 stale request는 abort한다.

## Skill과 플러그인 경계

canonical `.agents/skills/baley-manage-work/SKILL.md`에 silent extraction, 비추론 규칙, lifecycle mapping, preview/execute/idempotency/approval 동일 입력 규칙과 confirm 전 Journal 조회를 기록했다. 설치된 plugin cache를 직접 수정하지 않았다. 기존 `scripts/install-baley-codex-plugin.ps1`가 canonical `.agents/skills/baley-*`를 설치 산출물로 복사하므로 source-derived 경계를 유지했다.

## 변경 산출물

- 계약/문서/skill: `.agents/skills/baley-manage-work/SKILL.md`, `contracts/v1/commands.json`, `docs/baley-command-architecture.md`, `docs/baley-system-spec-v1.md`.
- 서버/DB/API: `server/migrations/00026_task_journal.sql`, application types/service, PostgreSQL repository, HTTP router, readiness/schema catalog.
- MCP: catalog, handler, profile/schema/URL tests.
- Viewer: `src/App.tsx`, API client/domain model/styles 및 관련 auth/navigation/client tests.
- 검증: migration 26, application unit, integration journal/auth/rollback/approval tests와 기존 migration count updates.
- 보존·귀속한 Task #182 선행 산출물: `docs/analysis/development-decision-log-db-audit.md`, `docs/analysis/development-decision-log-readonly.sql`, `task-records/development-decision-log/task-182-detailed-plan.md`, `task-records/development-decision-log/task-182-completion-report.md`.
- 계획/보고: `task-records/development-decision-log/task-183-detailed-plan.md`, 이 완료 보고서.

## 검증 결과

- `go test ./... -count=1` with disposable PostgreSQL 17.5: PASS, 모든 package 성공, integration 18.947s.
- `go vet ./...`: PASS.
- `npm test -- --silent`: PASS, 17 test files / 108 tests, 4.17s.
- `npm run build`: PASS, Vite production build 생성.
- `git diff --check`: PASS; Git의 기존 LF→CRLF working-copy warning만 출력.
- JSONB round trip/containment, source Event time/provenance, lifecycle coverage, backlog promotion, deterministic pagination/order, cross-Workspace isolation을 실제 PostgreSQL integration test로 검증했다.
- identical retry no-duplicate, changed-context idempotency conflict, stale CAS no-row, injected projection failure full rollback을 검증했다.
- approval grant 이후 context 변경 mismatch, 정확한 context confirm, 별도 grant discard, approved actor provenance를 검증했다.
- unauthenticated 401, cross-Workspace 404와 invalid query validation을 검증했다.
- append-only UPDATE/DELETE/non-empty TRUNCATE 거부와 migration up/down을 검증했다.

검증에는 `127.0.0.1:55483`의 일회성 `baley-task183-test` container만 사용했으며 완료 후 제거한다. `go run`은 사용하지 않았고 로컬 Go executable도 만들 필요가 없었다.

## 설계 판단과 선행 감사와의 관계

Task #182의 audit와 read-only SQL을 그대로 입력으로 사용했으며 운영 감사를 반복하지 않았다. #182가 제시한 별도 Decision identity/관계 모델은 이후 PM 계약에서 명시적으로 축소되었으므로, 이 Task는 D#나 추론된 rationale backfill 없이 Event 기반 Task Journal projection만 추가했다. 과거 Event는 사실에 없는 context를 복원하지 않기 위해 backfill하지 않는다.

## 잔여 위험과 운영 주의

- forward-only이므로 migration 이전 lifecycle은 Journal에 나타나지 않는다. 거짓 context를 만드는 backfill보다 의도된 안전성이다.
- `schema_version = 1` JSONB key 의미는 contract와 skill이 관리한다. 새로운 의미는 version 증가와 호환 reader가 필요하다.
- Viewer는 최신 50개만 표시한다. 전체 이력은 cursor 기반 HTTP/MCP query로 읽는다.
- append-only projection은 계속 증가하므로 장기 retention/partition/size monitoring은 운영 후속 과제다.
- production build는 성공했지만 Vite가 기존 약 1.95 MB chunk의 500 kB 초과 warning을 출력했다.
- `npm ci`는 현재 lockfile에서 6개 dependency audit finding(1 moderate, 5 high)을 보고했다. 범위 밖 자동 upgrade는 하지 않았다.
- migration의 unique constraint/table/index 생성에 운영 DDL lock 검토가 필요하다.

## 독립 리뷰와 커밋

- 구현 커밋: 독립 리뷰 전 생성 예정.
- 독립 리뷰: coordinator가 구현 커밋을 대상으로 별도 reviewer에게 dispatch할 예정. blocking/material finding은 수정 후 재검증·재리뷰한다.

## 운영 불변

운영 DB/schema/data/settings, 실행 중 서비스, 방화벽, Tailscale, 배포 환경과 운영 branch를 변경하지 않았다. push, merge, deploy도 수행하지 않았다.
