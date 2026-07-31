---
baley_record: 1
record_id: "0babc9a8-c8e3-4690-9916-6e57c46289fe"
task_id: 121
task_key: "lane-backlog-vertical-slice"
record_type: completion-report
created_at: "2026-07-26T18:10:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "785a29d8-133f-465c-9af0-879ea0474d30"
---

# Workspace 변경 시도 감사 활성화 완료 보고서

## 결론

Task #121의 기존 Lane Backlog vertical slice와 후속 Workspace mutation-attempt
감사 구현을 완료하고 운영 PostgreSQL에 적용했다. handoff-03에 남아 있던 새
`baley-server` 재기동, HTTP endpoint 활성화, 실제 command-service write 감사
검증도 완료했다.

Baley의 Task lifecycle 상태는 `in_progress`다. Task가 아직 비활성 미래
`embedding-enablement` Phase에 있어 implementation/completion-reporting Run과
`task.report_implemented` 전이를 서버가 `phase_inactive`로 거부한다. 이 도메인
제약은 우회하지 않았다.

## 전달 결과

- PostgreSQL migration `00009_mutation_attempts.sql`
- 모든 command-service write 시도의 성공·거부·실패·멱등 결과 기록
- `tasks` 직접 INSERT/UPDATE/DELETE 감지용 DB trigger
- application write와 trigger 기록의 중복 방지
- append-only UPDATE/DELETE/TRUNCATE 차단
- raw payload, raw idempotency key, lease token, 승인문, SQL/error message 비저장
- workspace, outcome, command, 시간 cursor 기반 HTTP/CLI/MCP 조회
- destructive integration test의 운영 DB 오접속 방지
- 포트 8080의 새 서버 바이너리 활성화 및 실제 운영 경로 검증

## 운영 검증

- 운영 migration version: 9
- 활성화 전 Workspace revision: 205
- 검증 후 Workspace revision: 207
- `GET /healthz`: HTTP 200
- `GET /v1/workspaces/00000000-0000-4000-8000-000000000001/mutation-attempts?limit=2`:
  HTTP 200
- 검증 Run: `6a866653-f632-4c43-9e4a-b51f83317ea9`
- `run.start` Event: `1c68947d-e4f0-49a9-94b7-afd0b9ec4e99`
- `run.succeed` Event: `2b4e41eb-92b5-4f63-af5b-c5e014c29c6b`
- 두 실제 write 모두 `source=command_service`, `outcome=succeeded`로 조회됨
- DB trigger의 `sql.tasks.update`, `source=database_trigger` 기록도 조회됨

## 회귀 검증

- `go test ./...`: PASS
- `go vet ./...`: PASS
- 격리 PostgreSQL mutation-attempt 통합 테스트: PASS
- `npm test -- --run`: 10 files, 38 tests PASS
- `npm run build`: PASS
- `git diff --check`: PASS
- 기존 production bundle size 경고만 존재

## Task 상태와 잔여 위험

- 기능 및 handoff-03 운영 활성화 결과는 완료됐다.
- #121은 미래 비활성 Phase 제약으로 Baley 상태를 `implemented`로 전이하지 않았다.
- 포트 8080 서버는 Windows 서비스가 아닌 수동 프로세스로 실행됐으므로 재부팅 후
  자동 시작되지 않는다.
- 완료보고서 작성 시점의 read-only 재확인에서는 포트 8080과 5173에 listener가
  남아 있었다. 사용자가 종료한 프로세스와 별도로 백그라운드 프로세스가 유지된
  것인지 확인이 필요하다.
- commit/push는 수행하지 않았다.
