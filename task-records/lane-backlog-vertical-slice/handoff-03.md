---
baley_record: 1
record_id: "f1f37380-d6d2-4bf2-9138-61bb06f88f41"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: handoff
created_at: "2026-07-26T17:20:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "e7e60a24-80a3-401c-82d1-769001a64634"
---

# Workspace 변경 시도 로그 구현 Handoff

## 완료된 내용

- 운영 Baley DB 복구 및 reconstruction 검증 완료
- destructive integration test의 운영 DB 오접속 차단 guard 추가
- PostgreSQL migration `00009_mutation_attempts.sql` 구현 및 운영 DB 적용
- 모든 command-service write 시도의 성공, 거부, 실패, 멱등 결과 기록
- raw payload, idempotency key, lease token, 승인문, SQL/error message 비저장
- `tasks` 직접 INSERT/UPDATE/DELETE 감지용 DB trigger
- application write와 DB trigger의 중복 방지
- append-only UPDATE/DELETE/TRUNCATE 차단
- workspace, outcome, command, 시간 cursor 기반 HTTP/CLI/MCP 조회
- 격리 PostgreSQL 통합 테스트와 전체 Go/React 회귀 검증

## 운영 검증

- 운영 migration version: 9
- 운영 workspace revision: 203 (migration과 no-op 검증으로 변경되지 않음)
- no-op 직접 Task UPDATE로 생성된 audit:
  - workspace: `00000000-0000-4000-8000-000000000001`
  - command: `sql.tasks.update`
  - source: `database_trigger`
  - outcome: `succeeded`
  - task: `47c2d962-9008-49cb-9e41-2b063d0213e4` (#121)

## 검증 결과

- `go test ./...`: PASS
- `go vet ./...`: PASS
- `npm test -- --run`: 10 files, 38 tests PASS
- `npm run build`: PASS (기존 bundle size warning만 존재)
- `git diff --check`: PASS

## 남은 활성화 작업

현재 포트 8080의 `baley-server`는 변경 전 바이너리이며, 이 세션 권한으로 해당
프로세스를 종료하려 하면 Access Denied가 발생한다. DB trigger 기록은 이미
활성화됐지만 command-service의 성공/거부/실패/멱등 기록과 새 HTTP endpoint는
서버 재기동 후 활성화된다.

관리 권한을 가진 원래 실행 주체에서 같은 안정적인
`BALEY_LEASE_TOKEN_SECRET`을 유지한 채 `baley-server serve`를 재기동한다.
현재 running Run은 0개다. 재기동 후 아래를 확인한다.

1. `GET /healthz`가 200을 반환한다.
2. `GET /v1/workspaces/{workspaceId}/mutation-attempts?limit=1`이 200을 반환한다.
3. harmless command preview가 아닌 실제 write 한 건을 실행한다.
4. 해당 workspace 조회에 `source=command_service` row가 나타난다.
