---
baley_record: 1
record_id: "caa23c84-5ce2-4dda-a3f8-68f21dc19b04"
task_id: 184
record_type: detailed-plan
run_id: "e592d93c-9e70-4ef7-bb42-74411b76f006"
created_at: "2026-09-10T01:21:37+09:00"
created_by: "codex-worker-term_d331bf0b"
supersedes: "1974dce4-5b32-42d1-9697-2953e88fa6ea"
status: ready_for_implementation
---

# Task #184 상세계획 02 — Task Journal 운영 완결 acceptance contract

## 1. 결론

Task #184의 완료는 migration 27 파일이 추가되거나 서비스가 재시작되는 시점이 아니다. 아래 결과가 모두 참일 때만 완료다.

1. reviewed #183 구현을 운영 branch에 정확히 통합하고 migration 27의 역사 projection이 원 Event에 명시된 사실만 사용한다.
2. schema 25 운영 DB의 검증 가능한 사전 backup과 격리 restore drill을 마친 뒤 schema 27로 전환한다.
3. API, Viewer, 로컬 MCP Gateway를 동일한 배포 commit으로 전환하고 `/versionz`로 그 commit과 schema 27을 식별할 수 있다.
4. #183의 역사 Journal을 API, Viewer, MCP에서 동일하게 조회하고, 새 canary Task가 기존 자연어 Task 흐름 안에서 Journal을 기록한다.
5. migration, 조회, 승인 결속, idempotency, rollback 경계를 자동화된 검증과 운영 smoke로 입증한다.
6. 독립 리뷰에서 blocking/material finding이 0이고, canary와 #184의 최종 `task.confirm`은 각각 signed-in Viewer의 별도 human approval로 처리한다.

`events`는 계속 source of truth이고 `task_journal_entries`는 재생 가능한 append-only read projection이다. 별도 Decision 엔터티, D# workflow, Journal 입력 폼, 추가 context 질문은 만들지 않는다.

## 2. 2026-09-10 읽기 전용 기준선

| 대상 | 확인된 사실 | 구현 시 보존 조건 |
| --- | --- | --- |
| feature worktree | branch `jazzcake/task-journal-rollout`, HEAD `7a15b5711912272e164f24f9574eb2fb3b2860a2` | #183 reviewed commits를 버리거나 재작성하지 않는다. |
| 미커밋 산출물 | `docs/task-records/184/detailed-plan-01.md`, `server/migrations/00027_task_journal_historical_backfill.sql`, `server/integration/testdata/task_journal_history_task_183.json` | 세 파일을 보존하고 review 가능한 commit으로 승격한다. |
| 운영 checkout | `D:\Project_AI\baley`, branch `jazzcake/mcp-login-membership-auth`, HEAD `ca44415`; remote tracking branch보다 1 commit 앞섬 | merge 전 fast-forward/ancestry를 다시 확인하고 운영 checkout의 고유 commit을 잃지 않는다. |
| 운영 checkout dirty state | untracked `debug.log` 954 bytes | 삭제하지 않는다. installer의 clean-tree gate 전에 소유자 보존 경로로 옮기거나 명시적으로 처리한다. |
| 운영 DB | container `local-dev-postgres` PostgreSQL 17.5, DB `baley`, schema version 25 | live DB에는 demo seed를 적용하지 않는다. migration 전 write quiesce와 backup/restore 검증이 필수다. |
| Baley DB role | `baley_app`, `local_admin`; 별도 `baley_backup` role 없음 | 존재하지 않는 backup role을 가정하지 않는다. 이번 rollout은 로컬 admin을 일회성 backup/restore에 사용하고 secret은 출력하지 않는다. role 신설은 별도 운영 개선이다. |
| Task 상태 | #183 `confirmed`, #184 `in_progress`, dependency `#183 -> #184` | terminal Task #183을 다시 쓰지 않는다. #184의 기존 parent/terminal reason/dependency를 유지한다. |
| 서비스 | `baley-api-1` healthy, `baley-viewer-1` running | `docker compose down`으로 전체 stack을 내리지 않는다. API/Viewer만 순차 교체한다. |
| API 식별 | `/readyz` = schema 25, `/versionz` = `dev/unknown/schema 25` | 새 배포는 commit/version을 식별 가능하게 빌드해야 한다. |
| MCP | PID의 binary `C:\dev-bin\baley\releases\39c58946924c\baley-mcp.exe`, loopback `127.0.0.1:8090` | Credential Manager와 credential-store metadata를 보존하고 revisioned binary로 교체한다. |
| Tailnet ingress | `https://jazzcake-home.tail87e929.ts.net/` -> Viewer 5174, `/api` same-origin proxy | Tailscale Serve와 firewall rule을 변경하지 않는다. 전후 status가 동일해야 한다. |

운영 DB의 현재 allow-list Event는 1,178건이다: `task.created` 190, `run.started` 691, `task.updated` 107, `task.rework_started` 1, `task.implemented_reported` 112, `task.confirmed` 68, `task.discarded` 9. 허용된 Event의 Task ID는 현재 모두 같은 Workspace의 Task로 해석된다. 실제 적용 건수는 API write를 멈춘 뒤 같은 query로 다시 freeze한다.

## 3. 기존 초안의 유효 범위와 누락

### 유지할 내용

- migration 26 이전 Event만 대상으로 하고 `payload.taskJournal`이 이미 있는 Event는 건너뛴다.
- Journal ID = source Event ID, `occurred_at`/`recorded_at` = source Event `created_at`으로 결정론적으로 투영한다.
- `reason`, `assessment`, `proceedReason`, before/after, Task snapshot, Run 필드처럼 source payload에 실제 존재하는 값만 복사한다.
- `(workspace_id,event_id)` idempotency, `(workspace_id,command_id)` 1-source-Event invariant, existing-row byte equivalence 검사를 유지한다.
- #183 sanitized fixture, disposable PostgreSQL 검증, backup/restore, Tailscale smoke, 독립 리뷰를 유지한다.

### 반드시 보완할 누락

1. 실제 `run.start` command는 같은 시각과 command ID로 `run.started`와 `task.started`를 모두 만든다. Journal source는 `run.started` 하나다. 현재 sanitized fixture에는 `task.started`만 있고 실제 `run.started` Event `1d49006d-4311-45c0-a56f-fd6a900aeaf3`가 빠져 있어 migration 27의 `run_started`를 검증하지 못한다.
2. migration 27 전용 통합 테스트가 없다. `server/integration/migration_27_test.go`를 추가해야 한다.
3. `server/cmd/baley-server/main.go`의 `expectedSchemaVersion`은 26이다. migration 27을 추가한 뒤 27로 올리지 않으면 새 API readiness가 실패한다.
4. migration 27 초안은 해석 실패 Event, missing Task, missing executor, entity mismatch를 조용히 제외할 수 있다. allow-list에 들어온 malformed Event는 skip하지 말고 migration 전체를 실패시켜야 한다.
5. 현재 Docker build는 `/versionz`를 `dev/unknown`으로 노출한다. 어느 commit이 배포됐는지 acceptance evidence로 사용할 수 없다.
6. 운영 branch의 `debug.log` 때문에 `install-baley-mcp-windows.ps1`의 clean-tree precondition이 현재 실패한다. 사용자 파일을 삭제하지 않는 선행 처리 절차가 필요하다.
7. migration 27 down은 row를 보존하면서 Goose version만 26으로 내린다. old API는 schema 25를 기대하고, migration 26 down은 Journal table을 삭제한다. 따라서 old image로의 단순 application rollback은 안전하지 않다.
8. 기존 backup helper는 현 compose에 더 이상 존재하지 않는 `postgres` service를 찾으므로 현재 운영 배치에서 그대로 사용할 수 없다. shared `local-dev-postgres` 전용 backup/restore 경로가 필요하다.
9. Viewer는 Task별 최신 50건만 표시하고 load-more가 없다. 이것은 이번 운영 통합의 허용된 제한이지만, 전체 역사는 HTTP/MCP cursor query로 접근 가능해야 하고 UI acceptance에 제한을 명시해야 한다.

## 4. 과거 Event backfill 계약

### 4.1 포함 범위

| source Event | Journal stage | Task 식별 | 복사 가능한 사실 |
| --- | --- | --- | --- |
| `task.created` | `created` | `payload.task.id` 또는 legacy `payload.task.ID` | title, description, currentSummary, terminalReason |
| `run.started` | `run_started` | `payload.taskId` | kind, clientRunId, sessionRef |
| `task.updated` | `updated` | `payload.taskId` | object인 before/after |
| `task.rework_started` | `rework_started` | `payload.taskId` | reason |
| `task.blocked` | `blocked` | `payload.taskId` | reason |
| `task.unblocked` | `unblocked` | `payload.taskId` | reason |
| `task.implemented_reported` | `implemented` | `payload.taskId` | assessment, proceedReason, warnings, acknowledgedWarningCodes |
| `task.confirmed` | `confirmed` | `payload.taskId` | source에 있을 때만 proceedReason |
| `task.discarded` | `discarded` | `payload.taskId` | reason, proceedReason, warnings, acknowledgedWarningCodes |

`task.started`는 `run.started`와 같은 command의 Task 상태 Event이므로 제외한다. `task.acceptance_evidence_reported`, `task.terminal_cleared`, Run terminal Event, Record/Git/Gate Event도 V1 Journal stage가 아니므로 제외한다. stage를 몰래 확장하지 않는다.

### 4.2 “명시된 사실만”의 판정

- narrative는 source의 `reason`, `assessment`, `proceedReason` 중 위 표가 지정한 값만 사용한다.
- title/description을 읽어 rationale, whyNow, goal, alternatives, judgmentUpdate, outcome을 요약하거나 분류하지 않는다.
- 빈 문자열, 잘못된 JSON type, 존재하지 않는 key는 값이 없는 것으로 취급한다.
- lifecycle Event의 존재, stage, command/Event/actor/time provenance 자체도 명시된 역사 사실이다. 따라서 narrative가 없는 역사 row는 허용하되 `narrative = NULL`, `context._backfill.narrativeState = "not_recorded"`로 표시한다. 이는 새로운 rationale가 아니라 “당시 narrative가 기록되지 않았다”는 결손 표식이다.
- 이 one-time 역사 정책을 신규 Event에 적용하지 않는다. 신규 command는 유의미한 `contextNote`가 있을 때만 Journal row를 만들며, 빈 note를 채우기 위한 질문이나 boilerplate를 생성하지 않는다.
- backfill row와 신규 row를 Viewer/MCP 소비자가 구분할 수 있도록 `_backfill.source = "historical_event"`, `projectionVersion = 1`을 유지한다.

### 4.3 fail-closed 조건

migration은 다음 중 하나라도 있으면 insert 전에 전체 transaction을 실패시킨다.

- allow-list Event의 Task ID가 비어 있거나 같은 Workspace Task와 연결되지 않음
- `run.started` 이외 source가 `entity_type='task'` 및 `entity_id=task_id`를 만족하지 않음
- `executed_by_actor_id`가 없거나 actor FK를 만족하지 않음
- 한 command에서 둘 이상의 allow-list source Event가 candidate가 됨
- 같은 Event/command에 기존 Journal row가 있으나 결정론적 projection과 byte-equivalent하지 않음
- source JSON type이 계약과 달라 값을 무시했는데 그 사실을 검증 query가 설명하지 못함

지원하지 않는 Event type은 의도적으로 제외하고, 적용 전/후 type별 source/candidate/inserted/skipped 수를 evidence로 저장한다. allow-list 내부의 조용한 skip은 0이어야 한다.

### 4.4 #183 고정 oracle

#183에는 정확히 다음 네 역사 row가 생겨야 한다.

| stage | source Event ID | 핵심 기대값 |
| --- | --- | --- |
| created | `7e40ccb3-a82d-46c8-8bce-eabd02c092b1` | legacy uppercase Task snapshot을 그대로 복사, narrative NULL |
| run_started | `1d49006d-4311-45c0-a56f-fd6a900aeaf3` | `kind=detailed_planning`, 원 clientRunId, narrative NULL |
| implemented | `74bb1376-7299-4107-9fdf-3347237cb081` | narrative와 context assessment가 원 assessment와 정확히 같음 |
| confirmed | `320bf0d1-618e-4e8f-aae4-9cc13edc15d1` | narrative NULL, `approved_by_actor_id` 보존, `not_recorded` 표시 |

`task.started`, `task.acceptance_evidence_reported`, `task.terminal_cleared`, confirmation attestation Event 자체에는 row를 만들지 않는다. confirmed row의 승인 actor/command 연결은 source `task.confirmed` Event와 `human_approval_attestations`의 실행 command를 통해 검증한다.

## 5. 구현 파일 계약

### 5.1 반드시 수정/추가할 파일

- `server/migrations/00027_task_journal_historical_backfill.sql`
  - 위 allow-list와 fail-closed 검사를 SQL 안에 구현한다.
  - `ON CONFLICT DO NOTHING`은 equivalent existing row 재실행에만 도달하도록 사전 충돌 검사를 유지한다.
  - Down은 역사 row를 삭제하지 않는 forward-only no-op임을 유지한다.
- `server/integration/testdata/task_journal_history_task_183.json`
  - 기존 6 Task Event와 attestation을 삭제/변형하지 않는다.
  - 동일 run.start command의 sanitized `run.started` Event를 추가하고 snapshot 설명을 “Task entity only”로 오해하지 않게 보완한다.
- `server/integration/migration_27_test.go`
  - schema 25 fixture restore -> 26 -> 27, #183 oracle, 전체 count, provenance, idempotent rehearsal, conflict/malformed full rollback을 검증한다.
- `server/cmd/baley-server/main.go`
  - `expectedSchemaVersion = 27`.
- migration 번호를 가정한 기존 integration tests
  - latest 26 주석/count/down-up loop를 27에 맞추되 migration 26 자체의 up/down acceptance는 계속 검증한다.
- `docker/Dockerfile.server`와 필요 시 `docker-compose.yml`
  - build version, commit, builtAt을 `-ldflags`로 주입하고 OCI revision label을 남긴다.
- `docker/Dockerfile.viewer`
  - 적어도 OCI revision label 또는 immutable image ID를 배포 evidence로 남긴다.
- `scripts/task-journal-rollout.ps1` 또는 동등한 shared-Postgres 전용 script
  - `Preflight`, `Backup`, `VerifyRestore`, `Migrate`, `Verify`, `Rollback`을 분리한다.
  - production DB 대상과 restore drill DB 대상을 명시적으로 검사하고 broad wildcard/drop을 금지한다.
- `docs/task-journal-rollout-operations.md`
  - 이 문서의 명령, RPO/rollback 결정점, 서비스 순서를 운영자가 그대로 실행할 수 있게 정리한다.

### 5.2 변경하지 않을 것

- #183 terminal Task의 content/status/evidence
- migration 25 이전 파일의 의미를 backfill 편의를 위해 재작성하는 일
- 방화벽 rule, Tailscale Serve mapping, shared PostgreSQL volume/network
- Credential Manager secret, local credential JSON의 credential material
- 별도 mandatory Decision 객체나 Journal write form

## 6. 구현 및 격리 검증 순서

### 6.1 source 통합 전 검증

```powershell
git status --short --branch
git log --oneline ca44415..HEAD
git diff --check ca44415..HEAD
git -C D:\Project_AI\baley status --short --branch
git -C D:\Project_AI\baley log -5 --oneline --decorate
```

운영 branch의 ahead commit과 `debug.log`를 먼저 기록한다. `debug.log`는 삭제/덮어쓰기하지 않는다. merge는 #183 reviewed range와 #184 implementation commit을 포함한 exact SHA를 대상으로 하며, merge 전후 `git diff --name-status ca44415..<deploy-sha>`를 evidence에 남긴다.

### 6.2 로컬 build/test

Go executable이 필요하면 반드시 `C:\dev-bin\baley\task-184\`에 build한다. `go run`은 사용하지 않는다.

```powershell
New-Item -ItemType Directory -Force C:\dev-bin\baley\task-184 | Out-Null
Push-Location server
go test ./internal/application ./cmd/baley-mcp -count=1
go test ./integration -run 'TestMigration2(6|7)|TestTaskJournal|TestRunStartAgainstPostgres' -count=1 -v
go test ./... -count=1
go vet ./...
go build -trimpath -o C:\dev-bin\baley\task-184\baley-server.exe ./cmd/baley-server
go build -trimpath -o C:\dev-bin\baley\task-184\baley-mcp.exe ./cmd/baley-mcp
Pop-Location
npm test -- --run
npm run build
git diff --check
```

`BALEY_TEST_DATABASE_URL`은 이름이 명백한 disposable local DB만 가리켜야 한다. integration safety guard가 production `baley` DB 또는 shared admin database를 거부하는지 먼저 확인한다. migration 27 test는 다음을 포함한다.

- 25 -> 26 -> 27 정상 적용과 expected version 27 readiness
- 같은 fixture를 27 down(no-op) -> 27 up으로 재실행해 동일 row 재사용
- mismatched preexisting row, duplicate source command, missing Task/executor/entity mismatch 각각에서 Goose version과 Journal row가 모두 rollback
- #183 네 row oracle, descending `(recorded_at,id)` pagination, Workspace 격리
- source Event payload와 Journal projection의 허용 필드 외 값이 생성되지 않음

## 7. 운영 실행 runbook

아래 명령은 구현·리뷰 PASS 뒤 `D:\Project_AI\baley`에서 실행한다. 현재 planning run에서는 실행하지 않는다.

### 7.1 preflight와 write freeze

```powershell
git status --short --branch
docker compose ps
docker ps --filter name=local-dev-postgres
tailscale serve status
Invoke-WebRequest http://127.0.0.1:8080/readyz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8080/versionz -UseBasicParsing
Invoke-WebRequest https://jazzcake-home.tail87e929.ts.net/api/readyz -UseBasicParsing
```

다음 snapshot을 한 묶음으로 보관한다: deploy SHA, old API/Viewer image IDs, MCP executable path/SHA-256, schema version, Workspace revision, #183/#184 상태, table별 row counts, allow-list Event counts, Tailscale Serve status. 이후 API를 중지해 lifecycle write를 quiesce한다. PostgreSQL과 Viewer를 먼저 내리지 않는다.

```powershell
docker compose stop api
```

API 8080 listener가 사라졌고 MCP mutation이 실패-닫힘인지 확인한다. backup freeze 이후에는 migration 완료 또는 rollback까지 Task command를 실행하지 않는다.

### 7.2 schema 25 사전 backup과 restore drill

backup은 repository 밖의 새 timestamp directory에 만든다. secret과 connection URL은 metadata에 쓰지 않는다.

```powershell
$backupRoot = 'D:\Project_AI\baley-backups\task-184\<UTC_TIMESTAMP>'
New-Item -ItemType Directory -Path $backupRoot | Out-Null
$remoteDump = '/tmp/baley-task184-pre.dump'
docker exec local-dev-postgres pg_dump -U local_admin -d baley --format=custom --no-owner --no-privileges --file=$remoteDump
docker cp "local-dev-postgres:$remoteDump" "$backupRoot\baley-schema25.dump"
docker exec local-dev-postgres rm -f -- $remoteDump
Get-FileHash "$backupRoot\baley-schema25.dump" -Algorithm SHA256
```

실제 script에서는 timestamp/UUID가 포함된 exact remote filename을 사용하고 finally에서 그 파일만 지운다. `backup.json`에는 SHA-256, deploy SHA, schema 25, Workspace/Task/Event/command 및 table별 counts, 생성시각을 기록한다.

restore drill DB 이름은 `baley_task184_restore_<14-digit UTC>_<8 hex>` 정규식과 정확히 일치해야 한다.

```powershell
docker exec local-dev-postgres createdb -U local_admin <verified-restore-db>
docker cp "$backupRoot\baley-schema25.dump" "local-dev-postgres:/tmp/<verified-restore-db>.dump"
docker exec local-dev-postgres pg_restore -U local_admin -d <verified-restore-db> --no-owner --no-privileges --exit-on-error --single-transaction /tmp/<verified-restore-db>.dump
```

복원 DB의 schema version, table별 counts, #183/#184 rows와 Event IDs가 backup metadata와 정확히 같아야 한다. 검증 후 정확한 이름만 `dropdb --if-exists --force`하고 remote dump 하나만 제거한다. restore 검증 실패 시 API를 old image로 다시 시작하고 migration을 시작하지 않는다.

### 7.3 immutable build와 migration

merge/review가 끝난 clean 운영 checkout에서 deploy SHA를 고정한다. old images를 image ID로 기록하고 rollback tag를 붙인 뒤 새 이미지를 build한다.

```powershell
$deploySha = (git rev-parse HEAD).Trim()
docker image tag baley-api:latest baley-api:pre-task184-<old-image-short-id>
docker image tag baley-viewer:latest baley-viewer:pre-task184-<old-image-short-id>
docker compose build --build-arg BALEY_BUILD_COMMIT=$deploySha api viewer
docker image inspect baley-api:latest baley-viewer:latest
```

API container entrypoint의 자동 migration에 의존해 migration과 service start를 한 동작으로 섞지 않는다. 새 API image로 one-shot migration을 먼저 실행한다.

```powershell
docker compose run --rm --no-deps --entrypoint /app/baley-server api migrate up
```

즉시 read-only SQL로 schema 27, table/index/trigger, candidate/inserted counts와 #183 oracle을 확인한다. 하나라도 다르면 Viewer/MCP를 교체하지 않고 rollback 판단으로 이동한다.

### 7.4 API와 Viewer 전환

```powershell
docker compose up -d --no-deps api
docker compose ps api
Invoke-WebRequest http://127.0.0.1:8080/healthz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8080/readyz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8080/versionz -UseBasicParsing
docker compose up -d --no-deps viewer
Invoke-WebRequest http://127.0.0.1:5174/ -UseBasicParsing
Invoke-WebRequest https://jazzcake-home.tail87e929.ts.net/api/readyz -UseBasicParsing
Invoke-WebRequest https://jazzcake-home.tail87e929.ts.net/api/versionz -UseBasicParsing
```

`/readyz`와 `/versionz`는 모두 schema 27과 `$deploySha`를 반환해야 한다. local 및 Tailscale endpoint가 동일한 version을 보여야 하고, API 8080/Viewer 5174는 계속 loopback bind여야 한다.

### 7.5 MCP Gateway 전환

운영 checkout이 clean이고 deploy SHA가 API와 같은지 확인한 뒤 installer를 실행한다. `debug.log`를 삭제해서 clean하게 만들지 않는다.

```powershell
.\scripts\install-baley-mcp-windows.ps1
Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort 8090 -State Listen
codex mcp get baley
```

acceptance는 다음과 같다.

- executable path가 `C:\dev-bin\baley\releases\<deploy-sha-12>\baley-mcp.exe`다.
- loopback `http://127.0.0.1:8090/mcp`만 listen하며 firewall 변경이 없다.
- credential store/Windows Credential Manager 연결이 유지되고 재로그인이 불필요하다. 재로그인이 필요하면 signed-in device-link flow만 사용하며 plaintext token을 요청하지 않는다.
- 새 session의 compact catalog에서 `baley_task_journal`, generic preview/execute bridge와 catalog version 1.2.0 이상을 확인한다.
- `baley_mcp_diagnostics`가 credential을 노출하지 않고 새 implementation/profile을 보고한다.

## 8. API / Viewer / MCP query acceptance

### 8.1 #183 역사 query

- HTTP Task route `GET /v1/workspaces/{workspaceId}/tasks/183/journal?limit=2`의 첫/다음 cursor를 합치면 정확히 네 oracle row이고 중복/누락이 없다.
- Workspace route에서 `taskId=183` filter 결과가 Task route와 byte-equivalent하다.
- 잘못된 cursor pair/limit은 repository read 전에 4xx, unauthenticated는 401, 다른 Workspace는 404 또는 empty 정책대로 실패-닫힘이다.
- MCP `baley_task_journal(taskId=183, limit=2)`의 두 page가 HTTP와 같은 Event ID, stage, narrative/context/provenance를 반환한다.
- Viewer에서 #183을 선택하면 최신순 `confirmed -> implemented -> run started -> created` 네 항목을 표시한다. `implemented` assessment는 읽을 수 있고, `confirmed`는 이유를 지어내지 않으며 historical/not-recorded 표식을 보여준다.
- Task를 빠르게 전환했을 때 이전 request가 abort되고 다른 Task의 Journal이 섞이지 않는다. 개발 trace는 event, target, React/store/controller/DOM 경계를 계속 기록하되 production console noise는 없어야 한다.
- Viewer의 50건 제한은 이번 Task의 허용된 residual risk다. 50건을 넘는 전체 역사는 HTTP/MCP keyset cursor로 모두 조회 가능해야 한다.

### 8.2 신규 lifecycle canary

배포 후 새 canary Task를 Baley command 경로로 생성한다. public ID를 미리 쓰지 않는다. topology 의도는 `parentTaskId=#184`, dependency 없는 intentional independent root이고, 독립 검증 leaf임을 `terminalReason`에 명시한다. title/description/currentSummary는 표준 네 섹션과 사용자 가치로 작성한다.

최소 lifecycle은 다음 순서다.

1. `task.create`: 배포 검증이라는 이미 정해진 `goal`과 `completionContract`만 `contextNote`에 기록.
2. `run.start(kind=implementation)`: schema 27과 세 서비스가 전환된 뒤 시작했다는 사실만 기록.
3. 실제 smoke와 query 완료 후 `task.report_implemented`: 실제 `outcome`과 관찰된 `residualRisks`만 기록.
4. signed-in Viewer의 canary Inspector에서 fresh preview와 `Confirm task` 두 단계 human approval. Agent가 grant/approver를 제조하지 않는다.

block/unblock, rework/update를 검증 목적으로 가짜 실행하지 않는다. 그 경로는 automated integration test로 검증하고, canary에서 실제 상황이 생겼을 때만 사실을 기록한다. human이 confirm 시 추가 맥락을 말하지 않았다면 confirm `contextNote`를 생략하는 것이 정답이며 새 Journal row를 만들기 위한 boilerplate를 넣지 않는다. 이 경우 confirmed Event/status는 존재하지만 Journal count는 그대로여야 한다. human이 명시적 outcome을 말했다면 exact note가 preview/hash/grant에 결속된 별도 command로 전달되어야 한다.

canary API, Viewer, MCP 결과는 Event ID, Journal ID, command ID, context, actor/time이 일치해야 한다. same idempotency retry는 같은 Journal ID를 반환하고, context를 바꾼 retry는 `idempotency_conflict`, stale revision과 mismatched approval grant는 row를 만들지 않아야 한다.

## 9. rollback 계약과 결정점

### 9.1 migration 전

backup/restore 또는 build 검증이 실패하면 migration을 하지 않고 old API/Viewer/MCP를 유지한다. 이미 API를 멈췄다면 old image로 API만 다시 시작해 schema 25 readiness를 확인한다.

### 9.2 schema 27 적용 후, 신규 write 전

old API image는 schema 25를 기대하므로 schema 27 DB 위에서 readiness를 통과하지 못한다. `migrate down` 한 번은 27을 26으로 표시할 뿐 row를 보존하고, 두 번은 migration 26이 Journal table을 삭제한다. 따라서 “old image tag로 되돌리고 끝” 또는 live DB에서 임의 `down`은 금지한다.

선택지는 둘뿐이다.

- 우선 선택: schema 27 compatible 새 API를 유지하고 roll-forward fix. Viewer와 MCP는 필요하면 이전 binary/image로 독립 복귀할 수 있다.
- 완전 rollback: human이 데이터 손실 경계를 승인한 경우에만 schema 25 사전 backup을 새 검증 DB에 다시 restore하고 counts/hash를 확인한 뒤, API write가 계속 멈춘 상태에서 운영 DB를 교체하거나 승인된 DB rename/connection cutback을 수행한다. 그 후 old API/Viewer/MCP를 시작하고 schema 25 readiness를 확인한다.

### 9.3 canary 또는 다른 schema 27 write 후

사전 backup restore는 freeze 이후의 모든 Task/Event/Journal write를 잃는다. 따라서 canary 생성 전 “schema 27 유지/roll-forward” 결정점을 명시하고, 그 뒤 완전 rollback은 human-only destructive recovery로 취급한다. rollback이 필요하면 손실 Event 목록과 reconciliation Task를 먼저 만든다. Agent가 운영 DB 교체를 자동 실행하지 않는다.

완료 후 schema 27 post-deploy backup과 별도 restore drill을 수행한다. 이것이 이후 recovery baseline이다.

## 10. 최종 체크리스트

### 코드와 데이터

- [ ] 기존 plan-01, migration 27, #183 fixture가 보존되어 reviewable commit에 포함됨
- [ ] fixture에 실제 `run.started` source Event가 추가되고 paired `task.started`는 제외 oracle로 유지됨
- [ ] migration 27 allow-list와 explicit-field projection이 표와 일치함
- [ ] allow-list 내부 silent skip 0, malformed candidate fail-closed
- [ ] migration 27 down이 역사 row를 삭제하지 않음
- [ ] expected schema version과 migration tests가 27로 갱신됨
- [ ] 전체 Go test/vet, UI test/build, diff check PASS
- [ ] 독립 리뷰 blocking 0, material 0

### 운영 안전

- [ ] 운영 branch ahead commit과 `debug.log`를 보존함
- [ ] old image IDs, MCP binary hash, schema/count/revision/Tailscale snapshot 기록
- [ ] API write freeze 후 schema 25 backup SHA-256 생성
- [ ] 별도 정확한 이름의 DB restore drill과 row/schema 비교 PASS
- [ ] migration 전 rollback 가능, migration 후 rollback 결정점과 RPO 승인 기록
- [ ] firewall/Tailscale Serve/shared volume/network 변경 없음

### 서비스와 UX

- [ ] API `/versionz` = deploy commit, `/readyz` = schema 27
- [ ] Viewer와 API image가 같은 deploy SHA evidence를 가짐
- [ ] MCP binary release path가 같은 deploy SHA이고 compact catalog에 Journal tool 존재
- [ ] #183 네 row가 HTTP/Viewer/MCP에서 동일함
- [ ] history narrative를 추론하지 않고 confirmed not-recorded 상태를 명확히 표시함
- [ ] pagination, auth, Workspace isolation, stale request UX PASS
- [ ] 신규 canary create/start/implemented Journal이 Event와 동일함
- [ ] canary human confirm은 별도 signed-in Viewer approval로 완료됨
- [ ] schema 27 post-deploy backup/restore drill PASS

### Baley 완료 경계

- [ ] canary가 정상 lifecycle을 거쳐 human confirmed
- [ ] #184 completion report, deployment SHA/image IDs, test/build, migration counts, backup/restore, API/Viewer/MCP/Tailscale smoke, 독립 리뷰가 Task Record와 Baley evidence에 연결됨
- [ ] #184가 `task.report_implemented`로 implemented가 된 뒤 signed-in Viewer에서 human이 별도 confirm

위 체크가 모두 끝나기 전에는 “Task+Journal 운영 완결” 또는 Task #184 완료를 주장하지 않는다.
