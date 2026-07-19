---
baley_record: 1
record_id: "963354a0-cc3f-415e-ba46-6780a8604076"
task_id: null
task_key: "run-record-vertical-slice"
record_type: completion-report
run_id: null
created_at: "2026-07-18T23:14:09.7947818+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Desktop Baseline 및 Run Start 완료보고

## 완료 범위

- Windows desktop 개발환경과 전용 `baley_test` PostgreSQL DB 검증
- migration `00001` 및 Run schema migration `00002` up/down/up
- 기존 Go/PostgreSQL/HTTP/MCP baseline과 Web Viewer build/browser 확인
- `run.start` domain plan → PostgreSQL transaction → generic HTTP → typed MCP 수직 단위
- pending Task의 `in_progress` 전환, Run 생성, `task.started`/`run.started` Event와 Workspace revision의 원자적 기록
- UUID client identity, parent/target provenance, phase/dependency/blocker policy와 idempotency
- raw lease token 비영속화와 HMAC 기반 안전한 재시도 복구
- active Phase outgoing Gate Focus와 ReactFlow view 전환 안정화

## 검증 결과

- PostgreSQL integration 및 HTTP contract: 통과, skip 0
- MCP stdio E2E: 15개 tool 목록, graph query와 실제 `baley_run_start` 호출 통과
- `go test ./...`, `go vet ./...`: 통과
- Linux Docker `go test -race ./...`: 통과
- `npm test`: 11개 통과
- `npm run typecheck`, `npm run build`: 통과
- Browser: Multi-lane, Client Lane Focus, Task #104 Inspector, 과거/현재 Gate Focus 확인
- 독립 Agent 리뷰: finding 반영 후 `CLOSE`

## 다음 단위

`run.heartbeat`와 terminal version CAS, lease timeout interruption을 다음 독립 수직 단위로 구현한다. 그 뒤 Task Record/Repository/Git observation persistence로 이동한다.

이번 완료는 live Baley command로 의미 상태를 갱신한 결과가 아니다. 현재 Viewer에 mutation tool이 연결되지 않아 네 Record는 `pending-bootstrap`으로 유지한다.

## 잔여 사항

- 외부 서비스와 process restart 복구에는 모든 server process에 같은 `BALEY_LEASE_TOKEN_SECRET`을 주입해야 한다.
- Vite production bundle의 500 kB 초과 warning은 기능 차단 사항은 아니며 후속 code splitting 대상이다.
- 인증, remote MCP와 secret rotation은 V1 현재 단위의 비범위다.
