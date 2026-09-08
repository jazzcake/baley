---
baley_record: 1
record_id: "764bd762-d858-4b4a-a07f-ea790d4d551c"
task_id: 183
record_type: independent-review
run_id: "afecce99-b636-45d6-b8f8-e30c58058f77"
created_at: "2026-09-08T17:26:48+09:00"
created_by: "codex-independent-reviewer"
supersedes: null
---

# Task #183 독립 리뷰

## 판정

`CHANGES_REQUIRED`. 미해결 blocking 1건, material 2건이다. 수정 후 동일 범위를 독립 재리뷰해야 한다.

## 검토 범위

`AGENTS.md`, Task #182의 계획·완료보고·운영 DB 감사, Task #183의 계획·완료보고, 기준 커밋 `ca44415cd2776981b558755c60a11886a25296ec`, 구현 커밋 `6da2c5c1560cb82de5e12480ce1962f393bd1c51`, 보고 보강 커밋 `2bfd1d2f8df1307e1affc4d32d64d7597dd8acdb`를 독립 검토했다. 별도 D# workflow나 입력 prompt가 생기지 않았고, Task #182 산출물 보존과 clean worktree도 확인했다.

## Findings

### Blocking — lifecycle Event가 맥락의 재구축 가능한 정본이 아님

- 근거: `server/internal/persistence/postgres/repository.go:1404`, `server/internal/persistence/postgres/repository.go:1419`, `server/internal/application/command_service.go:1802`, `server/internal/application/command_service.go:1910`.
- 재현: disposable PostgreSQL에서 Journal의 `problem`, `goal`, `alternatives`, `completionContract`가 대응 `task.created`, `task.updated`, `task.blocked`, `run.started` Event payload에는 없음을 read-only SQL로 확인했다. 구현도 typed context를 Event가 아니라 Journal에 별도 삽입한다.
- 영향: Journal 유실·rebuild·projection drift 감사 시 기존 Event만으로 개발 맥락을 복원할 수 없다. “Event가 source of truth이고 Journal은 projection”이라는 핵심 계약을 위반한다.
- 필수 수정: 정규화한 `schemaVersion`, `narrative`, `context`를 lifecycle Event payload에 먼저 영속화하고, Journal을 그 persisted Event에서 결정적으로 투영한다. Event 기반 rebuild 동등성 테스트를 추가한다.

### Material — context가 있는 `run.start`의 cross-key recovery 실패

- 근거: `server/internal/application/command_service.go:1453`, `server/internal/application/command_service.go:1460`, `server/internal/application/command_service.go:1852`, `server/internal/application/command_service.go:1907`, `server/internal/persistence/postgres/repository.go:885`.
- 재현: 최초 context 포함 Run 생성 뒤 새 idempotency key와 같은 `clientRunId`로 HTTP execute하면 Journal 수는 변하지 않고 `invalid_request: contextNote has no Task lifecycle target`가 반환됐다. 기존 Run recovery 분기에서 `plan.Run`과 Task target이 비어 있는 상태로 Journal mapping이 먼저 실패한다.
- 영향: 정상 cross-key recovery가 context 사용 시 불가능하며 변경된 context에 대한 명시적 conflict도 보장되지 않는다.
- 필수 수정: 기존 Run에서도 Task target을 확정한다. 원래 정규화 context 또는 digest를 영속 비교해 같으면 기존 command·Journal ID를 재사용하고 다르면 `idempotency_conflict`를 반환하는 통합 테스트를 추가한다.

### Material — full-profile typed MCP lifecycle adapter가 `contextNote`를 누락

- 근거: `server/cmd/baley-mcp/main.go:64`, `server/cmd/baley-mcp/main.go:428`, `server/cmd/baley-mcp/main.go:1189`, `server/cmd/baley-mcp/main.go:1238`, `server/cmd/baley-mcp/main.go:1273`, `server/cmd/baley-mcp/main.go:1502`.
- 재현: `taskCreateFields`, `runStartInput`, `taskReportImplementedInput`, confirm/discard preview·execute argument map에 `contextNote`가 없다. 이번 변경은 generic command bridge에서만 전달한다.
- 영향: 계속 지원되는 full-profile typed tool 사용자의 명시적 맥락이 조용히 유실된다. confirm/discard typed 경로에서는 그 맥락을 preview hash와 browser grant에 결속할 수도 없다.
- 필수 수정: 관련 typed lifecycle input schema와 preview/execute argument map에 동일 optional `contextNote`를 추가한다. 부재 시 기존 hash 호환, 전달, 승인 후 context 변경 mismatch를 테스트한다.

## 독립 검증

- disposable PostgreSQL 17.5에서 migration 26 up/down/up과 Task Journal 집중 5 tests를 통과했다.
- `go test ./... -count=1`, `go vet ./...`, UI 17 files/108 tests, production build, `git diff --check`가 통과했다.
- read-only SQL로 Event와 Journal을 대조했고, loopback HTTP로 `run.start` recovery 결함을 재현했다.
- 검토용 PostgreSQL container는 제거했다. 운영 DB·서비스·방화벽·Tailscale·remote branch·설치 plugin cache와 수동 브라우저 UI는 변경하지 않았다.

## 재리뷰 계약

세 finding을 모두 수정하고 관련 회귀 테스트를 추가한 뒤, 구현 diff와 전체 검증 결과를 동일 독립 리뷰어가 다시 확인한다. unresolved blocking/material finding이 0이 되기 전에는 Task #183을 implemented로 전환하지 않는다.
