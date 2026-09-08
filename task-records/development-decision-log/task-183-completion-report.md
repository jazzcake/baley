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
- 정규화된 `schemaVersion`, trimmed `narrative`, canonical JSON `context`와 `contextDigest`를 lifecycle Event payload의 `taskJournal` seed에 먼저 영속화한다. Task target은 Event의 기존 `task`/`taskId`, stage는 Event type에서 유도한다.
- repository는 저장된 source Event를 transaction 안에서 다시 읽어 Journal을 투영한다. Journal ID와 occurred/recorded time도 Event ID와 `created_at`을 그대로 사용하므로 동일 Event replay가 같은 projection을 만든다.
- projection insert는 command, Event, Task/Run 상태 전이와 같은 PostgreSQL transaction 안에서 실행된다. CAS 실패, 승인 실패, validation 실패 또는 projection insert 실패는 Journal을 포함한 전체 변경을 rollback한다.
- 동일 idempotency key와 동일 입력은 기존 결과 및 같은 journal entry ID를 재사용하고, 같은 key에서 `contextNote`가 달라지면 충돌한다.
- 같은 `clientRunId`의 cross-key `run.start` recovery는 원래 Task target과 Event-backed context digest를 비교한다. 모두 같을 때만 기존 command·Journal·lease 결과를 재사용하고, Task/context가 다르면 `idempotency_conflict`다.
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
- compact profile의 generic bridge와 full profile의 typed lifecycle tool 모두 선택적 `contextNote`를 전달한다. full profile은 task create/update, run start, implemented report, confirm/discard의 기존 typed schema와 argument map을 확장하고, rework/block/unblock preview·execute typed tool 6개를 추가했다.
- `contextNote`가 nil이면 argument map에 key를 만들지 않아 기존 wire/hash를 유지한다. catalog version은 `1.1.0`, compact는 15 tools / 4,700 bytes, full은 89 tools / 46,217 bytes다.

## Viewer와 진단

기존 Task Inspector의 confirmation 영역 바로 앞에 최신 Task Journal 50개를 읽기 전용으로 표시한다. 별도 form, Decision page 또는 workflow는 추가하지 않았다. React 경계에는 development-only structured trace를 남겨 user event, 계산된 대상 Task/revision, React state, request/controller 상태, store commit/failure, 최종 DOM render를 연결했으며 Task 전환 시 stale request는 abort한다.

## Skill과 플러그인 경계

canonical `.agents/skills/baley-manage-work/SKILL.md`에 silent extraction, 비추론 규칙, lifecycle mapping, preview/execute/idempotency/approval 동일 입력 규칙과 confirm 전 Journal 조회를 기록했다. 설치된 plugin cache를 직접 수정하지 않았다. 기존 `scripts/install-baley-codex-plugin.ps1`가 canonical `.agents/skills/baley-*`를 설치 산출물로 복사하므로 source-derived 경계를 유지했다.

## 변경 산출물

- 계약/문서/skill: `.agents/skills/baley-manage-work/SKILL.md`, `contracts/v1/commands.json`, `docs/baley-command-architecture.md`, `docs/baley-system-spec-v1.md`, `docs/streamable-http-mcp-operations.md`.
- 서버/DB/API: `server/migrations/00026_task_journal.sql`, application types/service, PostgreSQL repository, HTTP router, readiness/schema catalog.
- MCP: catalog, handler, profile/schema/URL tests.
- Viewer: `src/App.tsx`, API client/domain model/styles 및 관련 auth/navigation/client tests.
- 검증: migration 26, application unit, integration journal/auth/rollback/approval tests와 기존 migration count updates.
- 보존·귀속한 Task #182 선행 산출물: `docs/analysis/development-decision-log-db-audit.md`, `docs/analysis/development-decision-log-readonly.sql`, `task-records/development-decision-log/task-182-detailed-plan.md`, `task-records/development-decision-log/task-182-completion-report.md`.
- 계획/보고: `task-records/development-decision-log/task-183-detailed-plan.md`, PM 산출물 `task-183-independent-review.md`, 이 완료 보고서. 독립 리뷰 원문은 수정하지 않았다.

## 검증 결과

- `go test ./... -count=1` with disposable PostgreSQL 17.5: PASS, 모든 package 성공, integration 21.015s.
- `go vet ./...`: PASS.
- `npm test -- --silent`: PASS, 17 test files / 108 tests, 4.80s.
- `npm run build`: PASS, Vite production build 생성.
- `git diff --check`: PASS; Git의 기존 LF→CRLF working-copy warning만 출력.
- JSONB round trip/containment, source Event time/provenance, lifecycle coverage, backlog promotion, deterministic pagination/order, cross-Workspace isolation을 실제 PostgreSQL integration test로 검증했다.
- identical retry no-duplicate, changed-context idempotency conflict, stale CAS no-row, injected projection failure full rollback을 검증했다.
- approval grant 이후 context 변경 mismatch, 정확한 context confirm, 별도 grant discard, approved actor provenance를 검증했다.
- unauthenticated 401, cross-Workspace 404와 invalid query validation을 검증했다.
- append-only UPDATE/DELETE/non-empty TRUNCATE 거부와 migration up/down을 검증했다.
- lifecycle Event payload seed/digest와 저장된 Event → Journal rebuild 동등성, source Event ID/time 기반 stable envelope를 검증했다.
- context가 있는 `run.start`의 same-key 및 same-clientRunId cross-key 동일 recovery가 원래 command/Journal/lease를 재사용하고, changed/omitted context와 다른 Task target이 `idempotency_conflict`를 반환함을 검증했다.
- full-profile typed lifecycle 16개 surface의 optional schema, present forwarding, absent key 보존과 typed confirm context 변경 approval mismatch를 검증했다.

리뷰 수정 검증에는 `127.0.0.1:55484`의 일회성 `baley-task183-review-fix` PostgreSQL 17.5 container만 추가 사용했고 검증 후 제거했다. `go run`은 사용하지 않았고 로컬 Go executable도 만들 필요가 없었다.

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

- 구현 커밋: `6da2c5c1560cb82de5e12480ce1962f393bd1c51` (`feat: add seamless task journal`).
- 독립 리뷰: `task-records/development-decision-log/task-183-independent-review.md`, verdict `CHANGES_REQUIRED`, blocking 1건과 material 2건. 원문 SHA-256은 `44B3707E91F2AC068539C5926BFB4B369A1F41B3D3375CBB1F32FB4BE4B8C29E`이며 리뷰 수정 중 파일을 변경하지 않았다.
- blocking(Event source/rebuild): Event-first normalized seed/digest와 persisted-Event projection, rebuild 동등성 test로 수정했다.
- material(run.start recovery): Task target/context digest comparison과 same/different cross-key integration test로 수정했다.
- material(full typed MCP): 모든 명시된 typed lifecycle schema/argument forwarding, absent compatibility와 approval mismatch test로 수정했다.
- 리뷰 수정 커밋: 생성 예정.
- 이 보고서는 finding 수정 완료를 기록하지만 독립 재리뷰 통과를 주장하지 않는다. Task의 `implemented` 전환은 재리뷰에서 unresolved blocking/material finding 0건을 확인한 뒤에만 가능하다.

## 운영 불변

운영 DB/schema/data/settings, 실행 중 서비스, 방화벽, Tailscale, 배포 환경과 운영 branch를 변경하지 않았다. push, merge, deploy도 수행하지 않았다.
